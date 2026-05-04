package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"awghop/internal/api/respond"
)

// LoginRateLimiter — token bucket в памяти по client IP.
//
// Параметры подобраны под защиту от перебора пароля админки: ~5 попыток в минуту,
// burst 5; токен набегает по одному каждые 12с.
type LoginRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	max     int
	refill  time.Duration
}

type bucket struct {
	tokens int
	last   time.Time
}

func NewLoginRateLimiter() *LoginRateLimiter {
	return &LoginRateLimiter{
		buckets: make(map[string]*bucket),
		max:     5,
		refill:  12 * time.Second,
	}
}

// Allow возвращает true, если IP может выполнить ещё одну попытку.
func (l *LoginRateLimiter) Allow(ip string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[ip]
	if !ok {
		b = &bucket{tokens: l.max, last: now}
		l.buckets[ip] = b
	}
	add := int(now.Sub(b.last) / l.refill)
	if add > 0 {
		b.tokens += add
		if b.tokens > l.max {
			b.tokens = l.max
		}
		b.last = now
	}
	if b.tokens <= 0 {
		return false
	}
	b.tokens--
	return true
}

// Middleware оборачивает handler логина.
func (l *LoginRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !l.Allow(ip) {
			w.Header().Set("Retry-After", "60")
			respond.Error(w, http.StatusTooManyRequests, "rate_limited", "too many attempts; try again later")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
