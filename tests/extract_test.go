package tests

import (
	"net/url"
	"strings"
	"testing"

	"github.com/72sevenzy2/web-crawler"
)

func TestValidExtractBody(t *testing.T) {
	// sample html
	input := strings.NewReader(`<html> <body><a href="https://jsonplaceholder.typicode.com/guide/">test</a></body> </html>`)
	
	b, _ := url.Parse("https://jsonplaceholder.typicode.com/guide/")
	links, err := crawler.Extract(input, b)
	if err != nil {
		t.Fatalf("encountered: %s", err.Error())
		return
	}

	for _, link := range links {
		t.Log(link)
	}
}
