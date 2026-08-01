package middleware

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"reflect"
	"runtime"
	"testing"
	"time"
)

var testdataDir string

func init() {
	_, filename, _, _ := runtime.Caller(0)
	testdataDir = path.Join(path.Dir(filename), "/../testdata")
}

func TestWrapWriterHTTP2(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Proto != "HTTP/2.0" {
			t.Fatalf("request proto should be HTTP/2.0 but was %s", r.Proto)
		}
		_, fl := w.(http.Flusher)
		if !fl {
			t.Fatal("request should have been a http.Flusher")
		}
		_, hj := w.(http.Hijacker)
		if hj {
			t.Fatal("request should not have been a http.Hijacker")
		}
		_, rf := w.(io.ReaderFrom)
		if rf {
			t.Fatal("request should not have been an io.ReaderFrom")
		}
		_, ps := w.(http.Pusher)
		if !ps {
			t.Fatal("request should have been a http.Pusher")
		}

		w.Write([]byte("OK"))
	})

	wmw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(NewWrapResponseWriter(w, r.ProtoMajor), r)
		})
	}

	server := http.Server{
		Addr:    ":7072",
		Handler: wmw(handler),
	}
	// By serving over TLS, we get HTTP2 requests
	go server.ListenAndServeTLS(testdataDir+"/cert.pem", testdataDir+"/key.pem")
	defer server.Close()
	// We need the server to start before making the request
	time.Sleep(100 * time.Millisecond)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				// The certificates we are using are self signed
				InsecureSkipVerify: true,
			},
			ForceAttemptHTTP2: true,
		},
	}

	resp, err := client.Get("https://localhost:7072")
	if err != nil {
		t.Fatalf("could not get server: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("non 200 response: %v", resp.StatusCode)
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

func testRequestNoRedirect(t *testing.T, ts *httptest.Server, method, path string, body io.Reader) (*http.Response, string) {
	req, err := http.NewRequest(method, ts.URL+path, body)
	if err != nil {
		t.Fatal(err)
		return nil, ""
	}

	// http client that doesn't redirect
	httpClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := httpClient.Do(req)
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

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("expecting no error")
	}
}

func assertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expecting error")
	}
}

func assertEqual(t *testing.T, a, b interface{}) {
	t.Helper()
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("expecting values to be equal but got: '%v' and '%v'", a, b)
	}
}
