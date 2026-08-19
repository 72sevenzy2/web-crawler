package crawler

import (
	"golang.org/x/net/html"
	"io"
	"net/url"
)

// for parsing html anchor tags
func Extract(body io.Reader, BaseUrl string) ([]string, error) {
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
						resolved, err := base.Parse(attr.Val)
						if err == nil && (resolved.Scheme == "https" || resolved.Scheme == "http") {
							links = append(links, resolved.String())
						}
					}
				}
			}
		}
	}
}
