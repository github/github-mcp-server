// Copyright 2013 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
)

const (
	// baseURLPath is a non-empty Client.BaseURL path to use during tests,
	// to ensure relative URLs are used for all endpoints. See issue #752.
	baseURLPath = "/api-v3"
)

// raceSafeTestConn wraps a net.Conn to hide concrete connection types such as *net.TCPConn.
//
// Go's HTTP transport may enable OS-level sendfile optimizations when it sees a concrete
// TCP connection and an *os.File request body. Under the race detector on Windows, that
// specific optimized path can trigger a known data race in internal polling structures.
// Returning this wrapper from DialContext keeps behavior identical for tests while forcing
// the transport onto the generic copy path, which is stable under -race.
type raceSafeTestConn struct {
	net.Conn
}

// mustNewClient is a helper function that creates a new Client and fails the test if there is an error.
func mustNewClient(t *testing.T, opts ...ClientOptionsFunc) *Client {
	t.Helper()
	c, err := NewClient(opts...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// mustParseURL is a helper function that parses a URL and fails the test if there is an error.
func mustParseURL(t *testing.T, rawurl string) *url.URL {
	t.Helper()
	u, err := url.Parse(rawurl)
	if err != nil {
		t.Fatalf("Failed to parse URL %q: %v", rawurl, err)
	}
	return u
}

// setup sets up a test HTTP server along with a github.Client that is
// configured to talk to that test server. Tests should register handlers on
// mux which provide mock responses for the API method being tested.
func setup(t *testing.T) (client *Client, mux *http.ServeMux, serverURL string) {
	t.Helper()
	// mux is the HTTP request multiplexer used with the test server.
	mux = http.NewServeMux()

	// We want to ensure that tests catch mistakes where the endpoint URL is
	// specified as absolute rather than relative. It only makes a difference
	// when there's a non-empty base URL path. So, use that. See issue #752.
	apiHandler := http.NewServeMux()
	apiHandler.Handle(baseURLPath+"/", http.StripPrefix(baseURLPath, mux))
	apiHandler.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintln(os.Stderr, "FAIL: Client.BaseURL path prefix is not preserved in the request URL:")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "\t"+req.URL.String())
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "\tDid you accidentally use an absolute endpoint URL rather than relative?")
		fmt.Fprintln(os.Stderr, "\tSee https://github.com/google/go-github/issues/752 for information.")
		http.Error(w, "Client.BaseURL path prefix is not preserved in the request URL.", http.StatusInternalServerError)
	})

	// server is a test HTTP server used to provide mock API responses.
	server := httptest.NewServer(apiHandler)

	testDialer := &net.Dialer{Timeout: 30 * time.Second}

	// Create a custom transport with isolated connection pool
	transport := &http.Transport{
		// Wrap dialed connections so transport does not take concrete-TCP sendfile fast paths
		// that can race under Windows + -race in upload tests.
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := testDialer.DialContext(ctx, network, addr)
			if err != nil {
				return nil, err
			}
			return &raceSafeTestConn{Conn: conn}, nil
		},
		// Controls connection reuse - false allows reuse, true forces new connections for each request
		DisableKeepAlives: false,
		// Maximum concurrent connections per host (active + idle)
		MaxConnsPerHost: 10,
		// Maximum idle connections maintained per host for reuse
		MaxIdleConnsPerHost: 5,
		// Maximum total idle connections across all hosts
		MaxIdleConns: 20,
		// How long an idle connection remains in the pool before being closed
		IdleConnTimeout: 20 * time.Second,
	}

	// Create HTTP client with the isolated transport
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
	// client is the GitHub client being tested and is
	// configured to use test server.
	client = mustNewClient(t, WithHTTPClient(httpClient))

	url, _ := url.Parse(server.URL + baseURLPath + "/")
	client.baseURL = url
	client.uploadURL = url

	t.Cleanup(server.Close)

	return client, mux, server.URL
}

// openTestFile creates a new file with the given name and content for testing.
// In order to ensure the exact file name, this function will create a new temp
// directory, and create the file in that directory. The file is automatically
// cleaned up after the test.
func openTestFile(t *testing.T, name, content string) *os.File {
	t.Helper()
	fname := filepath.Join(t.TempDir(), name)
	err := os.WriteFile(fname, []byte(content), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(fname)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { file.Close() })

	return file
}

func testMethod(t *testing.T, r *http.Request, want string) {
	t.Helper()
	if got := r.Method; got != want {
		t.Errorf("Request method: %v, want %v", got, want)
	}
}

type values map[string]string

// testFormValues checks that the request form values match the expected values.
// It expects exactly one value per key.
func testFormValues(t *testing.T, r *http.Request, want values) {
	t.Helper()

	wantValues := url.Values{}
	for k, v := range want {
		wantValues.Add(k, v)
	}

	assertNilError(t, r.ParseForm())
	if got := r.Form; !cmp.Equal(got, wantValues) {
		t.Errorf("Request query parameters: %v, want %v", got, wantValues)
	}
}

// testFormValuesList checks that the request form values match the expected values.
// It allows for multiple values per key.
func testFormValuesList(t *testing.T, r *http.Request, want url.Values) {
	t.Helper()

	assertNilError(t, r.ParseForm())
	if got := r.Form; !cmp.Equal(got, want) {
		t.Errorf("Request query parameters: %v, want %v", got, want)
	}
}

func testHeader(t *testing.T, r *http.Request, header, want string) {
	t.Helper()
	if got := r.Header.Get(header); got != want {
		t.Errorf("Header.Get(%q) returned %q, want %q", header, got, want)
	}
}

func testURLParseError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Error("Expected error to be returned")
	}
	var uerr *url.Error
	if !errors.As(err, &uerr) || uerr.Op != "parse" {
		t.Errorf("Expected URL parse error, got %+v", err)
	}
}

func testPlainBody(t *testing.T, r *http.Request, want string) {
	t.Helper()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Errorf("Error reading request body: %v", err)
	}
	if got := string(b); got != want {
		t.Errorf("request Body is %v, want %v", got, want)
	}
}

func testJSONBody[T any](t *testing.T, r *http.Request, want T) {
	t.Helper()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Errorf("Error reading request body: %v", err)
	}

	var got T

	if err := json.Unmarshal(b, &got); err != nil {
		t.Errorf("Error unmarshaling request body JSON: %v", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("request JSON body mismatch (-want +got):\n%v", diff)
	}
}

// testJSONMarshal tests both JSON marshaling and unmarshaling of a value by comparing
// the marshaled output with the expected JSON string.
//
// This is the recommended function for most use cases.
// It performs a round-trip test that ensures both marshaling (Go value to JSON)
// and unmarshaling (JSON to Go value) work correctly and produce semantically equivalent results.
func testJSONMarshal[T any](t *testing.T, v T, want string, opts ...cmp.Option) {
	t.Helper()

	testJSONMarshalOnly(t, v, want)
	testJSONUnmarshalOnly(t, v, want, opts...)
}

// testJSONMarshalOnly tests JSON marshaling by comparing the marshaled output with the expected JSON string.
//
// This function compares JSON by unmarshaling both values into any and using cmp.Diff.
// This means the comparison ignores:
//   - White space differences
//   - Key ordering in objects
//   - Numeric type differences (e.g., int vs float with same value)
//
// In most cases, use testJSONMarshal instead.
// Only use this function in rare cases where you need to test marshaling behavior in isolation.
func testJSONMarshalOnly[T any](t *testing.T, v T, want string) {
	t.Helper()

	got, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Unable to marshal got JSON for %#v: %v", v, err)
	}

	// Unmarshal both the marshaled output and expected JSON into any
	// to enable semantic comparison that ignores formatting differences
	var gotAny any
	if err := json.Unmarshal(got, &gotAny); err != nil {
		t.Fatalf("Unable to unmarshal got JSON %v: %v", got, err)
	}

	var wantAny any
	if err := json.Unmarshal([]byte(want), &wantAny); err != nil {
		t.Fatalf("Unable to unmarshal want JSON %v: %v", want, err)
	}

	// Compare the semantic content
	if diff := cmp.Diff(wantAny, gotAny); diff != "" {
		t.Errorf("json.Marshal returned:\n%v\nwant:\n%v\ndiff:\n%v", got, want, diff)
	}
}

// testJSONUnmarshalOnly tests JSON unmarshaling by parsing the JSON string
// and comparing the result with the expected value.
//
// In most cases, use testJSONMarshal instead.
// Only use this function in rare cases where you need to test unmarshaling behavior in isolation.
func testJSONUnmarshalOnly[T any](t *testing.T, want T, v string, opts ...cmp.Option) {
	t.Helper()

	var got T
	if err := json.Unmarshal([]byte(v), &got); err != nil {
		t.Fatalf("Unable to unmarshal JSON %v: %v", v, err)
	}

	if diff := cmp.Diff(want, got, opts...); diff != "" {
		t.Errorf("json.Unmarshal returned:\n%#v\nwant:\n%#v\ndiff:\n%v", got, want, diff)
	}
}

// Test how bad options are handled. Method f under test should
// return an error.
func testBadOptions(t *testing.T, methodName string, f func() error) {
	t.Helper()
	if methodName == "" {
		t.Error("testBadOptions: must supply method methodName")
	}
	if err := f(); err == nil {
		t.Errorf("bad options %v err = nil, want error", methodName)
	}
}

// Test function under NewRequest failure and then s.client.Do failure.
// Method f should be a regular call that would normally succeed, but
// should return an error when NewRequest or s.client.Do fails.
func testNewRequestAndDoFailure(t *testing.T, methodName string, client *Client, f func() (*Response, error)) {
	testNewRequestAndDoFailureCategory(t, methodName, client, CoreCategory, f)
}

// testNewRequestAndDoFailureCategory works Like testNewRequestAndDoFailure, but allows setting the category.
func testNewRequestAndDoFailureCategory(t *testing.T, methodName string, client *Client, category RateLimitCategory, f func() (*Response, error)) {
	t.Helper()
	if methodName == "" {
		t.Error("testNewRequestAndDoFailure: must supply method methodName")
	}

	client.baseURL.Path = ""
	resp, err := f()
	if resp != nil {
		t.Errorf("client.BaseURL.Path='' %v resp = %#v, want nil", methodName, resp)
	}
	if err == nil {
		t.Errorf("client.BaseURL.Path='' %v err = nil, want error", methodName)
	}

	client.baseURL.Path = "/api-v3/"
	client.rateLimits[category].Reset.Time = time.Now().Add(10 * time.Minute)
	resp, err = f()
	if client.disableRateLimitCheck {
		return
	}
	if bypass := resp.Request.Context().Value(BypassRateLimitCheck); bypass != nil {
		return
	}
	if want := http.StatusForbidden; resp == nil || resp.Response.StatusCode != want {
		if resp != nil {
			t.Errorf("rate.Reset.Time > now %v resp = %#v, want StatusCode=%v", methodName, resp.Response, want)
		} else {
			t.Errorf("rate.Reset.Time > now %v resp = nil, want StatusCode=%v", methodName, want)
		}
	}
	if err == nil {
		t.Errorf("rate.Reset.Time > now %v err = nil, want error", methodName)
	}
}

// Test that all error response types contain the status code.
func testErrorResponseForStatusCode(t *testing.T, code int) {
	t.Helper()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/hooks", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(code)
	})

	_, _, err := client.Repositories.ListHooks(t.Context(), "o", "r", nil)

	var abuseErr *AbuseRateLimitError
	switch {
	case errors.As(err, new(*ErrorResponse)):
	case errors.As(err, new(*RateLimitError)):
	case errors.As(err, &abuseErr):
		if code != abuseErr.Response.StatusCode {
			t.Error("Error response does not contain status code")
		}
	default:
		t.Error("Unknown error response type")
	}
}

func assertNoDiff(t *testing.T, want, got any) {
	t.Helper()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("diff mismatch (-want +got):\n%v", diff)
	}
}

func assertNilError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func assertWrite(t *testing.T, w io.Writer, data []byte) {
	t.Helper()
	_, err := w.Write(data)
	assertNilError(t, err)
}

func TestWithHTTPClient(t *testing.T) {
	t.Parallel()

	t.Run("nil_client", func(t *testing.T) {
		t.Parallel()

		opts := clientOptions{}
		err := WithHTTPClient(nil)(&opts)
		if err == nil || err.Error() != "http client must not be nil" {
			t.Errorf("WithHTTPClient errored: %v", err)
		}
	})

	t.Run("non_nil_client", func(t *testing.T) {
		t.Parallel()

		customClient := &http.Client{Timeout: 10 * time.Second}
		opts := clientOptions{}
		err := WithHTTPClient(customClient)(&opts)
		if err != nil {
			t.Fatalf("WithHTTPClient errored: %v", err)
		}

		if opts.httpClient == nil {
			t.Fatal("httpClient is nil")
		}

		if opts.httpClient == customClient {
			t.Error("httpClient should be a shallow copy of the provided client, but is the same instance")
		}

		if opts.httpClient.Timeout != customClient.Timeout {
			t.Errorf("httpClient Timeout = %v, want %v", opts.httpClient.Timeout, customClient.Timeout)
		}
	})
}

func TestWithTransport(t *testing.T) {
	t.Parallel()

	t.Run("nil_transport", func(t *testing.T) {
		t.Parallel()

		opts := clientOptions{}
		err := WithTransport(nil)(&opts)
		if err == nil || err.Error() != "transport must not be nil" {
			t.Errorf("WithTransport errored: %v", err)
		}
	})

	t.Run("non_nil_transport", func(t *testing.T) {
		t.Parallel()

		customTransport := &http.Transport{IdleConnTimeout: 10 * time.Second}
		opts := clientOptions{}
		err := WithTransport(customTransport)(&opts)
		if err != nil {
			t.Fatalf("WithTransport errored: %v", err)
		}

		if opts.transport == nil {
			t.Fatal("transport is nil")
		}

		if opts.transport != customTransport {
			t.Errorf("transport = %v, want %v", opts.transport, customTransport)
		}
	})
}

func TestWithTimeout(t *testing.T) {
	t.Parallel()

	t.Run("negative_timeout", func(t *testing.T) {
		t.Parallel()

		opts := clientOptions{}
		err := WithTimeout(-1)(&opts)
		if err == nil || err.Error() != "timeout must not be negative" {
			t.Errorf("WithTimeout errored: %v", err)
		}
	})

	t.Run("zero_timeout", func(t *testing.T) {
		t.Parallel()

		opts := clientOptions{}
		err := WithTimeout(0)(&opts)
		if err != nil {
			t.Fatalf("WithTimeout errored: %v", err)
		}

		if opts.timeout == nil || *opts.timeout != 0 {
			t.Errorf("timeout = %v, want 0", opts.timeout)
		}
	})

	t.Run("valid_timeout", func(t *testing.T) {
		t.Parallel()

		timeout := 10 * time.Second
		opts := clientOptions{}
		err := WithTimeout(timeout)(&opts)
		if err != nil {
			t.Fatalf("WithTimeout errored: %v", err)
		}

		if opts.timeout == nil || *opts.timeout != timeout {
			t.Errorf("timeout = %v, want %v", opts.timeout, timeout)
		}
	})
}

func TestWithUserAgent(t *testing.T) {
	t.Parallel()

	t.Run("empty_user_agent", func(t *testing.T) {
		t.Parallel()

		opts := clientOptions{}
		err := WithUserAgent("")(&opts)
		if err != nil {
			t.Fatalf("WithUserAgent errored: %v", err)
		}

		if *opts.userAgent != "" {
			t.Errorf("userAgent = %v, want empty string", opts.userAgent)
		}
	})

	t.Run("custom_user_agent", func(t *testing.T) {
		t.Parallel()

		customUserAgent := "MyCustomUserAgent/1.0"
		opts := clientOptions{}
		err := WithUserAgent(customUserAgent)(&opts)
		if err != nil {
			t.Fatalf("WithUserAgent errored: %v", err)
		}

		if opts.userAgent == nil || *opts.userAgent != customUserAgent {
			t.Errorf("userAgent = %v, want %v", opts.userAgent, customUserAgent)
		}
	})
}

func TestWithEnvProxy(t *testing.T) {
	t.Parallel()

	opts := clientOptions{}
	err := WithEnvProxy()(&opts)
	if err != nil {
		t.Fatalf("WithEnvProxy errored: %v", err)
	}

	if !opts.envProxy {
		t.Error("envProxy is false, want true")
	}
}

func TestWithAuthToken(t *testing.T) {
	t.Parallel()

	t.Run("empty_token", func(t *testing.T) {
		t.Parallel()

		opts := clientOptions{}
		err := WithAuthToken("")(&opts)
		if err == nil || err.Error() != "token must not be empty" {
			t.Error("expected error for empty token, got nil")
		}
	})

	t.Run("valid_token", func(t *testing.T) {
		t.Parallel()

		validToken := "ghp_exampletoken1234567890"
		opts := clientOptions{}
		err := WithAuthToken(validToken)(&opts)
		if err != nil {
			t.Fatalf("WithAuthToken errored: %v", err)
		}

		if opts.token == nil || *opts.token != validToken {
			t.Errorf("token = %v, want %v", opts.token, validToken)
		}
	})
}

func TestWithURLs(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name          string
		baseURL       *string
		wantBaseURL   *string
		uploadURL     *string
		wantUploadURL *string
		wantErr       string
	}{
		{
			name:          "does_not_modify_urls_with_trailing_slash",
			baseURL:       Ptr("https://example.com/"),
			wantBaseURL:   Ptr("https://example.com/"),
			uploadURL:     Ptr("https://upload.example.com/"),
			wantUploadURL: Ptr("https://upload.example.com/"),
		},
		{
			name:          "adds_trailing_slash",
			baseURL:       Ptr("https://example.com"),
			wantBaseURL:   Ptr("https://example.com/"),
			uploadURL:     Ptr("https://upload.example.com"),
			wantUploadURL: Ptr("https://upload.example.com/"),
		},
		{
			name: "skips_unset",
		},
		{
			name:    "error_on_empty_base_url",
			baseURL: Ptr(""),
			wantErr: "invalid base url: url cannot be empty",
		},
		{
			name:    "error_on_bad_base_url",
			baseURL: Ptr("bogus\nbase\nURL"),
			wantErr: "invalid base url: invalid url",
		},
		{
			name:      "error_on_empty_upload_url",
			baseURL:   Ptr("https://example.com/"),
			uploadURL: Ptr(""),
			wantErr:   "invalid upload url: url cannot be empty",
		},
		{
			name:      "error_on_bad_upload_url",
			baseURL:   Ptr("https://example.com/"),
			uploadURL: Ptr("bogus\nupload\nURL"),
			wantErr:   "invalid upload url: invalid url",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := clientOptions{}
			err := WithURLs(tt.baseURL, tt.uploadURL)(&opts)
			if err != nil {
				if tt.wantErr == "" {
					t.Fatalf("unexpected error: %v", err)
				}

				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error to contain %v, got %v", tt.wantErr, err)
				}

				return
			}

			if tt.wantErr != "" {
				t.Fatalf("expected error to contain %v, got nil", tt.wantErr)
				return
			}

			if (opts.baseURL != nil) != (tt.wantBaseURL != nil) {
				t.Errorf("BaseURL set = %v, want %v", opts.baseURL != nil, tt.wantBaseURL != nil)
			}
			if opts.baseURL != nil && opts.baseURL.String() != *tt.wantBaseURL {
				t.Errorf("BaseURL is %v, want %v", opts.baseURL.String(), *tt.wantBaseURL)
			}

			if (opts.uploadURL != nil) != (tt.wantUploadURL != nil) {
				t.Errorf("UploadURL set = %v, want %v", opts.uploadURL != nil, tt.wantUploadURL != nil)
			}
			if opts.uploadURL != nil && opts.uploadURL.String() != *tt.wantUploadURL {
				t.Errorf("UploadURL is %v, want %v", opts.uploadURL.String(), *tt.wantUploadURL)
			}
		})
	}
}

