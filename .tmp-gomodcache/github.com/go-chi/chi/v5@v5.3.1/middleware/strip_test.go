package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestStripSlashes(t *testing.T) {
	r := chi.NewRouter()

	// This middleware must be mounted at the top level of the router, not at the end-handler
	// because then it'll be too late and will end up in a 404
	r.Use(StripSlashes)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("nothing here"))
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("root"))
	})

	r.Route("/accounts/{accountID}", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			accountID := chi.URLParam(r, "accountID")
			w.Write([]byte(accountID))
		})
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, resp := testRequest(t, ts, "GET", "/", nil); resp != "root" {
		t.Fatal(resp)
	}
	if _, resp := testRequest(t, ts, "GET", "//", nil); resp != "root" {
		t.Fatal(resp)
	}
	if _, resp := testRequest(t, ts, "GET", "/accounts/admin", nil); resp != "admin" {
		t.Fatal(resp)
	}
	if _, resp := testRequest(t, ts, "GET", "/accounts/admin/", nil); resp != "admin" {
		t.Fatal(resp)
	}
	if _, resp := testRequest(t, ts, "GET", "/nothing-here", nil); resp != "nothing here" {
		t.Fatal(resp)
	}
}

func TestStripSlashesInRoute(t *testing.T) {
	r := chi.NewRouter()

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("nothing here"))
	})

	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hi"))
	})

	r.Route("/accounts/{accountID}", func(r chi.Router) {
		r.Use(StripSlashes)
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("accounts index"))
		})
		r.Get("/query", func(w http.ResponseWriter, r *http.Request) {
			accountID := chi.URLParam(r, "accountID")
			w.Write([]byte(accountID))
		})
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, resp := testRequest(t, ts, "GET", "/hi", nil); resp != "hi" {
		t.Fatal(resp)
	}
	if _, resp := testRequest(t, ts, "GET", "/hi/", nil); resp != "nothing here" {
		t.Fatal(resp)
	}
	if _, resp := testRequest(t, ts, "GET", "/accounts/admin", nil); resp != "accounts index" {
		t.Fatal(resp)
	}
	if _, resp := testRequest(t, ts, "GET", "/accounts/admin/", nil); resp != "accounts index" {
		t.Fatal(resp)
	}
	if _, resp := testRequest(t, ts, "GET", "/accounts/admin/query", nil); resp != "admin" {
		t.Fatal(resp)
	}
	if _, resp := testRequest(t, ts, "GET", "/accounts/admin/query/", nil); resp != "admin" {
		t.Fatal(resp)
	}
}

func TestRedirectSlashes(t *testing.T) {
	r := chi.NewRouter()

	// This middleware must be mounted at the top level of the router, not at the end-handler
	// because then it'll be too late and will end up in a 404
	r.Use(RedirectSlashes)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("nothing here"))
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("root"))
	})

	r.Route("/accounts/{accountID}", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			accountID := chi.URLParam(r, "accountID")
			w.Write([]byte(accountID))
		})
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	if resp, body := testRequest(t, ts, "GET", "/", nil); body != "root" || resp.StatusCode != 200 {
		t.Fatal(body, resp.StatusCode)
	}

	// NOTE: the testRequest client will follow the redirection..
	if resp, body := testRequest(t, ts, "GET", "//", nil); body != "root" || resp.StatusCode != 200 {
		t.Fatal(body, resp.StatusCode)
	}

	if resp, body := testRequest(t, ts, "GET", "/accounts/admin", nil); body != "admin" || resp.StatusCode != 200 {
		t.Fatal(body, resp.StatusCode)
	}

	// NOTE: the testRequest client will follow the redirection..
	if resp, body := testRequest(t, ts, "GET", "/accounts/admin/", nil); body != "admin" || resp.StatusCode != 200 {
		t.Fatal(body, resp.StatusCode)
	}

	if resp, body := testRequest(t, ts, "GET", "/nothing-here", nil); body != "nothing here" || resp.StatusCode != 404 {
		t.Fatal(body, resp.StatusCode)
	}

	// Ensure redirect Location url is correct
	{
		resp, body := testRequestNoRedirect(t, ts, "GET", "/accounts/someuser/", nil)
		if resp.StatusCode != 301 {
			t.Fatal(body, resp.StatusCode)
		}
		location := resp.Header.Get("Location")
		if location != "/accounts/someuser" {
			t.Fatalf("invalid redirection, should be /accounts/someuser")
		}
	}

	// Ensure query params are kept in tact upon redirecting a slash
	{
		resp, body := testRequestNoRedirect(t, ts, "GET", "/accounts/someuser/?a=1&b=2", nil)
		if resp.StatusCode != 301 {
			t.Fatal(body, resp.StatusCode)
		}
		location := resp.Header.Get("Location")
		if location != "/accounts/someuser?a=1&b=2" {
			t.Fatalf("invalid redirection, should be /accounts/someuser?a=1&b=2")
		}
	}

	// Ensure that we don't redirect to 'evil.com', but rather to 'server.url/evil.com/'
	{
		paths := []string{"//evil.com/", "///evil.com/"}

		for _, p := range paths {
			resp, body := testRequest(t, ts, "GET", p, nil)
			if u, err := url.Parse(ts.URL); err != nil && resp.Request.URL.Host != u.Host {
				t.Fatalf("host should remain the same. got: %q, want: %q", resp.Request.URL.Host, ts.URL)
			}
			if body != "nothing here" || resp.StatusCode != 404 {
				t.Fatal(body, resp.StatusCode)
			}
		}
	}

	// Ensure that we don't redirect to 'evil.com', but rather to 'server.url/evil.com/'
	{
		resp, body := testRequest(t, ts, "GET", "//evil.com/", nil)
		if u, err := url.Parse(ts.URL); err != nil && resp.Request.URL.Host != u.Host {
			t.Fatalf("host should remain the same. got: %q, want: %q", resp.Request.URL.Host, ts.URL)
		}
		if body != "nothing here" || resp.StatusCode != 404 {
			t.Fatal(body, resp.StatusCode)
		}
	}
}

