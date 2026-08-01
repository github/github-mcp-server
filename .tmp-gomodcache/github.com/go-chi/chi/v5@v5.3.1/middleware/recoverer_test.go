package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func panickingHandler(http.ResponseWriter, *http.Request) { panic("foo") }

type testPrintLogger struct{}

func (testPrintLogger) Print(v ...interface{}) {}

func TestRecoverer(t *testing.T) {
	r := chi.NewRouter()

	oldRecovererErrorWriter := recovererErrorWriter
	defer func() { recovererErrorWriter = oldRecovererErrorWriter }()
	buf := &bytes.Buffer{}
	recovererErrorWriter = buf

	r.Use(Recoverer)
	r.Get("/", panickingHandler)

	ts := httptest.NewServer(r)
	defer ts.Close()

	res, _ := testRequest(t, ts, "GET", "/", nil)
	assertEqual(t, res.StatusCode, http.StatusInternalServerError)

	lines := strings.Split(buf.String(), "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "->") {
			if !strings.Contains(line, "panickingHandler") {
				t.Fatalf("First func call line should refer to panickingHandler, but actual line:\n%v\n", line)
			}
			return
		}
	}
	t.Fatal("First func call line should start with ->.")
}

func TestRecovererNoColor(t *testing.T) {
	// Force IsTTY so cW would emit color codes if useColor were true; this is
	// what makes the assertion below a real regression guard rather than a
	// no-op on non-TTY test runners (e.g. CI).
	oldIsTTY := IsTTY
	IsTTY = true
	defer func() { IsTTY = oldIsTTY }()

	oldRecovererErrorWriter := recovererErrorWriter
	defer func() { recovererErrorWriter = oldRecovererErrorWriter }()
	buf := &bytes.Buffer{}
	recovererErrorWriter = buf

	r := chi.NewRouter()
	r.Use(RequestLogger(&DefaultLogFormatter{Logger: testPrintLogger{}, NoColor: true}))
	r.Use(Recoverer)
	r.Get("/", panickingHandler)

	ts := httptest.NewServer(r)
	defer ts.Close()

	res, _ := testRequest(t, ts, "GET", "/", nil)
	assertEqual(t, res.StatusCode, http.StatusInternalServerError)

	// ANSI escape sequences start with \x1b[
	if strings.Contains(buf.String(), "\x1b[") {
		t.Fatal("Output should not contain ANSI color codes when NoColor is true")
	}
}

func TestRecovererAbortHandler(t *testing.T) {
	defer func() {
		rcv := recover()
		if rcv != http.ErrAbortHandler {
			t.Fatalf("http.ErrAbortHandler should not be recovered")
		}
	}()

	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Use(Recoverer)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	})

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	r.ServeHTTP(w, req)
}
