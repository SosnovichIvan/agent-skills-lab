package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"sync"
	"time"
)

type rateEntry struct {
	count int
	start time.Time
}
type rateLimiter struct {
	mu      sync.Mutex
	entries map[string]rateEntry
}

func newRateLimiter() *rateLimiter { return &rateLimiter{entries: make(map[string]rateEntry)} }
func (l *rateLimiter) allow(key string, now time.Time, limit int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entries[key]
	if e.start.IsZero() || !now.Before(e.start.Add(window)) {
		e = rateEntry{start: now}
	}
	if e.count >= limit {
		l.entries[key] = e
		return false
	}
	e.count++
	l.entries[key] = e
	return true
}

type idempotencyRecord struct {
	status  int
	header  http.Header
	body    []byte
	expires time.Time
}
type idempotencyStore struct {
	mu      sync.Mutex
	records map[string]idempotencyRecord
}

func newIdempotencyStore() *idempotencyStore {
	return &idempotencyStore{records: make(map[string]idempotencyRecord)}
}

func (s *idempotencyStore) handler(operation string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}
		fingerprint := sha256.Sum256([]byte(operation + "\x00" + key + "\x00" + requestIdentity(r)))
		cacheKey := hex.EncodeToString(fingerprint[:])
		s.mu.Lock()
		record, exists := s.records[cacheKey]
		if exists && time.Now().Before(record.expires) {
			s.mu.Unlock()
			replay(w, record)
			return
		}
		delete(s.records, cacheKey)
		s.mu.Unlock()
		capture := newCaptureWriter()
		next.ServeHTTP(capture, r)
		record = idempotencyRecord{status: capture.status, header: capture.header.Clone(), body: append([]byte(nil), capture.body.Bytes()...), expires: time.Now().Add(24 * time.Hour)}
		s.mu.Lock()
		s.records[cacheKey] = record
		s.mu.Unlock()
		capture.commit(w)
	})
}

func requestIdentity(r *http.Request) string {
	if claims, ok := ClaimsFromContext(r.Context()); ok {
		return "user:" + claims.Subject
	}
	return "ip:" + clientIP(r)
}
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

type captureWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newCaptureWriter() *captureWriter       { return &captureWriter{header: make(http.Header)} }
func (w *captureWriter) Header() http.Header { return w.header }
func (w *captureWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *captureWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(data)
}
func (w *captureWriter) commit(dst http.ResponseWriter) {
	copyHeaders(dst.Header(), w.header)
	dst.WriteHeader(w.status)
	_, _ = dst.Write(w.body.Bytes())
}
func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		dst[key] = append([]string(nil), values...)
	}
}
func replay(w http.ResponseWriter, record idempotencyRecord) {
	copyHeaders(w.Header(), record.header)
	w.WriteHeader(record.status)
	_, _ = w.Write(record.body)
}