func TestWithEnterpriseURLs(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name          string
		baseURL       string
		wantBaseURL   string
		uploadURL     string
		wantUploadURL string
		wantErr       string
	}{
		{
			name:          "does_not_modify_properly_formed_urls",
			baseURL:       "https://custom-url/api/v3/",
			wantBaseURL:   "https://custom-url/api/v3/",
			uploadURL:     "https://custom-upload-url/api/uploads/",
			wantUploadURL: "https://custom-upload-url/api/uploads/",
		},
		{
			name:          "adds_trailing_slash",
			baseURL:       "https://custom-url/api/v3",
			wantBaseURL:   "https://custom-url/api/v3/",
			uploadURL:     "https://custom-upload-url/api/uploads",
			wantUploadURL: "https://custom-upload-url/api/uploads/",
		},
		{
			name:          "adds_enterprise_suffix",
			baseURL:       "https://custom-url/",
			wantBaseURL:   "https://custom-url/api/v3/",
			uploadURL:     "https://custom-upload-url/",
			wantUploadURL: "https://custom-upload-url/api/uploads/",
		},
		{
			name:          "adds_enterprise_suffix_and_trailing_slash",
			baseURL:       "https://custom-url",
			wantBaseURL:   "https://custom-url/api/v3/",
			uploadURL:     "https://custom-upload-url",
			wantUploadURL: "https://custom-upload-url/api/uploads/",
		},
		{
			name:          "url_has_existing_api_prefix_adds_trailing_slash",
			baseURL:       "https://api.custom-url",
			wantBaseURL:   "https://api.custom-url/",
			uploadURL:     "https://api.custom-upload-url",
			wantUploadURL: "https://api.custom-upload-url/",
		},
		{
			name:          "url_has_existing_api_prefix_and_trailing_slash",
			baseURL:       "https://api.custom-url/",
			wantBaseURL:   "https://api.custom-url/",
			uploadURL:     "https://api.custom-upload-url/",
			wantUploadURL: "https://api.custom-upload-url/",
		},
		{
			name:          "url_has_api_subdomain_adds_trailing_slash",
			baseURL:       "https://catalog.api.custom-url",
			wantBaseURL:   "https://catalog.api.custom-url/",
			uploadURL:     "https://catalog.api.custom-upload-url",
			wantUploadURL: "https://catalog.api.custom-upload-url/",
		},
		{
			name:          "url_has_api_subdomain_and_trailing_slash",
			baseURL:       "https://catalog.api.custom-url/",
			wantBaseURL:   "https://catalog.api.custom-url/",
			uploadURL:     "https://catalog.api.custom-upload-url/",
			wantUploadURL: "https://catalog.api.custom-upload-url/",
		},
		{
			name:          "url_is_not_a_proper_api_subdomain_adds_enterprise_suffix_and_trailing_slash",
			baseURL:       "https://cloud-api.custom-url",
			wantBaseURL:   "https://cloud-api.custom-url/api/v3/",
			uploadURL:     "https://cloud-api.custom-upload-url",
			wantUploadURL: "https://cloud-api.custom-upload-url/api/uploads/",
		},
		{
			name:          "url_is_not_a_proper_api_subdomain_adds_enterprise_suffix",
			baseURL:       "https://cloud-api.custom-url/",
			wantBaseURL:   "https://cloud-api.custom-url/api/v3/",
			uploadURL:     "https://cloud-api.custom-upload-url/",
			wantUploadURL: "https://cloud-api.custom-upload-url/api/uploads/",
		},
		{
			name:          "url_has_uploads_subdomain_does_not_modify",
			baseURL:       "https://api.custom-url/",
			wantBaseURL:   "https://api.custom-url/",
			uploadURL:     "https://uploads.custom-upload-url/",
			wantUploadURL: "https://uploads.custom-upload-url/",
		},
		{
			name:      "empty_base_url",
			baseURL:   "",
			uploadURL: "https://custom-upload-url/api/uploads/",
			wantErr:   "invalid base url: url cannot be empty",
		},
		{
			name:      "invalid_base_url",
			baseURL:   "bogus\nbase\nURL",
			uploadURL: "https://custom-upload-url/api/uploads/",
			wantErr:   `invalid base url: invalid url`,
		},
		{
			name:      "empty_upload_url",
			baseURL:   "https://custom-url/api/v3/",
			uploadURL: "",
			wantErr:   "invalid upload url: url cannot be empty",
		},
		{
			name:      "invalid_upload_url",
			baseURL:   "https://custom-url/api/v3/",
			uploadURL: "bogus\nupload\nURL",
			wantErr:   `invalid upload url: invalid url`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := clientOptions{}
			err := WithEnterpriseURLs(tt.baseURL, tt.uploadURL)(&opts)
			if err != nil {
				if tt.wantErr == "" {
					t.Fatalf("WithEnterpriseURLs returned unexpected error: %v", err)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error does not contain expected string %q: %v", tt.wantErr, err)
				}
				return
			}
			if tt.wantErr != "" {
				t.Fatalf("WithEnterpriseURLs did not return expected error containing %q", tt.wantErr)
			}

			if opts.baseURL.String() != tt.wantBaseURL {
				t.Errorf("BaseURL is %v, want %v", opts.baseURL, tt.wantBaseURL)
			}
			if opts.uploadURL.String() != tt.wantUploadURL {
				t.Errorf("UploadURL is %v, want %v", opts.uploadURL, tt.wantUploadURL)
			}
		})
	}
}

func TestWithDisableRateLimitCheck(t *testing.T) {
	t.Parallel()

	opts := clientOptions{}
	err := WithDisableRateLimitCheck()(&opts)
	if err != nil {
		t.Fatalf("WithDisableRateLimitCheck errored: %v", err)
	}

	if !opts.disableRateLimitCheck {
		t.Error("disableRateLimitCheck is false, want true")
	}
}

func TestWithRateLimitRedirectionalEndpoints(t *testing.T) {
	t.Parallel()

	opts := clientOptions{}
	err := WithRateLimitRedirectionalEndpoints()(&opts)
	if err != nil {
		t.Fatalf("WithRateLimitRedirectionalEndpoints errored: %v", err)
	}

	if !opts.rateLimitRedirectionalEndpoints {
		t.Error("rateLimitRedirectionalEndpoints is false, want true")
	}
}

func TestWithMaxSecondaryRateLimitRetryAfterDuration(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		duration time.Duration
	}{
		{
			name:     "duration_zero",
			duration: 0,
		},
		{
			name:     "duration_set",
			duration: time.Minute,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opts := clientOptions{}
			err := WithMaxSecondaryRateLimitRetryAfterDuration(tt.duration)(&opts)
			if err != nil {
				t.Fatalf("WithMaxSecondaryRateLimitRetryAfterDuration errored: %v", err)
			}
			if *opts.maxSecondaryRateLimitRetryAfterDuration != tt.duration {
				t.Errorf("maxSecondaryRateLimitRetryAfterDuration is %v, want %v", *opts.maxSecondaryRateLimitRetryAfterDuration, tt.duration)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		opts    []ClientOptionsFunc
		wantErr string
	}{
		{
			name:    "no_options",
			opts:    []ClientOptionsFunc{},
			wantErr: "",
		},
		{
			name: "with_options",
			opts: []ClientOptionsFunc{
				WithHTTPClient(&http.Client{Timeout: 10 * time.Second}),
			},
			wantErr: "",
		},
		{
			name: "with_bad_options",
			opts: []ClientOptionsFunc{
				func(_ *clientOptions) error {
					return errors.New("bad option error")
				},
			},
			wantErr: "bad option error",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := NewClient(tt.opts...)
			if err != nil {
				if tt.wantErr == "" {
					t.Fatalf("NewClient returned unexpected error: %v", err)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error does not contain expected string %q: %v", tt.wantErr, err)
				}
				return
			}
			if tt.wantErr != "" {
				t.Fatalf("NewClient did not return expected error containing %q", tt.wantErr)
			}

			if c.client == nil {
				t.Error("NewClient client is not initialized")
			}
		})
	}
}

func Test_newClient(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		opts    clientOptions
		wantErr string
	}{
		{
			name:    "default_options",
			opts:    clientOptions{},
			wantErr: "",
		},
		{
			name: "with_http_client",
			opts: clientOptions{
				httpClient: &http.Client{Transport: &http.Transport{IdleConnTimeout: 5 * time.Second}},
			},
			wantErr: "",
		},
		{
			name: "with_transport",
			opts: clientOptions{
				transport: &http.Transport{IdleConnTimeout: 10 * time.Second},
			},
			wantErr: "",
		},
		{
			name: "with_all_options",
			opts: clientOptions{
				httpClient:                              &http.Client{Transport: &http.Transport{IdleConnTimeout: 5 * time.Second}},
				transport:                               &http.Transport{IdleConnTimeout: 10 * time.Second},
				timeout:                                 Ptr(15 * time.Second),
				apiVersionMin:                           Ptr(api20221128),
				apiVersionMax:                           Ptr(api20221128),
				userAgent:                               Ptr("CustomUserAgent/1.0"),
				baseURL:                                 mustParseURL(t, "https://custom-url/api/v3/"),
				uploadURL:                               mustParseURL(t, "https://custom-upload-url/api/uploads/"),
				disableRateLimitCheck:                   true,
				rateLimitRedirectionalEndpoints:         true,
				maxSecondaryRateLimitRetryAfterDuration: Ptr(2 * time.Minute),
			},
			wantErr: "",
		},
		{
			name: "with_rate_limit_options",
			opts: clientOptions{
				disableRateLimitCheck:                   false,
				rateLimitRedirectionalEndpoints:         true,
				maxSecondaryRateLimitRetryAfterDuration: Ptr(2 * time.Minute),
			},
			wantErr: "",
		},
		{
			name: "with_env_proxy",
			opts: clientOptions{
				envProxy: true,
			},
			wantErr: "",
		},
		{
			name: "with_incompatible_transport_for_env_proxy",
			opts: clientOptions{
				transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
					return nil, nil
				}),
				envProxy: true,
			},
			wantErr: "cannot set environment proxy on non-http transport",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := newClient(tt.opts)
			if err != nil {
				if tt.wantErr == "" {
					t.Fatalf("newClient returned unexpected error: %v", err)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error does not contain expected string %q: %v", tt.wantErr, err)
				}
				return
			}
			if tt.wantErr != "" {
				t.Fatalf("newClient did not return expected error containing %q", tt.wantErr)
			}

			if c.client == nil {
				t.Error("newClient http.Client is not initialized")
			}
			if tt.opts.httpClient != nil && c.client != tt.opts.httpClient {
				t.Error("newClient http.Client should be the same instance as the provided httpClient option")
			}
			if tt.opts.transport != nil && !tt.opts.envProxy && tt.opts.token == nil && c.client.Transport != tt.opts.transport {
				t.Error("newClient http.Client.Transport should be the same instance as the provided transport option")
			}
			if tt.opts.timeout != nil && c.client.Timeout != *tt.opts.timeout {
				t.Errorf("newClient http.Client.Timeout is %v, want %v", c.client.Timeout, *tt.opts.timeout)
			}

			if c.clientIgnoreRedirects == nil {
				t.Error("newClient http.Client used for redirects is not initialized")
			}
			if c.clientIgnoreRedirects.Transport != c.client.Transport {
				t.Error("newClient http.Client and http.Client used for redirects should share the same Transport instance")
			}
			if c.clientIgnoreRedirects.Timeout != c.client.Timeout {
				t.Errorf("newClient http.Client and http.Client used for redirects should have the same Timeout, got %v and %v", c.client.Timeout, c.clientIgnoreRedirects.Timeout)
			}
			if c.clientIgnoreRedirects.Jar != c.client.Jar {
				t.Error("newClient http.Client and http.Client used for redirects should share the same Jar instance")
			}
			if c.clientIgnoreRedirects.CheckRedirect == nil {
				t.Error("newClient http.Client used for redirects should have a CheckRedirect function")
			}

			if tt.opts.apiVersionMin != nil && c.apiVersionMin != *tt.opts.apiVersionMin {
				t.Errorf("newClient apiVersionMin is %v, want %v", c.apiVersionMin, *tt.opts.apiVersionMin)
			}
			if tt.opts.apiVersionMin == nil && c.apiVersionMin != api20221128 {
				t.Errorf("newClient apiVersionMin is %v, want %v", c.apiVersionMin, api20221128)
			}

			if tt.opts.apiVersionMax != nil && c.apiVersionMax != *tt.opts.apiVersionMax {
				t.Errorf("newClient apiVersionMax is %v, want %v", c.apiVersionMax, *tt.opts.apiVersionMax)
			}
			if tt.opts.apiVersionMax == nil && c.apiVersionMax != api20260310 {
				t.Errorf("newClient apiVersionMax is %v, want %v", c.apiVersionMax, api20260310)
			}

			if tt.opts.userAgent != nil && c.userAgent != *tt.opts.userAgent {
				t.Errorf("newClient userAgent is %v, want %v", c.userAgent, *tt.opts.userAgent)
			}
			if tt.opts.userAgent == nil && c.userAgent != defaultUserAgent {
				t.Errorf("newClient userAgent is %v, want %v", c.userAgent, defaultUserAgent)
			}

			if tt.opts.baseURL != nil && c.baseURL.String() != tt.opts.baseURL.String() {
				t.Errorf("newClient baseURL is %v, want %v", c.baseURL.String(), tt.opts.baseURL.String())
			}
			if tt.opts.baseURL == nil && c.baseURL.String() != defaultBaseURL {
				t.Errorf("newClient baseURL is %v, want %v", c.baseURL.String(), defaultBaseURL)
			}

			if tt.opts.uploadURL != nil && c.uploadURL.String() != tt.opts.uploadURL.String() {
				t.Errorf("newClient uploadURL is %v, want %v", c.uploadURL.String(), tt.opts.uploadURL.String())
			}
			if tt.opts.uploadURL == nil && c.uploadURL.String() != uploadBaseURL {
				t.Errorf("newClient uploadURL is %v, want %v", c.uploadURL.String(), uploadBaseURL)
			}

			if c.disableRateLimitCheck != tt.opts.disableRateLimitCheck {
				t.Errorf("newClient disableRateLimitCheck is %v, want %v", c.disableRateLimitCheck, tt.opts.disableRateLimitCheck)
			}
			if tt.opts.disableRateLimitCheck && (c.rateLimitRedirectionalEndpoints || c.maxSecondaryRateLimitRetryAfterDuration != 0) {
				t.Error("newClient should not set rate limit options when disableRateLimitCheck is true")
			}

			if !tt.opts.disableRateLimitCheck && c.rateLimitRedirectionalEndpoints != tt.opts.rateLimitRedirectionalEndpoints {
				t.Errorf("newClient rateLimitRedirectionalEndpoints is %v, want %v", c.rateLimitRedirectionalEndpoints, tt.opts.rateLimitRedirectionalEndpoints)
			}
			if !tt.opts.disableRateLimitCheck && tt.opts.maxSecondaryRateLimitRetryAfterDuration != nil && c.maxSecondaryRateLimitRetryAfterDuration != *tt.opts.maxSecondaryRateLimitRetryAfterDuration {
				t.Errorf("newClient maxSecondaryRateLimitRetryAfterDuration is %v, want %v", c.maxSecondaryRateLimitRetryAfterDuration, *tt.opts.maxSecondaryRateLimitRetryAfterDuration)
			}

			if c.common.client != c {
				t.Error("newClient common.client is not initialized or does not point to the client")
			}
			if c.Actions == nil || c.Activity == nil || c.Admin == nil || c.Apps == nil || c.Authorizations == nil || c.Billing == nil || c.Checks == nil || c.Classroom == nil || c.CodeScanning == nil || c.CodesOfConduct == nil || c.Codespaces == nil || c.Copilot == nil || c.Credentials == nil || c.Dependabot == nil || c.DependencyGraph == nil || c.Emojis == nil || c.Enterprise == nil || c.Gists == nil || c.Git == nil || c.Gitignores == nil || c.Interactions == nil || c.IssueImport == nil || c.Issues == nil || c.Licenses == nil || c.Markdown == nil || c.Marketplace == nil || c.Meta == nil || c.Migrations == nil || c.Organizations == nil || c.PrivateRegistries == nil || c.Projects == nil || c.PullRequests == nil || c.RateLimit == nil || c.Reactions == nil || c.Repositories == nil || c.SCIM == nil || c.Search == nil || c.SecretScanning == nil || c.SecurityAdvisories == nil || c.SubIssue == nil || c.Teams == nil || c.Users == nil {
				t.Error("newClient service fields are not all initialized")
			}

			if c.Marketplace.Stubbed != tt.opts.marketplaceStubbed {
				t.Errorf("newClient marketplaceStubbed is %v, want %v", c.Marketplace.Stubbed, tt.opts.marketplaceStubbed)
			}
		})
	}
}

func TestClient_UserAgent(t *testing.T) {
	t.Parallel()

	c := mustNewClient(t)

	if got, want := c.UserAgent(), defaultUserAgent; got != want {
		t.Errorf("Client.UserAgent() = %v, want %v", got, want)
	}

	customUserAgent := "CustomUserAgent/1.0"
	c.userAgent = customUserAgent

	if got, want := c.UserAgent(), customUserAgent; got != want {
		t.Errorf("Client.UserAgent() = %v, want %v", got, want)
	}
}

func TestClient_BaseURL(t *testing.T) {
	t.Parallel()

	customBaseURL := "https://custom-url/api/v3/"

	for _, tt := range []struct {
		name        string
		client      *Client
		wantBaseURL string
	}{
		{
			name:        "default_base_url",
			client:      mustNewClient(t),
			wantBaseURL: defaultBaseURL,
		},
		{
			name:        "custom_base_url",
			client:      &Client{baseURL: mustParseURL(t, customBaseURL)},
			wantBaseURL: customBaseURL,
		},
		{
			name:        "missing_base_url",
			client:      &Client{},
			wantBaseURL: "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got, want := tt.client.BaseURL(), tt.wantBaseURL; got != want {
				t.Errorf("Client.BaseURL() = %v, want %v", got, want)
			}
		})
	}
}

func TestClient_UploadURL(t *testing.T) {
	t.Parallel()

	customUploadURL := "https://custom-upload-url/api/uploads/"

	for _, tt := range []struct {
		name          string
		client        *Client
		wantUploadURL string
	}{
		{
			name:          "default_upload_url",
			client:        mustNewClient(t),
			wantUploadURL: uploadBaseURL,
		},
		{
			name:          "custom_upload_url",
			client:        &Client{uploadURL: mustParseURL(t, customUploadURL)},
			wantUploadURL: customUploadURL,
		},
		{
			name:          "missing_upload_url",
			client:        &Client{},
			wantUploadURL: "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got, want := tt.client.UploadURL(), tt.wantUploadURL; got != want {
				t.Errorf("Client.UploadURL() = %v, want %v", got, want)
			}
		})
	}
}

