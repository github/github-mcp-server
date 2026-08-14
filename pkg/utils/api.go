package utils //nolint:revive //TODO: figure out a better name for this package

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type APIHostResolver interface {
	BaseRESTURL(ctx context.Context) (*url.URL, error)
	GraphqlURL(ctx context.Context) (*url.URL, error)
	UploadURL(ctx context.Context) (*url.URL, error)
	RawURL(ctx context.Context) (*url.URL, error)
	AuthorizationServerURL(ctx context.Context) (*url.URL, error)
}

type APIHost struct {
	restURL                *url.URL
	gqlURL                 *url.URL
	uploadURL              *url.URL
	rawURL                 *url.URL
	authorizationServerURL *url.URL
}

var _ APIHostResolver = APIHost{}

func NewAPIHost(s string) (APIHostResolver, error) {
	a, err := parseAPIHost(s)

	if err != nil {
		return nil, err
	}

	return a, nil
}

// APIHostResolver implementation
func (a APIHost) BaseRESTURL(_ context.Context) (*url.URL, error) {
	return a.restURL, nil
}

func (a APIHost) GraphqlURL(_ context.Context) (*url.URL, error) {
	return a.gqlURL, nil
}

func (a APIHost) UploadURL(_ context.Context) (*url.URL, error) {
	return a.uploadURL, nil
}

func (a APIHost) RawURL(_ context.Context) (*url.URL, error) {
	return a.rawURL, nil
}

func (a APIHost) AuthorizationServerURL(_ context.Context) (*url.URL, error) {
	return a.authorizationServerURL, nil
}

func newDotcomHost() (APIHost, error) {
	baseRestURL, err := url.Parse("https://api.github.com/")
	if err != nil {
		return APIHost{}, fmt.Errorf("failed to parse dotcom REST URL: %w", err)
	}

	gqlURL, err := url.Parse("https://api.github.com/graphql")
	if err != nil {
		return APIHost{}, fmt.Errorf("failed to parse dotcom GraphQL URL: %w", err)
	}

	uploadURL, err := url.Parse("https://uploads.github.com")
	if err != nil {
		return APIHost{}, fmt.Errorf("failed to parse dotcom Upload URL: %w", err)
	}

	rawURL, err := url.Parse("https://raw.githubusercontent.com/")
	if err != nil {
		return APIHost{}, fmt.Errorf("failed to parse dotcom Raw URL: %w", err)
	}

	// The authorization server for GitHub.com is at github.com/login/oauth, not api.github.com
	authorizationServerURL, err := url.Parse("https://github.com/login/oauth")
	if err != nil {
		return APIHost{}, fmt.Errorf("failed to parse dotcom Authorization Server URL: %w", err)
	}

	return APIHost{
		restURL:                baseRestURL,
		gqlURL:                 gqlURL,
		uploadURL:              uploadURL,
		rawURL:                 rawURL,
		authorizationServerURL: authorizationServerURL,
	}, nil
}

func newGHECHost(hostname string) (APIHost, error) {
	u, err := url.Parse(hostname)
	if err != nil {
		return APIHost{}, fmt.Errorf("failed to parse GHEC URL: %w", err)
	}

	// Unsecured GHEC would be an error
	if u.Scheme == "http" {
		return APIHost{}, fmt.Errorf("GHEC URL must be HTTPS")
	}

	restURL, err := url.Parse(fmt.Sprintf("https://api.%s/", u.Hostname()))
	if err != nil {
		return APIHost{}, fmt.Errorf("failed to parse GHEC REST URL: %w", err)
	}

	gqlURL, err := url.Parse(fmt.Sprintf("https://api.%s/graphql", u.Hostname()))
	if err != nil {
		return APIHost{}, fmt.Errorf("failed to parse GHEC GraphQL URL: %w", err)
	}

	uploadURL, err := url.Parse(fmt.Sprintf("https://uploads.%s/", u.Hostname()))
	if err != nil {
		return APIHost{}, fmt.Errorf("failed to parse GHEC Upload URL: %w", err)
	}

	rawURL, err := url.Parse(fmt.Sprintf("https://raw.%s/", u.Hostname()))
	if err != nil {
		return APIHost{}, fmt.Errorf("failed to parse GHEC Raw URL: %w", err)
	}

	authorizationServerURL, err := url.Parse(fmt.Sprintf("https://%s/login/oauth", u.Hostname()))
	if err != nil {
		return APIHost{}, fmt.Errorf("failed to parse GHEC Authorization Server URL: %w", err)
	}

	return APIHost{
		restURL:                restURL,
		gqlURL:                 gqlURL,
		uploadURL:              uploadURL,
		rawURL:                 rawURL,
		authorizationServerURL: authorizationServerURL,
	}, nil
}

