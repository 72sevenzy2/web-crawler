package crawler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Crawler struct {
	depth int
	wg    sync.WaitGroup

	originsLocker sync.Mutex
	origins       map[string]bool

	// optionals
	allowCrossDomains bool
	maxRetries        int

	sem chan struct{}

	limiterMu    sync.RWMutex // allows for concurrent reads non-blocking.
	limitedHosts map[string]*rate.Limiter

	client *http.Client
}

func NewCrawler(depth int, allowCD bool, maxR int) *Crawler {
	return &Crawler{
		depth:             depth,
		maxRetries:        maxR,
		allowCrossDomains: allowCD, // determines whether crawler can visit external links (other than child host urls)
		origins:           make(map[string]bool),
		sem:               make(chan struct{}, 10), // 10 requests can be made for each crawler that spawns
		limitedHosts:      make(map[string]*rate.Limiter),
		client:            NewRetryClient(time.Millisecond*200, 5),
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
	limiter := c.LimitHost(url)
	if err := limiter.Wait(ctx); err != nil {
		slog.Error("encountered", "rate limiting err", err)
		return nil // expected error so not flagged here.
	}

	select {
	case c.sem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}

	defer func() {
		<-c.sem
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		slog.Error("request initialization error", "err", err)
		return err
	}

	resp, err := c.client.Do(req)

	if err != nil {
		slog.Error("request error", "err", err)
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	cType := resp.Header.Get("Content-Type")

	if !strings.Contains(cType, "text/html") {
		slog.Error("invalid header", "found", errors.New(cType))
		return nil // expected
	}

	links, err := Extract(resp.Body, url)
	if err != nil {
		return err
	}

	io.Copy(io.Discard, resp.Body)

	// recursive call untill max depth is exceeded.
	for _, link := range links {
		if !SameHost(url, link) && !c.allowCrossDomains {
			continue // skip if not same domain origin
		}
		slog.Info("scoured link:", "link", link)
		c.wg.Add(1)
		go func(l string) {
			defer c.wg.Done()
			c.crawl(ctx, l, depth+1)
		}(link)
	}

	return nil
}
