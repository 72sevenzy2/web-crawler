package tests

import (
	"context"
	"testing"
	"time"

	"github.com/72sevenzy2/web-crawler"
)

// testing

func TestCrawler(t *testing.T) {
	c := crawler.NewCrawler(10, false, 10)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute * 10)
	defer cancel()
	c.Start(ctx, "https://jsonplaceholder.typicode.com/guide/", 0)
}