func newGHESHost(hostname string) (APIHost, error) {
	u, err := url.Parse(hostname)
	if err != nil {
		return APIHost{}, fmt.Errorf("failed to parse GHES URL: %w", err)
	}

	// Preserve the full authority (host, port, and IPv6 brackets) for the
	// base-host URLs. u.Hostname() drops the port and strips IPv6 brackets,
	// which would silently retarget a loopback dev server to port 80 and produce
	// an unusable URL for [::1]. The subdomain-isolation URLs below still derive
	// from the bare hostname, since a label cannot be prepended to a host:port or
	// an IP literal.
	authority := u.Host

	restURL, err := url.Parse(fmt.Sprintf("%s://%s/api/v3/", u.Scheme, authority))
	if err != nil {
		return APIHost{}, fmt.Errorf("failed to parse GHES REST URL: %w", err)
	}

	gqlURL, err := url.Parse(fmt.Sprintf("%s://%s/api/graphql", u.Scheme, authority))
	if err != nil {
		return APIHost{}, fmt.Errorf("failed to parse GHES GraphQL URL: %w", err)
	}

	// Check if subdomain isolation is enabled
	// See https://docs.github.com/en/enterprise-server@3.17/admin/configuring-settings/hardening-security-for-your-enterprise/enabling-subdomain-isolation#about-subdomain-isolation
	hasSubdomainIsolation := checkSubdomainIsolation(u.Scheme, u.Hostname())

	var uploadURL *url.URL
	if hasSubdomainIsolation {
		// With subdomain isolation: https://uploads.hostname/
		uploadURL, err = url.Parse(fmt.Sprintf("%s://uploads.%s/", u.Scheme, u.Hostname()))
	} else {
		// Without subdomain isolation: https://hostname/api/uploads/
		uploadURL, err = url.Parse(fmt.Sprintf("%s://%s/api/uploads/", u.Scheme, authority))
	}
	if err != nil {
		return APIHost{}, fmt.Errorf("failed to parse GHES Upload URL: %w", err)
	}

	var rawURL *url.URL
	if hasSubdomainIsolation {
		// With subdomain isolation: https://raw.hostname/
		rawURL, err = url.Parse(fmt.Sprintf("%s://raw.%s/", u.Scheme, u.Hostname()))
	} else {
		// Without subdomain isolation: https://hostname/raw/
		rawURL, err = url.Parse(fmt.Sprintf("%s://%s/raw/", u.Scheme, authority))
	}
	if err != nil {
		return APIHost{}, fmt.Errorf("failed to parse GHES Raw URL: %w", err)
	}

	authorizationServerURL, err := url.Parse(fmt.Sprintf("%s://%s/login/oauth", u.Scheme, authority))
	if err != nil {
		return APIHost{}, fmt.Errorf("failed to parse GHES Authorization Server URL: %w", err)
	}

	return APIHost{
		restURL:                restURL,
		gqlURL:                 gqlURL,
		uploadURL:              uploadURL,
		rawURL:                 rawURL,
		authorizationServerURL: authorizationServerURL,
	}, nil
}

// checkSubdomainIsolation detects if GitHub Enterprise Server has subdomain isolation enabled
// by attempting to ping the raw.<host>/_ping endpoint on the subdomain. The raw subdomain must always exist for subdomain isolation.
func checkSubdomainIsolation(scheme, hostname string) bool {
	subdomainURL := fmt.Sprintf("%s://raw.%s/_ping", scheme, hostname)

	client := &http.Client{
		Timeout: 5 * time.Second,
		// Don't follow redirects - we just want to check if the endpoint exists
		//nolint:revive // parameters are required by http.Client.CheckRedirect signature
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(subdomainURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// Note that this does not handle ports yet, so development environments are out.
func parseAPIHost(s string) (APIHost, error) {
	if s == "" {
		return newDotcomHost()
	}

	u, err := url.Parse(s)
	if err != nil {
		return APIHost{}, fmt.Errorf("could not parse host as URL: %s", s)
	}

	if u.Scheme == "" {
		return APIHost{}, fmt.Errorf("host must have a scheme (http or https): %s", s)
	}

	// Enforce HTTPS centrally so no deployment (GHES in particular) can build
	// authenticated REST/GraphQL/upload/raw URLs over cleartext http, which
	// would leak the bearer token/PAT to anyone on the network.
	if err := requireSecureScheme(u); err != nil {
		return APIHost{}, err
	}

	switch classifyHost(u) {
	case HostTypeDotcom:
		return newDotcomHost()
	case HostTypeGHEC:
		return newGHECHost(s)
	default:
		return newGHESHost(s)
	}
}

// requireSecureScheme rejects hosts that would carry credentials over cleartext.
// Every REST/GraphQL/upload/raw/authorization URL is derived from this host and
// used for authenticated requests, so an http scheme would expose the bearer
// token/PAT to network interception and replay. http is permitted only for
// loopback hosts so that local development against a dev server still works.
func requireSecureScheme(u *url.URL) error {
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" && isLoopbackHost(u.Hostname()) {
		return nil
	}
	return fmt.Errorf(
		"host must use https to avoid sending credentials over cleartext: %s (http is only permitted for loopback hosts such as localhost, 127.0.0.1, or ::1)",
		u.Scheme+"://"+u.Hostname(),
	)
}

// isLoopbackHost reports whether hostname is a loopback address. Only exact
// loopback names/addresses qualify, so credentials are never sent in cleartext
// to a remote host.
func isLoopbackHost(hostname string) bool {
	switch strings.ToLower(hostname) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

// HostType identifies which GitHub deployment a host refers to. Tools use this
// to skip capabilities that only exist on some deployments.
type HostType int

const (
	HostTypeDotcom HostType = iota
	HostTypeGHEC
	HostTypeGHES
)

func classifyHost(u *url.URL) HostType {
	switch {
	case u.Hostname() == "github.com" || strings.HasSuffix(u.Hostname(), ".github.com"):
		return HostTypeDotcom
	case u.Hostname() == "ghe.com" || strings.HasSuffix(u.Hostname(), ".ghe.com"):
		return HostTypeGHEC
	default:
		return HostTypeGHES
	}
}

// ParseHostType classifies a host string. An empty string means github.com,
// matching NewAPIHost. It returns an error only when the string is not a URL
// with a scheme.
func ParseHostType(s string) (HostType, error) {
	if s == "" {
		return HostTypeDotcom, nil
	}

	u, err := url.Parse(s)
	if err != nil {
		return HostTypeDotcom, fmt.Errorf("could not parse host as URL: %s", s)
	}

	if u.Scheme == "" {
		return HostTypeDotcom, fmt.Errorf("host must have a scheme (http or https): %s", s)
	}

	return classifyHost(u), nil
}
