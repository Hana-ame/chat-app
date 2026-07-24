package handlers

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type loginRateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
}

func newLoginRateLimiter(limit int, window time.Duration) *loginRateLimiter {
	l := &loginRateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
	go func() {
		for {
			time.Sleep(window)
			l.mu.Lock()
			now := time.Now()
			for ip, times := range l.attempts {
				var kept []time.Time
				for _, t := range times {
					if now.Sub(t) < window {
						kept = append(kept, t)
					}
				}
				if len(kept) == 0 {
					delete(l.attempts, ip)
				} else {
					l.attempts[ip] = kept
				}
			}
			l.mu.Unlock()
		}
	}()
	return l
}

func (l *loginRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	times := l.attempts[ip]
	var kept []time.Time
	for _, t := range times {
		if now.Sub(t) < l.window {
			kept = append(kept, t)
		}
	}
	l.attempts[ip] = kept
	return len(kept) < l.limit
}

func (l *loginRateLimiter) record(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.attempts[ip] = append(l.attempts[ip], time.Now())
}

type registerLimiter struct {
	mu      sync.Mutex
	count   int
	limit   int
	window  time.Duration
	resetAt time.Time
}

func newRegisterLimiter(limit int, window time.Duration) *registerLimiter {
	return &registerLimiter{
		limit:   limit,
		window:  window,
		resetAt: time.Now().Add(window),
	}
}

func (l *registerLimiter) allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if time.Now().After(l.resetAt) {
		l.count = 0
		l.resetAt = time.Now().Add(l.window)
	}
	return l.count < l.limit
}

func (l *registerLimiter) record() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.count++
}

// clientIP extracts the client IP from a request.
// chimid.RealIP middleware has already set r.RemoteAddr from X-Forwarded-For / X-Real-IP,
// so we simply strip the port.
func clientIP(r *http.Request) string {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}
	return ip
}
