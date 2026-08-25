package tests

import (
	"testing"
	"strings"
	"net/url"

	"github.com/72sevenzy2/web-crawler"
)

func TestInvalidExtractBody(t *testing.T) {
	input := strings.NewReader(`<html> <body><p>test</p></body> </html>`)

	b, _ := url.Parse("https://jsonplaceholder.typicode.com/guide/")
	links, err := crawler.Extract(input, b)
	if err != nil {
		t.Fatalf("encountered: %s", err.Error())
		t.Log(err)
		return
	}

	if len(links) == 0 {
		t.Log("passed")
		return
	}

}
