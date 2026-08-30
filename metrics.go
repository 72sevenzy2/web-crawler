package crawler

import (
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
