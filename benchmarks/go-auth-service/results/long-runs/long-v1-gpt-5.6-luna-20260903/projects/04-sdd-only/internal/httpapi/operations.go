package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

type requestIDContextKey struct{}

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if !validRequestID(requestID) {
			requestID = newRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDContextKey{}, requestID)))
	})
}

func RequestIDFromContext(r *http.Request) string {
	if requestID, ok := r.Context().Value(requestIDContextKey{}).(string); ok {
		return requestID
	}
	return ""
}

func validRequestID(value string) bool {
	if value == "" || len(value) > 128 || strings.ContainsAny(value, "\r\n") {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func newRequestID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "request-id-unavailable"
	}
	return hex.EncodeToString(raw[:])
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		response := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(response, r)
		if response.status == 0 {
			response.status = http.StatusOK
		}
		entry := struct {
			Level     string `json:"level"`
			Event     string `json:"event"`
			Method    string `json:"method"`
			Status    int    `json:"status"`
			RequestID string `json:"request_id"`
			Duration  int64  `json:"duration_ms"`
		}{"info", "http_request", r.Method, response.status, RequestIDFromContext(r), time.Since(started).Milliseconds()}
		encoded, _ := json.Marshal(entry)
		log.Print(string(encoded))
	})
}

type Metrics struct {
	Requests atomic.Uint64
	Errors   atomic.Uint64
}

func MetricsMiddleware(metrics *Metrics, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metrics.Requests.Add(1)
		response := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(response, r)
		if response.status >= 400 {
			metrics.Errors.Add(1)
		}
	})
}

func MetricsData(metrics *Metrics) map[string]uint64 {
	return map[string]uint64{"requests_total": metrics.Requests.Load(), "errors_total": metrics.Errors.Load()}
}