// This tests a http.Handler that is not chi.Router
// In these cases, the routeContext is nil
func TestStripSlashesWithNilContext(t *testing.T) {
	r := http.NewServeMux()

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("root"))
	})

	r.HandleFunc("/accounts", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("accounts"))
	})

	r.HandleFunc("/accounts/admin", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("admin"))
	})

	ts := httptest.NewServer(StripSlashes(r))
	defer ts.Close()

	if _, resp := testRequest(t, ts, "GET", "/", nil); resp != "root" {
		t.Fatal(resp)
	}
	if _, resp := testRequest(t, ts, "GET", "//", nil); resp != "root" {
		t.Fatal(resp)
	}
	if _, resp := testRequest(t, ts, "GET", "/accounts", nil); resp != "accounts" {
		t.Fatal(resp)
	}
	if _, resp := testRequest(t, ts, "GET", "/accounts/", nil); resp != "accounts" {
		t.Fatal(resp)
	}
	if _, resp := testRequest(t, ts, "GET", "/accounts/admin", nil); resp != "admin" {
		t.Fatal(resp)
	}
	if _, resp := testRequest(t, ts, "GET", "/accounts/admin/", nil); resp != "admin" {
		t.Fatal(resp)
	}
}

func TestStripPrefix(t *testing.T) {
	r := chi.NewRouter()

	r.Use(StripPrefix("/api"))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("api root"))
	})

	r.Get("/accounts", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("api accounts"))
	})

	r.Get("/accounts/{accountID}", func(w http.ResponseWriter, r *http.Request) {
		accountID := chi.URLParam(r, "accountID")
		w.Write([]byte(accountID))
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, resp := testRequest(t, ts, "GET", "/api/", nil); resp != "api root" {
		t.Fatalf("got: %q, want: %q", resp, "api root")
	}
	if _, resp := testRequest(t, ts, "GET", "/api/accounts", nil); resp != "api accounts" {
		t.Fatalf("got: %q, want: %q", resp, "api accounts")
	}
	if _, resp := testRequest(t, ts, "GET", "/api/accounts/admin", nil); resp != "admin" {
		t.Fatalf("got: %q, want: %q", resp, "admin")
	}
	if _, resp := testRequest(t, ts, "GET", "/api-nope/", nil); resp != "404 page not found\n" {
		t.Fatalf("got: %q, want: %q", resp, "404 page not found\n")
	}
}

func TestRedirectSlashes_PreventBackslashRelativeOpenRedirect(t *testing.T) {
	h := RedirectSlashes(http.NotFoundHandler())

	tests := []struct {
		name   string
		target string
	}{
		{
			name:   `raw backslash: /\evil.com/`,
			target: `/\evil.com/`,
		},
		{
			name:   `encoded backslash: /%5Cevil.com/`,
			target: "/%5Cevil.com/",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://example.test"+tc.target, nil)
			rr := httptest.NewRecorder()

			h.ServeHTTP(rr, req)
			res := rr.Result()
			defer res.Body.Close()

			if res.StatusCode != http.StatusMovedPermanently {
				t.Fatalf("expected %d, got %d", http.StatusMovedPermanently, res.StatusCode)
			}

			loc := res.Header.Get("Location")
			if loc == "" {
				t.Fatalf("expected Location header to be set")
			}

			// The core security assertions:
			if strings.Contains(loc, `\`) {
				t.Fatalf("Location must not contain backslashes: %q", loc)
			}
			if strings.HasPrefix(loc, "//") {
				t.Fatalf("Location must not be protocol-relative: %q", loc)
			}
			if !strings.HasPrefix(loc, "/") {
				t.Fatalf("Location must be an absolute-path reference starting with '/': %q", loc)
			}

			// Optional stronger assertion if your middleware normalizes to /evil.com exactly:
			// (Keep or remove depending on your chosen behavior.)
			if loc != "/evil.com" {
				t.Fatalf("expected Location %q, got %q", "/evil.com", loc)
			}
		})
	}
}