func TestClient_Clone(t *testing.T) {
	t.Parallel()

	t.Run("uninitialized_client", func(t *testing.T) {
		t.Parallel()

		var c Client

		_, err := c.Clone()
		if err == nil || !errors.Is(err, errUninitialized) {
			t.Fatalf("Client.Clone returned unexpected error: %v", err)
		}
	})

	t.Run("initialized_client_opts_err", func(t *testing.T) {
		t.Parallel()

		c := mustNewClient(t)

		_, err := c.Clone(func(_ *clientOptions) error {
			return errors.New("test options error")
		})
		if err == nil || err.Error() != "test options error" {
			t.Fatalf("Client.Clone returned unexpected error: %v", err)
		}
	})

	t.Run("initialized_client_new_client_err", func(t *testing.T) {
		t.Parallel()

		c := mustNewClient(t)

		_, err := c.Clone(WithTransport(roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
			return nil, nil
		})), WithEnvProxy())
		if err == nil || err.Error() != "cannot set environment proxy on non-http transport" {
			t.Fatalf("Client.Clone returned unexpected error: %v", err)
		}
	})

	t.Run("initialized_client", func(t *testing.T) {
		t.Parallel()

		c := mustNewClient(t)
		c.apiVersionMin = api20221128
		c.apiVersionMax = api20221128
		c.userAgent = "CustomUserAgent/1.0"
		c.baseURL.Path = "/custom/"
		c.uploadURL.Path = "/custom-upload/"
		c.disableRateLimitCheck = false
		c.rateLimitRedirectionalEndpoints = true
		c.maxSecondaryRateLimitRetryAfterDuration = 2 * time.Minute
		c.Marketplace.Stubbed = true
		c.client.Transport = &http.Transport{IdleConnTimeout: 10 * time.Second}
		c.client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return nil }
		c.client.Timeout = 15 * time.Second
		c.rateLimits[CoreCategory].Remaining = 100
		c.secondaryRateLimitReset = time.Now().Add(30 * time.Second)

		cloned, err := c.Clone()
		if err != nil {
			t.Fatalf("Client.Clone returned error: %v", err)
		}

		if cloned.client == c.client {
			t.Error("Cloned Client has same http.Client instance, but should be different")
		}
		if cloned.client.Transport != c.client.Transport {
			t.Error("Cloned Client http.Client.Transport is not the same instance as original")
		}
		if cloned.client.CheckRedirect == nil || fmt.Sprintf("%p", cloned.client.CheckRedirect) != fmt.Sprintf("%p", c.client.CheckRedirect) {
			t.Error("Cloned Client http.Client.CheckRedirect is not the same function instance as original")
		}
		if cloned.client.Jar != c.client.Jar {
			t.Error("Cloned Client http.Client.Jar is not the same instance as original")
		}
		if cloned.client.Timeout != c.client.Timeout {
			t.Errorf("Cloned Client http.Client.Timeout is %v, want %v", cloned.client.Timeout, c.client.Timeout)
		}
		if got, want := cloned.userAgent, c.userAgent; got != want {
			t.Errorf("Cloned Client userAgent is %v, want %v", got, want)
		}
		if got, want := cloned.baseURL.String(), c.baseURL.String(); got != want {
			t.Errorf("Cloned Client baseURL is %v, want %v", got, want)
		}
		if got, want := cloned.uploadURL.String(), c.uploadURL.String(); got != want {
			t.Errorf("Cloned Client uploadURL is %v, want %v", got, want)
		}
		if cloned.disableRateLimitCheck != c.disableRateLimitCheck {
			t.Errorf("Cloned Client disableRateLimitCheck is %v, want %v", cloned.disableRateLimitCheck, c.disableRateLimitCheck)
		}
		if cloned.rateLimitRedirectionalEndpoints != c.rateLimitRedirectionalEndpoints {
			t.Errorf("Cloned Client rateLimitRedirectionalEndpoints is %v, want %v", cloned.rateLimitRedirectionalEndpoints, c.rateLimitRedirectionalEndpoints)
		}
		if cloned.maxSecondaryRateLimitRetryAfterDuration != c.maxSecondaryRateLimitRetryAfterDuration {
			t.Errorf("Cloned Client maxSecondaryRateLimitRetryAfterDuration is %v, want %v", cloned.maxSecondaryRateLimitRetryAfterDuration, c.maxSecondaryRateLimitRetryAfterDuration)
		}
		if cloned.Marketplace.Stubbed != c.Marketplace.Stubbed {
			t.Errorf("Cloned Client Marketplace.Stubbed is %v, want %v", cloned.Marketplace.Stubbed, c.Marketplace.Stubbed)
		}
		if cloned.rateLimits[CoreCategory].Remaining != c.rateLimits[CoreCategory].Remaining {
			t.Errorf("Cloned Client rateLimits[CoreCategory].Remaining is %v, want %v", cloned.rateLimits[CoreCategory].Remaining, c.rateLimits[CoreCategory].Remaining)
		}
		if !cloned.secondaryRateLimitReset.Equal(c.secondaryRateLimitReset) {
			t.Errorf("Cloned Client secondaryRateLimitReset is %v, want %v", cloned.secondaryRateLimitReset, c.secondaryRateLimitReset)
		}
	})

	t.Run("initialized_client_no_rate_limit_check", func(t *testing.T) {
		t.Parallel()

		c := mustNewClient(t)
		c.userAgent = "CustomUserAgent/1.0"
		c.baseURL.Path = "/custom/"
		c.uploadURL.Path = "/custom-upload/"
		c.disableRateLimitCheck = true
		c.Marketplace.Stubbed = true
		c.client.Transport = &http.Transport{IdleConnTimeout: 10 * time.Second}
		c.client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return nil }
		c.client.Timeout = 15 * time.Second
		c.rateLimits[CoreCategory].Remaining = 100
		c.secondaryRateLimitReset = time.Now().Add(30 * time.Second)

		cloned, err := c.Clone()
		if err != nil {
			t.Fatalf("Client.Clone returned error: %v", err)
		}

		if cloned.client == c.client {
			t.Error("Cloned Client has same http.Client instance, but should be different")
		}
		if cloned.client.Transport != c.client.Transport {
			t.Error("Cloned Client http.Client.Transport is not the same instance as original")
		}
		if cloned.client.CheckRedirect == nil || fmt.Sprintf("%p", cloned.client.CheckRedirect) != fmt.Sprintf("%p", c.client.CheckRedirect) {
			t.Error("Cloned Client http.Client.CheckRedirect is not the same function instance as original")
		}
		if cloned.client.Jar != c.client.Jar {
			t.Error("Cloned Client http.Client.Jar is not the same instance as original")
		}
		if cloned.client.Timeout != c.client.Timeout {
			t.Errorf("Cloned Client http.Client.Timeout is %v, want %v", cloned.client.Timeout, c.client.Timeout)
		}
		if got, want := cloned.userAgent, c.userAgent; got != want {
			t.Errorf("Cloned Client userAgent is %v, want %v", got, want)
		}
		if got, want := cloned.baseURL.String(), c.baseURL.String(); got != want {
			t.Errorf("Cloned Client baseURL is %v, want %v", got, want)
		}
		if got, want := cloned.uploadURL.String(), c.uploadURL.String(); got != want {
			t.Errorf("Cloned Client uploadURL is %v, want %v", got, want)
		}
		if cloned.disableRateLimitCheck != c.disableRateLimitCheck {
			t.Errorf("Cloned Client disableRateLimitCheck is %v, want %v", cloned.disableRateLimitCheck, c.disableRateLimitCheck)
		}
		if cloned.Marketplace.Stubbed != c.Marketplace.Stubbed {
			t.Errorf("Cloned Client Marketplace.Stubbed is %v, want %v", cloned.Marketplace.Stubbed, c.Marketplace.Stubbed)
		}
	})

	t.Run("initialized_client_with_http_client", func(t *testing.T) {
		t.Parallel()

		c := mustNewClient(t)
		c.client.Transport = &http.Transport{IdleConnTimeout: 10 * time.Second}
		c.client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return nil }
		c.client.Timeout = 15 * time.Second

		h := &http.Client{
			Transport:     &http.Transport{IdleConnTimeout: 20 * time.Second},
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
			Timeout:       25 * time.Second,
		}

		cloned, err := c.Clone(WithHTTPClient(h))
		if err != nil {
			t.Fatalf("Client.Clone returned error: %v", err)
		}

		if cloned.client == h {
			t.Error("Cloned Client has same http.Client instance as provided in WithHTTPClient, but should be a different instance")
		}
		if cloned.client.Transport != h.Transport {
			t.Error("Cloned Client http.Client.Transport is not the same instance as original")
		}
		if cloned.client.CheckRedirect == nil || fmt.Sprintf("%p", cloned.client.CheckRedirect) != fmt.Sprintf("%p", h.CheckRedirect) {
			t.Error("Cloned Client http.Client.CheckRedirect is not the same function instance as original")
		}
		if cloned.client.Jar != h.Jar {
			t.Error("Cloned Client http.Client.Jar is not the same instance as original")
		}
		if cloned.client.Timeout != h.Timeout {
			t.Errorf("Cloned Client http.Client.Timeout is %v, want %v", cloned.client.Timeout, h.Timeout)
		}
	})

	t.Run("initialized_client_with_transport", func(t *testing.T) {
		t.Parallel()

		c := mustNewClient(t)
		c.client.Transport = &http.Transport{IdleConnTimeout: 10 * time.Second}
		c.client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return nil }
		c.client.Timeout = 15 * time.Second

		tr := &http.Transport{IdleConnTimeout: 30 * time.Second}

		cloned, err := c.Clone(WithTransport(tr))
		if err != nil {
			t.Fatalf("Client.Clone returned error: %v", err)
		}

		if cloned.client == c.client {
			t.Error("Cloned Client has same http.Client instance, but should be different")
		}
		if cloned.client.Transport != tr {
			t.Error("Cloned Client http.Client.Transport is not the same instance as original")
		}
		if cloned.client.CheckRedirect == nil || fmt.Sprintf("%p", cloned.client.CheckRedirect) != fmt.Sprintf("%p", c.client.CheckRedirect) {
			t.Error("Cloned Client http.Client.CheckRedirect is not the same function instance as original")
		}
		if cloned.client.Jar != c.client.Jar {
			t.Error("Cloned Client http.Client.Jar is not the same instance as original")
		}
		if cloned.client.Timeout != c.client.Timeout {
			t.Errorf("Cloned Client http.Client.Timeout is %v, want %v", cloned.client.Timeout, c.client.Timeout)
		}
	})

	t.Run("initialized_client_with_timeout", func(t *testing.T) {
		t.Parallel()

		c := mustNewClient(t)
		c.client.Transport = &http.Transport{IdleConnTimeout: 10 * time.Second}
		c.client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return nil }
		c.client.Timeout = 15 * time.Second

		timeout := 30 * time.Second

		cloned, err := c.Clone(WithTimeout(timeout))
		if err != nil {
			t.Fatalf("Client.Clone returned error: %v", err)
		}

		if cloned.client == c.client {
			t.Error("Cloned Client has same http.Client instance, but should be different")
		}
		if cloned.client.Transport != c.client.Transport {
			t.Error("Cloned Client http.Client.Transport is not the same instance as original")
		}
		if cloned.client.CheckRedirect == nil || fmt.Sprintf("%p", cloned.client.CheckRedirect) != fmt.Sprintf("%p", c.client.CheckRedirect) {
			t.Error("Cloned Client http.Client.CheckRedirect is not the same function instance as original")
		}
		if cloned.client.Jar != c.client.Jar {
			t.Error("Cloned Client http.Client.Jar is not the same instance as original")
		}
		if cloned.client.Timeout != timeout {
			t.Errorf("Cloned Client http.Client.Timeout is %v, want %v", cloned.client.Timeout, timeout)
		}
	})

	t.Run("initialized_client_with_http_client_transport_timeout", func(t *testing.T) {
		t.Parallel()

		c := mustNewClient(t)
		c.client.Transport = &http.Transport{IdleConnTimeout: 10 * time.Second}
		c.client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return nil }
		c.client.Timeout = 15 * time.Second

		h := &http.Client{
			Transport:     &http.Transport{IdleConnTimeout: 20 * time.Second},
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
			Timeout:       25 * time.Second,
		}

		tr := &http.Transport{IdleConnTimeout: 30 * time.Second}

		timeout := 45 * time.Second

		cloned, err := c.Clone(WithHTTPClient(h), WithTransport(tr), WithTimeout(timeout))
		if err != nil {
			t.Fatalf("Client.Clone returned error: %v", err)
		}

		if cloned.client == h {
			t.Error("Cloned Client has same http.Client instance as provided in WithHTTPClient, but should be a different instance")
		}
		if cloned.client.Transport != tr {
			t.Error("Cloned Client http.Client.Transport is not the same instance as original")
		}
		if cloned.client.CheckRedirect == nil || fmt.Sprintf("%p", cloned.client.CheckRedirect) != fmt.Sprintf("%p", h.CheckRedirect) {
			t.Error("Cloned Client http.Client.CheckRedirect is not the same function instance as original")
		}
		if cloned.client.Jar != h.Jar {
			t.Error("Cloned Client http.Client.Jar is not the same instance as original")
		}
		if cloned.client.Timeout != timeout {
			t.Errorf("Cloned Client http.Client.Timeout is %v, want %v", cloned.client.Timeout, timeout)
		}
	})
}

func TestClient_Client(t *testing.T) {
	t.Parallel()
	c, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	c2 := c.Client()
	if c.client == c2 {
		t.Error("Client returned same http.Client, but should be different")
	}
}

// Ensure that length of Client.rateLimits is the same as number of fields in RateLimits struct.
func TestClient_rateLimits(t *testing.T) {
	t.Parallel()
	if got, want := len(Client{}.rateLimits), reflect.TypeFor[RateLimits]().NumField(); got != want {
		t.Errorf("len(Client{}.rateLimits) is %v, want %v", got, want)
	}
}

func TestNewRequest(t *testing.T) {
	t.Parallel()
	c := mustNewClient(t)

	inURL, outURL := "/foo", defaultBaseURL+"foo"
	inBody, outBody := &User{Login: Ptr("l")}, `{"login":"l"}`+"\n"
	req, _ := c.NewRequest(t.Context(), "GET", inURL, inBody)

	// test that relative URL was expanded
	if got, want := req.URL.String(), outURL; got != want {
		t.Errorf("NewRequest(%q) URL is %v, want %v", inURL, got, want)
	}

	// test that body was JSON encoded
	body, _ := io.ReadAll(req.Body)
	if got, want := string(body), outBody; got != want {
		t.Errorf("NewRequest(%q) Body is %v, want %v", inBody, got, want)
	}

	userAgent := req.Header.Get("User-Agent")

	// test that default user-agent is attached to the request
	if got, want := userAgent, c.userAgent; got != want {
		t.Errorf("NewRequest() User-Agent is %v, want %v", got, want)
	}

	if !strings.Contains(userAgent, Version) {
		t.Errorf("NewRequest() User-Agent should contain %v, found %v", Version, userAgent)
	}

	apiVersion := req.Header.Get(headerAPIVersion)
	if got, want := apiVersion, api20221128; got != want {
		t.Errorf("NewRequest() %v header is %v, want %v", headerAPIVersion, got, want)
	}

	req, _ = c.NewRequest(t.Context(), "GET", inURL, inBody, WithVersion("2022-11-29"))
	apiVersion = req.Header.Get(headerAPIVersion)
	if got, want := apiVersion, "2022-11-29"; got != want {
		t.Errorf("NewRequest() %v header is %v, want %v", headerAPIVersion, got, want)
	}
}

func TestNewRequest_invalidJSON(t *testing.T) {
	t.Parallel()
	c := mustNewClient(t)

	type T struct {
		F func()
	}
	_, err := c.NewRequest(t.Context(), "GET", ".", &T{})

	if err == nil {
		t.Error("Expected error to be returned.")
	}
}

func TestNewRequest_badURL(t *testing.T) {
	t.Parallel()
	c := mustNewClient(t)
	_, err := c.NewRequest(t.Context(), "GET", ":", nil)
	testURLParseError(t, err)
}

func TestNewRequest_badMethod(t *testing.T) {
	t.Parallel()
	c := mustNewClient(t)
	if _, err := c.NewRequest(t.Context(), "BOGUS\nMETHOD", ".", nil); err == nil {
		t.Fatal("NewRequest returned nil; expected error")
	}
}

// ensure that no User-Agent header is set if the client's UserAgent is empty.
// This caused a problem with Google's internal http client.
func TestNewRequest_emptyUserAgent(t *testing.T) {
	t.Parallel()
	c := mustNewClient(t)
	c.userAgent = ""
	req, err := c.NewRequest(t.Context(), "GET", ".", nil)
	if err != nil {
		t.Fatalf("NewRequest returned unexpected error: %v", err)
	}
	if _, ok := req.Header["User-Agent"]; ok {
		t.Fatal("constructed request contains unexpected User-Agent header")
	}
}

// If a nil body is passed to github.NewRequest, make sure that nil is also
// passed to http.NewRequest. In most cases, passing an io.Reader that returns
// no content is fine, since there is no difference between an HTTP request
// body that is an empty string versus one that is not set at all. However in
// certain cases, intermediate systems may treat these differently resulting in
// subtle errors.
func TestNewRequest_emptyBody(t *testing.T) {
	t.Parallel()
	c := mustNewClient(t)
	req, err := c.NewRequest(t.Context(), "GET", ".", nil)
	if err != nil {
		t.Fatalf("NewRequest returned unexpected error: %v", err)
	}
	if req.Body != nil {
		t.Fatal("constructed request contains a non-nil Body")
	}
}

func TestNewRequest_errorForNoTrailingSlash(t *testing.T) {
	t.Parallel()
	tests := []struct {
		rawurl    string
		wantError bool
	}{
		{rawurl: "https://example.com/api/v3", wantError: true},
		{rawurl: "https://example.com/api/v3/", wantError: false},
	}
	c := mustNewClient(t)
	for _, test := range tests {
		u, err := url.Parse(test.rawurl)
		if err != nil {
			t.Fatalf("url.Parse returned unexpected error: %v", err)
		}
		c.baseURL = u
		if _, err := c.NewRequest(t.Context(), "GET", "test", nil); test.wantError && err == nil {
			t.Fatal("Expected error to be returned.")
		} else if !test.wantError && err != nil {
			t.Fatalf("NewRequest returned unexpected error: %v", err)
		}
	}
}

func TestCheckURLPathTraversal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input   string
		wantErr error
	}{
		{"repos/o/r/contents/file.txt", nil},
		{"repos/o/r/contents/dir/file.txt", nil},
		{"repos/o/r/contents/file..txt", nil},
		{"repos/o/r?q=a..b", nil},
		{"repos/../admin/users", ErrPathForbidden},
		{"repos/x/../../../admin", ErrPathForbidden},
		{"../admin", ErrPathForbidden},
		{"repos/o/r/contents/..", ErrPathForbidden},
		{"repos/o/r/contents/../secrets", ErrPathForbidden},
		// Full URLs with scheme.
		{"https://api.github.com/repos/../admin", ErrPathForbidden},
		{"https://api.github.com/repos/o/r/contents/file.txt", nil},
		{"https://api.github.com/repos/o/r/contents/file..txt", nil},
		// URL with fragment.
		{"repos/o/r/contents/file.txt#section", nil},
		{"repos/../admin#frag", ErrPathForbidden},
		// URL with userinfo.
		{"https://user:pass@api.github.com/repos/../admin", ErrPathForbidden},
		{"https://user:pass@api.github.com/repos/o/r", nil},
		// Percent-encoded dots (%2e%2e) — url.Parse decodes them to ".." in Path.
		{"repos/%2e%2e/admin", ErrPathForbidden},
		{"repos/%2E%2E/admin", ErrPathForbidden},
		{"repos/x/%2e%2e/%2e%2e/%2e%2e/admin", ErrPathForbidden},
		{"x/%2e%2e/%2e%2e/%2e%2e/admin/users", ErrPathForbidden},
		{"repos/o/r/contents/file%2e%2etxt", nil},
	}
	for _, tt := range tests {
		err := checkURLPathTraversal(tt.input)
		if !errors.Is(err, tt.wantErr) {
			t.Errorf("checkURLPathTraversal(%q) = %v, want %v", tt.input, err, tt.wantErr)
		}
	}
}

