package crawler

import (
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
}

func NewCrawler(depth int) *Crawler {
	return &Crawler{
		depth: depth,
		origins: make(map[string]bool),
	}
}

func (c *Crawler) Crawl(url string, depth int) {
	if depth > c.depth {
		fmt.Println("max depth exceeded.")
		return
	}

	c.originsLocker.Lock()
	if c.origins[url] {
		c.originsLocker.Unlock()
		return
	}
	c.origins[url] = true
	c.originsLocker.Unlock()

	fmt.Println("crawling:", url)

	resp, err := http.Get(url)
	if err != nil {
		log.Println("request error when crawled:", url)
		return
	}

	if resp.StatusCode != http.StatusOK {
		return
	}

	cType := resp.Header.Get("Content-Type")
	if !strings.Contains(cType, "text/html") {
		return
	}

	defer resp.Body.Close()

	links, err := Extract(resp.Body, url)
	if err != nil {
		return
	}

	for _, link := range links {
		c.wg.Add(1)
		go func(l string) {
			defer c.wg.Done()
			c.Crawl(l, depth+1)
		}(link)
	}
}
