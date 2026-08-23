package crawler

import (
	"errors"
	"fmt"
	"io"
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
	var seen map[string]struct{}
	var links []string
	limited := io.LimitReader(body, maxHTMLParseSize)
	tokenizer := html.NewTokenizer(limited) // cap reading size
	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			err := tokenizer.Err()

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
	c.limiterMu.Lock()
	defer c.limiterMu.Unlock()
	lim, ok := c.limitedHosts[host]
	if !ok {
		lim = rate.NewLimiter(rate.Limit(10), 10) // refill 10/per second, with bursts of 10 per concurrent gorountine
		c.limitedHosts[host] = lim
	}
	return lim
}

type RetryTransport struct {
	Base  http.RoundTripper
	delay time.Duration

	allowedRetries map[int]int
	defaultRetries int
}

func NewRetryClient(delay time.Duration, maxR int) *http.Client {
	return &http.Client{
		Transport: &RetryTransport{
			Base:  http.DefaultTransport,
			delay: delay,
			allowedRetries: map[int]int{
				http.StatusRequestTimeout:     2,
				http.StatusMisdirectedRequest: 2,
				http.StatusTooEarly:           2,
			},
			defaultRetries: maxR,
		},
	}
}

func (r *RetryTransport) RetryOnTransientErr(status int) int {
	if v, ok := r.allowedRetries[status]; ok {
		return v
	}

	if status >= 500 {
		return r.defaultRetries
	}

	return 0
}

func (r *RetryTransport) RoundTrip(z *http.Request) (*http.Response, error) {
	t := r.Base
	var lastErr error

	if t == nil {
		t = http.DefaultTransport
	}

	for v := range r.defaultRetries {
		resp, err := t.RoundTrip(z)

		// clean resp body (avoiding connection leaks)
		if err == nil {
			retries := r.RetryOnTransientErr(resp.StatusCode)

			if retries == 0 {
				return resp, nil
			}

			if v >= retries {
				return resp, nil
			}

			io.Copy(io.Discard, resp.Body) // consume response body so that the underlying tcp connection can be reused by http transport.
			resp.Body.Close()
		}

		lastErr = err

		delay := time.Duration(1<<v) * r.delay
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-z.Context().Done():
			return nil, z.Context().Err()
		}
	}
	return nil, lastErr
}