func TestNewRequest_pathTraversal(t *testing.T) {
	t.Parallel()
	c := mustNewClient(t)

	tests := []struct {
		urlStr    string
		wantError bool
	}{
		{"repos/o/r/readme", false},
		{"repos/o/r/contents/file..txt", false},
		{"repos/x/../../../admin/users", true},
		{"repos/../admin", true},
	}
	for _, tt := range tests {
		_, err := c.NewRequest(t.Context(), "GET", tt.urlStr, nil)
		if tt.wantError && !errors.Is(err, ErrPathForbidden) {
			t.Errorf("NewRequest(%q): want ErrPathForbidden, got %v", tt.urlStr, err)
		} else if !tt.wantError && err != nil {
			t.Errorf("NewRequest(%q): unexpected error: %v", tt.urlStr, err)
		}
	}
}

func TestNewFormRequest_pathTraversal(t *testing.T) {
	t.Parallel()
	c := mustNewClient(t)

	_, err := c.NewFormRequest(t.Context(), "repos/x/../../../admin", nil)
	if !errors.Is(err, ErrPathForbidden) {
		t.Fatalf("NewFormRequest with path traversal: want ErrPathForbidden, got %v", err)
	}
}

func TestNewUploadRequest_pathTraversal(t *testing.T) {
	t.Parallel()
	c := mustNewClient(t)

	_, err := c.NewUploadRequest(t.Context(), "repos/x/../../../admin", nil, 0, "")
	if !errors.Is(err, ErrPathForbidden) {
		t.Fatalf("NewUploadRequest with path traversal: want ErrPathForbidden, got %v", err)
	}
}

func TestNewUploadRequest_setsGetBodyForSeekableReader(t *testing.T) {
	t.Parallel()
	c := mustNewClient(t)

	const content = "upload content"
	file := openTestFile(t, "upload.txt", content)

	req, err := c.NewUploadRequest(t.Context(), "https://example.com/", file, int64(len(content)), "text/plain")
	if err != nil {
		t.Fatalf("NewUploadRequest returned unexpected error: %v", err)
	}

	// GetBody must be set for rewindable readers so HTTP/2 can retry the request
	// after a refused stream. See issue #2113.
	if req.GetBody == nil {
		t.Fatal("NewUploadRequest did not set req.GetBody for a seekable/ReaderAt reader")
	}

	// Each GetBody call must return an independent copy yielding the identical
	// body so retries send the same bytes.
	for i := range 2 {
		body, err := req.GetBody()
		if err != nil {
			t.Fatalf("GetBody call %v returned unexpected error: %v", i, err)
		}
		got, err := io.ReadAll(body)
		body.Close()
		if err != nil {
			t.Fatalf("reading GetBody result on call %v: %v", i, err)
		}
		if string(got) != content {
			t.Errorf("GetBody call %v body = %q, want %q", i, got, content)
		}
	}

	// Reading via GetBody must not have disturbed the original body's read
	// position: req.Body must still yield the full content.
	gotBody, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("reading req.Body: %v", err)
	}
	if string(gotBody) != content {
		t.Errorf("req.Body after GetBody calls = %q, want %q", gotBody, content)
	}
}

// seekerOnlyReader implements io.Reader and io.Seeker but deliberately not
// io.ReaderAt, so it cannot provide an independent body copy.
type seekerOnlyReader struct {
	r *strings.Reader
}

func (s *seekerOnlyReader) Read(p []byte) (int, error) { return s.r.Read(p) }

func (s *seekerOnlyReader) Seek(offset int64, whence int) (int64, error) {
	return s.r.Seek(offset, whence)
}

func TestNewUploadRequest_noGetBodyWithoutReaderAt(t *testing.T) {
	t.Parallel()
	c := mustNewClient(t)

	tests := []struct {
		name   string
		reader io.Reader
	}{
		// *bytes.Buffer is neither io.Seeker nor io.ReaderAt.
		{"non-seekable", bytes.NewBufferString("upload content")},
		// io.Seeker alone is insufficient: without io.ReaderAt we cannot return
		// an independent body copy, so GetBody must stay nil rather than risk
		// disturbing the original body's read position.
		{"seeker without ReaderAt", &seekerOnlyReader{strings.NewReader("upload content")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req, err := c.NewUploadRequest(t.Context(), "https://example.com/", tt.reader, 14, "text/plain")
			if err != nil {
				t.Fatalf("NewUploadRequest returned unexpected error: %v", err)
			}
			if req.GetBody != nil {
				t.Errorf("NewUploadRequest set req.GetBody for %v reader, want nil", tt.name)
			}
		})
	}
}

func TestNewUploadRequest_returnsErrorWhenSeekFails(t *testing.T) {
	t.Parallel()
	c := mustNewClient(t)

	const content = "upload content"
	file := openTestFile(t, "upload.txt", content)
	// Closing the file makes Seek fail, exercising the GetBody offset-probe
	// error path (issue #2113).
	if err := file.Close(); err != nil {
		t.Fatalf("closing test file: %v", err)
	}

	_, err := c.NewUploadRequest(t.Context(), "https://example.com/", file, int64(len(content)), "text/plain")
	if err == nil {
		t.Error("NewUploadRequest returned nil error when Seek failed, want error")
	}
}

func TestNewFormRequest(t *testing.T) {
	t.Parallel()
	c := mustNewClient(t)

	inURL, outURL := "/foo", defaultBaseURL+"foo"
	form := url.Values{}
	form.Add("login", "l")
	inBody, outBody := strings.NewReader(form.Encode()), "login=l"
	req, err := c.NewFormRequest(t.Context(), inURL, inBody)
	if err != nil {
		t.Fatalf("NewFormRequest returned unexpected error: %v", err)
	}

	// test that relative URL was expanded
	if got, want := req.URL.String(), outURL; got != want {
		t.Errorf("NewFormRequest(%q) URL is %v, want %v", inURL, got, want)
	}

	// test that body was form encoded
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("Error reading request body: %v", err)
	}
	if got, want := string(body), outBody; got != want {
		t.Errorf("NewFormRequest() Body is %v, want %v", got, want)
	}

	// test that default user-agent is attached to the request
	if got, want := req.Header.Get("User-Agent"), c.userAgent; got != want {
		t.Errorf("NewFormRequest() User-Agent is %v, want %v", got, want)
	}

	apiVersion := req.Header.Get(headerAPIVersion)
	if got, want := apiVersion, api20221128; got != want {
		t.Errorf("NewFormRequest() %v header is %v, want %v", headerAPIVersion, got, want)
	}

	req, err = c.NewFormRequest(t.Context(), inURL, inBody, WithVersion("2022-11-29"))
	if err != nil {
		t.Fatalf("NewFormRequest with WithVersion returned unexpected error: %v", err)
	}
	apiVersion = req.Header.Get(headerAPIVersion)
	if got, want := apiVersion, "2022-11-29"; got != want {
		t.Errorf("NewFormRequest() %v header is %v, want %v", headerAPIVersion, got, want)
	}
}

func TestNewFormRequest_badURL(t *testing.T) {
	t.Parallel()
	c := mustNewClient(t)
	_, err := c.NewFormRequest(t.Context(), ":", nil)
	testURLParseError(t, err)
}

func TestNewFormRequest_emptyUserAgent(t *testing.T) {
	t.Parallel()
	c := mustNewClient(t)
	c.userAgent = ""
	req, err := c.NewFormRequest(t.Context(), ".", nil)
	if err != nil {
		t.Fatalf("NewFormRequest returned unexpected error: %v", err)
	}
	if _, ok := req.Header["User-Agent"]; ok {
		t.Fatal("constructed request contains unexpected User-Agent header")
	}
}

func TestNewFormRequest_emptyBody(t *testing.T) {
	t.Parallel()
	c := mustNewClient(t)
	req, err := c.NewFormRequest(t.Context(), ".", nil)
	if err != nil {
		t.Fatalf("NewFormRequest returned unexpected error: %v", err)
	}
	if req.Body != nil {
		t.Fatal("constructed request contains a non-nil Body")
	}
}

func TestNewFormRequest_errorForNoTrailingSlash(t *testing.T) {
	t.Parallel()
	tests := []struct {
		rawURL    string
		wantError bool
	}{
		{rawURL: "https://example.com/api/v3", wantError: true},
		{rawURL: "https://example.com/api/v3/", wantError: false},
	}
	c := mustNewClient(t)
	for _, test := range tests {
		u, err := url.Parse(test.rawURL)
		if err != nil {
			t.Fatalf("url.Parse returned unexpected error: %v", err)
		}
		c.baseURL = u
		if _, err := c.NewFormRequest(t.Context(), "test", nil); test.wantError && err == nil {
			t.Fatal("Expected error to be returned.")
		} else if !test.wantError && err != nil {
			t.Fatalf("NewFormRequest returned unexpected error: %v", err)
		}
	}
}

func TestNewUploadRequest_WithVersion(t *testing.T) {
	t.Parallel()
	c := mustNewClient(t)
	req, _ := c.NewUploadRequest(t.Context(), "https://example.com/", nil, 0, "")

	apiVersion := req.Header.Get(headerAPIVersion)
	if got, want := apiVersion, api20221128; got != want {
		t.Errorf("NewRequest() %v header is %v, want %v", headerAPIVersion, got, want)
	}

	req, _ = c.NewUploadRequest(t.Context(), "https://example.com/", nil, 0, "", WithVersion("2022-11-29"))
	apiVersion = req.Header.Get(headerAPIVersion)
	if got, want := apiVersion, "2022-11-29"; got != want {
		t.Errorf("NewRequest() %v header is %v, want %v", headerAPIVersion, got, want)
	}
}

func TestNewUploadRequest_badURL(t *testing.T) {
	t.Parallel()
	c := mustNewClient(t)
	_, err := c.NewUploadRequest(t.Context(), ":", nil, 0, "")
	testURLParseError(t, err)

	const methodName = "NewUploadRequest"
	testBadOptions(t, methodName, func() (err error) {
		_, err = c.NewUploadRequest(t.Context(), "\n", nil, -1, "\n")
		return err
	})
}

func TestNewUploadRequest_errorForNoTrailingSlash(t *testing.T) {
	t.Parallel()
	tests := []struct {
		rawurl    string
		wantError bool
	}{
		{rawurl: "https://example.com/api/uploads", wantError: true},
		{rawurl: "https://example.com/api/uploads/", wantError: false},
	}
	c := mustNewClient(t)
	for _, test := range tests {
		u, err := url.Parse(test.rawurl)
		if err != nil {
			t.Fatalf("url.Parse returned unexpected error: %v", err)
		}
		c.uploadURL = u
		if _, err = c.NewUploadRequest(t.Context(), "test", nil, 0, ""); test.wantError && err == nil {
			t.Fatal("Expected error to be returned.")
		} else if !test.wantError && err != nil {
			t.Fatalf("NewUploadRequest returned unexpected error: %v", err)
		}
	}
}

func TestResponse_populatePageValues(t *testing.T) {
	t.Parallel()
	r := http.Response{
		Header: http.Header{
			"Link": {
				`<https://api.github.com/?page=1>; rel="first",` +
					` <https://api.github.com/?page=2>; rel="prev",` +
					` <https://api.github.com/?page=4>; rel="next",` +
					` <https://api.github.com/?page=5>; rel="last"`,
			},
		},
	}

	response := newResponse(&r)
	if got, want := response.FirstPage, 1; got != want {
		t.Errorf("response.FirstPage: %v, want %v", got, want)
	}
	if got, want := response.PrevPage, 2; want != got {
		t.Errorf("response.PrevPage: %v, want %v", got, want)
	}
	if got, want := response.NextPage, 4; want != got {
		t.Errorf("response.NextPage: %v, want %v", got, want)
	}
	if got, want := response.LastPage, 5; want != got {
		t.Errorf("response.LastPage: %v, want %v", got, want)
	}
	if got, want := response.NextPageToken, ""; want != got {
		t.Errorf("response.NextPageToken: %v, want %v", got, want)
	}
}

func TestResponse_populateSinceValues(t *testing.T) {
	t.Parallel()
	r := http.Response{
		Header: http.Header{
			"Link": {
				`<https://api.github.com/?since=1>; rel="first",` +
					` <https://api.github.com/?since=2>; rel="prev",` +
					` <https://api.github.com/?since=4>; rel="next",` +
					` <https://api.github.com/?since=5>; rel="last"`,
			},
		},
	}

	response := newResponse(&r)
	if got, want := response.FirstPage, 1; got != want {
		t.Errorf("response.FirstPage: %v, want %v", got, want)
	}
	if got, want := response.PrevPage, 2; want != got {
		t.Errorf("response.PrevPage: %v, want %v", got, want)
	}
	if got, want := response.NextPage, 4; want != got {
		t.Errorf("response.NextPage: %v, want %v", got, want)
	}
	if got, want := response.LastPage, 5; want != got {
		t.Errorf("response.LastPage: %v, want %v", got, want)
	}
	if got, want := response.NextPageToken, ""; want != got {
		t.Errorf("response.NextPageToken: %v, want %v", got, want)
	}
}

func TestResponse_SinceWithPage(t *testing.T) {
	t.Parallel()
	r := http.Response{
		Header: http.Header{
			"Link": {
				`<https://api.github.com/?since=2021-12-04T10%3A43%3A42Z&page=1>; rel="first",` +
					` <https://api.github.com/?since=2021-12-04T10%3A43%3A42Z&page=2>; rel="prev",` +
					` <https://api.github.com/?since=2021-12-04T10%3A43%3A42Z&page=4>; rel="next",` +
					` <https://api.github.com/?since=2021-12-04T10%3A43%3A42Z&page=5>; rel="last"`,
			},
		},
	}

	response := newResponse(&r)
	if got, want := response.FirstPage, 1; got != want {
		t.Errorf("response.FirstPage: %v, want %v", got, want)
	}
	if got, want := response.PrevPage, 2; want != got {
		t.Errorf("response.PrevPage: %v, want %v", got, want)
	}
	if got, want := response.NextPage, 4; want != got {
		t.Errorf("response.NextPage: %v, want %v", got, want)
	}
	if got, want := response.LastPage, 5; want != got {
		t.Errorf("response.LastPage: %v, want %v", got, want)
	}
	if got, want := response.NextPageToken, ""; want != got {
		t.Errorf("response.NextPageToken: %v, want %v", got, want)
	}
}

func TestResponse_cursorPagination(t *testing.T) {
	t.Parallel()
	r := http.Response{
		Header: http.Header{
			"Status": {"200 OK"},
			"Link":   {`<https://api.github.com/resource?per_page=2&page=url-encoded-next-page-token>; rel="next"`},
		},
	}

	response := newResponse(&r)
	if got, want := response.FirstPage, 0; got != want {
		t.Errorf("response.FirstPage: %v, want %v", got, want)
	}
	if got, want := response.PrevPage, 0; want != got {
		t.Errorf("response.PrevPage: %v, want %v", got, want)
	}
	if got, want := response.NextPage, 0; want != got {
		t.Errorf("response.NextPage: %v, want %v", got, want)
	}
	if got, want := response.LastPage, 0; want != got {
		t.Errorf("response.LastPage: %v, want %v", got, want)
	}
	if got, want := response.NextPageToken, "url-encoded-next-page-token"; want != got {
		t.Errorf("response.NextPageToken: %v, want %v", got, want)
	}

	// cursor-based pagination with "cursor" param
	r = http.Response{
		Header: http.Header{
			"Link": {
				`<https://api.github.com/?cursor=v1_12345678>; rel="next"`,
			},
		},
	}

	response = newResponse(&r)
	if got, want := response.Cursor, "v1_12345678"; got != want {
		t.Errorf("response.Cursor: %v, want %v", got, want)
	}
}

func TestResponse_beforeAfterPagination(t *testing.T) {
	t.Parallel()
	r := http.Response{
		Header: http.Header{
			"Link": {
				`<https://api.github.com/?after=a1b2c3&before=>; rel="next",` +
					` <https://api.github.com/?after=&before=>; rel="first",` +
					` <https://api.github.com/?after=&before=d4e5f6>; rel="prev",`,
			},
		},
	}

	response := newResponse(&r)
	if got, want := response.Before, "d4e5f6"; got != want {
		t.Errorf("response.Before: %v, want %v", got, want)
	}
	if got, want := response.After, "a1b2c3"; got != want {
		t.Errorf("response.After: %v, want %v", got, want)
	}
	if got, want := response.FirstPage, 0; got != want {
		t.Errorf("response.FirstPage: %v, want %v", got, want)
	}
	if got, want := response.PrevPage, 0; want != got {
		t.Errorf("response.PrevPage: %v, want %v", got, want)
	}
	if got, want := response.NextPage, 0; want != got {
		t.Errorf("response.NextPage: %v, want %v", got, want)
	}
	if got, want := response.LastPage, 0; want != got {
		t.Errorf("response.LastPage: %v, want %v", got, want)
	}
	if got, want := response.NextPageToken, ""; want != got {
		t.Errorf("response.NextPageToken: %v, want %v", got, want)
	}
}

func TestResponse_populatePageValues_invalid(t *testing.T) {
	t.Parallel()
	r := http.Response{
		Header: http.Header{
			"Link": {
				`<https://api.github.com/?page=1>,` +
					`<https://api.github.com/?page=abc>; rel="first",` +
					`https://api.github.com/?page=2; rel="prev",` +
					`<https://api.github.com/>; rel="next",` +
					`<https://api.github.com/?page=>; rel="last"`,
			},
		},
	}

	response := newResponse(&r)
	if got, want := response.FirstPage, 0; got != want {
		t.Errorf("response.FirstPage: %v, want %v", got, want)
	}
	if got, want := response.PrevPage, 0; got != want {
		t.Errorf("response.PrevPage: %v, want %v", got, want)
	}
	if got, want := response.NextPage, 0; got != want {
		t.Errorf("response.NextPage: %v, want %v", got, want)
	}
	if got, want := response.LastPage, 0; got != want {
		t.Errorf("response.LastPage: %v, want %v", got, want)
	}

	// more invalid URLs
	r = http.Response{
		Header: http.Header{
			"Link": {`<https://api.github.com/%?page=2>; rel="first"`},
		},
	}

	response = newResponse(&r)
	if got, want := response.FirstPage, 0; got != want {
		t.Errorf("response.FirstPage: %v, want %v", got, want)
	}
}

