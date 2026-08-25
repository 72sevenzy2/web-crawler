package crawler

import (
	"io"
	"errors"
	"net/http"
	"net"
	"time"
	"context"
)

func NewRetryClient(initDelay time.Duration, maxRetries int) *http.Client {
	return &http.Client{
		Timeout: time.Second * 30, // safeguards against hung TCP handshakes/unresponesive backends.
		Transport: &RetryTransport{
			Base:         http.DefaultTransport,
			InitialDelay: initDelay,
			Delay:        time.Second * 3,
			AllowedRetries: map[int]int{
				http.StatusRequestTimeout:     2, // 408
				http.StatusTooEarly:           2, // 425
				http.StatusMisdirectedRequest: 2, // 421
				http.StatusTooManyRequests:    3, // 429
				http.StatusBadGateway:         3, // 502
				http.StatusServiceUnavailable: 3, // 503
				http.StatusGatewayTimeout:     3, // 504
			},
		},
	}
}

// Determines whether status is warrant to retries.
func (r *RetryTransport) IsRetryableStatus(status, attempt int) bool {
	if t, ok := r.AllowedRetries[status]; ok { // is warrant to retry
		return attempt < t
	}

	// general server-side errors
	if status > 500 {
		return attempt < r.MaxRetries // default to r.MaxRetries if so.
	}

	return false
}

func (r *RetryTransport) IsRetryableNetErr(err error) bool {
	if err == nil {
		return false // nil errors cannot be retryable as there had been no errors during network call.
	}

	// verify whether conetxt had been canceled/deadline exceeded.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) { // temporary client network glitches/timeouts.
		return true
	}

	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true // caused by server abruptly closing clients keep-alive socket.
	}

	return false
}
