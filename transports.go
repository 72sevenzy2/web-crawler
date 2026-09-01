package crawler

import (
	"log/slog"
	"net/http"
	"time"
)

// LoggerTransport is a transport layer in which detours to RetryTransport's .RoundTrip() method, after its own .RoundTrip().
type LoggerTransport struct {
	Base   http.RoundTripper
	Logger *slog.Logger
}

// NewLoggerTransport returns an LoggerTransport for logging Request details, and capping response body size.
func NewLoggerTransport() *LoggerTransport {
	return &LoggerTransport{
		// LoggerTransport is the last transport layer, the base being http.DefaultTransport.
		Base:   NewMetricsTransport(),
		Logger: slog.Default(),
	}
}

// MetricsTransport represents the final transport layer between the crawler's request and the actual website, for logging the requests methodology.
type MetricsTransport struct {
	Base    http.RoundTripper
	Metrics MetricsRecorder

	logger *slog.Logger
}

func NewMetricsTransport() *MetricsTransport {
	return &MetricsTransport{
		Base:    http.DefaultTransport,
		Metrics: *NewMetricsRecorder(),
		logger:  slog.Default(),
	}
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

func NewRetryClient(initDelay time.Duration, maxRetries int) *http.Client {
	return &http.Client{
		Timeout: time.Second * 30, // safeguards against hung TCP handshakes/unresponesive backends.
		Transport: &RetryTransport{
			Base:         NewLoggerTransport(),
			InitialDelay: initDelay,
			MaxRetries:   maxRetries,
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
