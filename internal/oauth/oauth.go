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

const (
	// DefaultAuthTimeout is the default timeout for the OAuth authorization flow
	DefaultAuthTimeout = 5 * time.Minute
)

// Config holds the OAuth configuration
type Config struct {
	ClientID      string
	ClientSecret  string // Recommended for GitHub OAuth apps
	RedirectURL   string
	Scopes        []string
	AuthURL       string
	TokenURL      string
	Host          string // GitHub host (for constructing OAuth URLs)
	DeviceAuthURL string // Device authorization URL (for device flow)
	CallbackPort  int    // Fixed callback port (0 for random)
}

// Result contains the OAuth flow result
type Result struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	Expiry       time.Time
}

// generatePKCEVerifier generates a PKCE code verifier
func generatePKCEVerifier() (string, error) {
	// Generate 32 random bytes (256 bits)
	// Base64URL encoding of 32 bytes gives us 43 characters
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)
	return verifier, nil
}

// isRunningInDocker detects if the process is running inside a Docker container
func isRunningInDocker() bool {
	// Check for .dockerenv file (most common indicator)
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// Check cgroup for docker (fallback)
	data, err := os.ReadFile("/proc/1/cgroup")
	if err == nil && (strings.Contains(string(data), "docker") || strings.Contains(string(data), "containerd")) {
		return true
	}

	return false
}

// startLocalServer starts a local HTTP server on the specified port
// If port is 0, uses a random available port
func startLocalServer(port int) (net.Listener, int, error) {
	addr := fmt.Sprintf("localhost:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to start listener on %s: %w", addr, err)
	}

	actualPort := listener.Addr().(*net.TCPAddr).Port
	return listener, actualPort, nil
}

// createCallbackHandler creates an HTTP handler for the OAuth callback
func createCallbackHandler(expectedState string, codeChan chan<- string, errChan chan<- error) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		// Check for errors from OAuth provider
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

		// Verify state for CSRF protection
		if state := r.URL.Query().Get("state"); state != expectedState {
			errChan <- fmt.Errorf("state mismatch (possible CSRF attack)")
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}

		// Get authorization code
		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no authorization code received")
			http.Error(w, "No code received", http.StatusBadRequest)
			return
		}

		// Send code to channel
		codeChan <- code

		// Display success page
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := successTemplate.Execute(w, nil); err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
		}
	})

	return mux
}

// createCallbackServer creates an HTTP server for the OAuth callback
// Used by Manager for proper lifecycle management
func createCallbackServer(expectedState string, codeChan chan<- string, errChan chan<- error, listener net.Listener) *http.Server {
	handler := createCallbackHandler(expectedState, codeChan, errChan)
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second, // Prevent Slowloris attacks
	}

	// Start server in background
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("server error: %w", err)
		}
	}()

	return server
}

// openBrowser tries to open the URL in the default browser
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		// Try xdg-open first (most Linux distributions)
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	// Redirect output to prevent noise
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	return cmd.Start()
}

// GetGitHubOAuthConfig returns the GitHub OAuth configuration for the specified host
// host can be empty for github.com, or a full URL like "https://github.enterprise.com" for GHES
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

// getOAuthEndpoints returns the appropriate OAuth endpoints based on the host
func getOAuthEndpoints(host string) (authURL, tokenURL, deviceAuthURL string) {
	// Default to github.com
	if host == "" {
		return "https://github.com/login/oauth/authorize",
			"https://github.com/login/oauth/access_token",
			"https://github.com/login/device/code"
	}

	// For GHES/GHEC, OAuth endpoints are at the main domain, not api subdomain
	// Parse the host to extract the base domain
	hostURL := host
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		hostURL = "https://" + host
	}

	// Extract scheme and hostname
	var scheme, hostname string
	if strings.HasPrefix(hostURL, "https://") {
		scheme = "https"
		hostname = strings.TrimPrefix(hostURL, "https://")
	} else if strings.HasPrefix(hostURL, "http://") {
		scheme = "http"
		hostname = strings.TrimPrefix(hostURL, "http://")
	}

	// Remove any trailing slashes or paths
	// strings.Index returns -1 if not found, and we want to keep everything if there's no slash
	// If slash is at index 0, that would be invalid (e.g., "/example"), so we check > 0
	if idx := strings.Index(hostname, "/"); idx > 0 {
		hostname = hostname[:idx]
	}

	// For github.com, strip api. subdomain (api.github.com → github.com)
	// For ghe.com (GHEC), keep the full tenant domain (mycompany.ghe.com stays as-is)
	if hostname == "api.github.com" {
		hostname = "github.com"
	}

	authURL = fmt.Sprintf("%s://%s/login/oauth/authorize", scheme, hostname)
	tokenURL = fmt.Sprintf("%s://%s/login/oauth/access_token", scheme, hostname)
	deviceAuthURL = fmt.Sprintf("%s://%s/login/device/code", scheme, hostname)

	return authURL, tokenURL, deviceAuthURL
}
