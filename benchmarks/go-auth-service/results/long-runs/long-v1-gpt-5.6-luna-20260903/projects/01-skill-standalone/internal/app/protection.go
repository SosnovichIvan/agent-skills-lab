package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	buckets map[string]rateBucket
}
type rateBucket struct {
	started time.Time
	count   int
}
type idempotencyKeyContext struct{}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{limit: limit, window: window, buckets: make(map[string]rateBucket)}
}
func (l *rateLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.buckets[key]
	if b.started.IsZero() || !now.Before(b.started.Add(l.window)) {
		b = rateBucket{started: now}
	}
	b.count++
	l.buckets[key] = b
	return b.count <= l.limit
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return host
	}
	if strings.TrimSpace(r.RemoteAddr) != "" {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return "unknown"
}
func (a *App) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.rateLimiter.allow("ip:"+clientIP(r), a.clock.Now().UTC()) {
			w.Header().Set("Retry-After", "60")
			writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests", requestID(r))
			return
		}
		next.ServeHTTP(w, r)
	})
}

type idempotencyEntry struct {
	fingerprint string
	status      int
	body        []byte
	expires     time.Time
	done        chan struct{}
}
type idempotencyStore struct {
	mu      sync.Mutex
	entries map[string]*idempotencyEntry
	ttl     time.Duration
}

func newIdempotencyStore(ttl time.Duration) *idempotencyStore {
	return &idempotencyStore{entries: make(map[string]*idempotencyEntry), ttl: ttl}
}
func (s *idempotencyStore) begin(key, fingerprint string, now time.Time) (*idempotencyEntry, []byte, int, error) {
	for {
		s.mu.Lock()
		e := s.entries[key]
		if e == nil || !now.Before(e.expires) {
			e = &idempotencyEntry{fingerprint: fingerprint, expires: now.Add(s.ttl), done: make(chan struct{})}
			s.entries[key] = e
			s.mu.Unlock()
			return e, nil, 0, nil
		}
		if e.fingerprint != fingerprint {
			s.mu.Unlock()
			return nil, nil, 0, errors.New("idempotency key reused with different request")
		}
		if e.body != nil {
			b, status := append([]byte(nil), e.body...), e.status
			s.mu.Unlock()
			return nil, b, status, nil
		}
		done := e.done
		s.mu.Unlock()
		<-done
	}
}
func (s *idempotencyStore) complete(key string, e *idempotencyEntry, status int, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries[key] == e && e.body == nil {
		e.status = status
		e.body = append([]byte(nil), body...)
		close(e.done)
	}
}
func (s *idempotencyStore) fail(key string, e *idempotencyEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries[key] == e {
		delete(s.entries, key)
		close(e.done)
	}
}
func idempotencyRequest(r *http.Request) (string, string, error) {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		return "", "", nil
	}
	if len(key) > 255 {
		return "", "", errors.New("invalid idempotency key")
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, maxJSONBodySize+1))
	if err != nil {
		return "", "", err
	}
	if int64(len(data)) > maxJSONBodySize {
		return "", "", errors.New("request body too large")
	}
	r.Body = io.NopCloser(strings.NewReader(string(data)))
	sum := sha256.Sum256(data)
	return key, hex.EncodeToString(sum[:]), nil
}
func replay(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
func marshalResponse(value any) []byte { body, _ := json.Marshal(value); return append(body, '\n') }
