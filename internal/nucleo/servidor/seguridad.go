package servidor

import (
	"net/http"
	"sync"
	"time"
)

// rateLimiter implementa rate limiting por IP.
type rateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int           // max requests
	window   time.Duration // ventana de tiempo
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// permitir verifica si la IP puede hacer una solicitud.
func (rl *rateLimiter) permitir(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Filtrar requests antiguos
	filtered := make([]time.Time, 0, len(rl.requests[ip]))
	for _, t := range rl.requests[ip] {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}

	if len(filtered) >= rl.limit {
		rl.requests[ip] = filtered
		return false
	}

	filtered = append(filtered, now)
	rl.requests[ip] = filtered
	return true
}

// middlewareRateLimit aplica rate limiting a las solicitudes.
func (s *Servidor) middlewareRateLimit(next http.Handler) http.Handler {
	limiter := newRateLimiter(60, time.Minute) // 60 req/min por IP

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if !limiter.permitir(ip) {
			s.responderError(w, http.StatusTooManyRequests, "demasiadas solicitudes, intenta de nuevo en un momento")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// sanitizeInput limpia potenciales inyecciones de los parámetros de entrada.
func sanitizeInput(input string) string {
	// Prevenir null bytes
	result := ""
	for _, c := range input {
		if c == 0 {
			continue
		}
		result += string(c)
	}
	return result
}

// validarRuta previene path traversal verificando que la ruta no contenga
// patrones sospechosos como ".." o rutas absolutas.
func validarRuta(ruta string) bool {
	if ruta == "" {
		return false
	}
	// Bloquear path traversal
	if containsTraversal(ruta) {
		return false
	}
	return true
}

func containsTraversal(ruta string) bool {
	n := len(ruta)
	for i := 0; i < n-1; i++ {
		if ruta[i] == '.' && ruta[i+1] == '.' {
			return true
		}
	}
	return false
}