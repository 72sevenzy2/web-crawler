package crawler

import (
	"errors"
	"io"
	"math/rand"
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
func Extract(body io.Reader, baseURL *url.URL) ([]string, error) {
	// map in which to hold seen urls to avoid duplicated results in links
	seen := make(map[string]struct{})
	var links []string
	limited := io.LimitReader(body, maxHTMLParseSize)
	tokenizer := html.NewTokenizer(limited) // cap reading size
	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			if errors.Is(tokenizer.Err(), io.EOF) { // end of stream
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

						resolved, err := baseURL.Parse(attr.Val)
						if err != nil {
							continue
						}

						// stripping URL fragments to avoid crawling a page more than once.
						// eg sections#post will be stripped to sections.
						resolved.Fragment = ""

						if resolved.Scheme == "https" || resolved.Scheme == "http" {
							link := resolved.String()
							if _, ok := seen[link]; !ok {
								seen[link] = struct{}{}
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
	a, err1 := url.ParseRequestURI(u1)
	b, err2 := url.ParseRequestURI(u2)
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

// computing backoff delay (for retry.go) upon request failure with jittered duration, this avoids the "thundering herd" design flaw.
func (r *RetryTransport) CalculateBackoffDelay(attempt int) time.Duration {
	backoff := float64(attempt) * float64(int(1)<<attempt) // 2 ^ attempt after big left shift.
	// verify backoff doesnt exceed r.MaxDelay
	if r.Delay > 0 && time.Duration(backoff) > r.Delay {
		backoff = float64(r.Delay) // fallback of r.Delay if exceeded.
	}

	jitteredV := rand.Float64() * backoff
	return time.Duration(jitteredV)
}

func IsValidLink(l string) (*url.URL, error) {
	base, err := url.ParseRequestURI(l)
	if err != nil {
		return nil, err // invalid url
	}
	return base, nil
}
