package crawler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/time/rate"
)

// for OOM attack prevention, (out of memory).
// usually to prevent parsing massive decompression bombs/endless streams.
const maxHTMLParseSize = 10 * 1024 * 1024 // cap to 10 MB

// for parsing html anchor tags
func Extract(body io.Reader, BaseUrl string) ([]string, error) {
	base, err := url.Parse(BaseUrl)
	if err != nil {
		return nil, fmt.Errorf("error parsing %s: %w", BaseUrl, err)
	}

	// map in which to hold seen urls to avoid duplicated results in links
	seen := make(map[string]struct{})
	var links []string
	limited := io.LimitReader(body, maxHTMLParseSize)
	tokenizer := html.NewTokenizer(limited) // cap reading size
	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:

			if errors.Is(err, io.EOF) { // end of stream
				return links, nil
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			// DataAtom being the fast path to determine whether anchor tag exists, fallback to string comparison with token.Data
			if token.DataAtom.String() == "a" || token.Data == "a" {
				for _, attr := range token.Attr {
					if attr.Key == "href" {
						val := strings.TrimSpace(attr.Val)
						// skip empty links and none-HTTP urls early.
						if val == "" || strings.HasPrefix(val, "javascript:") || strings.HasPrefix(val, "mailto:") {
							continue
						}

						resolved, err := base.Parse(BaseUrl)
						if err != nil {
							continue
						}

						// stripping URL fragments to avoid crawling a page more than once.
						// eg sections#post will be stripped to sections.
						resolved.Fragment = ""

						if resolved.Scheme == "https" || resolved.Scheme == "http" {
							link := resolved.String()
							if _, ok := seen[link]; !ok {
								seen[link] = struct{}{} // mark as seen
								links = append(links, link)
							}
						}
					}
				}
			}
		}
	}
}

// strings.EqualFold() allows for RFC compliancy f
// comparing hosts per link to avoid cross-domain recurse
func SameHost(u1, u2 string) bool {
	a, err1 := url.Parse(u1)
	b, err2 := url.Parse(u2)
	if err1 != nil || err2 != nil {
		return false
	}

	// a/b.Hostname() strips ports (if present), strings.EqualFold() for case insensivity for domain matching.
	return strings.EqualFold(a.Hostname(), b.Hostname())
}

// limiters for each hosts
func (c *Crawler) LimitHost(host string) *rate.Limiter {
	// normalise before lookups
	t := strings.ToLower(strings.TrimSpace(host))

	// fast path
	c.LimiterLock.RLock() // RLock() allows for high concurrent reads without blocking
	lim, ok := c.LimitedHosts[t]
	c.LimiterLock.RUnlock()
	if ok {
		return lim
	}

	// store in lookup map and return limiter
	c.LimiterLock.Lock() // exclusive lock for write operations
	defer c.LimiterLock.Unlock()

	// double check if gorountines race here after c.limiterMu.Unlock()
	if lim, ok := c.LimitedHosts[t]; ok {
		return lim
	}

	c.LimitedHosts[t] = rate.NewLimiter(10, 10) // refill 10 per/sec, with bursts of 10.
	return c.LimitedHosts[t]                    // return limiter to specificed host
}

type RetryTransport struct {
	Base         http.RoundTripper
	Delay        time.Duration
	InitialDelay time.Duration
	MaxRetries   int

	// will hold how many retries specific request status codes can do.
	AllowedRetries map[int]int
}

func NewRetryClient(initDelay time.Duration, maxRetries int) *http.Client {
	return &http.Client{
		Timeout: time.Second * 30, // safeguards against hung TCP handshakes/unresponesive backends.
		Transport: &RetryTransport{
			Base:         http.DefaultTransport,
			InitialDelay: initDelay,
			Delay:        time.Second * 3,
			AllowedRetries: map[int]int{
				http.StatusRequestTimeout:     2, // 408
				http.StatusTooEarly:           2, // 425
				http.StatusMisdirectedRequest: 2, // 421
				http.StatusTooManyRequests:    3, // 429
				http.StatusBadGateway:         3, // 502
				http.StatusServiceUnavailable: 3, // 503
				http.StatusGatewayTimeout:     3, // 504
			},
		},
	}
}

// Determines whether status is warrant to retries.
func (r *RetryTransport) isRetryableStatus(status, attempt int) bool {
	if t, ok := r.AllowedRetries[status]; ok { // is warrant to retry
		return attempt < t
	}

	// general server-side errors
	if status > 500 {
		return attempt < r.MaxRetries // default to r.MaxRetries if so.
	}

	return false
}

// Determines whether error is transient to network glitch
func (r *RetryTransport) isRetryableNetworkErr(err error) bool {
	if err == nil {
		return false
	}

	// check if err was caused by caller
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var nErr net.Error
	if errors.As(err, &nErr) { // caused by temporary OS network glitches and such.
		return true
	}

	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true // caused by server abruptly closing client keep-alive socket.
	}

	return false
}

func (r *RetryTransport) RoundTrip(z *http.Request) (*http.Response, error) {
	t := r.Base
	if t == nil {
		t = http.DefaultTransport
	}

	var (
		maxAttempts = r.MaxRetries + 1 // MaxRetries must exclude initial request
		lastResp    *http.Response
		lastErr     error
	)

	for v := range r.MaxRetries {
		// verify context deadline/cancellation before proceeding to network call
		if err := z.Context().Err(); err != nil {
			return nil, z.Context().Err()
		}

		resp, err := t.RoundTrip(z)
		lastErr = err

		if err == nil {
			// if succesfully, return immediately after making sure it isnt an retryable status.
			if !r.isRetryableStatus(resp.StatusCode, v) {
				return resp, nil
			}
			lastResp = resp
		} else {
			// check if is isnt an transient network error and if there are attempts remaining.
			if !r.isRetryableNetworkErr(err) || v <= maxAttempts-1 { // -1 as we added 1 initially for first request.
				return nil, err
			}
			lastErr = err
		}

		lastResp = nil
		DrainClose(resp) // drain response body.

		// check if we have attempts remaining.
		if v == maxAttempts-1 {
			break // exit loop and return results
		}

		delay := time.Duration(1<<v) * r.Delay
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-z.Context().Done():
			return nil, z.Context().Err()
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return lastResp, nil
}

// MaxDrainSize is to limit number of bytes when draining resp.Body.
// Also prevents draining massive payloads from HTML, which can hang for long periods.
const MaxDrainSize = 64 * 1024 // 64 KiB

// for consuming underlying TCP connection to remote node, so that http transport can reuse connection.
func DrainClose(resp *http.Response) {
	// resp == nil as fast path
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, MaxDrainSize))
	_ = resp.Body.Close()
}
