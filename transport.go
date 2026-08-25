package crawler

import (
	"net/http"
	"time"
)

type RetryTransport struct {
	Base         http.RoundTripper
	Delay        time.Duration
	InitialDelay time.Duration
	MaxRetries   int

	// will hold how many retries specific request status codes can do.
	AllowedRetries map[int]int
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
			lastResp = resp
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
