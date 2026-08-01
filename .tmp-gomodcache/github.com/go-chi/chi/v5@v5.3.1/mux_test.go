package chi

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestMuxBasic(t *testing.T) {
	var count uint64
	countermw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count++
			next.ServeHTTP(w, r)
		})
	}

	usermw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxKey{"user"}, "peter")
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}

	exmw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), ctxKey{"ex"}, "a")
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}

	logbuf := bytes.NewBufferString("")
	logmsg := "logmw test"
	logmw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logbuf.WriteString(logmsg)
			next.ServeHTTP(w, r)
		})
	}

	cxindex := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := ctx.Value(ctxKey{"user"}).(string)
		w.WriteHeader(200)
		w.Write([]byte(fmt.Sprintf("hi %s", user)))
	}

	ping := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("."))
	}

	headPing := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ping", "1")
		w.WriteHeader(200)
	}

	createPing := func(w http.ResponseWriter, r *http.Request) {
		// create ....
		w.WriteHeader(201)
	}

	pingAll := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ping all"))
	}

	pingAll2 := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ping all2"))
	}

	pingOne := func(w http.ResponseWriter, r *http.Request) {
		idParam := URLParam(r, "id")
		w.WriteHeader(200)
		w.Write([]byte(fmt.Sprintf("ping one id: %s", idParam)))
	}

	pingWoop := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("woop." + URLParam(r, "iidd")))
	}

	catchAll := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("catchall"))
	}

	m := NewRouter()
	m.Use(countermw)
	m.Use(usermw)
	m.Use(exmw)
	m.Use(logmw)
	m.Get("/", cxindex)
	m.Method("GET", "/ping", http.HandlerFunc(ping))
	m.MethodFunc("GET", "/pingall", pingAll)
	m.MethodFunc("get", "/ping/all", pingAll)
	m.Get("/ping/all2", pingAll2)

	m.Head("/ping", headPing)
	m.Post("/ping", createPing)
	m.Get("/ping/{id}", pingWoop)
	m.Get("/ping/{id}", pingOne) // expected to overwrite to pingOne handler
	m.Get("/ping/{iidd}/woop", pingWoop)
	m.HandleFunc("/admin/*", catchAll)
	// m.Post("/admin/*", catchAll)

	ts := httptest.NewServer(m)
	defer ts.Close()

	// GET /
	if _, body := testRequest(t, ts, "GET", "/", nil); body != "hi peter" {
		t.Fatal(body)
	}
	tlogmsg, _ := logbuf.ReadString(0)
	if tlogmsg != logmsg {
		t.Error("expecting log message from middleware:", logmsg)
	}

	// GET /ping
	if _, body := testRequest(t, ts, "GET", "/ping", nil); body != "." {
		t.Fatal(body)
	}

	// GET /pingall
	if _, body := testRequest(t, ts, "GET", "/pingall", nil); body != "ping all" {
		t.Fatal(body)
	}

	// GET /ping/all
	if _, body := testRequest(t, ts, "GET", "/ping/all", nil); body != "ping all" {
		t.Fatal(body)
	}

	// GET /ping/all2
	if _, body := testRequest(t, ts, "GET", "/ping/all2", nil); body != "ping all2" {
		t.Fatal(body)
	}

	// GET /ping/123
	if _, body := testRequest(t, ts, "GET", "/ping/123", nil); body != "ping one id: 123" {
		t.Fatal(body)
	}

	// GET /ping/allan
	if _, body := testRequest(t, ts, "GET", "/ping/allan", nil); body != "ping one id: allan" {
		t.Fatal(body)
	}

	// GET /ping/1/woop
	if _, body := testRequest(t, ts, "GET", "/ping/1/woop", nil); body != "woop.1" {
		t.Fatal(body)
	}

	// HEAD /ping
	resp, err := http.Head(ts.URL + "/ping")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Error("head failed, should be 200")
	}
	if resp.Header.Get("X-Ping") == "" {
		t.Error("expecting X-Ping header")
	}

	// GET /admin/catch-this
	if _, body := testRequest(t, ts, "GET", "/admin/catch-thazzzzz", nil); body != "catchall" {
		t.Fatal(body)
	}

	// POST /admin/catch-this
	resp, err = http.Post(ts.URL+"/admin/casdfsadfs", "text/plain", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatal(err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Error("POST failed, should be 200")
	}

	if string(body) != "catchall" {
		t.Error("expecting response body: 'catchall'")
	}

	// Custom http method DIE /ping/1/woop
	if resp, body := testRequest(t, ts, "DIE", "/ping/1/woop", nil); body != "" || resp.StatusCode != 405 {
		t.Fatalf("expecting 405 status and empty body, got %d '%s'", resp.StatusCode, body)
	}
}

func TestMuxMounts(t *testing.T) {
	r := NewRouter()

	r.Get("/{hash}", func(w http.ResponseWriter, r *http.Request) {
		v := URLParam(r, "hash")
		w.Write([]byte(fmt.Sprintf("/%s", v)))
	})

	r.Route("/{hash}/share", func(r Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			v := URLParam(r, "hash")
			w.Write([]byte(fmt.Sprintf("/%s/share", v)))
		})
		r.Get("/{network}", func(w http.ResponseWriter, r *http.Request) {
			v := URLParam(r, "hash")
			n := URLParam(r, "network")
			w.Write([]byte(fmt.Sprintf("/%s/share/%s", v, n)))
		})
	})

	m := NewRouter()
	m.Mount("/sharing", r)

	ts := httptest.NewServer(m)
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/sharing/aBc", nil); body != "/aBc" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "GET", "/sharing/aBc/share", nil); body != "/aBc/share" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "GET", "/sharing/aBc/share/twitter", nil); body != "/aBc/share/twitter" {
		t.Fatal(body)
	}
}

func TestMuxPlain(t *testing.T) {
	r := NewRouter()
	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("bye"))
	})
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("nothing here"))
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/hi", nil); body != "bye" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "GET", "/nothing-here", nil); body != "nothing here" {
		t.Fatal(body)
	}
}

func TestMuxEmptyRoutes(t *testing.T) {
	mux := NewRouter()

	apiRouter := NewRouter()
	// oops, we forgot to declare any route handlers

	mux.Handle("/api*", apiRouter)

	if _, body := testHandler(t, mux, "GET", "/", nil); body != "404 page not found\n" {
		t.Fatal(body)
	}

	if _, body := testHandler(t, apiRouter, "GET", "/", nil); body != "404 page not found\n" {
		t.Fatal(body)
	}
}