func TestResponse_populateSinceValues_invalid(t *testing.T) {
	t.Parallel()
	r := http.Response{
		Header: http.Header{
			"Link": {
				`<https://api.github.com/?since=1>,` +
					`<https://api.github.com/?since=abc>; rel="first",` +
					`https://api.github.com/?since=2; rel="prev",` +
					`<https://api.github.com/>; rel="next",` +
					`<https://api.github.com/?since=>; rel="last"`,
			},
		},
	}

	response := newResponse(&r)
	if got, want := response.FirstPage, 0; got != want {
		t.Errorf("response.FirstPage: %v, want %v", got, want)
	}
	if got, want := response.PrevPage, 0; got != want {
		t.Errorf("response.PrevPage: %v, want %v", got, want)
	}
	if got, want := response.NextPage, 0; got != want {
		t.Errorf("response.NextPage: %v, want %v", got, want)
	}
	if got, want := response.LastPage, 0; got != want {
		t.Errorf("response.LastPage: %v, want %v", got, want)
	}

	// more invalid URLs
	r = http.Response{
		Header: http.Header{
			"Link": {`<https://api.github.com/%?since=2>; rel="first"`},
		},
	}

	response = newResponse(&r)
	if got, want := response.FirstPage, 0; got != want {
		t.Errorf("response.FirstPage: %v, want %v", got, want)
	}
}

func TestDo(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	type foo struct {
		A string
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"A":"a"}`)
	})

	req, _ := client.NewRequest(t.Context(), "GET", ".", nil)
	var body *foo
	_, err := client.Do(req, &body)
	assertNilError(t, err)

	want := &foo{"a"}
	if !cmp.Equal(body, want) {
		t.Errorf("Response body = %v, want %v", body, want)
	}
}

func TestDo_httpError(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Bad Request", 400)
	})

	req, _ := client.NewRequest(t.Context(), "GET", ".", nil)
	resp, err := client.Do(req, nil)

	if err == nil {
		t.Fatal("Expected HTTP 400 error, got no error.")
	}
	if resp.StatusCode != 400 {
		t.Errorf("Expected HTTP 400 error, got %v status code.", resp.StatusCode)
	}
}

// Test handling of an error caused by the internal http client's Do()
// function. A redirect loop is pretty unlikely to occur within the GitHub
// API, but does allow us to exercise the right code path.
func TestDo_redirectLoop(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, baseURLPath, http.StatusFound)
	})

	req, _ := client.NewRequest(t.Context(), "GET", ".", nil)
	_, err := client.Do(req, nil)

	if err == nil {
		t.Error("Expected error to be returned.")
	}
	if !errors.As(err, new(*url.Error)) {
		t.Errorf("Expected a URL error; got %#v", err)
	}
}

func TestDo_preservesResponseInHTTPError(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{
			"message": "Resource not found",
			"documentation_url": "https://docs.github.com/rest/reference/repos#get-a-repository"
		}`)
	})

	req, _ := client.NewRequest(t.Context(), "GET", ".", nil)
	var resp *Response
	var data any
	resp, err := client.Do(req, &data)

	if err == nil {
		t.Fatal("Expected error response")
	}

	// Verify error type and access to status code
	var errResp *ErrorResponse
	if !errors.As(err, &errResp) {
		t.Fatalf("Expected *ErrorResponse error, got %T", err)
	}

	// Verify status code is accessible from both Response and ErrorResponse
	if resp == nil {
		t.Fatal("Expected response to be returned even with error")
	}
	if got, want := resp.StatusCode, http.StatusNotFound; got != want {
		t.Errorf("Response status = %v, want %v", got, want)
	}
	if got, want := errResp.Response.StatusCode, http.StatusNotFound; got != want {
		t.Errorf("Error response status = %v, want %v", got, want)
	}

	// Verify error contains proper message
	if !strings.Contains(errResp.Message, "Resource not found") {
		t.Errorf("Error message = %q, want to contain 'Resource not found'", errResp.Message)
	}
}

// TestDo_AcceptedError_LargeBodyTruncated verifies that when the API returns a
// 202 Accepted with a body larger than maxErrorBodySize, the client reads at
// most maxErrorBodySize bytes into AcceptedError.Raw and does not allocate
// unbounded memory.
func TestDo_AcceptedError_LargeBodyTruncated(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	// Serve a 202 response whose body exceeds the cap by one byte.
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, strings.Repeat("x", maxErrorBodySize+1))
	})

	req, _ := client.NewRequest(t.Context(), "GET", ".", nil)
	_, err := client.Do(req, nil)
	if err == nil {
		t.Fatal("Expected AcceptedError, got nil")
	}

	var aerr *AcceptedError
	if !errors.As(err, &aerr) {
		t.Fatalf("Expected *AcceptedError, got %T: %v", err, err)
	}

	if got, want := len(aerr.Raw), maxErrorBodySize; got != want {
		t.Errorf("AcceptedError.Raw length = %v, want %v (maxErrorBodySize)", got, want)
	}
}

// Test that an error caused by the internal http client's Do() function
// does not leak the client secret.
func TestDo_sanitizeURL(t *testing.T) {
	t.Parallel()
	tp := &UnauthenticatedRateLimitedTransport{
		ClientID:     "id",
		ClientSecret: "secret",
	}
	unauthedClient := mustNewClient(t, WithHTTPClient(tp.Client()))
	unauthedClient.baseURL = &url.URL{Scheme: "http", Host: "127.0.0.1:0", Path: "/"} // Use port 0 on purpose to trigger a dial TCP error, expect to get "dial tcp 127.0.0.1:0: connect: can't assign requested address".
	req, err := unauthedClient.NewRequest(t.Context(), "GET", ".", nil)
	if err != nil {
		t.Fatalf("NewRequest returned unexpected error: %v", err)
	}
	_, err = unauthedClient.Do(req, nil)
	if err == nil {
		t.Fatal("Expected error to be returned.")
	}
	if strings.Contains(err.Error(), "client_secret=secret") {
		t.Errorf("Do error contains secret, should be redacted:\n%q", err)
	}
}

func TestDo_rateLimit(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(HeaderRateLimit, "60")
		w.Header().Set(HeaderRateRemaining, "59")
		w.Header().Set(HeaderRateUsed, "1")
		w.Header().Set(HeaderRateReset, referenceUnixTimeStr)
		w.Header().Set(HeaderRateResource, "core")
	})

	req, _ := client.NewRequest(t.Context(), "GET", ".", nil)
	resp, err := client.Do(req, nil)
	if err != nil {
		t.Errorf("Do returned unexpected error: %v", err)
	}
	if got, want := resp.Rate.Limit, 60; got != want {
		t.Errorf("Client rate limit = %v, want %v", got, want)
	}
	if got, want := resp.Rate.Remaining, 59; got != want {
		t.Errorf("Client rate remaining = %v, want %v", got, want)
	}
	if got, want := resp.Rate.Used, 1; got != want {
		t.Errorf("Client rate used = %v, want %v", got, want)
	}
	if !resp.Rate.Reset.UTC().Equal(referenceTime) {
		t.Errorf("Client rate reset = %v, want %v", resp.Rate.Reset.UTC(), referenceTime)
	}
	if got, want := resp.Rate.Resource, "core"; got != want {
		t.Errorf("Client rate resource = %v, want %v", got, want)
	}
}

func TestDo_rateLimitCategory(t *testing.T) {
	t.Parallel()
	tests := []struct {
		method   string
		url      string
		category RateLimitCategory
	}{
		{
			method:   "GET",
			url:      "/",
			category: CoreCategory,
		},
		{
			method:   "GET",
			url:      "/search/issues?q=rate",
			category: SearchCategory,
		},
		{
			method:   "GET",
			url:      "/graphql",
			category: GraphqlCategory,
		},
		{
			method:   "POST",
			url:      "/app-manifests/code/conversions",
			category: IntegrationManifestCategory,
		},
		{
			method:   "GET",
			url:      "/app-manifests/code/conversions",
			category: CoreCategory, // only POST requests are in the integration manifest category
		},
		{
			method:   "PUT",
			url:      "/repos/google/go-github/import",
			category: SourceImportCategory,
		},
		{
			method:   "GET",
			url:      "/repos/google/go-github/import",
			category: CoreCategory, // only PUT requests are in the source import category
		},
		{
			method:   "POST",
			url:      "/repos/google/go-github/code-scanning/sarifs",
			category: CodeScanningUploadCategory,
		},
		{
			method:   "GET",
			url:      "/scim/v2/organizations/ORG/Users",
			category: ScimCategory,
		},
		{
			method:   "POST",
			url:      "/repos/google/go-github/dependency-graph/snapshots",
			category: DependencySnapshotsCategory,
		},
		{
			method:   "GET",
			url:      "/search/code?q=rate",
			category: CodeSearchCategory,
		},
		{
			method:   "GET",
			url:      "/orgs/google/audit-log",
			category: AuditLogCategory,
		},
		{
			method:   "GET",
			url:      "/repos/google/go-github/dependency-graph/sbom",
			category: DependencySBOMCategory,
		},
		// missing a check for actionsRunnerRegistrationCategory: API not found
	}

	for _, tt := range tests {
		if got, want := GetRateLimitCategory(tt.method, tt.url), tt.category; got != want {
			t.Errorf("expecting category %v, found %v", got, want)
		}
	}
}

// Ensure rate limit is still parsed, even for error responses.
func TestDo_rateLimit_errorResponse(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(HeaderRateLimit, "60")
		w.Header().Set(HeaderRateRemaining, "59")
		w.Header().Set(HeaderRateUsed, "1")
		w.Header().Set(HeaderRateReset, referenceUnixTimeStr)
		w.Header().Set(HeaderRateResource, "core")
		http.Error(w, "Bad Request", 400)
	})

	req, _ := client.NewRequest(t.Context(), "GET", ".", nil)
	resp, err := client.Do(req, nil)
	if err == nil {
		t.Error("Expected error to be returned.")
	}
	if errors.As(err, new(*RateLimitError)) {
		t.Errorf("Did not expect a *RateLimitError error; got %#v", err)
	}
	if got, want := resp.Rate.Limit, 60; got != want {
		t.Errorf("Client rate limit = %v, want %v", got, want)
	}
	if got, want := resp.Rate.Remaining, 59; got != want {
		t.Errorf("Client rate remaining = %v, want %v", got, want)
	}
	if got, want := resp.Rate.Used, 1; got != want {
		t.Errorf("Client rate used = %v, want %v", got, want)
	}
	if !resp.Rate.Reset.UTC().Equal(referenceTime) {
		t.Errorf("Client rate reset = %v, want %v", resp.Rate.Reset, referenceTime)
	}
	if got, want := resp.Rate.Resource, "core"; got != want {
		t.Errorf("Client rate resource = %v, want %v", got, want)
	}
}

// Ensure *RateLimitError is returned when API rate limit is exceeded.
func TestDo_rateLimit_rateLimitError(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(HeaderRateLimit, "60")
		w.Header().Set(HeaderRateRemaining, "0")
		w.Header().Set(HeaderRateUsed, "60")
		w.Header().Set(HeaderRateReset, referenceUnixTimeStr)
		w.Header().Set(HeaderRateResource, "core")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintln(w, `{
   "message": "API rate limit exceeded for xxx.xxx.xxx.xxx. (But here's the good news: Authenticated requests get a higher rate limit. Check out the documentation for more details.)",
   "documentation_url": "https://docs.github.com/en/rest/overview/resources-in-the-rest-api#abuse-rate-limits"
}`)
	})

	req, _ := client.NewRequest(t.Context(), "GET", ".", nil)
	_, err := client.Do(req, nil)

	if err == nil {
		t.Error("Expected error to be returned.")
	}
	var rateLimitErr *RateLimitError
	if !errors.As(err, &rateLimitErr) {
		t.Fatalf("Expected a *RateLimitError error; got %#v", err)
	}
	if got, want := rateLimitErr.Rate.Limit, 60; got != want {
		t.Errorf("rateLimitErr rate limit = %v, want %v", got, want)
	}
	if got, want := rateLimitErr.Rate.Remaining, 0; got != want {
		t.Errorf("rateLimitErr rate remaining = %v, want %v", got, want)
	}
	if got, want := rateLimitErr.Rate.Used, 60; got != want {
		t.Errorf("rateLimitErr rate used = %v, want %v", got, want)
	}
	if !rateLimitErr.Rate.Reset.UTC().Equal(referenceTime) {
		t.Errorf("rateLimitErr rate reset = %v, want %v", rateLimitErr.Rate.Reset.UTC(), referenceTime)
	}
	if got, want := rateLimitErr.Rate.Resource, "core"; got != want {
		t.Errorf("rateLimitErr rate resource = %v, want %v", got, want)
	}
}

// Ensure a network call is not made when it's known that API rate limit is still exceeded.
func TestDo_rateLimit_noNetworkCall(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	reset := time.Now().UTC().Add(time.Minute).Round(time.Second) // Rate reset is a minute from now, with 1 second precision.

	mux.HandleFunc("/first", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(HeaderRateLimit, "60")
		w.Header().Set(HeaderRateRemaining, "0")
		w.Header().Set(HeaderRateUsed, "60")
		w.Header().Set(HeaderRateReset, fmt.Sprint(reset.Unix()))
		w.Header().Set(HeaderRateResource, "core")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintln(w, `{
   "message": "API rate limit exceeded for xxx.xxx.xxx.xxx. (But here's the good news: Authenticated requests get a higher rate limit. Check out the documentation for more details.)",
   "documentation_url": "https://docs.github.com/en/rest/overview/resources-in-the-rest-api#abuse-rate-limits"
}`)
	})

	madeNetworkCall := false
	mux.HandleFunc("/second", func(http.ResponseWriter, *http.Request) {
		madeNetworkCall = true
	})

	// First request is made, and it makes the client aware of rate reset time being in the future.
	req, _ := client.NewRequest(t.Context(), "GET", "first", nil)

	_, err := client.Do(req, nil)
	if err == nil {
		t.Error("Expected error to be returned.")
	}

	// Second request should not cause a network call to be made, since client can predict a rate limit error.
	req, _ = client.NewRequest(t.Context(), "GET", "second", nil)
	_, err = client.Do(req, nil)

	if madeNetworkCall {
		t.Fatal("Network call was made, even though rate limit is known to still be exceeded.")
	}

	if err == nil {
		t.Error("Expected error to be returned.")
	}
	var rateLimitErr *RateLimitError
	if !errors.As(err, &rateLimitErr) {
		t.Fatalf("Expected a *RateLimitError error; got %#v", err)
	}
	if got, want := rateLimitErr.Rate.Limit, 60; got != want {
		t.Errorf("rateLimitErr rate limit = %v, want %v", got, want)
	}
	if got, want := rateLimitErr.Rate.Remaining, 0; got != want {
		t.Errorf("rateLimitErr rate remaining = %v, want %v", got, want)
	}
	if got, want := rateLimitErr.Rate.Used, 60; got != want {
		t.Errorf("rateLimitErr rate used = %v, want %v", got, want)
	}
	if !rateLimitErr.Rate.Reset.UTC().Equal(reset) {
		t.Errorf("rateLimitErr rate reset = %v, want %v", rateLimitErr.Rate.Reset.UTC(), reset)
	}
	if got, want := rateLimitErr.Rate.Resource, "core"; got != want {
		t.Errorf("rateLimitErr rate resource = %v, want %v", got, want)
	}
}

// Ignore rate limit headers if the response was served from cache.
func TestDo_rateLimit_ignoredFromCache(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	reset := time.Now().UTC().Add(time.Minute).Round(time.Second) // Rate reset is a minute from now, with 1 second precision.

	// By adding the X-From-Cache header we pretend this is served from a cache.
	mux.HandleFunc("/first", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-From-Cache", "1")
		w.Header().Set(HeaderRateLimit, "60")
		w.Header().Set(HeaderRateRemaining, "0")
		w.Header().Set(HeaderRateUsed, "60")
		w.Header().Set(HeaderRateReset, fmt.Sprint(reset.Unix()))
		w.Header().Set(HeaderRateResource, "core")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintln(w, `{
   "message": "API rate limit exceeded for xxx.xxx.xxx.xxx. (But here's the good news: Authenticated requests get a higher rate limit. Check out the documentation for more details.)",
   "documentation_url": "https://docs.github.com/en/rest/overview/resources-in-the-rest-api#abuse-rate-limits"
}`)
	})

	madeNetworkCall := false
	mux.HandleFunc("/second", func(http.ResponseWriter, *http.Request) {
		madeNetworkCall = true
	})

	// First request is made so afterwards we can check the returned rate limit headers were ignored.
	req, _ := client.NewRequest(t.Context(), "GET", "first", nil)
	_, err := client.Do(req, nil)
	if err == nil {
		t.Error("Expected error to be returned.")
	}

	// Second request should not be hindered by rate limits.
	req, _ = client.NewRequest(t.Context(), "GET", "second", nil)
	_, err = client.Do(req, nil)
	if err != nil {
		t.Fatalf("Second request failed, even though the rate limits from the cache should've been ignored: %v", err)
	}
	if !madeNetworkCall {
		t.Fatal("Network call was not made, even though the rate limits from the cache should've been ignored")
	}
}

// Ensure sleeps until the rate limit is reset when the client is rate limited.
func TestDo_rateLimit_sleepUntilResponseResetLimit(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	reset := time.Now().UTC().Add(time.Second)

	firstRequest := true
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		if firstRequest {
			firstRequest = false
			w.Header().Set(HeaderRateLimit, "60")
			w.Header().Set(HeaderRateRemaining, "0")
			w.Header().Set(HeaderRateUsed, "60")
			w.Header().Set(HeaderRateReset, fmt.Sprint(reset.Unix()))
			w.Header().Set(HeaderRateResource, "core")
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintln(w, `{
   "message": "API rate limit exceeded for xxx.xxx.xxx.xxx. (But here's the good news: Authenticated requests get a higher rate limit. Check out the documentation for more details.)",
   "documentation_url": "https://docs.github.com/en/rest/overview/resources-in-the-rest-api#abuse-rate-limits"
}`)
			return
		}
		w.Header().Set(HeaderRateLimit, "5000")
		w.Header().Set(HeaderRateRemaining, "5000")
		w.Header().Set(HeaderRateUsed, "0")
		w.Header().Set(HeaderRateReset, fmt.Sprint(reset.Add(time.Hour).Unix()))
		w.Header().Set(HeaderRateResource, "core")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{}`)
	})

	req, _ := client.NewRequest(context.WithValue(t.Context(), SleepUntilPrimaryRateLimitResetWhenRateLimited, true), "GET", ".", nil)
	resp, err := client.Do(req, nil)
	if err != nil {
		t.Errorf("Do returned unexpected error: %v", err)
	}
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Errorf("Response status code = %v, want %v", got, want)
	}
}

// Ensure tries to sleep until the rate limit is reset when the client is rate limited, but only once.
func TestDo_rateLimit_sleepUntilResponseResetLimitRetryOnce(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	reset := time.Now().UTC().Add(time.Second)

	var requestCount atomic.Int32
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.Header().Set(HeaderRateLimit, "60")
		w.Header().Set(HeaderRateRemaining, "0")
		w.Header().Set(HeaderRateUsed, "60")
		w.Header().Set(HeaderRateReset, fmt.Sprint(reset.Unix()))
		w.Header().Set(HeaderRateResource, "core")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintln(w, `{
   "message": "API rate limit exceeded for xxx.xxx.xxx.xxx. (But here's the good news: Authenticated requests get a higher rate limit. Check out the documentation for more details.)",
   "documentation_url": "https://docs.github.com/en/rest/overview/resources-in-the-rest-api#abuse-rate-limits"
}`)
	})

	req, _ := client.NewRequest(context.WithValue(t.Context(), SleepUntilPrimaryRateLimitResetWhenRateLimited, true), "GET", ".", nil)
	_, err := client.Do(req, nil)
	if err == nil {
		t.Error("Expected error to be returned.")
	}
	if got, want := int(requestCount.Load()), 2; got != want {
		t.Errorf("Expected 2 requests, got %v", got)
	}
}

// Ensure a network call is not made when it's known that API rate limit is still exceeded.
func TestDo_rateLimit_sleepUntilClientResetLimit(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	reset := time.Now().UTC().Add(time.Second)
	client.rateLimits[CoreCategory] = Rate{Limit: 5000, Remaining: 0, Reset: Timestamp{reset}}
	var requestCount atomic.Int32
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.Header().Set(HeaderRateLimit, "5000")
		w.Header().Set(HeaderRateRemaining, "5000")
		w.Header().Set(HeaderRateUsed, "0")
		w.Header().Set(HeaderRateReset, fmt.Sprint(reset.Add(time.Hour).Unix()))
		w.Header().Set(HeaderRateResource, "core")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{}`)
	})
	req, _ := client.NewRequest(context.WithValue(t.Context(), SleepUntilPrimaryRateLimitResetWhenRateLimited, true), "GET", ".", nil)
	resp, err := client.Do(req, nil)
	if err != nil {
		t.Errorf("Do returned unexpected error: %v", err)
	}
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Errorf("Response status code = %v, want %v", got, want)
	}
	if got, want := int(requestCount.Load()), 1; got != want {
		t.Errorf("Expected 1 request, got %v", got)
	}
}

