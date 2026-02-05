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
}

type APIHost struct {
	restURL   *url.URL
	gqlURL    *url.URL
	uploadURL *url.URL
	rawURL    *url.URL
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

	return APIHost{
		restURL:   baseRestURL,
		gqlURL:    gqlURL,
		uploadURL: uploadURL,
		rawURL:    rawURL,
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

	return APIHost{
		restURL:   restURL,
		gqlURL:    gqlURL,
		uploadURL: uploadURL,
		rawURL:    rawURL,
	}, nil
}

func newGHESHost(hostname string) (APIHost, error) {
	u, err := url.Parse(hostname)
	if err != nil {
		return APIHost{}, fmt.Errorf("failed to parse GHES URL: %w", err)
	}

	restURL, err := url.Parse(fmt.Sprintf("%s://%s/api/v3/", u.Scheme, u.Hostname()))
	if err != nil {
		return APIHost{}, fmt.Errorf("failed to parse GHES REST URL: %w", err)
	}

	gqlURL, err := url.Parse(fmt.Sprintf("%s://%s/api/graphql", u.Scheme, u.Hostname()))
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
		uploadURL, err = url.Parse(fmt.Sprintf("%s://%s/api/uploads/", u.Scheme, u.Hostname()))
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
		rawURL, err = url.Parse(fmt.Sprintf("%s://%s/raw/", u.Scheme, u.Hostname()))
	}
	if err != nil {
		return APIHost{}, fmt.Errorf("failed to parse GHES Raw URL: %w", err)
	}

	return APIHost{
		restURL:   restURL,
		gqlURL:    gqlURL,
		uploadURL: uploadURL,
		rawURL:    rawURL,
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

	if strings.HasSuffix(u.Hostname(), "github.com") {
		return newDotcomHost()
	}

	if strings.HasSuffix(u.Hostname(), "ghe.com") {
		return newGHECHost(s)
	}

	return newGHESHost(s)
}
