package ratelimit

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Limiter struct {
	mu        sync.Mutex
	visitors  map[string]*visitor
	rate      rate.Limit
	burst     int
	maxPerMin int
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func New(r rate.Limit, burst int) *Limiter {
	l := &Limiter{
		visitors: make(map[string]*visitor),
		rate:     r,
		burst:    burst,
	}
	go l.cleanup()
	return l
}

func (l *Limiter) Get(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	v, ok := l.visitors[ip]
	if !ok {
		limiter := rate.NewLimiter(l.rate, l.burst)
		l.visitors[ip] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

func (l *Limiter) Allow(ip string) bool {
	return l.Get(ip).Allow()
}

func (l *Limiter) cleanup() {
	for {
		time.Sleep(time.Minute)
		l.mu.Lock()
		for ip, v := range l.visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(l.visitors, ip)
			}
		}
		l.mu.Unlock()
	}
}

func ExtractIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		return xff
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
