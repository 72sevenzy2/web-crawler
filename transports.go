package crawler

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// LoggerTransport is a transport layer in which detours to RetryTransport's .RoundTrip() method, after its own .RoundTrip().
type LoggerTransport struct {
	Base              http.RoundTripper
	Logger            *slog.Logger
	RequestStatus     string
	RequestStatusCode int
	RequestProtocol   string

	RequestBody io.Reader
}

// if resp.Body exceeds a certain size when read, MaxBodyReadSize will cap the reading size when storing body to LoggerTranspot.RequestBody.
const (
	MaxBodyReadSize = 64 * 1024 * 1024 // 64 MiB
	// InitialReadingSize is the limit of which resp.Body will get read, for verifying if it exceeds.
	// Avoids potential Out-Of-Memory attacks (OOM) from getting storred and transferred to the next Transport layer.
	InitialReadingSize = 128 * 1024 * 1024 // 124 MiB
)

func (t *LoggerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	b := t.Base
	if b == nil { // prevents detouring to nil Transport layer, if nil then defaulting to http's defaultTransport
		b = http.DefaultTransport
	}

	resp, err := b.RoundTrip(r)
	if err != nil {
		t.Logger.Error("request error", "err", err)
		return nil, fmt.Errorf("encountered err: %w", err)
	}

	t.RequestStatusCode = resp.StatusCode
	t.RequestProtocol = resp.Proto
	t.RequestStatus = resp.Status

	if resp.Body != nil {
		// limiting reader to MaxBodyReadSize for comparison.
		r, _ := io.ReadAll(io.LimitReader(resp.Body, InitialReadingSize)) // errs are usually EOF, ignored here.
		_ = resp.Body.Close()
		// providing a downstream of body for callers to intact with.
		resp.Body = io.NopCloser(bytes.NewReader(r))
		// verifying r does not exceed MaxBodyReadSize
		if len(r) > MaxBodyReadSize { // indicates a valid size to be stored at t.RequestBody.
			t.RequestBody = bytes.NewReader(r[:MaxBodyReadSize]) // truncate to cap at MaxBodyReadSize.
		} else {
			t.RequestBody = bytes.NewReader(r)
		}
	}

	return resp, nil
}

// RetryTransport is a HTTP transport layer in which http's default transport will eventually call .RoundTrip() upon detouring form RetryTransport
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
