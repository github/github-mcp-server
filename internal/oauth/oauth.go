package oauth

import (
	"crypto/rand"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

//go:embed templates/*.html
var templateFS embed.FS

var (
	errorTemplate   *template.Template
	successTemplate *template.Template
)

func init() {
	var err error
	errorTemplate, err = template.ParseFS(templateFS, "templates/error.html")
	if err != nil {
		panic(fmt.Sprintf("failed to parse error template: %v", err))
	}
	successTemplate, err = template.ParseFS(templateFS, "templates/success.html")
	if err != nil {
		panic(fmt.Sprintf("failed to parse success template: %v", err))
	}
}

// DefaultAuthTimeout is the timeout for the OAuth authorization flow.
const DefaultAuthTimeout = 5 * time.Minute

// Config holds the OAuth configuration.
type Config struct {
	ClientID      string
	ClientSecret  string
	Scopes        []string
	AuthURL       string
	TokenURL      string
	Host          string // GitHub host for constructing OAuth URLs
	DeviceAuthURL string
	CallbackPort  int // Fixed callback port (0 for random)
}

// Result contains the OAuth flow result.
//
// GitHub OAuth App tokens do not expire, but GitHub App tokens do.
// Callers should handle re-authentication when API calls fail with auth errors.
type Result struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	Expiry       time.Time
}

// generatePKCEVerifier generates a PKCE code verifier (43 base64url chars from 32 random bytes).
func generatePKCEVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate PKCE verifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// generateRandomToken generates a cryptographically random URL-safe token.
func generateRandomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// IsRunningInDocker detects if the process is running inside a Docker container.
// On non-Linux systems this always returns false since detection relies on
// Linux-specific filesystem paths.
func IsRunningInDocker() bool {
	if runtime.GOOS != "linux" {
		return false
	}

	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	data, err := os.ReadFile("/proc/1/cgroup")
	if err == nil && (strings.Contains(string(data), "docker") || strings.Contains(string(data), "containerd")) {
		return true
	}

	return false
}

// startLocalServer starts a local HTTP callback server.
// When port is 0 (random), binds to 127.0.0.1 only (native binary, secure).
// When port is explicitly set, binds to 0.0.0.0 so Docker port mapping
// (iptables DNAT to the container's eth0) can reach it.
func startLocalServer(port int) (net.Listener, int, error) {
	host := "127.0.0.1"
	if port > 0 {
		host = "0.0.0.0"
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to start listener on %s: %w", addr, err)
	}

	actualPort := listener.Addr().(*net.TCPAddr).Port
	return listener, actualPort, nil
}

// createCallbackHandler creates an HTTP handler for the OAuth callback.
// It validates the state parameter for CSRF protection and captures the authorization code.
func createCallbackHandler(expectedState string, codeChan chan<- string, errChan chan<- error) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			errDesc := r.URL.Query().Get("error_description")
			if errDesc != "" {
				errMsg = fmt.Sprintf("%s: %s", errMsg, errDesc)
			}
			errChan <- fmt.Errorf("authorization failed: %s", errMsg)

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			// html/template auto-escapes ErrorMessage to prevent XSS
			if err := errorTemplate.Execute(w, struct{ ErrorMessage string }{ErrorMessage: errMsg}); err != nil {
				http.Error(w, "Internal error", http.StatusInternalServerError)
			}
			return
		}

		if state := r.URL.Query().Get("state"); state != expectedState {
			errChan <- fmt.Errorf("state mismatch (possible CSRF attack)")
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no authorization code received")
			http.Error(w, "No code received", http.StatusBadRequest)
			return
		}

		codeChan <- code

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := successTemplate.Execute(w, nil); err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
		}
	})

	return mux
}

// createCallbackServer creates and starts an HTTP server for the OAuth callback.
func createCallbackServer(expectedState string, codeChan chan<- string, errChan chan<- error, listener net.Listener) *http.Server {
	handler := createCallbackHandler(expectedState, codeChan, errChan)
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second, // Prevent Slowloris attacks
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("callback server error: %w", err)
		}
	}()

	return server
}

// openBrowser tries to open the URL in the default browser.
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Start()
}

// GetGitHubOAuthConfig returns a Config for the specified GitHub host.
func GetGitHubOAuthConfig(clientID, clientSecret string, scopes []string, host string, callbackPort int) Config {
	authURL, tokenURL, deviceAuthURL := getOAuthEndpoints(host)

	return Config{
		ClientID:      clientID,
		ClientSecret:  clientSecret,
		Scopes:        scopes,
		AuthURL:       authURL,
		TokenURL:      tokenURL,
		DeviceAuthURL: deviceAuthURL,
		Host:          host,
		CallbackPort:  callbackPort,
	}
}

// getOAuthEndpoints returns the appropriate OAuth endpoints based on the host.
func getOAuthEndpoints(host string) (authURL, tokenURL, deviceAuthURL string) {
	if host == "" {
		return "https://github.com/login/oauth/authorize",
			"https://github.com/login/oauth/access_token",
			"https://github.com/login/device/code"
	}

	hostURL := host
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		hostURL = "https://" + host
	}

	var scheme, hostname string
	if strings.HasPrefix(hostURL, "https://") {
		scheme = "https"
		hostname = strings.TrimPrefix(hostURL, "https://")
	} else if strings.HasPrefix(hostURL, "http://") {
		scheme = "http"
		hostname = strings.TrimPrefix(hostURL, "http://")
	}

	if idx := strings.Index(hostname, "/"); idx > 0 {
		hostname = hostname[:idx]
	}

	// Strip api. subdomain for github.com (api.github.com → github.com)
	if hostname == "api.github.com" {
		hostname = "github.com"
	}

	authURL = fmt.Sprintf("%s://%s/login/oauth/authorize", scheme, hostname)
	tokenURL = fmt.Sprintf("%s://%s/login/oauth/access_token", scheme, hostname)
	deviceAuthURL = fmt.Sprintf("%s://%s/login/device/code", scheme, hostname)

	return authURL, tokenURL, deviceAuthURL
}