// Test a mux that routes a trailing slash, see also middleware/strip_test.go
// for an example of using a middleware to handle trailing slashes.
func TestMuxTrailingSlash(t *testing.T) {
	r := NewRouter()
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("nothing here"))
	})

	subRoutes := NewRouter()
	indexHandler := func(w http.ResponseWriter, r *http.Request) {
		accountID := URLParam(r, "accountID")
		w.Write([]byte(accountID))
	}
	subRoutes.Get("/", indexHandler)

	r.Mount("/accounts/{accountID}", subRoutes)
	r.Get("/accounts/{accountID}/", indexHandler)

	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/accounts/admin", nil); body != "admin" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "GET", "/accounts/admin/", nil); body != "admin" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "GET", "/nothing-here", nil); body != "nothing here" {
		t.Fatal(body)
	}
}

func TestMuxNestedNotFound(t *testing.T) {
	r := NewRouter()

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(context.WithValue(r.Context(), ctxKey{"mw"}, "mw"))
			next.ServeHTTP(w, r)
		})
	})

	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("bye"))
	})

	r.With(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(context.WithValue(r.Context(), ctxKey{"with"}, "with"))
			next.ServeHTTP(w, r)
		})
	}).NotFound(func(w http.ResponseWriter, r *http.Request) {
		chkMw := r.Context().Value(ctxKey{"mw"}).(string)
		chkWith := r.Context().Value(ctxKey{"with"}).(string)
		w.WriteHeader(404)
		w.Write([]byte(fmt.Sprintf("root 404 %s %s", chkMw, chkWith)))
	})

	sr1 := NewRouter()

	sr1.Get("/sub", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("sub"))
	})
	sr1.Group(func(sr1 Router) {
		sr1.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r = r.WithContext(context.WithValue(r.Context(), ctxKey{"mw2"}, "mw2"))
				next.ServeHTTP(w, r)
			})
		})
		sr1.NotFound(func(w http.ResponseWriter, r *http.Request) {
			chkMw2 := r.Context().Value(ctxKey{"mw2"}).(string)
			w.WriteHeader(404)
			w.Write([]byte(fmt.Sprintf("sub 404 %s", chkMw2)))
		})
	})

	sr2 := NewRouter()
	sr2.Get("/sub", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("sub2"))
	})

	r.Mount("/admin1", sr1)
	r.Mount("/admin2", sr2)

	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/hi", nil); body != "bye" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "GET", "/nothing-here", nil); body != "root 404 mw with" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "GET", "/admin1/sub", nil); body != "sub" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "GET", "/admin1/nope", nil); body != "sub 404 mw2" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "GET", "/admin2/sub", nil); body != "sub2" {
		t.Fatal(body)
	}

	// Not found pages should bubble up to the root.
	if _, body := testRequest(t, ts, "GET", "/admin2/nope", nil); body != "root 404 mw with" {
		t.Fatal(body)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	r := NewRouter()

	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hi, get"))
	})

	r.Head("/hi", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hi, head"))
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	t.Run("Registered Method", func(t *testing.T) {
		resp, _ := testRequest(t, ts, "GET", "/hi", nil)
		if resp.StatusCode != 200 {
			t.Fatal(resp.Status)
		}
		if resp.Header.Values("Allow") != nil {
			t.Fatal("allow should be empty when method is registered")
		}
	})

	t.Run("Unregistered Method", func(t *testing.T) {
		resp, _ := testRequest(t, ts, "POST", "/hi", nil)
		if resp.StatusCode != 405 {
			t.Fatal(resp.Status)
		}
		allowedMethods := resp.Header.Values("Allow")
		if len(allowedMethods) != 2 || ((allowedMethods[0] != "GET" || allowedMethods[1] != "HEAD") &&
			(allowedMethods[1] != "GET" || allowedMethods[0] != "HEAD")) {
			t.Fatal("Allow header should contain 2 headers: GET, HEAD. Received: ", allowedMethods)
		}
	})
}

func TestMuxNestedMethodNotAllowed(t *testing.T) {
	r := NewRouter()
	r.Get("/root", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("root"))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(405)
		w.Write([]byte("root 405"))
	})

	sr1 := NewRouter()
	sr1.Get("/sub1", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("sub1"))
	})
	sr1.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(405)
		w.Write([]byte("sub1 405"))
	})

	sr2 := NewRouter()
	sr2.Get("/sub2", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("sub2"))
	})

	pathVar := NewRouter()
	pathVar.Get("/{var}", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pv"))
	})
	pathVar.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(405)
		w.Write([]byte("pv 405"))
	})

	r.Mount("/prefix1", sr1)
	r.Mount("/prefix2", sr2)
	r.Mount("/pathVar", pathVar)

	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/root", nil); body != "root" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "PUT", "/root", nil); body != "root 405" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "GET", "/prefix1/sub1", nil); body != "sub1" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "PUT", "/prefix1/sub1", nil); body != "sub1 405" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "GET", "/prefix2/sub2", nil); body != "sub2" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "PUT", "/prefix2/sub2", nil); body != "root 405" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "GET", "/pathVar/myvar", nil); body != "pv" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "DELETE", "/pathVar/myvar", nil); body != "pv 405" {
		t.Fatal(body)
	}
}

func TestMuxComplicatedNotFound(t *testing.T) {
	decorateRouter := func(r *Mux) {
		// Root router with groups
		r.Get("/auth", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("auth get"))
		})
		r.Route("/public", func(r Router) {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("public get"))
			})
		})

		// sub router with groups
		sub0 := NewRouter()
		sub0.Route("/resource", func(r Router) {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("private get"))
			})
		})
		r.Mount("/private", sub0)

		// sub router with groups
		sub1 := NewRouter()
		sub1.Route("/resource", func(r Router) {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("private get"))
			})
		})
		r.With(func(next http.Handler) http.Handler { return next }).Mount("/private_mw", sub1)
	}

	testNotFound := func(t *testing.T, r *Mux) {
		ts := httptest.NewServer(r)
		defer ts.Close()

		// check that we didn't break correct routes
		if _, body := testRequest(t, ts, "GET", "/auth", nil); body != "auth get" {
			t.Fatal(body)
		}
		if _, body := testRequest(t, ts, "GET", "/public", nil); body != "public get" {
			t.Fatal(body)
		}
		if _, body := testRequest(t, ts, "GET", "/public/", nil); body != "public get" {
			t.Fatal(body)
		}
		if _, body := testRequest(t, ts, "GET", "/private/resource", nil); body != "private get" {
			t.Fatal(body)
		}
		// check custom not-found on all levels
		if _, body := testRequest(t, ts, "GET", "/nope", nil); body != "custom not-found" {
			t.Fatal(body)
		}
		if _, body := testRequest(t, ts, "GET", "/public/nope", nil); body != "custom not-found" {
			t.Fatal(body)
		}
		if _, body := testRequest(t, ts, "GET", "/private/nope", nil); body != "custom not-found" {
			t.Fatal(body)
		}
		if _, body := testRequest(t, ts, "GET", "/private/resource/nope", nil); body != "custom not-found" {
			t.Fatal(body)
		}
		if _, body := testRequest(t, ts, "GET", "/private_mw/nope", nil); body != "custom not-found" {
			t.Fatal(body)
		}
		if _, body := testRequest(t, ts, "GET", "/private_mw/resource/nope", nil); body != "custom not-found" {
			t.Fatal(body)
		}
		// check custom not-found on trailing slash routes
		if _, body := testRequest(t, ts, "GET", "/auth/", nil); body != "custom not-found" {
			t.Fatal(body)
		}
	}

	t.Run("pre", func(t *testing.T) {
		r := NewRouter()
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("custom not-found"))
		})
		decorateRouter(r)
		testNotFound(t, r)
	})

	t.Run("post", func(t *testing.T) {
		r := NewRouter()
		decorateRouter(r)
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("custom not-found"))
		})
		testNotFound(t, r)
	})
}

