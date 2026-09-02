package benchmarks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/72sevenzy2/web-crawler"
)

func BenchmarkCrawling(t *testing.B) {
	c := crawler.NewCrawler(10, true, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{ "id": 13, "name": "josh", "email": "something.com" }]`))
	}))
	defer server.Close()

	t.ResetTimer()
	for t.Loop() {
		c.Start(context.Background(), server.URL)
	}
}
