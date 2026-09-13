package httpx

import (
	"net/http"
	"strings"
)

// CORS ใส่ header ให้เฉพาะ origin ที่อยู่ใน allow-list และตอบ preflight (OPTIONS) เอง
// ต้องครอบ mux ไว้ข้างนอก เพราะ route แบบ "GET /api/fleet" จะตอบ OPTIONS เป็น 405
func CORS(allowed []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Add("Vary", "Origin")
			if OriginAllowed(allowed, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				w.Header().Set("Access-Control-Max-Age", "600")
			}
		}
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// OriginAllowed เทียบ origin กับ allow-list
// แต่ละรายการมี * ได้หนึ่งตัว สำหรับโดเมน preview เช่น https://map-maker-group-*.vercel.app
func OriginAllowed(allowed []string, origin string) bool {
	for _, pattern := range allowed {
		prefix, suffix, wildcard := strings.Cut(pattern, "*")
		if !wildcard {
			if pattern == origin {
				return true
			}
			continue
		}
		if len(origin) <= len(prefix)+len(suffix) ||
			!strings.HasPrefix(origin, prefix) || !strings.HasSuffix(origin, suffix) {
			continue
		}
		// ส่วนที่ * แทนต้องเป็นชื่อชั้นเดียว กันคนตั้งโดเมนแบบ map-maker-group-x.evil.com
		middle := origin[len(prefix) : len(origin)-len(suffix)]
		if !strings.ContainsAny(middle, "./:@") {
			return true
		}
	}
	return false
}