func TestMuxWith(t *testing.T) {
	var cmwInit1, cmwHandler1 uint64
	var cmwInit2, cmwHandler2 uint64
	mw1 := func(next http.Handler) http.Handler {
		cmwInit1++
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cmwHandler1++
			r = r.WithContext(context.WithValue(r.Context(), ctxKey{"inline1"}, "yes"))
			next.ServeHTTP(w, r)
		})
	}
	mw2 := func(next http.Handler) http.Handler {
		cmwInit2++
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cmwHandler2++
			r = r.WithContext(context.WithValue(r.Context(), ctxKey{"inline2"}, "yes"))
			next.ServeHTTP(w, r)
		})
	}

	r := NewRouter()
	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("bye"))
	})
	r.With(mw1).With(mw2).Get("/inline", func(w http.ResponseWriter, r *http.Request) {
		v1 := r.Context().Value(ctxKey{"inline1"}).(string)
		v2 := r.Context().Value(ctxKey{"inline2"}).(string)
		w.Write([]byte(fmt.Sprintf("inline %s %s", v1, v2)))
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/hi", nil); body != "bye" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "GET", "/inline", nil); body != "inline yes yes" {
		t.Fatal(body)
	}
	if cmwInit1 != 1 {
		t.Fatalf("expecting cmwInit1 to be 1, got %d", cmwInit1)
	}
	if cmwHandler1 != 1 {
		t.Fatalf("expecting cmwHandler1 to be 1, got %d", cmwHandler1)
	}
	if cmwInit2 != 1 {
		t.Fatalf("expecting cmwInit2 to be 1, got %d", cmwInit2)
	}
	if cmwHandler2 != 1 {
		t.Fatalf("expecting cmwHandler2 to be 1, got %d", cmwHandler2)
	}
}

func TestMuxHandlePatternValidation(t *testing.T) {
	testCases := []struct {
		name           string
		pattern        string
		method         string
		path           string
		expectedBody   string
		expectedStatus int
		shouldPanic    bool
	}{
		// Valid patterns
		{
			name:           "Valid pattern without HTTP GET",
			pattern:        "/user/{id}",
			shouldPanic:    false,
			method:         "GET",
			path:           "/user/123",
			expectedBody:   "without-prefix GET",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Valid pattern with HTTP POST",
			pattern:        "POST /products/{id}",
			shouldPanic:    false,
			method:         "POST",
			path:           "/products/456",
			expectedBody:   "with-prefix POST",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Valid pattern with multiple whitespace after method",
			pattern:        "PATCH \t /",
			shouldPanic:    false,
			method:         "PATCH",
			path:           "/",
			expectedBody:   "extended-whitespace PATCH",
			expectedStatus: http.StatusOK,
		},
		// Invalid patterns
		{
			name:        "Invalid pattern with no method",
			pattern:     "INVALID/user/{id}",
			shouldPanic: true,
		},
		{
			name:        "Invalid pattern with supported method",
			pattern:     "GET/user/{id}",
			shouldPanic: true,
		},
		{
			name:        "Invalid pattern with unsupported method",
			pattern:     "UNSUPPORTED /unsupported-method",
			shouldPanic: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil && !tc.shouldPanic {
					t.Errorf("Unexpected panic for pattern %s:\n%v", tc.pattern, r)
				}
			}()

			r1 := NewRouter()
			r1.Handle(tc.pattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(tc.expectedBody))
			}))

			// Test that HandleFunc also handles method patterns
			r2 := NewRouter()
			r2.HandleFunc(tc.pattern, func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(tc.expectedBody))
			})

			if !tc.shouldPanic {
				for _, r := range []Router{r1, r2} {
					// Use testRequest for valid patterns
					ts := httptest.NewServer(r)
					defer ts.Close()

					resp, body := testRequest(t, ts, tc.method, tc.path, nil)
					if body != tc.expectedBody || resp.StatusCode != tc.expectedStatus {
						t.Errorf("Expected status %d and body %s; got status %d and body %s for pattern %s",
							tc.expectedStatus, tc.expectedBody, resp.StatusCode, body, tc.pattern)
					}
				}
			}
		})
	}
}

func TestRouterFromMuxWith(t *testing.T) {
	t.Parallel()

	r := NewRouter()

	with := r.With(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	})

	with.Get("/with_middleware", func(w http.ResponseWriter, r *http.Request) {})

	ts := httptest.NewServer(with)
	defer ts.Close()

	// Without the fix this test was committed with, this causes a panic.
	testRequest(t, ts, http.MethodGet, "/with_middleware", nil)
}

func TestMuxMiddlewareStack(t *testing.T) {
	var stdmwInit, stdmwHandler uint64
	stdmw := func(next http.Handler) http.Handler {
		stdmwInit++
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			stdmwHandler++
			next.ServeHTTP(w, r)
		})
	}
	_ = stdmw

	var ctxmwInit, ctxmwHandler uint64
	ctxmw := func(next http.Handler) http.Handler {
		ctxmwInit++
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctxmwHandler++
			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxKey{"count.ctxmwHandler"}, ctxmwHandler)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}

	var inCtxmwInit, inCtxmwHandler uint64
	inCtxmw := func(next http.Handler) http.Handler {
		inCtxmwInit++
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			inCtxmwHandler++
			next.ServeHTTP(w, r)
		})
	}

	r := NewRouter()
	r.Use(stdmw)
	r.Use(ctxmw)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/ping" {
				w.Write([]byte("pong"))
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	var handlerCount uint64

	r.With(inCtxmw).Get("/", func(w http.ResponseWriter, r *http.Request) {
		handlerCount++
		ctx := r.Context()
		ctxmwHandlerCount := ctx.Value(ctxKey{"count.ctxmwHandler"}).(uint64)
		w.Write([]byte(fmt.Sprintf("inits:%d reqs:%d ctxValue:%d", ctxmwInit, handlerCount, ctxmwHandlerCount)))
	})

	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("wooot"))
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	testRequest(t, ts, "GET", "/", nil)
	testRequest(t, ts, "GET", "/", nil)
	var body string
	_, body = testRequest(t, ts, "GET", "/", nil)
	if body != "inits:1 reqs:3 ctxValue:3" {
		t.Fatalf("got: '%s'", body)
	}

	_, body = testRequest(t, ts, "GET", "/ping", nil)
	if body != "pong" {
		t.Fatalf("got: '%s'", body)
	}
}

