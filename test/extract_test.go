package tests

import (
	"strings"
	"testing"

	"github.com/72sevenzy2/web-crawler"
)

func TestValidExtractBody(t *testing.T) {
	// sample html
	input := strings.NewReader(`<html> <body><a href="https://jsonplaceholder.typicode.com/guide/">test</a></body> </html>`)
	
	links, err := crawler.Extract(input, "https://jsonplaceholder.typicode.com/guide/")
	if err != nil {
		t.Fatalf("encountered: %s", err.Error())
	}

	for _, link := range links {
		t.Log(link)
	}
}