// Ensure sleep is aborted when the context is cancelled.
//
// The test verifies that when a request receives a 403 rate-limit response,
// the client begins sleeping until the reset time, and that canceling the
// context during (or just before) that sleep aborts it and returns
// context.Canceled.
//
// Determinism: The handler signals via requestReceived once it has run,
// guaranteeing exactly one request before cancellation. A tiny goroutine
// waits for that signal and then calls cancel, so the context is cancelled
// after the handler returns but independently of wall-clock timing. There
// are no time.After timeouts or time.Sleep delays: the test either completes
// in microseconds or hangs (caught by the test framework's global timeout).
func TestDo_rateLimit_abortSleepContextCancelled(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	// We use a 1 minute reset time to ensure the sleep is not completed.
	reset := time.Now().UTC().Add(time.Minute)
	var requestCount atomic.Int32
	// requestReceived is signaled once the handler has run, so the test can
	// verify the request count independently of when the context is cancelled.
	requestReceived := make(chan struct{})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.Header().Set(HeaderRateLimit, "60")
		w.Header().Set(HeaderRateRemaining, "0")
		w.Header().Set(HeaderRateUsed, "60")
		w.Header().Set(HeaderRateReset, fmt.Sprint(reset.Unix()))
		w.Header().Set(HeaderRateResource, "core")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintln(w, `{
   "message": "API rate limit exceeded for xxx.xxx.xxx.xxx. (But here's the good news: Authenticated requests get a higher rate limit. Check out the documentation for more details.)",
   "documentation_url": "https://docs.github.com/en/rest/overview/resources-in-the-rest-api#abuse-rate-limits"
}`)
		close(requestReceived)
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	req, _ := client.NewRequest(context.WithValue(ctx, SleepUntilPrimaryRateLimitResetWhenRateLimited, true), "GET", ".", nil)

	// Cancel the context as soon as the handler has processed the request.
	// The handler always runs and returns before the client begins the
	// rate-limit sleep (the HTTP transport is synchronous), so cancel fires
	// either just before or during the sleep — both paths are handled
	// identically by sleepUntilResetWithBuffer's select.
	go func() {
		<-requestReceived
		cancel()
	}()

	_, err := client.Do(req, nil)

	if got, want := int(requestCount.Load()), 1; got != want {
		t.Errorf("Expected 1 request, got %v", got)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context cancelled error, got: %v", err)
	}
}

// Ensure sleep is aborted when the context is cancelled on initial request.
//
// Determinism: The context is pre-cancelled before Do is called, so
// checkRateLimitBeforeDo → sleepUntilResetWithBuffer sees ctx.Done() already
// closed and returns immediately. No wall-clock timeout is involved.
func TestDo_rateLimit_abortSleepContextCancelledClientLimit(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	reset := time.Now().UTC().Add(time.Minute)
	client.rateLimits[CoreCategory] = Rate{Limit: 5000, Remaining: 0, Reset: Timestamp{reset}}
	var requestCount atomic.Int32
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.Header().Set(HeaderRateLimit, "5000")
		w.Header().Set(HeaderRateRemaining, "5000")
		w.Header().Set(HeaderRateUsed, "0")
		w.Header().Set(HeaderRateReset, fmt.Sprint(reset.Add(time.Hour).Unix()))
		w.Header().Set(HeaderRateResource, "core")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{}`)
	})
	// Pre-cancel the context: checkRateLimitBeforeDo will call
	// sleepUntilResetWithBuffer which immediately returns ctx.Err() because
	// ctx.Done() is already closed. This is fully deterministic — no timing
	// race between a short timeout and goroutine scheduling.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	req, _ := client.NewRequest(context.WithValue(ctx, SleepUntilPrimaryRateLimitResetWhenRateLimited, true), "GET", ".", nil)
	_, err := client.Do(req, nil)
	var rateLimitError *RateLimitError
	if !errors.As(err, &rateLimitError) {
		t.Fatalf("Expected a *rateLimitError error; got %#v", err)
	}
	if got, wantSuffix := rateLimitError.Message, "Context cancelled while waiting for rate limit to reset until"; !strings.HasPrefix(got, wantSuffix) {
		t.Errorf("Expected request to be prevented because context cancellation, got: %v.", got)
	}
	if got, want := int(requestCount.Load()), 0; got != want {
		t.Errorf("Expected 0 requests, got %v", got)
	}
}

// Ensure *AbuseRateLimitError is returned when the response indicates that
// the client has triggered an abuse detection mechanism.
func TestDo_rateLimit_abuseRateLimitError(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		// When the abuse rate limit error is of the "temporarily blocked from content creation" type,
		// there is no "Retry-After" header.
		fmt.Fprintln(w, `{
   "message": "You have triggered an abuse detection mechanism and have been temporarily blocked from content creation. Please retry your request again later.",
   "documentation_url": "https://docs.github.com/en/rest/overview/resources-in-the-rest-api#abuse-rate-limits"
}`)
	})

	req, _ := client.NewRequest(t.Context(), "GET", ".", nil)
	_, err := client.Do(req, nil)

	if err == nil {
		t.Error("Expected error to be returned.")
	}
	var abuseRateLimitErr *AbuseRateLimitError
	if !errors.As(err, &abuseRateLimitErr) {
		t.Fatalf("Expected a *AbuseRateLimitError error; got %#v", err)
	}
	if got, want := abuseRateLimitErr.RetryAfter, (*time.Duration)(nil); got != want {
		t.Errorf("abuseRateLimitErr RetryAfter = %v, want %v", got, want)
	}
}

// Ensure *AbuseRateLimitError is returned when the response indicates that
// the client has triggered an abuse detection mechanism on GitHub Enterprise.
func TestDo_rateLimit_abuseRateLimitErrorEnterprise(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		// When the abuse rate limit error is of the "temporarily blocked from content creation" type,
		// there is no "Retry-After" header.
		// This response returns a documentation url like the one returned for GitHub Enterprise, this
		// url changes between versions but follows roughly the same format.
		fmt.Fprintln(w, `{
   "message": "You have triggered an abuse detection mechanism and have been temporarily blocked from content creation. Please retry your request again later.",
   "documentation_url": "https://docs.github.com/en/rest/overview/resources-in-the-rest-api#abuse-rate-limits"
}`)
	})

	req, _ := client.NewRequest(t.Context(), "GET", ".", nil)
	_, err := client.Do(req, nil)

	if err == nil {
		t.Error("Expected error to be returned.")
	}
	var abuseRateLimitErr *AbuseRateLimitError
	if !errors.As(err, &abuseRateLimitErr) {
		t.Fatalf("Expected a *AbuseRateLimitError error; got %#v", err)
	}
	if got, want := abuseRateLimitErr.RetryAfter, (*time.Duration)(nil); got != want {
		t.Errorf("abuseRateLimitErr RetryAfter = %v, want %v", got, want)
	}
}

// Ensure *AbuseRateLimitError.RetryAfter is parsed correctly for the Retry-After header.
func TestDo_rateLimit_abuseRateLimitError_retryAfter(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		client, mux, _ := setup(t)

		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set(headerRetryAfter, "123") // Retry after value of 123 seconds.
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintln(w, `{
   "message": "You have triggered an abuse detection mechanism ...",
   "documentation_url": "https://docs.github.com/en/rest/overview/resources-in-the-rest-api#abuse-rate-limits"
}`)
		})

		req, _ := client.NewRequest(t.Context(), "GET", ".", nil)
		_, err := client.Do(req, nil)

		if err == nil {
			t.Error("Expected error to be returned.")
		}
		var abuseRateLimitErr *AbuseRateLimitError
		if !errors.As(err, &abuseRateLimitErr) {
			t.Fatalf("Expected a *AbuseRateLimitError error; got %#v", err)
		}
		if abuseRateLimitErr.RetryAfter == nil {
			t.Fatal("abuseRateLimitErr RetryAfter is nil, expected not-nil")
		}
		if got, want := *abuseRateLimitErr.RetryAfter, 123*time.Second; got != want {
			t.Errorf("abuseRateLimitErr RetryAfter = %v, want %v", got, want)
		}

		// expect prevention of a following request
		if _, err = client.Do(req, nil); err == nil {
			t.Error("Expected error to be returned.")
		}
		if !errors.As(err, &abuseRateLimitErr) {
			t.Fatalf("Expected a *AbuseRateLimitError error; got %#v", err)
		}
		if abuseRateLimitErr.RetryAfter == nil {
			t.Fatal("abuseRateLimitErr RetryAfter is nil, expected not-nil")
		}
		if got, want := 123*time.Second, *abuseRateLimitErr.RetryAfter; got != want {
			t.Errorf("abuseRateLimitErr RetryAfter = %v, want %v", got, want)
		}
		if got, wantSuffix := abuseRateLimitErr.Message, "not making remote request."; !strings.HasSuffix(got, wantSuffix) {
			t.Errorf("Expected request to be prevented because of secondary rate limit, got: %v.", got)
		}
	})
}

// Ensure *AbuseRateLimitError.RetryAfter is parsed correctly for the x-ratelimit-reset header.
func TestDo_rateLimit_abuseRateLimitError_xRateLimitReset(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		client, mux, _ := setup(t)

		// x-ratelimit-reset value of 123 seconds into the future.
		blockUntil := time.Now().UTC().Add(123 * time.Second).Unix()

		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set(HeaderRateReset, strconv.Itoa(int(blockUntil)))
			w.Header().Set(HeaderRateRemaining, "1") // set remaining to a value > 0 to distinct from a primary rate limit
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintln(w, `{
   "message": "You have triggered an abuse detection mechanism ...",
   "documentation_url": "https://docs.github.com/en/rest/overview/resources-in-the-rest-api#abuse-rate-limits"
}`)
		})

		req, _ := client.NewRequest(t.Context(), "GET", ".", nil)
		_, err := client.Do(req, nil)

		if err == nil {
			t.Error("Expected error to be returned.")
		}
		var abuseRateLimitErr *AbuseRateLimitError
		if !errors.As(err, &abuseRateLimitErr) {
			t.Fatalf("Expected a *AbuseRateLimitError error; got %#v", err)
		}
		if abuseRateLimitErr.RetryAfter == nil {
			t.Fatal("abuseRateLimitErr RetryAfter is nil, expected not-nil")
		}

		if got, want := *abuseRateLimitErr.RetryAfter, 123*time.Second; got != want {
			t.Errorf("abuseRateLimitErr RetryAfter = %v, want %v", got, want)
		}

		// expect prevention of a following request
		if _, err = client.Do(req, nil); err == nil {
			t.Error("Expected error to be returned.")
		}
		if !errors.As(err, &abuseRateLimitErr) {
			t.Fatalf("Expected a *AbuseRateLimitError error; got %#v", err)
		}
		if abuseRateLimitErr.RetryAfter == nil {
			t.Fatal("abuseRateLimitErr RetryAfter is nil, expected not-nil")
		}
		if got, want := *abuseRateLimitErr.RetryAfter, 123*time.Second; got != want {
			t.Errorf("abuseRateLimitErr RetryAfter = %v, want %v", got, want)
		}
		if got, wantSuffix := abuseRateLimitErr.Message, "not making remote request."; !strings.HasSuffix(got, wantSuffix) {
			t.Errorf("Expected request to be prevented because of secondary rate limit, got: %v.", got)
		}
	})
}

// Ensure *AbuseRateLimitError.RetryAfter respect a max duration if specified.
func TestDo_rateLimit_abuseRateLimitError_maxDuration(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	// specify a max retry after duration of 1 min
	client.maxSecondaryRateLimitRetryAfterDuration = 60 * time.Second

	// x-ratelimit-reset value of 1h into the future, to make sure we are way over the max wait time duration.
	blockUntil := time.Now().Add(1 * time.Hour).Unix()

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set(HeaderRateReset, strconv.Itoa(int(blockUntil)))
		w.Header().Set(HeaderRateRemaining, "1") // set remaining to a value > 0 to distinct from a primary rate limit
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintln(w, `{
   "message": "You have triggered an abuse detection mechanism ...",
   "documentation_url": "https://docs.github.com/en/rest/overview/resources-in-the-rest-api#abuse-rate-limits"
}`)
	})

	req, _ := client.NewRequest(t.Context(), "GET", ".", nil)
	_, err := client.Do(req, nil)

	if err == nil {
		t.Error("Expected error to be returned.")
	}
	var abuseRateLimitErr *AbuseRateLimitError
	if !errors.As(err, &abuseRateLimitErr) {
		t.Fatalf("Expected a *AbuseRateLimitError error; got %#v", err)
	}
	if abuseRateLimitErr.RetryAfter == nil {
		t.Fatal("abuseRateLimitErr RetryAfter is nil, expected not-nil")
	}
	// check that the retry after is set to be the max allowed duration
	if got, want := *abuseRateLimitErr.RetryAfter, client.maxSecondaryRateLimitRetryAfterDuration; got != want {
		t.Errorf("abuseRateLimitErr RetryAfter = %v, want %v", got, want)
	}
}

// Make network call if client has disabled the rate limit check.
func TestDo_rateLimit_disableRateLimitCheck(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	client.disableRateLimitCheck = true

	reset := time.Now().UTC().Add(60 * time.Second)
	client.rateLimits[CoreCategory] = Rate{Limit: 5000, Remaining: 0, Reset: Timestamp{reset}}
	var requestCount atomic.Int32
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.Header().Set(HeaderRateLimit, "5000")
		w.Header().Set(HeaderRateRemaining, "5000")
		w.Header().Set(HeaderRateUsed, "0")
		w.Header().Set(HeaderRateReset, fmt.Sprint(reset.Add(time.Hour).Unix()))
		w.Header().Set(HeaderRateResource, "core")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{}`)
	})
	req, _ := client.NewRequest(t.Context(), "GET", ".", nil)
	resp, err := client.Do(req, nil)
	if err != nil {
		t.Errorf("Do returned unexpected error: %v", err)
	}
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Errorf("Response status code = %v, want %v", got, want)
	}
	if got, want := int(requestCount.Load()), 1; got != want {
		t.Errorf("Expected 1 request, got %v", got)
	}
	if got, want := client.rateLimits[CoreCategory].Remaining, 0; got != want {
		t.Errorf("Expected 0 requests remaining, got %v", got)
	}
}