func TestMuxRouteGroups(t *testing.T) {
	var stdmwInit, stdmwHandler uint64

	stdmw := func(next http.Handler) http.Handler {
		stdmwInit++
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			stdmwHandler++
			next.ServeHTTP(w, r)
		})
	}

	var stdmwInit2, stdmwHandler2 uint64
	stdmw2 := func(next http.Handler) http.Handler {
		stdmwInit2++
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			stdmwHandler2++
			next.ServeHTTP(w, r)
		})
	}

	r := NewRouter()
	r.Group(func(r Router) {
		r.Use(stdmw)
		r.Get("/group", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("root group"))
		})
	})
	r.Group(func(r Router) {
		r.Use(stdmw2)
		r.Get("/group2", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("root group2"))
		})
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	// GET /group
	_, body := testRequest(t, ts, "GET", "/group", nil)
	if body != "root group" {
		t.Fatalf("got: '%s'", body)
	}
	if stdmwInit != 1 || stdmwHandler != 1 {
		t.Logf("stdmw counters failed, should be 1:1, got %d:%d", stdmwInit, stdmwHandler)
	}

	// GET /group2
	_, body = testRequest(t, ts, "GET", "/group2", nil)
	if body != "root group2" {
		t.Fatalf("got: '%s'", body)
	}
	if stdmwInit2 != 1 || stdmwHandler2 != 1 {
		t.Fatalf("stdmw2 counters failed, should be 1:1, got %d:%d", stdmwInit2, stdmwHandler2)
	}
}

func TestMuxBig(t *testing.T) {
	r := bigMux()

	ts := httptest.NewServer(r)
	defer ts.Close()

	var body, expected string

	_, body = testRequest(t, ts, "GET", "/favicon.ico", nil)
	if body != "fav" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/4/view", nil)
	if body != "/hubs/4/view reqid:1 session:anonymous" {
		t.Fatalf("got '%v'", body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/4/view/index.html", nil)
	if body != "/hubs/4/view/index.html reqid:1 session:anonymous" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "POST", "/hubs/ethereumhub/view/index.html", nil)
	if body != "/hubs/ethereumhub/view/index.html reqid:1 session:anonymous" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/", nil)
	if body != "/ reqid:1 session:elvis" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/suggestions", nil)
	if body != "/suggestions reqid:1 session:elvis" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/woot/444/hiiii", nil)
	if body != "/woot/444/hiiii" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/123", nil)
	expected = "/hubs/123 reqid:1 session:elvis"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/123/touch", nil)
	if body != "/hubs/123/touch reqid:1 session:elvis" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/123/webhooks", nil)
	if body != "/hubs/123/webhooks reqid:1 session:elvis" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/123/posts", nil)
	if body != "/hubs/123/posts reqid:1 session:elvis" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/folders", nil)
	if body != "404 page not found\n" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/folders/", nil)
	if body != "/folders/ reqid:1 session:elvis" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/folders/public", nil)
	if body != "/folders/public reqid:1 session:elvis" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/folders/nothing", nil)
	if body != "404 page not found\n" {
		t.Fatalf("got '%s'", body)
	}
}

func bigMux() Router {
	var r *Mux
	var sr3 *Mux
	// var sr1, sr2, sr3, sr4, sr5, sr6 *Mux
	r = NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), ctxKey{"requestID"}, "1")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	})
	r.Group(func(r Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := context.WithValue(r.Context(), ctxKey{"session.user"}, "anonymous")
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("fav"))
		})
		r.Get("/hubs/{hubID}/view", func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			s := fmt.Sprintf("/hubs/%s/view reqid:%s session:%s", URLParam(r, "hubID"),
				ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
			w.Write([]byte(s))
		})
		r.Get("/hubs/{hubID}/view/*", func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			s := fmt.Sprintf("/hubs/%s/view/%s reqid:%s session:%s", URLParamFromCtx(ctx, "hubID"),
				URLParam(r, "*"), ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
			w.Write([]byte(s))
		})
		r.Post("/hubs/{hubSlug}/view/*", func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			s := fmt.Sprintf("/hubs/%s/view/%s reqid:%s session:%s", URLParamFromCtx(ctx, "hubSlug"),
				URLParam(r, "*"), ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
			w.Write([]byte(s))
		})
	})
	r.Group(func(r Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := context.WithValue(r.Context(), ctxKey{"session.user"}, "elvis")
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			s := fmt.Sprintf("/ reqid:%s session:%s", ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
			w.Write([]byte(s))
		})
		r.Get("/suggestions", func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			s := fmt.Sprintf("/suggestions reqid:%s session:%s", ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
			w.Write([]byte(s))
		})

		r.Get("/woot/{wootID}/*", func(w http.ResponseWriter, r *http.Request) {
			s := fmt.Sprintf("/woot/%s/%s", URLParam(r, "wootID"), URLParam(r, "*"))
			w.Write([]byte(s))
		})

		r.Route("/hubs", func(r Router) {
			_ = r.(*Mux) // sr1
			r.Route("/{hubID}", func(r Router) {
				_ = r.(*Mux) // sr2
				r.Get("/", func(w http.ResponseWriter, r *http.Request) {
					ctx := r.Context()
					s := fmt.Sprintf("/hubs/%s reqid:%s session:%s",
						URLParam(r, "hubID"), ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
					w.Write([]byte(s))
				})
				r.Get("/touch", func(w http.ResponseWriter, r *http.Request) {
					ctx := r.Context()
					s := fmt.Sprintf("/hubs/%s/touch reqid:%s session:%s", URLParam(r, "hubID"),
						ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
					w.Write([]byte(s))
				})

				sr3 = NewRouter()
				sr3.Get("/", func(w http.ResponseWriter, r *http.Request) {
					ctx := r.Context()
					s := fmt.Sprintf("/hubs/%s/webhooks reqid:%s session:%s", URLParam(r, "hubID"),
						ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
					w.Write([]byte(s))
				})
				sr3.Route("/{webhookID}", func(r Router) {
					_ = r.(*Mux) // sr4
					r.Get("/", func(w http.ResponseWriter, r *http.Request) {
						ctx := r.Context()
						s := fmt.Sprintf("/hubs/%s/webhooks/%s reqid:%s session:%s", URLParam(r, "hubID"),
							URLParam(r, "webhookID"), ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
						w.Write([]byte(s))
					})
				})

				r.Mount("/webhooks", Chain(func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{"hook"}, true)))
					})
				}).Handler(sr3))

				r.Route("/posts", func(r Router) {
					_ = r.(*Mux) // sr5
					r.Get("/", func(w http.ResponseWriter, r *http.Request) {
						ctx := r.Context()
						s := fmt.Sprintf("/hubs/%s/posts reqid:%s session:%s", URLParam(r, "hubID"),
							ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
						w.Write([]byte(s))
					})
				})
			})
		})

		r.Route("/folders/", func(r Router) {
			_ = r.(*Mux) // sr6
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				s := fmt.Sprintf("/folders/ reqid:%s session:%s",
					ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
				w.Write([]byte(s))
			})
			r.Get("/public", func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				s := fmt.Sprintf("/folders/public reqid:%s session:%s",
					ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
				w.Write([]byte(s))
			})
		})
	})

	return r
}

