package github

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"

	ghinstallation "github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRSAPEM is a real (but insecure, test-only) 2048-bit RSA private key in
// PKCS#1 PEM format. ghinstallation.New parses the key, so we need a valid
// one. This key was generated solely for use in tests and has no operational
// value.
var testRSAPEM = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEArkiICrOMw0NUrk8RWwbkULIQDNO/JrcnWOEclj9HxpUFl3jW
nAVUvx03HEv5pox8hPpBJB3tlBP+gNtJGGLQTmBGahJgBJ+qKoKU+bY6yYAYnqSJ
d/vcIh3kZsaBO9uvk5Jmm4koLDeX+l5MXvXL/gZmgw3SlCwxgXZ44aeUusXXYeT8
n1q7jo8WLWOilk8EZIoiFJQAHzD4Wifxftp1uZ7mj84MgZ+ZsRZ9S7ylxVi531nx
vNmMHrBgIDSQYZxZxNPhOQGBCkSxqsPG2RfWnc9WbT1E+psH47P81hWgMEfgwYZB
JaY91gPjlQ7ehaLSFC/Ep4KgYBEhJ1GnL2Z1NQIDAQABAoIBAFBO/dkoiWmEwiWc
K5wtXdHNa1Xt0LxPU2NCQAD/4dwg2TXGKeY1EqsKFFYGsGBNhidkhdXVsJ82Y2UP
JuyExAYJmQjRtMsMo8X47BrxHL+zNiUMHohaV0KlUZUGTZC+l3ZB1ORA3HEboP1u
rHRpgPlqC5zCJtG+V4WXiPY6WW+CbQeX2KB0VtUrX3xF77jm2xtJ5IsDi20Mf2op
OUgtChW8hf3RAXh5EcZg8e5VC4B0H9anL43svG4C9jS8QKHSMjBeViaz8BT7H5Vv
2FITQV9L3m+6G9lTcyRcDtZHsiIoXQ98qABbT5zPrtn040SF+3dMcxZrnH7+MJYD
iX4Ho5UCgYEA6xmk/HvxFp/chcaC7ZHwbj4JJpZOPpf56546tyziagLWU2u3xS1r
eN9IS91f4XXz2jiQy9ytSE0c1OxNlTPQeSJiF99U/WXjwLLGFNZWqqsJKvjw2WAd
WZlrYY8M178gHd8xS8dlkamcwQA/qd0susJq+OABx4lDNfBlXEo2U/cCgYEAvcbU
xZWd0YJZlTYdKEgWkKOZtia+49jQ9uzm5m+Vz5Z7B07r/g6pUEVEFN6QxEwkWhi+
QvfY1IzkPEZnZc2iNDbH5HwzSyJqG9Up1nIuYwlN35rHqf3DhQrOLbdvqFcpChn8
h5kbQ2u9w/RXDs3nDG6zqReUn/Pd1Xo/QxvFXTMCgYEAjhpxFD/iSLeV9rI3n1uQ
BUKwC0fcwY7g/F8mxGN383YFkGTSrnc2t9fWfiyv8Lp4C9YXB3I6tzINFFJEzsOD
5kQ3IJDYcVXt5SLqAdxQhFZfcz8HbYzgELFgK5bov1uCESxAQrqilPn9itcYpBbR
G426VPYpfS9llavZyH/++J8CgYAUtovomODlyhVe/M4H5H5aARE42VfCZJrCKK82
/XzbcHAzJwEI9K60LSs2H+irFChvkP3LL2QCJvKORZzpdp06l7QPkyLCE5qDOSvc
1Q+NDanrOuiJ/EGH1tsUEE5mkETRbm6qmiJopGzM43FRE1YhfD+tt/4nyyUuNK6M
844CEwKBgAub2q9Ld01n5OiPRPBLPWLdk7v7TkLk+dxmwFLvgAjU/E855eAlcD3r
tYQ3EfGcXdruZv7BecIplwsc2dRNlZcLew5AmDts8VJKO4F4x3nScc2YgDnOl5dm
8h+lLn4+lqUbsmQhRvpyaesUnNPH2Rxw8IZbtRqeOTxG77oKztnW
-----END RSA PRIVATE KEY-----`)

// makeInstallations is a convenience helper that builds a map[string]int64
// from alternating key/value pairs, e.g.:
//
//	makeInstallations("my-org", 111, "other-org", 222)
func makeInstallations(pairs ...interface{}) map[string]int64 {
	m := make(map[string]int64, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i].(string)] = int64(pairs[i+1].(int))
	}
	return m
}

// --- getInstallationID tests ---

func TestGetInstallationID_ExactMatch(t *testing.T) {
	f := &MultiOrgClientFactory{
		installations:  makeInstallations("my-org", 111, "other-org", 222),
		defaultInstall: 0,
	}
	assert.Equal(t, int64(111), f.getInstallationID("my-org"))
}

func TestGetInstallationID_DefaultFallback(t *testing.T) {
	f := &MultiOrgClientFactory{
		installations:  makeInstallations("my-org", 111),
		defaultInstall: 999,
	}
	assert.Equal(t, int64(999), f.getInstallationID("unknown-org"),
		"unknown org should fall back to default installation")
}

func TestGetInstallationID_NoMatchNoDefault(t *testing.T) {
	f := &MultiOrgClientFactory{
		installations:  makeInstallations("my-org", 111),
		defaultInstall: 0,
	}
	assert.Equal(t, int64(0), f.getInstallationID("unknown-org"),
		"no match and no default should return 0")
}

func TestGetInstallationID_NormalizationUppercase(t *testing.T) {
	// Keys stored lowercase-dashed; lookup with uppercase should still match.
	f := &MultiOrgClientFactory{
		installations:  makeInstallations("my-org", 111),
		defaultInstall: 0,
	}
	assert.Equal(t, int64(111), f.getInstallationID("MY-ORG"),
		"uppercase owner should normalize to lowercase and match")
}

func TestGetInstallationID_NormalizationUnderscores(t *testing.T) {
	// Keys stored as dashed; lookup with underscores should normalize.
	f := &MultiOrgClientFactory{
		installations:  makeInstallations("my-org", 111),
		defaultInstall: 0,
	}
	assert.Equal(t, int64(111), f.getInstallationID("my_org"),
		"underscored owner should normalize to dashed and match")
}

func TestGetInstallationID_NormalizationMixed(t *testing.T) {
	// Mixed case + underscores → lowercase-dashed.
	f := &MultiOrgClientFactory{
		installations:  makeInstallations("tidal-engineering", 555),
		defaultInstall: 0,
	}
	assert.Equal(t, int64(555), f.getInstallationID("TIDAL_ENGINEERING"),
		"mixed-case underscored owner should normalize correctly")
}

// --- NewMultiOrgClientFactory tests ---

func TestNewMultiOrgClientFactory_ExtractsDefault(t *testing.T) {
	installations := map[string]int64{
		"my-org":   111,
		"_default": 999,
	}
	f := NewMultiOrgClientFactory(1, []byte("key"), installations, nil, nil, "v1")

	assert.Equal(t, int64(999), f.defaultInstall,
		"_default key should be extracted into defaultInstall")
	_, hasDefault := f.installations["_default"]
	assert.False(t, hasDefault, "_default key should be removed from installations map")
	assert.Equal(t, int64(111), f.installations["my-org"],
		"other keys should remain in installations map")
}

func TestNewMultiOrgClientFactory_NoDefault(t *testing.T) {
	f := NewMultiOrgClientFactory(1, []byte("key"), makeInstallations("my-org", 111), nil, nil, "v1")
	assert.Equal(t, int64(0), f.defaultInstall, "no _default key means defaultInstall is 0")
}

// --- Fail-closed tests ---

func TestGetRESTClient_FailClosed_NoMatchNoDefault(t *testing.T) {
	f := &MultiOrgClientFactory{
		installations:  makeInstallations("my-org", 111),
		defaultInstall: 0,
		transports:     make(map[string]*ghinstallation.Transport),
	}

	_, err := f.GetRESTClient(context.Background(), "unknown-org")
	require.Error(t, err, "should return error when no installation matches and no default")
	assert.Contains(t, err.Error(), "unknown-org", "error should mention the owner")
	assert.Contains(t, err.Error(), "no GitHub App installation", "error should be descriptive")
}

func TestGetGQLClient_FailClosed_NoMatchNoDefault(t *testing.T) {
	f := &MultiOrgClientFactory{
		installations:  makeInstallations("my-org", 111),
		defaultInstall: 0,
		transports:     make(map[string]*ghinstallation.Transport),
	}

	_, err := f.GetGQLClient(context.Background(), "unknown-org")
	require.Error(t, err, "should return error when no installation matches and no default")
	assert.Contains(t, err.Error(), "unknown-org")
}

func TestGetRawClient_FailClosed_NoRawURL(t *testing.T) {
	// Even with a valid installation, missing rawURL should error.
	f := &MultiOrgClientFactory{
		installations:  makeInstallations("my-org", 111),
		defaultInstall: 111,
		transports:     make(map[string]*ghinstallation.Transport),
		privateKey:     testRSAPEM,
		appID:          1234,
		rawURL:         nil, // not configured
	}

	_, err := f.GetRawClient(context.Background(), "my-org")
	require.Error(t, err, "should return error when rawURL is not configured")
	assert.Contains(t, err.Error(), "rawURL not configured")
}

func TestGetRawClient_FailClosed_NoMatchNoDefault(t *testing.T) {
	rawURL, _ := url.Parse("https://raw.githubusercontent.com/")
	f := &MultiOrgClientFactory{
		installations:  makeInstallations("my-org", 111),
		defaultInstall: 0,
		transports:     make(map[string]*ghinstallation.Transport),
		rawURL:         rawURL,
	}

	_, err := f.GetRawClient(context.Background(), "unknown-org")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no GitHub App installation")
}

// --- ResolvePrivateKey tests ---

func TestResolvePrivateKey_FromContent(t *testing.T) {
	content := []byte("my-private-key-content")
	result, err := ResolvePrivateKey(content, "")
	require.NoError(t, err)
	assert.Equal(t, content, result)
}

func TestResolvePrivateKey_FromFile(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "private.pem")
	keyContent := []byte("key-from-file")
	require.NoError(t, os.WriteFile(keyPath, keyContent, 0600))

	result, err := ResolvePrivateKey(nil, keyPath)
	require.NoError(t, err)
	assert.Equal(t, keyContent, result)
}

func TestResolvePrivateKey_ContentTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "private.pem")
	require.NoError(t, os.WriteFile(keyPath, []byte("file-content"), 0600))

	inlineContent := []byte("inline-content")
	result, err := ResolvePrivateKey(inlineContent, keyPath)
	require.NoError(t, err)
	assert.Equal(t, inlineContent, result, "inline content should take precedence over file path")
}

func TestResolvePrivateKey_NeitherProvided(t *testing.T) {
	_, err := ResolvePrivateKey(nil, "")
	require.Error(t, err, "should return error when neither content nor path is provided")
}

func TestResolvePrivateKey_FileNotFound(t *testing.T) {
	_, err := ResolvePrivateKey(nil, "/nonexistent/path/key.pem")
	require.Error(t, err, "should return error when file does not exist")
	assert.Contains(t, err.Error(), "failed to read private key file")
}

// --- Transport caching tests ---

func TestTransportCaching_SameOrgReturnsSameTransport(t *testing.T) {
	f := NewMultiOrgClientFactory(1234, testRSAPEM, makeInstallations("my-org", 111), nil, nil, "test")

	t1, err := f.getOrCreateTransport("my-org", 111)
	require.NoError(t, err)

	t2, err := f.getOrCreateTransport("my-org", 111)
	require.NoError(t, err)

	assert.Same(t, t1, t2, "same org should return the same cached transport pointer")
}

func TestTransportCaching_DifferentOrgsGetDifferentTransports(t *testing.T) {
	f := NewMultiOrgClientFactory(1234, testRSAPEM, makeInstallations("org-a", 111, "org-b", 222), nil, nil, "test")

	ta, err := f.getOrCreateTransport("org-a", 111)
	require.NoError(t, err)

	tb, err := f.getOrCreateTransport("org-b", 222)
	require.NoError(t, err)

	assert.NotSame(t, ta, tb, "different orgs should get different transports")
}

// --- Thread safety test ---

func TestTransportCaching_ConcurrentAccess(t *testing.T) {
	f := NewMultiOrgClientFactory(1234, testRSAPEM, makeInstallations("my-org", 111), nil, nil, "test")

	const goroutines = 50
	results := make([]*ghinstallation.Transport, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			tr, err := f.getOrCreateTransport("my-org", 111)
			if err == nil {
				results[i] = tr
			}
		}()
	}
	wg.Wait()

	// All goroutines should have gotten the same transport pointer (cached).
	var first *ghinstallation.Transport
	for _, r := range results {
		if r != nil {
			first = r
			break
		}
	}
	require.NotNil(t, first, "at least one goroutine should have gotten a transport")
	for i, r := range results {
		if r != nil {
			assert.Same(t, first, r, "goroutine %d got a different transport pointer", i)
		}
	}
}
