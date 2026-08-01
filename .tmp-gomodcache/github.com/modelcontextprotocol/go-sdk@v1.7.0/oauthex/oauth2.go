// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package oauthex implements extensions to OAuth2.

package oauthex

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/internal/json"
	"github.com/modelcontextprotocol/go-sdk/internal/util"
)

type httpStatusError struct {
	StatusCode int
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("bad status %d", e.StatusCode)
}

// getJSON retrieves JSON and unmarshals JSON from the URL, as specified in both
// RFC 9728 and RFC 8414.
// It will not read more than limit bytes from the body.
func getJSON[T any](ctx context.Context, c *http.Client, url string, limit int64) (*T, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if c == nil {
		c = http.DefaultClient
	}
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, &httpStatusError{StatusCode: res.StatusCode}
	}
	ct := res.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil || mediaType != "application/json" {
		return nil, fmt.Errorf("bad content type %q", ct)
	}

	var t T
	dec := json.NewDecoder(io.LimitReader(res.Body, limit))
	if err := dec.Decode(&t); err != nil {
		return nil, err
	}
	return &t, nil
}

// checkURLScheme ensures that its argument is a valid URL with a scheme
// that prevents XSS attacks.
// See #526.
// Note: a copy of this function exists in auth/extauth/oidc_login.go; keep these in sync.
func checkURLScheme(u string) error {
	if u == "" {
		return nil
	}
	uu, err := url.Parse(u)
	if err != nil {
		return err
	}
	scheme := strings.ToLower(uu.Scheme)
	if scheme == "javascript" || scheme == "data" || scheme == "vbscript" {
		return fmt.Errorf("URL has disallowed scheme %q", scheme)
	}
	return nil
}

func checkHTTPSOrLoopback(addr string) error {
	if addr == "" {
		return nil
	}
	u, err := url.Parse(addr)
	if err != nil {
		return err
	}
	if !util.IsLoopback(u.Host) && u.Scheme != "https" {
		return fmt.Errorf("URL %q does not use HTTPS or is not a loopback address", addr)
	}
	return nil
}