func TestMuxSubroutesBasic(t *testing.T) {
	hIndex := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("index"))
	})
	hArticlesList := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("articles-list"))
	})
	hSearchArticles := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("search-articles"))
	})
	hGetArticle := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf("get-article:%s", URLParam(r, "id"))))
	})
	hSyncArticle := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf("sync-article:%s", URLParam(r, "id"))))
	})

	r := NewRouter()
	// var rr1, rr2 *Mux
	r.Get("/", hIndex)
	r.Route("/articles", func(r Router) {
		// rr1 = r.(*Mux)
		r.Get("/", hArticlesList)
		r.Get("/search", hSearchArticles)
		r.Route("/{id}", func(r Router) {
			// rr2 = r.(*Mux)
			r.Get("/", hGetArticle)
			r.Get("/sync", hSyncArticle)
		})
	})

	// log.Println("~~~~~~~~~")
	// log.Println("~~~~~~~~~")
	// debugPrintTree(0, 0, r.tree, 0)
	// log.Println("~~~~~~~~~")
	// log.Println("~~~~~~~~~")

	// log.Println("~~~~~~~~~")
	// log.Println("~~~~~~~~~")
	// debugPrintTree(0, 0, rr1.tree, 0)
	// log.Println("~~~~~~~~~")
	// log.Println("~~~~~~~~~")

	// log.Println("~~~~~~~~~")
	// log.Println("~~~~~~~~~")
	// debugPrintTree(0, 0, rr2.tree, 0)
	// log.Println("~~~~~~~~~")
	// log.Println("~~~~~~~~~")

	ts := httptest.NewServer(r)
	defer ts.Close()

	var body, expected string

	_, body = testRequest(t, ts, "GET", "/", nil)
	expected = "index"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/articles", nil)
	expected = "articles-list"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/articles/search", nil)
	expected = "search-articles"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/articles/123", nil)
	expected = "get-article:123"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/articles/123/sync", nil)
	expected = "sync-article:123"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
}

func TestMuxSubroutes(t *testing.T) {
	hHubView1 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hub1"))
	})
	hHubView2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hub2"))
	})
	hHubView3 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hub3"))
	})
	hAccountView1 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("account1"))
	})
	hAccountView2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("account2"))
	})

	r := NewRouter()
	r.Get("/hubs/{hubID}/view", hHubView1)
	r.Get("/hubs/{hubID}/view/*", hHubView2)

	sr := NewRouter()
	sr.Get("/", hHubView3)
	r.Mount("/hubs/{hubID}/users", sr)
	r.Get("/hubs/{hubID}/users/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hub3 override"))
	})

	sr3 := NewRouter()
	sr3.Get("/", hAccountView1)
	sr3.Get("/hi", hAccountView2)

	// var sr2 *Mux
	r.Route("/accounts/{accountID}", func(r Router) {
		_ = r.(*Mux) // sr2
		// r.Get("/", hAccountView1)
		r.Mount("/", sr3)
	})

	// This is the same as the r.Route() call mounted on sr2
	// sr2 := NewRouter()
	// sr2.Mount("/", sr3)
	// r.Mount("/accounts/{accountID}", sr2)

	ts := httptest.NewServer(r)
	defer ts.Close()

	var body, expected string

	_, body = testRequest(t, ts, "GET", "/hubs/123/view", nil)
	expected = "hub1"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/123/view/index.html", nil)
	expected = "hub2"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/123/users", nil)
	expected = "hub3"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/123/users/", nil)
	expected = "hub3 override"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/accounts/44", nil)
	expected = "account1"
	if body != expected {
		t.Fatalf("request:%s expected:%s got:%s", "GET /accounts/44", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/accounts/44/hi", nil)
	expected = "account2"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}

	// Test that we're building the routingPatterns properly
	router := r
	req, _ := http.NewRequest("GET", "/accounts/44/hi", nil)

	rctx := NewRouteContext()
	req = req.WithContext(context.WithValue(req.Context(), RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	body = w.Body.String()
	expected = "account2"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}

	routePatterns := rctx.RoutePatterns
	if len(rctx.RoutePatterns) != 3 {
		t.Fatalf("expected 3 routing patterns, got:%d", len(rctx.RoutePatterns))
	}
	expected = "/accounts/{accountID}/*"
	if routePatterns[0] != expected {
		t.Fatalf("routePattern, expected:%s got:%s", expected, routePatterns[0])
	}
	expected = "/*"
	if routePatterns[1] != expected {
		t.Fatalf("routePattern, expected:%s got:%s", expected, routePatterns[1])
	}
	expected = "/hi"
	if routePatterns[2] != expected {
		t.Fatalf("routePattern, expected:%s got:%s", expected, routePatterns[2])
	}

}

func TestSingleHandler(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := URLParam(r, "name")
		w.Write([]byte("hi " + name))
	})

	r, _ := http.NewRequest("GET", "/", nil)
	rctx := NewRouteContext()
	r = r.WithContext(context.WithValue(r.Context(), RouteCtxKey, rctx))
	rctx.URLParams.Add("name", "joe")

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	body := w.Body.String()
	expected := "hi joe"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
}

// TODO: a Router wrapper test..
//
// type ACLMux struct {
// 	*Mux
// 	XX string
// }
//
// func NewACLMux() *ACLMux {
// 	return &ACLMux{Mux: NewRouter(), XX: "hihi"}
// }
//
// // TODO: this should be supported...
// func TestWoot(t *testing.T) {
// 	var r Router = NewRouter()
//
// 	var r2 Router = NewACLMux() //NewRouter()
// 	r2.Get("/hi", func(w http.ResponseWriter, r *http.Request) {
// 		w.Write([]byte("hi"))
// 	})
//
// 	r.Mount("/", r2)
// }

