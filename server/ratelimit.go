package server

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type rateLimiter struct {
	mu      sync.Mutex
	clients map[string]int
}

func newRateLimiter() *rateLimiter {
	rl := &rateLimiter{clients: make(map[string]int)}
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			rl.mu.Lock()
			rl.clients = make(map[string]int)
			rl.mu.Unlock()
		}
	}()
	return rl
}

func (rl *rateLimiter) limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if host, _, err := net.SplitHostPort(ip); err == nil {
			ip = host
		}
		if cf := r.Header.Get("CF-Connecting-IP"); cf != "" {
			ip = cf
		} else if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			ip = strings.Split(xff, ",")[0]
		}

		rl.mu.Lock()
		rl.clients[ip]++
		count := rl.clients[ip]
		rl.mu.Unlock()

		if count > 30 {
			writeJSON(w, 429, map[string]string{"error": "too many requests"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
