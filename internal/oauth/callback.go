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

// callbackResult 在浏览器重定向到达后由 callback server 传递。code 和 err 恰有一个被设置。
type callbackResult struct {
	code string
	err  error
}

// callbackServer 是短生命周期的本地 HTTP server，用于从 OAuth 重定向中捕获 authorization code。
type callbackServer struct {
	server   *http.Server
	listener net.Listener
	redirect string
	results  chan callbackResult
}

// listenCallback 绑定本地 callback listener。
//
// 默认绑定到 loopback (127.0.0.1)，确保 callback server 不会暴露在其他接口上。仅在 container 内设置 bindAll，
// 此时 Docker 的 published-port DNAT 将流量传递到 container 的 eth0 而非 loopback；主机侧暴露范围仍由发布限制
// （例如 -p 127.0.0.1:8085:8085）。native 运行即使使用固定端口也保持在 loopback。
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

// newCallbackServer 在 listener 上启动 callback server，验证 state 并通过 buffered channel 报告结果。
// redirect URI 始终使用 localhost，以匹配 OAuth/GitHub App 中注册的值。
func newCallbackServer(listener net.Listener, expectedState string) *callbackServer {
	cs := &callbackServer{
		server:   &http.Server{ReadHeaderTimeout: 10 * time.Second}, // ReadHeaderTimeout 防范 Slowloris。
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

// handler 渲染 callback endpoint。它恰好报告一次结果，并始终向用户显示友好页面。
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

// report 传递第一个结果并丢弃后续结果（channel 的缓冲区为一；后续重定向重试不得阻塞 handler）。
func (cs *callbackServer) report(res callbackResult) {
	select {
	case cs.results <- res:
	default:
	}
}

// wait 阻塞等待 callback 结果或 ctx 取消，然后关闭 server。每个 server 调用一次是安全的。
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

// renderError 显示失败页面。html/template 会自动转义 msg，因此恶意 error_description 无法注入 markup。
func renderError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := errorTemplate.Execute(w, struct{ ErrorMessage string }{ErrorMessage: msg}); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
