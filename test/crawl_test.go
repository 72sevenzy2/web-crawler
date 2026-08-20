package main

import (
	"testing"

	"github.com/72sevenzy2/web-crawler"
)

// testing

func TestCrawler(t *testing.T) {
	c := crawler.NewCrawler(10)

	c.Start("https://jsonplaceholder.typicode.com/guide/", 0)
}
