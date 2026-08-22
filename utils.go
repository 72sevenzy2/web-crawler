package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

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

// wrapping request with retries.
func RequestWithRetry(ctx *context.Context, url string, maxRetries int) (*http.Response, error) {
	return nil, nil
}
