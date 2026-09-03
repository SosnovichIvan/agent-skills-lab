package httpapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type rateBucket struct {
	started time.Time
	count   int
}

// RateLimiter enforces the configured limit independently for IP and identity
// keys, so one noisy identity cannot consume another client's IP allowance.
type RateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	buckets map[string]rateBucket
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{limit: limit, window: window, buckets: make(map[string]rateBucket)}
}

func (l *RateLimiter) Allow(ip, identity string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	keys := []string{"ip:" + ip}
	if identity != "" {
		keys = append(keys, "identity:"+identity)
	}
	allowed := true
	for _, key := range keys {
		bucket := l.buckets[key]
		if bucket.started.IsZero() || !now.Before(bucket.started.Add(l.window)) {
			bucket = rateBucket{started: now}
		}
		if bucket.count >= l.limit {
			allowed = false
		}
		bucket.count++
		l.buckets[key] = bucket
	}
	return allowed
}

func RateLimitMiddleware(limiter *RateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity := ""
		if claims, ok := ClaimsFromContext(r.Context()); ok {
			identity = claims.Subject
		}
		if !limiter.Allow(clientIP(r), identity, time.Now()) {
			WriteError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
