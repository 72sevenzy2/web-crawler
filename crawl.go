package crawler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
)

type Crawler struct {
	depth int
	wg    sync.WaitGroup

	originsLocker sync.Mutex
	origins       map[string]bool

	allowCrossDomains bool
	sem               chan struct{}
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
			log.Println(err.Error())
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
		return fmt.Errorf("invalid content-type with request, found: %s", cType)
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

		fmt.Println(link) // printing discovered links
		c.wg.Add(1)
		go func(l string) {
			defer c.wg.Done()
			c.crawl(ctx, l, depth+1)
		}(link)
	}

	return nil
}
