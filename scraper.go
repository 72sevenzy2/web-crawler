package scraper

import (
	"fmt"
	"sync"
)

type Crawler struct {
	depth int

	mu      sync.Mutex
	origins map[string]bool
}

func NewCrawler() *Crawler {
	return &Crawler{
		origins: make(map[string]bool),
	}
}

func (c *Crawler) Crawl(url string, depth int) {
	if depth > c.depth {
		fmt.Println("max depth exceeded.")
		return
	}
}
