package oauth

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"time"
)

//go:embed templates/*.html
var templateFS embed.FS

var (
	errorTemplate   = template.Must(template.ParseFS(templateFS, "templates/error.html"))
	successTemplate = template.Must(template.ParseFS(templateFS, "templates/success.html"))
)

// callbackResult is delivered by the callback server once the browser redirect
// arrives. Exactly one of code or err is set.
type callbackResult struct {
	code string
	err  error
}

// callbackServer is a short-lived local HTTP server that captures the
// authorization code from the OAuth redirect.
type callbackServer struct {
	server   *http.Server
	listener net.Listener
	redirect string
	results  chan callbackResult
}

// listenCallback binds the local callback listener.
//
// It binds to loopback (127.0.0.1) by default so the callback server is never
// exposed on other interfaces. bindAll is set only inside a container, where
// Docker's published-port DNAT delivers traffic to the container's eth0 rather
// than to loopback; host-side exposure is still constrained by the publish
// (e.g. -p 127.0.0.1:8085:8085). A native run — even with a fixed port — stays
// on loopback.
func listenCallback(port int, bindAll bool) (net.Listener, error) {
	host := "127.0.0.1"
	if bindAll {
		host = "0.0.0.0"
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("starting callback listener on %s: %w", addr, err)
	}
	return listener, nil
}

// newCallbackServer starts a callback server on listener that validates state
// and reports the result on a buffered channel. The redirect URI always uses
// localhost so it matches the value registered on the OAuth/GitHub App.
func newCallbackServer(listener net.Listener, expectedState string) *callbackServer {
	cs := &callbackServer{
		server:   &http.Server{ReadHeaderTimeout: 10 * time.Second}, // ReadHeaderTimeout guards against Slowloris.
		listener: listener,
		redirect: fmt.Sprintf("http://localhost:%d/callback", listener.Addr().(*net.TCPAddr).Port),
		results:  make(chan callbackResult, 1),
	}
	cs.server.Handler = cs.handler(expectedState)

	go func() {
		if err := cs.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			cs.report(callbackResult{err: fmt.Errorf("callback server: %w", err)})
		}
	}()

	return cs
}

// handler renders the callback endpoint. It reports the outcome exactly once and
// always shows the user a friendly page.
func (cs *callbackServer) handler(expectedState string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		if errCode := q.Get("error"); errCode != "" {
			msg := errCode
			if desc := q.Get("error_description"); desc != "" {
				msg = fmt.Sprintf("%s: %s", errCode, desc)
			}
			cs.report(callbackResult{err: fmt.Errorf("authorization failed: %s", msg)})
			renderError(w, msg)
			return
		}

		if q.Get("state") != expectedState {
			cs.report(callbackResult{err: fmt.Errorf("state mismatch (possible CSRF)")})
			renderError(w, "state mismatch")
			return
		}

		code := q.Get("code")
		if code == "" {
			cs.report(callbackResult{err: fmt.Errorf("no authorization code in callback")})
			renderError(w, "no authorization code received")
			return
		}

		cs.report(callbackResult{code: code})
		renderSuccess(w)
	})
	return mux
}

// report delivers the first outcome and drops later ones (the channel is
// buffered for one; subsequent redirect retries must not block the handler).
func (cs *callbackServer) report(res callbackResult) {
	select {
	case cs.results <- res:
	default:
	}
}

// wait blocks for the callback outcome or ctx cancellation, then shuts the
// server down. It is safe to call once per server.
func (cs *callbackServer) wait(ctx context.Context) (string, error) {
	defer cs.close()
	select {
	case res := <-cs.results:
		return res.code, res.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (cs *callbackServer) close() {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = cs.server.Shutdown(shutdownCtx)
	_ = cs.listener.Close()
}

func renderSuccess(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := successTemplate.Execute(w, nil); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// renderError shows the failure page. html/template auto-escapes msg, so a
// hostile error_description cannot inject markup.
func renderError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := errorTemplate.Execute(w, struct{ ErrorMessage string }{ErrorMessage: msg}); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
