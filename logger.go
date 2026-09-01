package crawler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
)

type RequestLog struct {
	RequestBody       io.Reader
	RequestStatus     string
	RequestStatusCode int
	RequestProtocol   string

	Err error
}

type RequestLogRec struct {
	mu   sync.Mutex
	logs []*RequestLog
}

func (r *RequestLogRec) Add(log *RequestLog) {
	r.mu.Lock()
	r.logs = append(r.logs, log)
	r.mu.Unlock()
}

// All() is called by the caller to fetch stored request logs via []RequestLog.
func (r *RequestLogRec) All() []*RequestLog {
	r.mu.Lock()
	defer r.mu.Unlock()
	// returning a copy since slices arent safe for use across concurrent gorountines (can mutate underlying array).
	out := make([]*RequestLog, len(r.logs))
	copy(r.logs, out)
	return out
}

type requestLogRecKey struct{}

func WithReqLogRecorder(ctx context.Context, rec *RequestLogRec) context.Context {
	return context.WithValue(ctx, requestLogRecKey{}, rec)
}

func RequestLogRecFromCtx(ctx context.Context) (*RequestLogRec, bool) {
	val, ok := ctx.Value(requestLogRecKey{}).(*RequestLogRec)
	return val, ok
}

// if resp.Body exceeds a certain size when read, MaxBodyReadSize will cap the reading size when storing body to LoggerTranspot.RequestBody.
const (
	MaxBodyReadSize = 64 * 1024 * 1024 // 64 MiB
	// InitialReadingSize is the limit of which resp.Body will get read, for verifying if it exceeds.
	// Avoids potential Out-Of-Memory attacks (OOM) from getting storred and transferred to the next Transport layer.
	InitialReadingSize = 128 * 1024 * 1024 // 124 MiB
)

func (t *LoggerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	select {
	case <-r.Context().Done():
		return nil, r.Context().Err()
	default:
	}

	b := t.Base
	if b == nil { // prevents detouring to nil Transport layer, if nil then defaulting to http's defaultTransport
		b = http.DefaultTransport
	}

	recs, hasRecs := RequestLogRecFromCtx(r.Context())

	resp, err := b.RoundTrip(r)
	if err != nil {
		t.Logger.Error("request error", "err", err)
		if hasRecs {
			recs.Add(&RequestLog{Err: err})
		}
		return nil, fmt.Errorf("encountered err: %w", err)
	}

	if !hasRecs {
		return resp, nil
	}

	log := &RequestLog{
		RequestStatus:     resp.Status,
		RequestStatusCode: resp.StatusCode,
		RequestProtocol:   resp.Proto,
	}

	if resp.Body != nil {
		// limiting reader to MaxBodyReadSize for comparison.
		r, _ := io.ReadAll(io.LimitReader(resp.Body, InitialReadingSize)) // errs are usually EOF, ignored here.
		_ = resp.Body.Close()
		// providing a downstream of body for callers to intact with.
		resp.Body = io.NopCloser(bytes.NewReader(r))
		// verifying r does not exceed MaxBodyReadSize
		if len(r) > MaxBodyReadSize { // indicates a valid size to be stored at t.RequestBody.
			log.RequestBody = bytes.NewReader(r[:MaxBodyReadSize]) // truncate to cap at MaxBodyReadSize.
		} else {
			log.RequestBody = bytes.NewReader(r)
		}
	}

	recs.Add(log)
	return resp, nil
}
