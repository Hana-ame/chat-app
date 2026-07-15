package handlers

import (
	"net"
	"net/http"
	"strings"
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

var cloudflareNets []*net.IPNet

func init() {
	cidrs := []string{
		"103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22",
		"104.16.0.0/13", "104.24.0.0/14", "108.162.192.0/18",
		"131.0.72.0/22", "141.101.64.0/18", "162.158.0.0/15",
		"172.64.0.0/13", "173.245.48.0/20", "188.114.96.0/20",
		"190.93.240.0/20", "197.234.240.0/22", "198.41.128.0/17",
		"2400:cb00::/32", "2606:4700::/32", "2803:f800::/32",
		"2405:b500::/32", "2405:8100::/32", "2a06:98c0::/29",
		"2c0f:f248::/32",
	}
	for _, c := range cidrs {
		_, p, err := net.ParseCIDR(c)
		if err == nil {
			cloudflareNets = append(cloudflareNets, p)
		}
	}
}

func isCloudflareIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	for _, n := range cloudflareNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func clientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		return ip
	}
	parts := strings.Split(xff, ",")
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if strings.Contains(s, ":") {
			if ip, _, err := net.SplitHostPort(s); err == nil {
				if !isCloudflareIP(net.ParseIP(ip)) {
					return ip
				}
			}
		}
		if ip := net.ParseIP(s); ip != nil && !isCloudflareIP(ip) {
			return s
		}
	}
	// all CF IPs, fallback to RemoteAddr
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}
