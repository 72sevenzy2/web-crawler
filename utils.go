package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/time/rate"
)

// for parsing html anchor tags
func Extract(body io.Reader, BaseUrl string) ([]string, error) {
	base, err := url.Parse(BaseUrl)
	if err != nil {
		return nil, fmt.Errorf("error parsing url: %s", BaseUrl)
	}

	var links []string
	tokenizer := html.NewTokenizer(body)
	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			return links, nil
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			if token.Data == "a" {
				for _, attr := range token.Attr {
					if attr.Key == "href" {
						resolved, err := base.Parse(attr.Val)
						if err == nil && (resolved.Scheme == "https" || resolved.Scheme == "http") {
							links = append(links, resolved.String())
						}
					}
				}
			}
		}
	}
}

// comparing hosts per link to avoid cross-domain recurse
func SameHost(u1, u2 string) bool {
	url, err1 := url.Parse(u1)
	url2, err2 := url.Parse(u2)
	if err1 != nil || err2 != nil {
		return false
	}

	return url.Host == url2.Host
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

// retry logic configurations
type RetryTransport struct {
	Base       http.RoundTripper
	delay      time.Duration
	ctx        context.Context
	maxRetries int
}

func NewRetryClient(delay time.Duration, maxR int) *http.Client {
	return &http.Client{
		Transport: &RetryTransport{
			Base: http.DefaultTransport,
			delay: delay,
			maxRetries: maxR,
		},
	}
}

func (r *RetryTransport) RoundTrip(z *http.Request) (*http.Response, error) {
	t := r.Base
	var lastErr error

	if t == nil {
		t = http.DefaultTransport
	}

	for v := range r.maxRetries {
		resp, err := t.RoundTrip(z)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		// clean resp body (avoiding connection leaks)
		if err == nil {
			defer resp.Body.Close()
			io.Copy(io.Discard, resp.Body) // consume response body so that the underlying tcp connection can be reused by http transport.
		}

		lastErr = err

		delay := time.Duration(1<<v) * r.delay
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-z.Context().Done():
			return resp, z.Context().Err()
		}
	}
	return nil, lastErr
}
