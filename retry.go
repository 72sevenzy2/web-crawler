package crawler

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"time"
)

// Determines whether status is warrant to retries.
func (r *RetryTransport) IsRetryableStatus(status, attempt int) bool {
	if t, ok := r.AllowedRetries[status]; ok { // is warrant to retry
		return attempt < t
	}

	// general server-side errors
	if status >= 500 {
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
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && !dnsErr.IsTimeout && !dnsErr.IsTemporary { // temporary client network glitches/timeouts.
		return false // this means such host does not exist.
	}
	if errors.As(err, &netErr) {
		return true
	}

	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true // caused by server abruptly closing clients keep-alive socket.
	}

	return false
}

func (r *RetryTransport) RoundTrip(z *http.Request) (*http.Response, error) {
	t := r.Base
	if t == nil {
		t = http.DefaultTransport
	}

	var (
		maxAttempts = r.MaxRetries + 1 // MaxRetries must exclude initial request
		lastResp    *http.Response
		lastErr     error
	)

	for v := range maxAttempts {
		// verify context deadline/cancellation before proceeding to network call
		if err := z.Context().Err(); err != nil {
			return nil, z.Context().Err()
		}

		resp, err := t.RoundTrip(z)
		lastErr = err

		if err == nil {
			// if succesfully, return immediately after making sure it isnt an retryable status.
			if !r.IsRetryableStatus(resp.StatusCode, v) {
				return resp, nil
			}
		} else {
			// check if is isnt an transient network error and if there are attempts remaining.
			if !r.IsRetryableNetErr(err) || v == maxAttempts-1 { // -1 as we added 1 initially for first request.
				return resp, err
			}
			lastErr = err
		}

		lastResp = resp

		// check if we have attempts remaining.
		if v == maxAttempts-1 {
			break
		} else {
			// if we have more attempts then drain previous requests resp.Body to avoid file descriptor/socket leaks.
			// at out final requst, we'ed break and return resp and drain via DrainClose() in crawl.go.
			DrainClose(resp) // doing so before reaching the next attempt.
		}

		delay := r.CalculateBackoffDelay(v)
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-z.Context().Done():
			return nil, z.Context().Err()
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return lastResp, nil
}