func TestServeHTTPExistingContext(t *testing.T) {
	r := NewRouter()
	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) {
		s, _ := r.Context().Value(ctxKey{"testCtx"}).(string)
		w.Write([]byte(s))
	})
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		s, _ := r.Context().Value(ctxKey{"testCtx"}).(string)
		w.WriteHeader(404)
		w.Write([]byte(s))
	})

	testcases := []struct {
		Ctx            context.Context
		Method         string
		Path           string
		ExpectedBody   string
		ExpectedStatus int
	}{
		{
			Method:         "GET",
			Path:           "/hi",
			Ctx:            context.WithValue(context.Background(), ctxKey{"testCtx"}, "hi ctx"),
			ExpectedStatus: 200,
			ExpectedBody:   "hi ctx",
		},
		{
			Method:         "GET",
			Path:           "/hello",
			Ctx:            context.WithValue(context.Background(), ctxKey{"testCtx"}, "nothing here ctx"),
			ExpectedStatus: 404,
			ExpectedBody:   "nothing here ctx",
		},
	}

	for _, tc := range testcases {
		resp := httptest.NewRecorder()
		req, err := http.NewRequest(tc.Method, tc.Path, nil)
		if err != nil {
			t.Fatalf("%v", err)
		}
		req = req.WithContext(tc.Ctx)
		r.ServeHTTP(resp, req)
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("%v", err)
		}
		if resp.Code != tc.ExpectedStatus {
			t.Fatalf("%v != %v", tc.ExpectedStatus, resp.Code)
		}
		if string(b) != tc.ExpectedBody {
			t.Fatalf("%s != %s", tc.ExpectedBody, b)
		}
	}
}

func TestNestedGroups(t *testing.T) {
	handlerPrintCounter := func(w http.ResponseWriter, r *http.Request) {
		counter, _ := r.Context().Value(ctxKey{"counter"}).(int)
		w.Write([]byte(fmt.Sprintf("%v", counter)))
	}

	mwIncreaseCounter := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			counter, _ := ctx.Value(ctxKey{"counter"}).(int)
			counter++
			ctx = context.WithValue(ctx, ctxKey{"counter"}, counter)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	// Each route represents value of its counter (number of applied middlewares).
	r := NewRouter() // counter == 0
	r.Get("/0", handlerPrintCounter)
	r.Group(func(r Router) {
		r.Use(mwIncreaseCounter) // counter == 1
		r.Get("/1", handlerPrintCounter)

		// r.Handle(GET, "/2", Chain(mwIncreaseCounter).HandlerFunc(handlerPrintCounter))
		r.With(mwIncreaseCounter).Get("/2", handlerPrintCounter)

		r.Group(func(r Router) {
			r.Use(mwIncreaseCounter, mwIncreaseCounter) // counter == 3
			r.Get("/3", handlerPrintCounter)
		})
		r.Route("/", func(r Router) {
			r.Use(mwIncreaseCounter, mwIncreaseCounter) // counter == 3

			// r.Handle(GET, "/4", Chain(mwIncreaseCounter).HandlerFunc(handlerPrintCounter))
			r.With(mwIncreaseCounter).Get("/4", handlerPrintCounter)

			r.Group(func(r Router) {
				r.Use(mwIncreaseCounter, mwIncreaseCounter) // counter == 5
				r.Get("/5", handlerPrintCounter)
				// r.Handle(GET, "/6", Chain(mwIncreaseCounter).HandlerFunc(handlerPrintCounter))
				r.With(mwIncreaseCounter).Get("/6", handlerPrintCounter)

			})
		})
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	for _, route := range []string{"0", "1", "2", "3", "4", "5", "6"} {
		if _, body := testRequest(t, ts, "GET", "/"+route, nil); body != route {
			t.Errorf("expected %v, got %v", route, body)
		}
	}
}

func TestMiddlewarePanicOnLateUse(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello\n"))
	}

	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}

	defer func() {
		if recover() == nil {
			t.Error("expected panic()")
		}
	}()

	r := NewRouter()
	r.Get("/", handler)
	r.Use(mw) // Too late to apply middleware, we're expecting panic().
}

func TestMountingExistingPath(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {}

	defer func() {
		if recover() == nil {
			t.Error("expected panic()")
		}
	}()

	r := NewRouter()
	r.Get("/", handler)
	r.Mount("/hi", http.HandlerFunc(handler))
	r.Mount("/hi", http.HandlerFunc(handler))
}

func TestMountingSimilarPattern(t *testing.T) {
	r := NewRouter()
	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("bye"))
	})

	r2 := NewRouter()
	r2.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("foobar"))
	})

	r3 := NewRouter()
	r3.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("foo"))
	})

	r.Mount("/foobar", r2)
	r.Mount("/foo", r3)

	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/hi", nil); body != "bye" {
		t.Fatal(body)
	}
}

func TestMuxEmptyParams(t *testing.T) {
	r := NewRouter()
	r.Get(`/users/{x}/{y}/{z}`, func(w http.ResponseWriter, r *http.Request) {
		x := URLParam(r, "x")
		y := URLParam(r, "y")
		z := URLParam(r, "z")
		w.Write([]byte(fmt.Sprintf("%s-%s-%s", x, y, z)))
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/users/a/b/c", nil); body != "a-b-c" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "GET", "/users///c", nil); body != "--c" {
		t.Fatal(body)
	}
}

func TestMuxMissingParams(t *testing.T) {
	r := NewRouter()
	r.Get(`/user/{userId:\d+}`, func(w http.ResponseWriter, r *http.Request) {
		userID := URLParam(r, "userId")
		w.Write([]byte(fmt.Sprintf("userId = '%s'", userID)))
	})
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("nothing here"))
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/user/123", nil); body != "userId = '123'" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "GET", "/user/", nil); body != "nothing here" {
		t.Fatal(body)
	}
}

func TestMuxWildcardRoute(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {}

	defer func() {
		if recover() == nil {
			t.Error("expected panic()")
		}
	}()

	r := NewRouter()
	r.Get("/*/wildcard/must/be/at/end", handler)
}

func TestMuxWildcardRouteCheckTwo(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {}

	defer func() {
		if recover() == nil {
			t.Error("expected panic()")
		}
	}()

	r := NewRouter()
	r.Get("/*/wildcard/{must}/be/at/end", handler)
}

func TestMuxRegexp(t *testing.T) {
	r := NewRouter()
	r.Route("/{param:[0-9]*}/test", func(r Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(fmt.Sprintf("Hi: %s", URLParam(r, "param"))))
		})
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "//test", nil); body != "Hi: " {
		t.Fatal(body)
	}
}

