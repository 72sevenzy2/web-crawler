package main

import (
	"github.com/72sevenzy2/web-crawler"
)

// testing

func main() {
	c := crawler.NewCrawler()
	c.Crawl("https://jsonplaceholder.typicode.com/guide/", 5)
}
