package middleware

import (
	"bufio"
	"bytes"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type testLoggerWriter struct {
	*httptest.ResponseRecorder
}

func (cw testLoggerWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, nil
}

func TestRequestLogger(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := w.(http.Hijacker)
		if !ok {
			t.Errorf("http.Hijacker is unavailable on the writer. add the interface methods.")
		}
	})

	r := httptest.NewRequest("GET", "/", nil)
	w := testLoggerWriter{
		ResponseRecorder: httptest.NewRecorder(),
	}

	handler := DefaultLogger(testHandler)
	handler.ServeHTTP(w, r)
}

func TestRequestLoggerReadFrom(t *testing.T) {
	data := []byte("file data")
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "file", time.Time{}, bytes.NewReader(data))
	})

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler := DefaultLogger(testHandler)
	handler.ServeHTTP(w, r)

	assertEqual(t, data, w.Body.Bytes())
}