func TestMuxRegexp2(t *testing.T) {
	r := NewRouter()
	r.Get("/foo-{suffix:[a-z]{2,3}}.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(URLParam(r, "suffix")))
	})
	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/foo-.json", nil); body != "" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "GET", "/foo-abc.json", nil); body != "abc" {
		t.Fatal(body)
	}
}

func TestMuxRegexp3(t *testing.T) {
	r := NewRouter()
	r.Get("/one/{firstId:[a-z0-9-]+}/{secondId:[a-z]+}/first", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("first"))
	})
	r.Get("/one/{firstId:[a-z0-9-_]+}/{secondId:[0-9]+}/second", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("second"))
	})
	r.Delete("/one/{firstId:[a-z0-9-_]+}/{secondId:[0-9]+}/second", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("third"))
	})

	r.Route("/one", func(r Router) {
		r.Get("/{dns:[a-z-0-9_]+}", func(writer http.ResponseWriter, request *http.Request) {
			writer.Write([]byte("_"))
		})
		r.Get("/{dns:[a-z-0-9_]+}/info", func(writer http.ResponseWriter, request *http.Request) {
			writer.Write([]byte("_"))
		})
		r.Delete("/{id:[0-9]+}", func(writer http.ResponseWriter, request *http.Request) {
			writer.Write([]byte("forth"))
		})
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/one/hello/peter/first", nil); body != "first" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "GET", "/one/hithere/123/second", nil); body != "second" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "DELETE", "/one/hithere/123/second", nil); body != "third" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "DELETE", "/one/123", nil); body != "forth" {
		t.Fatal(body)
	}
}

func TestMuxSubrouterWildcardParam(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "param:%v *:%v", URLParam(r, "param"), URLParam(r, "*"))
	})

	r := NewRouter()

	r.Get("/bare/{param}", h)
	r.Get("/bare/{param}/*", h)

	r.Route("/case0", func(r Router) {
		r.Get("/{param}", h)
		r.Get("/{param}/*", h)
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/bare/hi", nil); body != "param:hi *:" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "GET", "/bare/hi/yes", nil); body != "param:hi *:yes" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "GET", "/case0/hi", nil); body != "param:hi *:" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "GET", "/case0/hi/yes", nil); body != "param:hi *:yes" {
		t.Fatal(body)
	}
}

func TestMuxContextIsThreadSafe(t *testing.T) {
	router := NewRouter()
	router.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 1*time.Millisecond)
		defer cancel()

		<-ctx.Done()
	})

	wg := sync.WaitGroup{}

	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 10000 {
				w := httptest.NewRecorder()
				r, err := http.NewRequest("GET", "/ok", nil)
				if err != nil {
					t.Error(err)
					return
				}

				ctx, cancel := context.WithCancel(r.Context())
				r = r.WithContext(ctx)

				go func() {
					cancel()
				}()
				router.ServeHTTP(w, r)
			}
		}()
	}
	wg.Wait()
}

func TestEscapedURLParams(t *testing.T) {
	m := NewRouter()
	m.Get("/api/{identifier}/{region}/{size}/{rotation}/*", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		rctx := RouteContext(r.Context())
		if rctx == nil {
			t.Error("no context")
			return
		}
		identifier := URLParam(r, "identifier")
		if identifier != "http:%2f%2fexample.com%2fimage.png" {
			t.Errorf("identifier path parameter incorrect %s", identifier)
			return
		}
		region := URLParam(r, "region")
		if region != "full" {
			t.Errorf("region path parameter incorrect %s", region)
			return
		}
		size := URLParam(r, "size")
		if size != "max" {
			t.Errorf("size path parameter incorrect %s", size)
			return
		}
		rotation := URLParam(r, "rotation")
		if rotation != "0" {
			t.Errorf("rotation path parameter incorrect %s", rotation)
			return
		}
		w.Write([]byte("success"))
	})

	ts := httptest.NewServer(m)
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/api/http:%2f%2fexample.com%2fimage.png/full/max/0/color.png", nil); body != "success" {
		t.Fatal(body)
	}
}

func TestCustomHTTPMethod(t *testing.T) {
	// first we must register this method to be accepted, then we
	// can define method handlers on the router below
	RegisterMethod("BOO")

	r := NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("."))
	})

	// note the custom BOO method for route /hi
	r.MethodFunc("BOO", "/hi", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("custom method"))
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/", nil); body != "." {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "BOO", "/hi", nil); body != "custom method" {
		t.Fatal(body)
	}

	var expectRoutes = map[string]string{
		"GET": "/",
		"BOO": "/hi",
	}
	Walk(r, func(method string, route string, handler http.Handler, _ ...func(http.Handler) http.Handler) error {
		r, ok := expectRoutes[method]
		if !ok {
			t.Fatalf("unexpected method %s", method)
		}
		if r != route {
			t.Fatalf("expected route %s, got %s", r, route)
		}
		delete(expectRoutes, method)

		return nil
	})
	if len(expectRoutes) != 0 {
		t.Fatalf("missing expected methods: %v", expectRoutes)
	}
}

func TestQueryHTTPMethod(t *testing.T) {
	r := NewRouter()

	r.Get("/search", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("get"))
	})

	// QUERY is a safe, idempotent http method that conveys a request body,
	// see RFC 10008.
	r.Query("/search", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Write([]byte(fmt.Sprintf("query: %s", body)))
	})

	r.MethodFunc("QUERY", "/reports", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("reports"))
	})

	r.HandleFunc("/any", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("any"))
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, body := testRequest(t, ts, "QUERY", "/search", bytes.NewReader([]byte("select 1"))); body != "query: select 1" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "GET", "/search", nil); body != "get" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "QUERY", "/reports", nil); body != "reports" {
		t.Fatal(body)
	}
	if _, body := testRequest(t, ts, "QUERY", "/any", nil); body != "any" {
		t.Fatal(body)
	}

	// An unregistered method on the same route responds 405 with QUERY
	// listed in the Allow header.
	resp, _ := testRequest(t, ts, "POST", "/search", nil)
	if resp.StatusCode != 405 {
		t.Fatal(resp.Status)
	}
	allowedMethods := resp.Header.Values("Allow")
	if len(allowedMethods) != 2 || ((allowedMethods[0] != "GET" || allowedMethods[1] != "QUERY") &&
		(allowedMethods[1] != "GET" || allowedMethods[0] != "QUERY")) {
		t.Fatal("Allow header should contain 2 headers: GET, QUERY. Received: ", allowedMethods)
	}
}

