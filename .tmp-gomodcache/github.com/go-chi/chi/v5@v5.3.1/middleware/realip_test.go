package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestXRealIP(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Add("X-Real-IP", "100.100.100.100")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Use(RealIP)

	realIP := ""
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		realIP = r.RemoteAddr
		w.Write([]byte("Hello World"))
	})
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatal("Response Code should be 200")
	}

	if realIP != "100.100.100.100" {
		t.Fatal("Test get real IP error.")
	}
}

func TestXForwardForIP(t *testing.T) {
	xForwardedForIPs := []string{
		"100.100.100.100",
		"100.100.100.100, 200.200.200.200",
		"100.100.100.100,200.200.200.200",
	}

	r := chi.NewRouter()
	r.Use(RealIP)

	for _, v := range xForwardedForIPs {
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Add("X-Forwarded-For", v)

		w := httptest.NewRecorder()

		realIP := ""
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			realIP = r.RemoteAddr
			w.Write([]byte("Hello World"))
		})
		r.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatal("Response Code should be 200")
		}

		if realIP != "100.100.100.100" {
			t.Fatal("Test get real IP error.")
		}
	}
}

func TestXForwardForXRealIPPrecedence(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Add("X-Forwarded-For", "0.0.0.0")
	req.Header.Add("X-Real-IP", "100.100.100.100")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Use(RealIP)

	realIP := ""
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		realIP = r.RemoteAddr
		w.Write([]byte("Hello World"))
	})
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatal("Response Code should be 200")
	}

	if realIP != "100.100.100.100" {
		t.Fatal("Test get real IP precedence error.")
	}
}

func TestInvalidIP(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Add("X-Real-IP", "100.100.100.1000")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Use(RealIP)

	realIP := ""
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		realIP = r.RemoteAddr
		w.Write([]byte("Hello World"))
	})
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatal("Response Code should be 200")
	}

	if realIP != "" {
		t.Fatal("Invalid IP used.")
	}
}
