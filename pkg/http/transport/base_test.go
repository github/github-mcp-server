package transport

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBaseTransport_DefaultTransport(t *testing.T) {
	t.Parallel()

	rt := NewBaseTransport(false)
	assert.Same(t, http.DefaultTransport, rt, "expected http.DefaultTransport when skipSSLVerify is false")
}

func TestNewBaseTransport_SkipSSLVerify(t *testing.T) {
	t.Parallel()

	rt := NewBaseTransport(true)
	require.NotNil(t, rt)

	httpTransport, ok := rt.(*http.Transport)
	require.True(t, ok, "expected *http.Transport when skipSSLVerify is true")
	require.NotNil(t, httpTransport.TLSClientConfig)
	assert.True(t, httpTransport.TLSClientConfig.InsecureSkipVerify)
}

func TestNewBaseTransport_SkipSSLVerify_ConnectsToSelfSignedServer(t *testing.T) {
	t.Parallel()

	// Start a TLS test server with a self-signed certificate
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Without skip: connection should fail due to unknown certificate authority
	strictClient := &http.Client{Transport: NewBaseTransport(false)}
	_, err := strictClient.Get(server.URL) //nolint:noctx
	assert.Error(t, err, "expected TLS error when connecting without skip")
	assert.Contains(t, err.Error(), "certificate")

	// With skip: connection should succeed
	skipClient := &http.Client{Transport: NewBaseTransport(true)}
	resp, err := skipClient.Get(server.URL) //nolint:noctx
	require.NoError(t, err, "expected successful connection when skipping SSL verification")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestNewBaseTransport_NoSkip_UsesDefaultTLSVerification(t *testing.T) {
	t.Parallel()

	// Start a TLS test server with a self-signed certificate
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	transport := NewBaseTransport(false)

	req, err := http.NewRequest(http.MethodGet, server.URL, nil) //nolint:noctx
	require.NoError(t, err)

	_, err = transport.RoundTrip(req)

	var tlsErr *tls.CertificateVerificationError
	assert.ErrorAs(t, err, &tlsErr, "expected a TLS certificate verification error")
}
