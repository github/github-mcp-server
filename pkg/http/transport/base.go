package transport

import (
	"crypto/tls"
	"net/http"
)

// NewBaseTransport returns an http.RoundTripper to use as the base of the
// transport chain. When skipSSLVerify is true, TLS certificate verification
// is disabled — intended only for private GitHub Enterprise instances with
// self-signed or missing certificates.
func NewBaseTransport(skipSSLVerify bool) http.RoundTripper {
	if !skipSSLVerify {
		return http.DefaultTransport
	}
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec
		},
	}
}