// Make network call if client has bypassed the rate limit check.
func TestDo_rateLimit_bypassRateLimitCheck(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	reset := time.Now().UTC().Add(60 * time.Second)
	client.rateLimits[CoreCategory] = Rate{Limit: 5000, Remaining: 0, Reset: Timestamp{reset}}
	var requestCount atomic.Int32
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.Header().Set(HeaderRateLimit, "5000")
		w.Header().Set(HeaderRateRemaining, "5000")
		w.Header().Set(HeaderRateUsed, "0")
		w.Header().Set(HeaderRateReset, fmt.Sprint(reset.Add(time.Hour).Unix()))
		w.Header().Set(HeaderRateResource, "core")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{}`)
	})
	req, _ := client.NewRequest(context.WithValue(t.Context(), BypassRateLimitCheck, true), "GET", ".", nil)
	resp, err := client.Do(req, nil)
	if err != nil {
		t.Errorf("Do returned unexpected error: %v", err)
	}
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Errorf("Response status code = %v, want %v", got, want)
	}
	if got, want := int(requestCount.Load()), 1; got != want {
		t.Errorf("Expected 1 request, got %v", got)
	}
	if got, want := client.rateLimits[CoreCategory].Remaining, 5000; got != want {
		t.Errorf("Expected 5000 requests remaining, got %v", got)
	}
}

func TestDo_noContent(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	var body json.RawMessage

	req, _ := client.NewRequest(t.Context(), "GET", ".", nil)
	_, err := client.Do(req, &body)
	if err != nil {
		t.Fatalf("Do returned unexpected error: %v", err)
	}
}

func TestClient_checkRequestAPIVersionBeforeDo(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name       string
		version    string
		versionMin string
		versionMax string
		wantErr    bool
	}{
		{
			name:       "version_not_set",
			version:    "",
			versionMin: api20221128,
			versionMax: api20260310,
			wantErr:    false,
		},
		{
			name:       "version_less_than_min",
			version:    "2022-01-01",
			versionMin: api20221128,
			versionMax: api20260310,
			wantErr:    true,
		},
		{
			name:       "version_equal_to_min",
			version:    api20221128,
			versionMin: api20221128,
			versionMax: api20260310,
			wantErr:    false,
		},
		{
			name:       "version_between_min_and_max",
			version:    "2023-01-01",
			versionMin: api20221128,
			versionMax: api20260310,
			wantErr:    false,
		},
		{
			name:       "version_equal_to_max",
			version:    api20260310,
			versionMin: api20221128,
			versionMax: api20260310,
			wantErr:    false,
		},
		{
			name:       "version_greater_than_max",
			version:    api20260310,
			versionMin: api20221128,
			versionMax: api20221128,
			wantErr:    true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := mustNewClient(t)
			client.apiVersionMin = tt.versionMin
			client.apiVersionMax = tt.versionMax

			req, _ := http.NewRequestWithContext(t.Context(), "GET", ".", nil)
			req.Header.Set(headerAPIVersion, tt.version)

			err := client.checkRequestAPIVersionBeforeDo(req)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Expected error to be returned, got nil")
				}
				if !errors.Is(err, ErrUnsupportedAPIVersion) {
					t.Errorf("Expected ErrUnsupportedAPIVersion; got %#v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Expected no error to be returned, got: %v", err)
			}
		})
	}
}

func TestClient_bareDo_errors_with_unsupported_api_version(t *testing.T) {
	t.Parallel()

	c := mustNewClient(t)
	c.apiVersionMin = api20221128
	c.apiVersionMax = api20221128

	req, _ := http.NewRequestWithContext(t.Context(), "GET", ".", nil)
	req.Header.Set(headerAPIVersion, api20260310)

	_, err := c.bareDo(c.client, req)
	if err == nil {
		t.Fatal("Expected error to be returned, got nil")
	}
	if !errors.Is(err, ErrUnsupportedAPIVersion) {
		t.Errorf("Expected ErrUnsupportedAPIVersion; got %#v", err)
	}
}

func TestBareDoUntilFound_redirectLoop(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, baseURLPath, http.StatusMovedPermanently)
	})

	req, _ := client.NewRequest(t.Context(), "GET", ".", nil)
	_, _, err := client.bareDoUntilFound(req, 1)

	if err == nil {
		t.Error("Expected error to be returned.")
	}
	if !errors.As(err, new(*RedirectionError)) {
		t.Errorf("Expected a Redirection error; got %#v", err)
	}
}

func TestBareDoUntilFound_UnexpectedRedirection(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, baseURLPath, http.StatusSeeOther)
	})

	req, _ := client.NewRequest(t.Context(), "GET", ".", nil)
	_, _, err := client.bareDoUntilFound(req, 1)

	if err == nil {
		t.Error("Expected error to be returned.")
	}
	if !errors.As(err, new(*RedirectionError)) {
		t.Errorf("Expected a Redirection error; got %#v", err)
	}
}

// TestBareDoUntilFound_RejectsCrossHostRedirect verifies that bareDoUntilFound
// refuses to follow a 301 redirect whose Location points to a different host,
// which would otherwise leak the Authorization header (added by the auth
// transport) to an attacker-controlled server.
func TestBareDoUntilFound_RejectsCrossHostRedirect(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "https://evil.example.com/steal")
		w.WriteHeader(http.StatusMovedPermanently)
	})

	req, _ := client.NewRequest(t.Context(), "GET", ".", nil)
	_, _, err := client.bareDoUntilFound(req, 1)
	if err == nil {
		t.Fatal("Expected cross-host redirect to be rejected, got nil error.")
	}
	if !strings.Contains(err.Error(), "cross-host redirect") {
		t.Errorf("Expected cross-host redirect error, got: %v", err)
	}
}

// TestRoundTripWithOptionalFollowRedirect_RejectsCrossHostRedirect verifies
// that roundTripWithOptionalFollowRedirect refuses to follow a 301 redirect to
// a different host, preventing Authorization-header leakage to attacker-
// controlled servers via a malicious or compromised API response.
func TestRoundTripWithOptionalFollowRedirect_RejectsCrossHostRedirect(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "https://evil.example.com/steal")
		w.WriteHeader(http.StatusMovedPermanently)
	})

	_, err := client.roundTripWithOptionalFollowRedirect(t.Context(), ".", 1)
	if err == nil {
		t.Fatal("Expected cross-host redirect to be rejected, got nil error.")
	}
	if !strings.Contains(err.Error(), "cross-host redirect") {
		t.Errorf("Expected cross-host redirect error, got: %v", err)
	}
}

// TestRoundTripWithOptionalFollowRedirect_AllowsSameHostRedirect ensures the
// cross-host check does not break legitimate same-host 301 follow behavior
// (the path that rate-limit redirection relies on).
func TestRoundTripWithOptionalFollowRedirect_AllowsSameHostRedirect(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	var followed atomic.Bool
	mux.HandleFunc("/archive", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", baseURLPath+"/archive-target")
		w.WriteHeader(http.StatusMovedPermanently)
	})
	mux.HandleFunc("/archive-target", func(w http.ResponseWriter, _ *http.Request) {
		followed.Store(true)
		w.WriteHeader(http.StatusOK)
	})

	resp, err := client.roundTripWithOptionalFollowRedirect(t.Context(), "archive", 2)
	if err != nil {
		t.Fatalf("Unexpected error on same-host redirect: %v", err)
	}
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if !followed.Load() {
		t.Error("Expected same-host redirect to be followed.")
	}
}

func TestSanitizeURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in, want string
	}{
		{"/?a=b", "/?a=b"},
		{"/?a=b&client_secret=secret", "/?a=b&client_secret=REDACTED"},
		{"/?a=b&client_id=id&client_secret=secret", "/?a=b&client_id=id&client_secret=REDACTED"},
		{"/?a=b&access_token=secret", "/?a=b&access_token=REDACTED"},
		{"/?a=b&token=secret", "/?a=b&token=REDACTED"},
		{"/?client_secret=s&access_token=t&token=u", "/?access_token=REDACTED&client_secret=REDACTED&token=REDACTED"},
	}

	for _, tt := range tests {
		inURL, _ := url.Parse(tt.in)
		want, _ := url.Parse(tt.want)

		if got := sanitizeURL(inURL); !cmp.Equal(got, want) {
			t.Errorf("sanitizeURL(%v) returned %v, want %v", tt.in, got, want)
		}
	}
}

func TestCheckResponse(t *testing.T) {
	t.Parallel()
	res := &http.Response{
		Request:    &http.Request{},
		StatusCode: http.StatusBadRequest,
		Body: io.NopCloser(strings.NewReader(`{"message":"m",
			"errors": [{"resource": "r", "field": "f", "code": "c"}],
			"block": {"reason": "dmca", "created_at": ` + referenceTimeStr + `}}`)),
	}
	var err *ErrorResponse
	errors.As(CheckResponse(res), &err)

	if err == nil {
		t.Error("Expected error response.")
	}

	want := &ErrorResponse{
		Response: res,
		Message:  "m",
		Errors:   []Error{{Resource: "r", Field: "f", Code: "c"}},
		Block: &ErrorBlock{
			Reason:    "dmca",
			CreatedAt: &referenceTimestamp,
		},
	}
	if !errors.Is(err, want) {
		t.Errorf("Error = %#v, want %#v", err, want)
	}
}

func TestCheckResponse_RateLimit(t *testing.T) {
	t.Parallel()
	res := &http.Response{
		Request:    &http.Request{},
		StatusCode: http.StatusForbidden,
		Header:     http.Header{},
		Body: io.NopCloser(strings.NewReader(`{"message":"m",
			"documentation_url": "url"}`)),
	}
	res.Header.Set(HeaderRateLimit, "60")
	res.Header.Set(HeaderRateRemaining, "0")
	res.Header.Set(HeaderRateUsed, "1")
	res.Header.Set(HeaderRateReset, "243424")
	res.Header.Set(HeaderRateResource, "core")

	var err *RateLimitError
	errors.As(CheckResponse(res), &err)

	if err == nil {
		t.Error("Expected error response.")
	}

	want := &RateLimitError{
		Rate:     parseRate(res),
		Response: res,
		Message:  "m",
	}
	if !errors.Is(err, want) {
		t.Errorf("Error = %#v, want %#v", err, want)
	}
}

func TestCheckResponse_AbuseRateLimit(t *testing.T) {
	t.Parallel()
	res := &http.Response{
		Request:    &http.Request{},
		StatusCode: http.StatusForbidden,
		Body: io.NopCloser(strings.NewReader(`{"message":"m",
			"documentation_url": "docs.github.com/en/rest/overview/resources-in-the-rest-api#abuse-rate-limits"}`)),
	}
	var err *AbuseRateLimitError
	errors.As(CheckResponse(res), &err)

	if err == nil {
		t.Error("Expected error response.")
	}

	want := &AbuseRateLimitError{
		Response: res,
		Message:  "m",
	}
	if !errors.Is(err, want) {
		t.Errorf("Error = %#v, want %#v", err, want)
	}
}

// TestCheckResponse_RateLimit_TooManyRequests tests that HTTP 429 with
// X-RateLimit-Remaining: 0 is correctly detected as RateLimitError.
// GitHub API can return either 403 or 429 for rate limiting.
// See: https://docs.github.com/rest/using-the-rest-api/rate-limits-for-the-rest-api?apiVersion=2022-11-28
func TestCheckResponse_RateLimit_TooManyRequests(t *testing.T) {
	t.Parallel()
	res := &http.Response{
		Request:    &http.Request{},
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{},
		Body: io.NopCloser(strings.NewReader(`{"message":"m",
			"documentation_url": "url"}`)),
	}
	res.Header.Set(HeaderRateLimit, "60")
	res.Header.Set(HeaderRateRemaining, "0")
	res.Header.Set(HeaderRateUsed, "60")
	res.Header.Set(HeaderRateReset, "243424")
	res.Header.Set(HeaderRateResource, "core")

	var err *RateLimitError
	errors.As(CheckResponse(res), &err)

	if err == nil {
		t.Error("Expected error response.")
	}

	want := &RateLimitError{
		Rate:     parseRate(res),
		Response: res,
		Message:  "m",
	}
	if !errors.Is(err, want) {
		t.Errorf("Error = %#v, want %#v", err, want)
	}
}

// TestCheckResponse_AbuseRateLimit_TooManyRequests tests that HTTP 429 with
// secondary rate limit documentation_url is correctly detected as AbuseRateLimitError.
// GitHub API can return either 403 or 429 for secondary rate limits.
// See: https://docs.github.com/rest/using-the-rest-api/rate-limits-for-the-rest-api?apiVersion=2022-11-28#about-secondary-rate-limits
func TestCheckResponse_AbuseRateLimit_TooManyRequests(t *testing.T) {
	t.Parallel()
	res := &http.Response{
		Request:    &http.Request{},
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{},
		Body: io.NopCloser(strings.NewReader(`{"message":"m",
			"documentation_url": "https://docs.github.com/rest/overview/rate-limits-for-the-rest-api#about-secondary-rate-limits"}`)),
	}
	res.Header.Set(headerRetryAfter, "60")

	var err *AbuseRateLimitError
	errors.As(CheckResponse(res), &err)

	if err == nil {
		t.Fatal("Expected error response.")
	}

	if err.Response != res {
		t.Errorf("Response = %v, want %v", err.Response, res)
	}
	if err.Message != "m" {
		t.Errorf("Message = %q, want %q", err.Message, "m")
	}
	if err.RetryAfter == nil {
		t.Error("Expected RetryAfter to be set")
	} else if *err.RetryAfter != 60*time.Second {
		t.Errorf("RetryAfter = %v, want %v", *err.RetryAfter, 60*time.Second)
	}
}

func TestCheckResponse_RedirectionError(t *testing.T) {
	t.Parallel()
	urlStr := "/foo/bar"

	res := &http.Response{
		Request:    &http.Request{},
		StatusCode: http.StatusFound,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(``)),
	}
	res.Header.Set("Location", urlStr)
	var err *RedirectionError
	errors.As(CheckResponse(res), &err)

	if err == nil {
		t.Error("Expected error response.")
	}

	wantedURL, parseErr := url.Parse(urlStr)
	if parseErr != nil {
		t.Errorf("Error parsing fixture url: %v", parseErr)
	}

	want := &RedirectionError{
		Response:   res,
		StatusCode: http.StatusFound,
		Location:   wantedURL,
	}
	if !errors.Is(err, want) {
		t.Errorf("Error = %#v, want %#v", err, want)
	}
}

func TestCompareHttpResponse(t *testing.T) {
	t.Parallel()
	testcases := map[string]struct {
		h1       *http.Response
		h2       *http.Response
		expected bool
	}{
		"both are nil": {
			expected: true,
		},
		"both are non nil - same StatusCode": {
			expected: true,
			h1:       &http.Response{StatusCode: 200},
			h2:       &http.Response{StatusCode: 200},
		},
		"both are non nil - different StatusCode": {
			expected: false,
			h1:       &http.Response{StatusCode: 200},
			h2:       &http.Response{StatusCode: 404},
		},
		"one is nil, other is not": {
			expected: false,
			h2:       &http.Response{},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			v := compareHTTPResponse(tc.h1, tc.h2)
			if tc.expected != v {
				t.Errorf("Expected %t, got %t for (%#v, %#v)", tc.expected, v, tc.h1, tc.h2)
			}
		})
	}
}

func TestErrorResponse_Is(t *testing.T) {
	t.Parallel()
	err := &ErrorResponse{
		Response: &http.Response{},
		Message:  "m",
		Errors:   []Error{{Resource: "r", Field: "f", Code: "c"}},
		Block: &ErrorBlock{
			Reason:    "r",
			CreatedAt: &referenceTimestamp,
		},
		DocumentationURL: "https://github.com",
	}
	testcases := map[string]struct {
		wantSame   bool
		otherError error
	}{
		"errors are same": {
			wantSame: true,
			otherError: &ErrorResponse{
				Response: &http.Response{},
				Errors:   []Error{{Resource: "r", Field: "f", Code: "c"}},
				Message:  "m",
				Block: &ErrorBlock{
					Reason:    "r",
					CreatedAt: &referenceTimestamp,
				},
				DocumentationURL: "https://github.com",
			},
		},
		"errors have different values - Message": {
			wantSame: false,
			otherError: &ErrorResponse{
				Response: &http.Response{},
				Errors:   []Error{{Resource: "r", Field: "f", Code: "c"}},
				Message:  "m1",
				Block: &ErrorBlock{
					Reason:    "r",
					CreatedAt: &referenceTimestamp,
				},
				DocumentationURL: "https://github.com",
			},
		},
		"errors have different values - DocumentationURL": {
			wantSame: false,
			otherError: &ErrorResponse{
				Response: &http.Response{},
				Errors:   []Error{{Resource: "r", Field: "f", Code: "c"}},
				Message:  "m",
				Block: &ErrorBlock{
					Reason:    "r",
					CreatedAt: &referenceTimestamp,
				},
				DocumentationURL: "https://google.com",
			},
		},
		"errors have different values - Response is nil": {
			wantSame: false,
			otherError: &ErrorResponse{
				Errors:  []Error{{Resource: "r", Field: "f", Code: "c"}},
				Message: "m",
				Block: &ErrorBlock{
					Reason:    "r",
					CreatedAt: &referenceTimestamp,
				},
				DocumentationURL: "https://github.com",
			},
		},
		"errors have different values - Errors": {
			wantSame: false,
			otherError: &ErrorResponse{
				Response: &http.Response{},
				Errors:   []Error{{Resource: "r1", Field: "f1", Code: "c1"}},
				Message:  "m",
				Block: &ErrorBlock{
					Reason:    "r",
					CreatedAt: &referenceTimestamp,
				},
				DocumentationURL: "https://github.com",
			},
		},
		"errors have different values - Errors have different length": {
			wantSame: false,
			otherError: &ErrorResponse{
				Response: &http.Response{},
				Errors:   []Error{},
				Message:  "m",
				Block: &ErrorBlock{
					Reason:    "r",
					CreatedAt: &referenceTimestamp,
				},
				DocumentationURL: "https://github.com",
			},
		},
		"errors have different values - Block - one is nil, other is not": {
			wantSame: false,
			otherError: &ErrorResponse{
				Response:         &http.Response{},
				Errors:           []Error{{Resource: "r", Field: "f", Code: "c"}},
				Message:          "m",
				DocumentationURL: "https://github.com",
			},
		},
		"errors have different values - Block - different Reason": {
			wantSame: false,
			otherError: &ErrorResponse{
				Response: &http.Response{},
				Errors:   []Error{{Resource: "r", Field: "f", Code: "c"}},
				Message:  "m",
				Block: &ErrorBlock{
					Reason:    "r1",
					CreatedAt: &referenceTimestamp,
				},
				DocumentationURL: "https://github.com",
			},
		},
		"errors have different values - Block - different CreatedAt #1": {
			wantSame: false,
			otherError: &ErrorResponse{
				Response: &http.Response{},
				Errors:   []Error{{Resource: "r", Field: "f", Code: "c"}},
				Message:  "m",
				Block: &ErrorBlock{
					Reason:    "r",
					CreatedAt: nil,
				},
				DocumentationURL: "https://github.com",
			},
		},
		"errors have different values - Block - different CreatedAt #2": {
			wantSame: false,
			otherError: &ErrorResponse{
				Response: &http.Response{},
				Errors:   []Error{{Resource: "r", Field: "f", Code: "c"}},
				Message:  "m",
				Block: &ErrorBlock{
					Reason:    "r",
					CreatedAt: &Timestamp{time.Date(2017, time.March, 17, 15, 39, 46, 0, time.UTC)},
				},
				DocumentationURL: "https://github.com",
			},
		},
		"errors have different types": {
			wantSame:   false,
			otherError: errors.New("github"),
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if tc.wantSame != err.Is(tc.otherError) {
				t.Errorf("Error = %#v, want %#v", err, tc.otherError)
			}
		})
	}
}

func TestRateLimitError_Is(t *testing.T) {
	t.Parallel()
	err := &RateLimitError{
		Response: &http.Response{},
		Message:  "Github",
	}
	testcases := map[string]struct {
		wantSame   bool
		err        *RateLimitError
		otherError error
	}{
		"errors are same": {
			wantSame: true,
			err:      err,
			otherError: &RateLimitError{
				Response: &http.Response{},
				Message:  "Github",
			},
		},
		"errors are same - Response is nil": {
			wantSame: true,
			err: &RateLimitError{
				Message: "Github",
			},
			otherError: &RateLimitError{
				Message: "Github",
			},
		},
		"errors have different values - Rate": {
			wantSame: false,
			err:      err,
			otherError: &RateLimitError{
				Rate:     Rate{Limit: 10},
				Response: &http.Response{},
				Message:  "Gitlab",
			},
		},
		"errors have different values - Response is nil": {
			wantSame: false,
			err:      err,
			otherError: &RateLimitError{
				Message: "Github",
			},
		},
		"errors have different values - StatusCode": {
			wantSame: false,
			err:      err,
			otherError: &RateLimitError{
				Response: &http.Response{StatusCode: 200},
				Message:  "Github",
			},
		},
		"errors have different types": {
			wantSame:   false,
			err:        err,
			otherError: errors.New("github"),
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if tc.wantSame != tc.err.Is(tc.otherError) {
				t.Errorf("Error = %#v, want %#v", tc.err, tc.otherError)
			}
		})
	}
}

func TestAbuseRateLimitError_Is(t *testing.T) {
	t.Parallel()
	t1 := 1 * time.Second
	t2 := 2 * time.Second
	err := &AbuseRateLimitError{
		Response:   &http.Response{},
		Message:    "Github",
		RetryAfter: &t1,
	}
	testcases := map[string]struct {
		wantSame   bool
		err        *AbuseRateLimitError
		otherError error
	}{
		"errors are same": {
			wantSame: true,
			err:      err,
			otherError: &AbuseRateLimitError{
				Response:   &http.Response{},
				Message:    "Github",
				RetryAfter: &t1,
			},
		},
		"errors are same - Response is nil": {
			wantSame: true,
			err: &AbuseRateLimitError{
				Message:    "Github",
				RetryAfter: &t1,
			},
			otherError: &AbuseRateLimitError{
				Message:    "Github",
				RetryAfter: &t1,
			},
		},
		"errors have different values - Message": {
			wantSame: false,
			err:      err,
			otherError: &AbuseRateLimitError{
				Response:   &http.Response{},
				Message:    "Gitlab",
				RetryAfter: nil,
			},
		},
		"errors have different values - RetryAfter": {
			wantSame: false,
			err:      err,
			otherError: &AbuseRateLimitError{
				Response:   &http.Response{},
				Message:    "Github",
				RetryAfter: &t2,
			},
		},
		"errors have different values - Response is nil": {
			wantSame: false,
			err:      err,
			otherError: &AbuseRateLimitError{
				Message:    "Github",
				RetryAfter: &t1,
			},
		},
		"errors have different values - StatusCode": {
			wantSame: false,
			err:      err,
			otherError: &AbuseRateLimitError{
				Response:   &http.Response{StatusCode: 200},
				Message:    "Github",
				RetryAfter: &t1,
			},
		},
		"errors have different types": {
			wantSame:   false,
			err:        err,
			otherError: errors.New("github"),
		},
		"errors are same - RetryAfter equal value but distinct pointers": {
			wantSame: true,
			err:      err,
			otherError: &AbuseRateLimitError{
				Response:   &http.Response{},
				Message:    "Github",
				RetryAfter: &t1,
			},
		},
		"errors are same - RetryAfter both nil": {
			wantSame: true,
			err: &AbuseRateLimitError{
				Response:   &http.Response{},
				Message:    "Github",
				RetryAfter: nil,
			},
			otherError: &AbuseRateLimitError{
				Response:   &http.Response{},
				Message:    "Github",
				RetryAfter: nil,
			},
		},
		"errors differ - one RetryAfter nil, other non-nil": {
			wantSame: false,
			err:      err,
			otherError: &AbuseRateLimitError{
				Response:   &http.Response{},
				Message:    "Github",
				RetryAfter: nil,
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if tc.wantSame != tc.err.Is(tc.otherError) {
				t.Errorf("Error = %#v, want %#v", tc.err, tc.otherError)
			}
		})
	}
}

func TestAcceptedError_Is(t *testing.T) {
	t.Parallel()
	err := &AcceptedError{Raw: []byte("Github")}
	testcases := map[string]struct {
		wantSame   bool
		otherError error
	}{
		"errors are same": {
			wantSame:   true,
			otherError: &AcceptedError{Raw: []byte("Github")},
		},
		"errors have different values": {
			wantSame:   false,
			otherError: &AcceptedError{Raw: []byte("Gitlab")},
		},
		"errors have different types": {
			wantSame:   false,
			otherError: errors.New("github"),
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if tc.wantSame != err.Is(tc.otherError) {
				t.Errorf("Error = %#v, want %#v", err, tc.otherError)
			}
		})
	}
}

// Ensure that we properly handle API errors that do not contain a response body.
func TestCheckResponse_noBody(t *testing.T) {
	t.Parallel()
	res := &http.Response{
		Request:    &http.Request{},
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader("")),
	}
	var err *ErrorResponse
	errors.As(CheckResponse(res), &err)

	if err == nil {
		t.Error("Expected error response.")
	}

	want := &ErrorResponse{
		Response: res,
	}
	if !errors.Is(err, want) {
		t.Errorf("Error = %#v, want %#v", err, want)
	}
}

func TestCheckResponse_unexpectedErrorStructure(t *testing.T) {
	t.Parallel()
	httpBody := `{"message":"m", "errors": ["error 1"]}`
	res := &http.Response{
		Request:    &http.Request{},
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader(httpBody)),
	}
	var err *ErrorResponse
	errors.As(CheckResponse(res), &err)

	if err == nil {
		t.Error("Expected error response.")
	}

	want := &ErrorResponse{
		Response: res,
		Message:  "m",
		Errors:   []Error{{Message: "error 1"}},
	}
	if !errors.Is(err, want) {
		t.Errorf("Error = %#v, want %#v", err, want)
	}
	data, err2 := io.ReadAll(err.Response.Body)
	if err2 != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if got := string(data); got != httpBody {
		t.Errorf("ErrorResponse.Response.Body = %q, want %q", got, httpBody)
	}
}

// TestCheckResponse_LargeBodyTruncated verifies that CheckResponse reads at
// most maxErrorBodySize bytes from an error response body, so that a
// malicious or buggy server cannot cause the client to allocate unbounded
// memory.
func TestCheckResponse_LargeBodyTruncated(t *testing.T) {
	t.Parallel()
	// Build a body that is one byte larger than the cap.
	body := strings.Repeat("x", maxErrorBodySize+1)
	res := &http.Response{
		Request:    &http.Request{},
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	// CheckResponse should not return an error from the read itself; the HTTP
	// error status is the expected error.
	if err := CheckResponse(res); err == nil {
		t.Fatal("Expected error from CheckResponse, got nil")
	}

	// After CheckResponse, the body is restored with the (truncated) bytes that
	// were actually read.  Verify the restored body is exactly maxErrorBodySize
	// bytes — not the full maxErrorBodySize+1 that the server sent.
	restored, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("io.ReadAll on restored body: %v", err)
	}
	if got, want := len(restored), maxErrorBodySize; got != want {
		t.Errorf("restored body length = %v, want %v (maxErrorBodySize)", got, want)
	}
}

func TestParseBooleanResponse_true(t *testing.T) {
	t.Parallel()
	result, err := parseBoolResponse(nil)
	if err != nil {
		t.Errorf("parseBoolResponse returned error: %+v", err)
	}

	if want := true; result != want {
		t.Errorf("parseBoolResponse returned %+v, want: %+v", result, want)
	}
}

func TestParseBooleanResponse_false(t *testing.T) {
	t.Parallel()
	v := &ErrorResponse{Response: &http.Response{StatusCode: http.StatusNotFound}}
	result, err := parseBoolResponse(v)
	if err != nil {
		t.Errorf("parseBoolResponse returned error: %+v", err)
	}

	if want := false; result != want {
		t.Errorf("parseBoolResponse returned %+v, want: %+v", result, want)
	}
}

func TestParseBooleanResponse_error(t *testing.T) {
	t.Parallel()
	v := &ErrorResponse{Response: &http.Response{StatusCode: http.StatusBadRequest}}
	result, err := parseBoolResponse(v)

	if err == nil {
		t.Error("Expected error to be returned.")
	}

	if want := false; result != want {
		t.Errorf("parseBoolResponse returned %+v, want: %+v", result, want)
	}
}

func TestErrorResponse_Error(t *testing.T) {
	t.Parallel()
	res := &http.Response{Request: &http.Request{}}
	err := ErrorResponse{Message: "m", Response: res}
	if err.Error() == "" {
		t.Error("Expected non-empty ErrorResponse.Error()")
	}

	// don't panic if request is nil
	res = &http.Response{}
	err = ErrorResponse{Message: "m", Response: res}
	if err.Error() == "" {
		t.Error("Expected non-empty ErrorResponse.Error()")
	}

	// don't panic if response is nil
	err = ErrorResponse{Message: "m"}
	if err.Error() == "" {
		t.Error("Expected non-empty ErrorResponse.Error()")
	}
}

func TestError_Error(t *testing.T) {
	t.Parallel()
	err := Error{}
	if err.Error() == "" {
		t.Error("Expected non-empty Error.Error()")
	}
}

func TestSetCredentialsAsHeaders(t *testing.T) {
	t.Parallel()
	req := new(http.Request)
	id, secret := "id", "secret"
	modifiedRequest := setCredentialsAsHeaders(req, id, secret)

	actualID, actualSecret, ok := modifiedRequest.BasicAuth()
	if !ok {
		t.Error("request does not contain basic credentials")
	}

	if actualID != id {
		t.Errorf("id is %v, want %v", actualID, id)
	}

	if actualSecret != secret {
		t.Errorf("secret is %v, want %v", actualSecret, secret)
	}
}

func TestUnauthenticatedRateLimitedTransport(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	clientID, clientSecret := "id", "secret"
	mux.HandleFunc("/", func(_ http.ResponseWriter, r *http.Request) {
		id, secret, ok := r.BasicAuth()
		if !ok {
			t.Error("request does not contain basic auth credentials")
		}
		if id != clientID {
			t.Errorf("request contained basic auth username %q, want %q", id, clientID)
		}
		if secret != clientSecret {
			t.Errorf("request contained basic auth password %q, want %q", secret, clientSecret)
		}
	})

	tp := &UnauthenticatedRateLimitedTransport{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}
	unauthedClient := mustNewClient(t, WithHTTPClient(tp.Client()))
	unauthedClient.baseURL = client.baseURL
	req, _ := unauthedClient.NewRequest(t.Context(), "GET", ".", nil)
	_, err := unauthedClient.Do(req, nil)
	assertNilError(t, err)
}

func TestUnauthenticatedRateLimitedTransport_missingFields(t *testing.T) {
	t.Parallel()
	// missing ClientID
	tp := &UnauthenticatedRateLimitedTransport{
		ClientSecret: "secret",
	}
	_, err := tp.RoundTrip(nil)
	if err == nil {
		t.Error("Expected error to be returned")
	}

	// missing ClientSecret
	tp = &UnauthenticatedRateLimitedTransport{
		ClientID: "id",
	}
	_, err = tp.RoundTrip(nil)
	if err == nil {
		t.Error("Expected error to be returned")
	}
}

func TestUnauthenticatedRateLimitedTransport_transport(t *testing.T) {
	t.Parallel()
	// default transport
	tp := &UnauthenticatedRateLimitedTransport{
		ClientID:     "id",
		ClientSecret: "secret",
	}
	if tp.transport() != http.DefaultTransport {
		t.Error("Expected http.DefaultTransport to be used.")
	}

	// custom transport
	tp = &UnauthenticatedRateLimitedTransport{
		ClientID:     "id",
		ClientSecret: "secret",
		Transport:    &http.Transport{},
	}
	if tp.transport() == http.DefaultTransport {
		t.Error("Expected custom transport to be used.")
	}
}

func TestBasicAuthTransport(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	username, password, otp := "u", "p", "123456"

	mux.HandleFunc("/", func(_ http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok {
			t.Error("request does not contain basic auth credentials")
		}
		if u != username {
			t.Errorf("request contained basic auth username %q, want %q", u, username)
		}
		if p != password {
			t.Errorf("request contained basic auth password %q, want %q", p, password)
		}
		if got, want := r.Header.Get(headerOTP), otp; got != want {
			t.Errorf("request contained OTP %q, want %q", got, want)
		}
	})

	tp := &BasicAuthTransport{
		Username: username,
		Password: password,
		OTP:      otp,
	}
	basicAuthClient := mustNewClient(t, WithHTTPClient(tp.Client()))
	basicAuthClient.baseURL = client.baseURL
	req, _ := basicAuthClient.NewRequest(t.Context(), "GET", ".", nil)
	_, err := basicAuthClient.Do(req, nil)
	assertNilError(t, err)
}

func TestBasicAuthTransport_transport(t *testing.T) {
	t.Parallel()
	// default transport
	tp := &BasicAuthTransport{}
	if tp.transport() != http.DefaultTransport {
		t.Error("Expected http.DefaultTransport to be used.")
	}

	// custom transport
	tp = &BasicAuthTransport{
		Transport: &http.Transport{},
	}
	if tp.transport() == http.DefaultTransport {
		t.Error("Expected custom transport to be used.")
	}
}

func TestFormatRateReset(t *testing.T) {
	t.Parallel()
	d := 120*time.Minute + 12*time.Second
	got := formatRateReset(d)
	want := "[rate reset in 120m12s]"
	if got != want {
		t.Errorf("Format is wrong. got: %v, want: %v", got, want)
	}

	d = 14*time.Minute + 2*time.Second
	got = formatRateReset(d)
	want = "[rate reset in 14m02s]"
	if got != want {
		t.Errorf("Format is wrong. got: %v, want: %v", got, want)
	}

	d = 2*time.Minute + 2*time.Second
	got = formatRateReset(d)
	want = "[rate reset in 2m02s]"
	if got != want {
		t.Errorf("Format is wrong. got: %v, want: %v", got, want)
	}

	d = 12 * time.Second
	got = formatRateReset(d)
	want = "[rate reset in 12s]"
	if got != want {
		t.Errorf("Format is wrong. got: %v, want: %v", got, want)
	}

	d = -1 * (2*time.Hour + 2*time.Second)
	got = formatRateReset(d)
	want = "[rate limit was reset 120m02s ago]"
	if got != want {
		t.Errorf("Format is wrong. got: %v, want: %v", got, want)
	}
}

func TestNestedStructAccessorNoPanic(t *testing.T) {
	t.Parallel()
	issue := &Issue{User: nil}
	got := issue.GetUser().GetPlan().GetName()
	want := ""
	if got != want {
		t.Errorf("Issues.Get.GetUser().GetPlan().GetName() returned %+v, want %+v", got, want)
	}
}

func TestTwoFactorAuthError(t *testing.T) {
	t.Parallel()
	u, err := url.Parse("https://example.com")
	if err != nil {
		t.Fatal(err)
	}

	e := &TwoFactorAuthError{
		Response: &http.Response{
			Request:    &http.Request{Method: "PUT", URL: u},
			StatusCode: http.StatusTooManyRequests,
		},
		Message: "<msg>",
	}
	if got, want := e.Error(), "PUT https://example.com: 429 <msg> []"; got != want {
		t.Errorf("TwoFactorAuthError = %q, want %q", got, want)
	}
}

func TestRateLimitError(t *testing.T) {
	t.Parallel()
	u, err := url.Parse("https://example.com")
	if err != nil {
		t.Fatal(err)
	}

	r := &RateLimitError{
		Response: &http.Response{
			Request:    &http.Request{Method: "PUT", URL: u},
			StatusCode: http.StatusTooManyRequests,
		},
		Message: "<msg>",
	}
	if got, want := r.Error(), "PUT https://example.com: 429 <msg> [rate limit was reset"; !strings.Contains(got, want) {
		t.Errorf("RateLimitError = %q, want %q", got, want)
	}
}

func TestAcceptedError(t *testing.T) {
	t.Parallel()
	a := &AcceptedError{}
	if got, want := a.Error(), "try again later"; !strings.Contains(got, want) {
		t.Errorf("AcceptedError = %q, want %q", got, want)
	}
}

func TestAbuseRateLimitError(t *testing.T) {
	t.Parallel()
	u, err := url.Parse("https://example.com")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("nil RetryAfter", func(t *testing.T) {
		t.Parallel()
		r := &AbuseRateLimitError{
			Response: &http.Response{
				Request:    &http.Request{Method: "PUT", URL: u},
				StatusCode: http.StatusTooManyRequests,
			},
			Message: "<msg>",
		}
		if got, want := r.Error(), "PUT https://example.com: 429 <msg>"; got != want {
			t.Errorf("AbuseRateLimitError = %q, want %q", got, want)
		}
	})

	t.Run("with RetryAfter", func(t *testing.T) {
		t.Parallel()
		d := 60 * time.Second
		r := &AbuseRateLimitError{
			Response: &http.Response{
				Request:    &http.Request{Method: "GET", URL: u},
				StatusCode: http.StatusForbidden,
			},
			Message:    "rate limited",
			RetryAfter: &d,
		}
		if got, want := r.Error(), "GET https://example.com: 403 rate limited [retry after 1m0s]"; got != want {
			t.Errorf("AbuseRateLimitError = %q, want %q", got, want)
		}
	})

	t.Run("zero RetryAfter", func(t *testing.T) {
		t.Parallel()
		d := 0 * time.Second
		r := &AbuseRateLimitError{
			Response: &http.Response{
				Request:    &http.Request{Method: "POST", URL: u},
				StatusCode: http.StatusForbidden,
			},
			Message:    "rate limited",
			RetryAfter: &d,
		}
		if got, want := r.Error(), "POST https://example.com: 403 rate limited"; got != want {
			t.Errorf("AbuseRateLimitError = %q, want %q", got, want)
		}
	})
}

func TestBareDo_returnsOpenBody(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	expectedBody := "Hello from the other side !"

	mux.HandleFunc("/test-url", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, expectedBody)
	})

	req, err := client.NewRequest(t.Context(), "GET", "test-url", nil)
	if err != nil {
		t.Fatalf("client.NewRequest returned error: %v", err)
	}

	resp, err := client.BareDo(req)
	if err != nil {
		t.Fatalf("client.BareDo returned error: %v", err)
	}

	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("io.ReadAll returned error: %v", err)
	}
	if string(got) != expectedBody {
		t.Fatalf("Expected %q, got %q", expectedBody, string(got))
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("resp.Body.Close() returned error: %v", err)
	}
}

func TestError_UnmarshalJSON(t *testing.T) {
	t.Parallel()
	testJSONMarshal(t, &Error{}, `{
		"resource": "",
		"field": "",
		"code": "",
		"message": ""
	}`)

	u := &Error{
		Resource: "res",
		Field:    "field",
		Code:     "code",
		Message:  "msg",
	}

	want := `{
		"resource": "res",
		"field": "field",
		"code": "code",
		"message": "msg"
	}`

	testJSONMarshal(t, u, want)
}

func TestParseTokenExpiration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		header string
		want   Timestamp
	}{
		{
			header: "",
			want:   Timestamp{},
		},
		{
			header: "this is a garbage",
			want:   Timestamp{},
		},
		{
			header: "2021-09-03 02:34:04 UTC",
			want:   Timestamp{time.Date(2021, time.September, 3, 2, 34, 4, 0, time.UTC)},
		},
		{
			header: "2021-09-03 14:34:04 UTC",
			want:   Timestamp{time.Date(2021, time.September, 3, 14, 34, 4, 0, time.UTC)},
		},
		// Some tokens include the timezone offset instead of the timezone.
		// https://github.com/google/go-github/issues/2649
		{
			header: "2023-04-26 20:23:26 +0200",
			want:   Timestamp{time.Date(2023, time.April, 26, 18, 23, 26, 0, time.UTC)},
		},
	}

	for _, tt := range tests {
		res := &http.Response{
			Request: &http.Request{},
			Header:  http.Header{},
		}

		res.Header.Set(headerTokenExpiration, tt.header)
		exp := parseTokenExpiration(res)
		if !exp.Equal(tt.want) {
			t.Errorf("parseTokenExpiration of %q\nreturned %#v\n    want %#v", tt.header, exp, tt.want)
		}
	}
}

func TestClientCopy_leak_transport(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		accessToken := r.Header.Get("Authorization")
		_, _ = fmt.Fprintf(w, `{"login": "%v"}`, accessToken)
	}))
	clientPreconfiguredWithURLs := mustNewClient(t, WithURLs(&srv.URL, &srv.URL))

	aliceClient, err := clientPreconfiguredWithURLs.Clone(WithAuthToken("alice"))
	if err != nil {
		t.Fatal(err)
	}
	bobClient, err := clientPreconfiguredWithURLs.Clone(WithAuthToken("bob"))
	if err != nil {
		t.Fatal(err)
	}

	alice, _, err := aliceClient.Users.Get(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}

	assertNoDiff(t, "Bearer alice", alice.GetLogin())

	bob, _, err := bobClient.Users.Get(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}

	assertNoDiff(t, "Bearer bob", bob.GetLogin())
}

func TestPtr(t *testing.T) {
	t.Parallel()
	equal := func(t *testing.T, want, got any) {
		t.Helper()
		if !cmp.Equal(want, got) {
			t.Errorf("want %#v, got %#v", want, got)
		}
	}

	equal(t, true, *Ptr(true))
	equal(t, int(10), *Ptr(int(10)))
	equal(t, int64(-10), *Ptr(int64(-10)))
	equal(t, "str", *Ptr("str"))
}

func TestDeploymentProtectionRuleEvent_GetRunID(t *testing.T) {
	t.Parallel()

	var want int64 = 123456789
	url := "https://api.github.com/repos/dummy-org/dummy-repo/actions/runs/123456789/deployment_protection_rule"

	e := DeploymentProtectionRuleEvent{
		DeploymentCallbackURL: &url,
	}

	got, _ := e.GetRunID()
	if got != want {
		t.Errorf("want %#v, got %#v", want, got)
	}

	want = 123456789
	url = "repos/dummy-org/dummy-repo/actions/runs/123456789/deployment_protection_rule"

	e = DeploymentProtectionRuleEvent{
		DeploymentCallbackURL: &url,
	}

	got, _ = e.GetRunID()
	if got != want {
		t.Errorf("want %#v, got %#v", want, got)
	}

	want = -1
	url = "https://api.github.com/repos/dummy-org/dummy-repo/actions/runs/abc123/deployment_protection_rule"
	got, err := e.GetRunID()
	if err == nil {
		t.Error("Expected error to be returned")
	}

	if got != want {
		t.Errorf("want %#v, got %#v", want, got)
	}
}
