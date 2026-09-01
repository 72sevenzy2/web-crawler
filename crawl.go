package crawler


import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Crawler struct {
	Client *http.Client
	wg     sync.WaitGroup

	LimiterLock  sync.RWMutex
	LimitedHosts map[string]*rate.Limiter

	OriginsLock sync.Mutex
	Origins     map[string]bool

	Depth int

	// optionals
	AllowCrossDomains bool
	MaxRetries        int

	// Sem is to act as a semaphore to cap number of network requests at flight by limiting number of gorountines, (via buffered channel).
	Sem chan struct{}
}

func NewCrawler(depth int, allowCD bool, maxR int) *Crawler {
	return &Crawler{
		Depth:             depth,
		MaxRetries:        maxR,
		AllowCrossDomains: allowCD, // determines whether crawler can visit external links (other than child host urls)
		Origins:           make(map[string]bool),
		Sem:               make(chan struct{}, 10), // 10 requests can be made for each crawler that spawns
		LimitedHosts:      make(map[string]*rate.Limiter),
		Client:            NewRetryClient(time.Millisecond*200, 5),
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

	// checks for duplicated urls before IsValidLink() computation.
	c.OriginsLock.Lock()
	if c.Origins[url] {
		c.OriginsLock.Unlock()
		return nil
	}
	c.Origins[url] = true
	c.OriginsLock.Unlock()

	// validate url initially
	base, err := IsValidLink(url)
	if err != nil {
		return err
	}

	if depth > c.Depth {
		return nil
	}

	fmt.Println("crawling:", url)

	// claim token before acquiring a slot via semaphore.
	limiter := c.LimitHost(url)
	if err := limiter.Wait(ctx); err != nil {
		slog.Error("encountered", "rate limiting err", err)
		return nil // expected error so not flagged here.
	}

	select {
	case c.Sem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}

	defer func() {
		<-c.Sem
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		slog.Error("request initialization error", "err", err)
		return err
	}

	resp, err := c.Client.Do(req)

	if err != nil {
		slog.Error("request error", "err", err)
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	// avoids parsing pages with non-html contents for Extract()
	cType := resp.Header.Get("Content-Type")
	if !strings.Contains(cType, "text/html") {
		return nil // expected
	}

	links, err := Extract(resp.Body, base)
	if err != nil {
		return err
	}

	// DrainClose closes resp.Body while freeing its underlying TCP connection for reuse by HTTP transport.
	// Draining before recusion to avoid socket/file descriptor leaks at high concurrent requests during crawler lifetime.
	DrainClose(resp)

	// recursive call untill max depth is exceeded.
	for _, link := range links {
		l := link // explicitly defining each loop variable.

		// SameHost reuses the previously computed base to avoid computing host names for 2 urls at every cycle.
		if !SameHost(base, l) && !c.AllowCrossDomains {
			continue // skip if not same domain origin
		}
		slog.Info("scoured link:", "link", l)
		c.wg.Add(1)
		go func(l string) {
			defer c.wg.Done()
			c.crawl(ctx, l, depth+1)
		}(l)
	}

	return nil
}
