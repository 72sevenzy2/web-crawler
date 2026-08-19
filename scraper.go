package scraper

import (
	"fmt"
	"log"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/net/html"
)

type Crawler struct {
	depth int
	wg    sync.WaitGroup

	originsLocker sync.Mutex
	origins       map[string]bool
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
		log.Fatal("request error when crawled:", url)
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

	links, err := extract(resp.Body, url)
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

func extract(body io.Reader, BaseUrl string) ([]string, error) {
	base, err := url.Parse(BaseUrl)
	if err != nil {
		return nil, err
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
						resolved, err := base.Parse(attr.Key)
						if err == nil && (resolved.Scheme == "https" || resolved.Scheme == "http") {
							links = append(links, resolved.String())
						}
					}
				}
			}
		}
	}
}
