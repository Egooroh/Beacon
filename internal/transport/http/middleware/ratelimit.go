package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type ipBucket struct {
	count int
	reset time.Time
}

type rateLimiter struct {
	mu     sync.Mutex
	m      map[string]*ipBucket
	limit  int
	window time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		m:      make(map[string]*ipBucket),
		limit:  limit,
		window: window,
	}
	go rl.periodicCleanup()
	return rl
}

func (rl *rateLimiter) allow(ip string) bool {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()
	b, ok := rl.m[ip]
	if !ok || now.After(b.reset) {
		rl.m[ip] = &ipBucket{count: 1, reset: now.Add(rl.window)}
		return true
	}
	if b.count >= rl.limit {
		return false
	}
	b.count++
	return true
}

func (rl *rateLimiter) periodicCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		rl.mu.Lock()
		for ip, b := range rl.m {
			if now.After(b.reset) {
				delete(rl.m, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimit returns a middleware that enforces a fixed-window per-IP rate limit.
// limit is the maximum number of requests allowed per window duration.
func RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	rl := newRateLimiter(limit, window)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.allow(realIP(r)) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func realIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if idx := strings.Index(forwarded, ","); idx != -1 {
			return strings.TrimSpace(forwarded[:idx])
		}
		return strings.TrimSpace(forwarded)
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}
