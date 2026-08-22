package crawler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/time/rate"
)

type Crawler struct {
	depth int
	wg    sync.WaitGroup

	originsLocker sync.Mutex
	origins       map[string]bool

	allowCrossDomains bool
	sem               chan struct{}

	limiterMu    sync.Mutex
	limitedHosts map[string]*rate.Limiter
}

func NewCrawler(depth int, allowCD bool) *Crawler {
	return &Crawler{
		depth:             depth,
		allowCrossDomains: allowCD, // determines whether crawler can visit external links (other than child host urls)
		origins:           make(map[string]bool),
		sem:               make(chan struct{}, 10), // 10 requests can be made for each crawler that spawns
	}
}

func (c *Crawler) Start(ctx context.Context, url string, startDepth int) {
	c.wg.Go(func() { // auto increments wg counter and decrements after completion.
		err := c.crawl(ctx, url, startDepth)
		if err != nil {
			slog.Error("encountered crawl error", "err", err)
		}
	})
	c.wg.Wait()
}

func (c *Crawler) crawl(ctx context.Context, url string, depth int) error {
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	if depth > c.depth {
		return nil
	}

	c.originsLocker.Lock()
	if c.origins[url] {
		c.originsLocker.Unlock()
		return nil
	}
	c.origins[url] = true
	c.originsLocker.Unlock()

	fmt.Println("crawling:", url)

	// claim token before acquiring a slot via semaphore.
	limiter := c.LimitHost(ctx, url)
	if err := limiter.Wait(ctx); err != nil {
		slog.Error("encountered", "rate limiting err", err)
		return nil // expected error so not flagged here.
	}

	c.sem <- struct{}{} // capping number of requests made with semaphore
	resp, err := http.Get(url)
	<-c.sem

	if err != nil {
		return nil
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	cType := resp.Header.Get("Content-Type")

	if !strings.Contains(cType, "text/html") {
		slog.Error("invalid header", "err", errors.New(cType))
		return nil // expected
	}

	links, err := Extract(resp.Body, url)
	if err != nil {
		return err
	}

	// recursive call untill max depth is exceeded.
	for _, link := range links {
		if !SameHost(url, link) && !c.allowCrossDomains {
			continue // skip if not same domain origin
		}
		slog.Info("scoured link:", "link", link)
		c.wg.Add(1)
		c.wg.Go(func() {
			c.crawl(ctx, link, depth+1)
		})
	}

	return nil
}