func TestMuxMatch(t *testing.T) {
	r := NewRouter()
	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "yes")
		w.Write([]byte("bye"))
	})
	r.Route("/articles", func(r Router) {
		r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
			id := URLParam(r, "id")
			w.Header().Set("X-Article", id)
			w.Write([]byte("article:" + id))
		})
	})
	r.Route("/users", func(r Router) {
		r.Head("/{id}", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-User", "-")
			w.Write([]byte("user"))
		})
		r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
			id := URLParam(r, "id")
			w.Header().Set("X-User", id)
			w.Write([]byte("user:" + id))
		})
	})

	tctx := NewRouteContext()

	tctx.Reset()
	if r.Match(tctx, "GET", "/users/1") == false {
		t.Fatal("expecting to find match for route:", "GET", "/users/1")
	}

	tctx.Reset()
	if r.Match(tctx, "HEAD", "/articles/10") == true {
		t.Fatal("not expecting to find match for route:", "HEAD", "/articles/10")
	}
}

func TestMuxMatch_HasBasePath(t *testing.T) {
	r := NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "yes")
		w.Write([]byte(""))
	})

	tctx := NewRouteContext()

	tctx.Reset()
	if r.Match(tctx, "GET", "/") != true {
		t.Fatal("expecting to find match for route:", "GET", "/")
	}
}

func TestMuxMatch_DoesNotHaveBasePath(t *testing.T) {
	r := NewRouter()

	tctx := NewRouteContext()

	tctx.Reset()
	if r.Match(tctx, "GET", "/") != false {
		t.Fatal("not expecting to find match for route:", "GET", "/")
	}
}

func TestMuxFind(t *testing.T) {
	r := NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "yes")
		w.Write([]byte(""))
	})
	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "yes")
		w.Write([]byte("bye"))
	})
	r.Route("/yo", func(r Router) {
		r.Get("/sup", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("sup"))
		})
	})
	r.Route("/articles", func(r Router) {
		r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
			id := URLParam(r, "id")
			w.Header().Set("X-Article", id)
			w.Write([]byte("article:" + id))
		})
	})
	r.Route("/users", func(r Router) {
		r.Head("/{id}", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-User", "-")
			w.Write([]byte("user"))
		})
		r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
			id := URLParam(r, "id")
			w.Header().Set("X-User", id)
			w.Write([]byte("user:" + id))
		})
	})
	r.Route("/api", func(r Router) {
		r.Route("/groups", func(r Router) {
			r.Route("/v2", func(r Router) {
				r.Get("/", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("groups"))
				})
				r.Post("/{id}", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("POST groups"))
				})
			})
		})
	})

	tctx := NewRouteContext()

	tctx.Reset()
	if r.Find(tctx, "GET", "") == "/" {
		t.Fatal("expecting to find pattern / for route: GET")
	}

	tctx.Reset()
	if r.Find(tctx, "GET", "/nope") != "" {
		t.Fatal("not expecting to find pattern for route: GET /nope")
	}

	tctx.Reset()
	if r.Find(tctx, "GET", "/users/1") != "/users/{id}" {
		t.Fatal("expecting to find pattern /users/{id} for route: GET /users/1")
	}

	tctx.Reset()
	if r.Find(tctx, "HEAD", "/articles/10") != "" {
		t.Fatal("not expecting to find pattern for route: HEAD /articles/10")
	}

	tctx.Reset()
	if r.Find(tctx, "GET", "/yo/sup") != "/yo/sup" {
		t.Fatal("expecting to find pattern /yo/sup for route: GET /yo/sup")
	}

	tctx.Reset()
	if r.Find(tctx, "GET", "/api/groups/v2/") != "/api/groups/v2/" {
		t.Fatal("expecting to find pattern /api/groups/v2/ for route: GET /api/groups/v2/")
	}

	tctx.Reset()
	if r.Find(tctx, "POST", "/api/groups/v2/1") != "/api/groups/v2/{id}" {
		t.Fatal("expecting to find pattern /api/groups/v2/{id} for route: POST /api/groups/v2/1")
	}
}

func TestServerBaseContext(t *testing.T) {
	r := NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		baseYes := r.Context().Value(ctxKey{"base"}).(string)
		if _, ok := r.Context().Value(http.ServerContextKey).(*http.Server); !ok {
			panic("missing server context")
		}
		if _, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); !ok {
			panic("missing local addr context")
		}
		w.Write([]byte(baseYes))
	})

	// Setup http Server with a base context
	ctx := context.WithValue(context.Background(), ctxKey{"base"}, "yes")
	ts := httptest.NewUnstartedServer(r)
	ts.Config.BaseContext = func(_ net.Listener) context.Context {
		return ctx
	}
	ts.Start()

	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/", nil); body != "yes" {
		t.Fatal(body)
	}
}

func testRequest(t *testing.T, ts *httptest.Server, method, path string, body io.Reader) (*http.Response, string) {
	req, err := http.NewRequest(method, ts.URL+path, body)
	if err != nil {
		t.Fatal(err)
		return nil, ""
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
		return nil, ""
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
		return nil, ""
	}
	defer resp.Body.Close()

	return resp, string(respBody)
}

func testHandler(t *testing.T, h http.Handler, method, path string, body io.Reader) (*http.Response, string) {
	r, _ := http.NewRequest(method, path, body)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Result(), w.Body.String()
}

type ctxKey struct {
	name string
}

func (k ctxKey) String() string {
	return "context value " + k.name
}

func BenchmarkMux(b *testing.B) {
	h1 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	h2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	h3 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	h4 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	h5 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	h6 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	mx := NewRouter()
	mx.Get("/", h1)
	mx.Get("/hi", h2)
	mx.Post("/hi-post", h2) // used to benchmark 405 responses
	mx.Get("/sup/{id}/and/{this}", h3)
	mx.Get("/sup/{id}/{bar:foo}/{this}", h3)

	mx.Route("/sharing/{x}/{hash}", func(mx Router) {
		mx.Get("/", h4)          // subrouter-1
		mx.Get("/{network}", h5) // subrouter-1
		mx.Get("/twitter", h5)
		mx.Route("/direct", func(mx Router) {
			mx.Get("/", h6) // subrouter-2
			mx.Get("/download", h6)
		})
	})

	routes := []string{
		"/",
		"/hi",
		"/hi-post",
		"/sup/123/and/this",
		"/sup/123/foo/this",
		"/sharing/z/aBc",                 // subrouter-1
		"/sharing/z/aBc/twitter",         // subrouter-1
		"/sharing/z/aBc/direct",          // subrouter-2
		"/sharing/z/aBc/direct/download", // subrouter-2
	}

	for _, path := range routes {
		b.Run("route:"+path, func(b *testing.B) {
			w := httptest.NewRecorder()
			r, _ := http.NewRequest("GET", path, nil)

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				mx.ServeHTTP(w, r)
			}
		})
	}
}
