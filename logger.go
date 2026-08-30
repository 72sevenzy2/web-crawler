package crawler

import "log/slog"

//import "net/http"

// NewLoggerTransport returns an LoggerTransport for logging Request details, and capping response body size.
func NewLoggerTransport() *LoggerTransport {
	return &LoggerTransport{
		// LoggerTransport is the last transport layer, the base being http.DefaultTransport.
		Base: NewMetricsTransport(),
		Logger: slog.Default(),
	}
}
