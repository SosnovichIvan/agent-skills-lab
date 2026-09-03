package app

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// metrics contains only process counters. It intentionally has no labels
// derived from users, tokens, IP addresses, or URLs.
type metrics struct {
	requests  atomic.Uint64
	responses atomic.Uint64
	errors    atomic.Uint64
}

func metricsHandler(m *metrics) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = fmt.Fprintf(w,
			"iam_http_requests_total %d\niam_http_responses_total %d\niam_http_errors_total %d\n",
			m.requests.Load(), m.responses.Load(), m.errors.Load())
	}
}

type responseObserver struct {
	http.ResponseWriter
	status int
}

func (w *responseObserver) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseObserver) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func (a *App) observabilityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		a.metrics.requests.Add(1)
		observed := &responseObserver{ResponseWriter: w}
		next.ServeHTTP(observed, r)
		if observed.status == 0 {
			observed.status = http.StatusOK
		}
		a.metrics.responses.Add(1)
		if observed.status >= http.StatusInternalServerError {
			a.metrics.errors.Add(1)
		}
		a.logger.Info("http request",
			"request_id", requestID(r),
			"method", r.Method,
			"status", observed.status,
			"duration_ms", time.Since(started).Seconds()*1000,
		)
	})
}
