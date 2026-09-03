package app

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"

	"log/slog"
)

type operationMetrics struct {
	requests atomic.Uint64
	errors   atomic.Uint64
}

func (a *App) metrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = fmt.Fprintf(w, "iam_http_requests_total %d\niam_http_errors_total %d\n", a.Metrics.requests.Load(), a.Metrics.errors.Load())
}

func logPath(path string) string {
	parts := strings.Split(path, "/")
	for index := range parts {
		if index > 0 && parts[index-1] == "invites" {
			parts[index] = "[redacted]"
		}
	}
	return strings.Join(parts, "/")
}

func logRequest(logger *slog.Logger, request *http.Request, requestID string) {
	logger.Info("http request", "request_id", requestID, "method", request.Method, "path", logPath(request.URL.Path))
}
