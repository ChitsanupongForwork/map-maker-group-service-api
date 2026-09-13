package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

var testOrigins = []string{
	"http://localhost:3000",
	"https://map-maker-group.vercel.app",
	"https://map-maker-group-*.vercel.app",
}

func TestOriginAllowed(t *testing.T) {
	tests := map[string]bool{
		"http://localhost:3000":                                true,
		"https://map-maker-group.vercel.app":                   true,
		"https://map-maker-group-git-new-api-chits.vercel.app": true, // โดเมน preview ของ Vercel
		"http://localhost:3001":                                false,
		"https://evil.example":                                 false,
		"https://map-maker-group-.vercel.app":                  false, // * ต้องแทนอะไรสักอย่าง
		"https://map-maker-group-x.evil.com":                   false,
		"https://map-maker-group-a.b.vercel.app":               false, // * แทนได้ชั้นเดียว
	}
	for origin, want := range tests {
		if got := OriginAllowed(testOrigins, origin); got != want {
			t.Errorf("OriginAllowed(%q) = %v, want %v", origin, got, want)
		}
	}
}

func TestCORSPreflightAndHeaders(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := CORS(testOrigins, next)

	preflight := httptest.NewRequest(http.MethodOptions, "/api/fleet", nil)
	preflight.Header.Set("Origin", "https://map-maker-group.vercel.app")
	preflight.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, preflight)
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "https://map-maker-group.vercel.app" {
		t.Fatalf("preflight = %d %v, want 204 with allow-origin", rec.Code, rec.Header())
	}

	blocked := httptest.NewRequest(http.MethodGet, "/api/fleet", nil)
	blocked.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, blocked)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("disallowed origin got Access-Control-Allow-Origin %q", got)
	}
}
