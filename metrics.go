package crawler

import (
	"net/http"
	"sync"
	"time"
)

// MetricsRecorder holds the necessary fields in which will be included in metrics logging.
type MetricsRecorder struct {
	RequestDuration time.Duration

	RequestStatusLock sync.Mutex
	RequestStatus     map[int]int

	RequestHost       string
	MetricsInfoCount  int
	RequestErrorCount int
}

func NewMetricsRecorder() *MetricsRecorder {
	return &MetricsRecorder{
		RequestStatus: make(map[int]int),
	}
}

func (m *MetricsRecorder) ObserveRequest(host string, err error, status int, latency time.Duration) {
	m.RequestStatusLock.Lock() // safeguards m.RequestStatus.

	m.RequestStatus[status]++
	m.RequestStatusLock.Unlock()
	m.RequestHost = host
	m.RequestDuration = latency
	m.MetricsInfoCount++
	if err != nil {
		m.RequestErrorCount++
		return
	}
}

func (m *MetricsTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	// verify context state before network call.
	select {
	case <-r.Context().Done():
		return nil, r.Context().Err()
	default:
	}

	b := m.Base
	if b == nil {
		b = http.DefaultTransport
	}
	//metricsDetails := NewMetricsRecorder()

	start := time.Now()
	resp, err := b.RoundTrip(r)
	end := time.Since(start)
	if err != nil {
		return nil, err
	}

	// recording request metrics details to metricsDetails
	m.Metrics.ObserveRequest(r.URL.Hostname(), err, resp.StatusCode, end)

	m.logger.Info("Request Metrics", "metrics_count", m.Metrics.MetricsInfoCount, "host_name", m.Metrics.RequestHost, "errors_encountered", m.Metrics.RequestErrorCount, "latency", m.Metrics.RequestDuration, "status_frequency", m.Metrics.RequestStatus[resp.StatusCode])

	return resp, nil
}
