package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/72sevenzy2/web-crawler"
)

func TestRequestWithRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := http.Client{}
	resp, err := crawler.RequestWithRetry(context.Background(), server.URL, 5, &client)
	if err != nil {
		t.Fatal(err)
	}

	if attempts != 3 {
		t.Fatalf("expected %d attempts, got %d", 3, attempts)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}
