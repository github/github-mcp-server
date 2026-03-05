package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	ghinstallation "github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/github/github-mcp-server/pkg/http/transport"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/utils"
	gogithub "github.com/google/go-github/v82/github"
	"github.com/shurcooL/githubv4"
)

// MultiOrgClientFactory creates GitHub clients routed by organization using
// GitHub App installation tokens. It maintains a cache of
// ghinstallation.Transport per org to avoid re-creating transports on every
// request.
//
// Owner normalization: all org keys are stored and looked up as lowercase with
// underscores replaced by dashes, matching the env-var normalization in
// parseOrgInstallations (GITHUB_INSTALLATION_ID_MY_ORG → "my-org").
//
// Fail-closed: if no installation is found for an owner and no default is
// configured, the factory returns an error rather than falling back to an
// anonymous client or a non-deterministic "first available" installation.
type MultiOrgClientFactory struct {
	appID          int64
	privateKey     []byte
	installations  map[string]int64 // org (lowercase-dashed) → installation_id
	defaultInstall int64
	transports     map[int64]*ghinstallation.Transport // keyed by installation_id (bounded)
	transportsMu   sync.RWMutex
	apiHost        utils.APIHostResolver
	rawURL         *url.URL // pre-resolved for raw client construction
	version        string
	userAgent      string // set via SetUserAgent after initialize handshake
	userAgentMu    sync.RWMutex
}

// NewMultiOrgClientFactory creates a new factory.
//
// The installations map may contain a "_default" key for the fallback
// installation. That key is extracted and stored separately; all other keys
// should already be normalized (lowercase + underscore-to-dash).
//
// rawURL is stored for use by GetRawClient. Pass nil to skip raw client
// support (GetRawClient will return an error).
func NewMultiOrgClientFactory(
	appID int64,
	privateKey []byte,
	installations map[string]int64,
	apiHost utils.APIHostResolver,
	rawURL *url.URL,
	version string,
) *MultiOrgClientFactory {
	defaultInstall := installations["_default"]
	installsCopy := make(map[string]int64, len(installations))
	for k, v := range installations {
		if k != "_default" {
			installsCopy[k] = v
		}
	}
	return &MultiOrgClientFactory{
		appID:          appID,
		privateKey:     privateKey,
		installations:  installsCopy,
		defaultInstall: defaultInstall,
		transports:     make(map[int64]*ghinstallation.Transport),
		apiHost:        apiHost,
		rawURL:         rawURL,
		version:        version,
	}
}

// ResolvePrivateKey returns the private key bytes from either inline content or
// a file path. If both are provided, content takes precedence. Returns an error
// if neither is provided or the file cannot be read.
//
// This is a package-level helper so callers (e.g. server wiring) can resolve
// the key before constructing the factory.
func ResolvePrivateKey(content []byte, filePath string) ([]byte, error) {
	if len(content) > 0 {
		return content, nil
	}
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key file %q: %w", filePath, err)
		}
		return data, nil
	}
	return nil, fmt.Errorf("private key must be provided as content or file path")
}

// getInstallationID returns the installation ID for the given owner.
//
// Normalization: lowercase + underscore-to-dash, matching the env-var
// normalization in parseOrgInstallations.
//
// Lookup order: exact normalized match → defaultInstall → 0.
// A return value of 0 means no installation is configured.
func (f *MultiOrgClientFactory) getInstallationID(owner string) int64 {
	// Normalize to match env var normalization in parseOrgInstallations
	owner = strings.ToLower(owner)
	owner = strings.ReplaceAll(owner, "_", "-")

	if id, ok := f.installations[owner]; ok {
		return id
	}
	return f.defaultInstall
}

// getOrCreateTransport returns a cached transport for the given installation ID,
// creating one if needed. Uses double-checked locking for thread safety.
//
// The cache is keyed by installation ID (not owner string) to prevent unbounded
// growth: unknown owners that fall back to the default installation reuse the
// same transport instead of creating a new entry per unique owner string.
func (f *MultiOrgClientFactory) getOrCreateTransport(installID int64) (*ghinstallation.Transport, error) {
	f.transportsMu.RLock()
	if t, ok := f.transports[installID]; ok {
		f.transportsMu.RUnlock()
		return t, nil
	}
	f.transportsMu.RUnlock()

	f.transportsMu.Lock()
	defer f.transportsMu.Unlock()

	// Double-check after acquiring write lock (another goroutine may have
	// created the transport between the RUnlock and Lock above).
	if t, ok := f.transports[installID]; ok {
		return t, nil
	}

	t, err := ghinstallation.New(http.DefaultTransport, f.appID, installID, f.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create installation transport for install %d: %w", installID, err)
	}

	// Configure GHE base URL if the resolver points to a non-dotcom host.
	// ghinstallation.Transport.BaseURL is the scheme+host for the GitHub API
	// (e.g. "https://github.example.com/api/v3"). We derive it from the
	// resolver's REST base URL.
	if f.apiHost != nil {
		restURL, err := f.apiHost.BaseRESTURL(context.Background())
		if err == nil && restURL != nil {
			baseURL := strings.TrimRight(restURL.String(), "/")
			// Only override when not the default dotcom API to avoid a no-op
			// assignment that could confuse future readers.
			if baseURL != "https://api.github.com" {
				t.BaseURL = baseURL
			}
		}
	}

	f.transports[installID] = t
	return t, nil
}

// normalizeOwner applies the same normalization as getInstallationID so that
// lookups are consistent with the installations map keys.
func normalizeOwner(owner string) string {
	owner = strings.ToLower(owner)
	owner = strings.ReplaceAll(owner, "_", "-")
	return owner
}

// SetUserAgent updates the user agent string used for all clients created by
// this factory. Called after the MCP initialize handshake provides client info.
// Thread-safe: may be called concurrently with client creation.
func (f *MultiOrgClientFactory) SetUserAgent(ua string) {
	f.userAgentMu.Lock()
	f.userAgent = ua
	f.userAgentMu.Unlock()
}

// getUserAgent returns the current user agent string. Falls back to the
// version-based default if SetUserAgent has not been called yet.
func (f *MultiOrgClientFactory) getUserAgent() string {
	f.userAgentMu.RLock()
	ua := f.userAgent
	f.userAgentMu.RUnlock()
	if ua != "" {
		return ua
	}
	return "github-mcp-server/" + f.version
}

// GetRESTClient returns a REST client authenticated as the GitHub App
// installation for the given owner.
//
// Returns an error if:
//   - No installation is configured for owner and no default is set (fail-closed).
//   - The installation transport cannot be created.
func (f *MultiOrgClientFactory) GetRESTClient(ctx context.Context, owner string) (*gogithub.Client, error) {
	installID := f.getInstallationID(owner)
	if installID == 0 {
		return nil, fmt.Errorf(
			"no GitHub App installation configured for org %q and no default installation set",
			owner,
		)
	}

	ghTransport, err := f.getOrCreateTransport(installID)
	if err != nil {
		return nil, err
	}

	client := gogithub.NewClient(&http.Client{Transport: ghTransport})
	client.UserAgent = f.getUserAgent()

	// Set the REST base URL from the resolver so GHE installations work.
	if f.apiHost != nil {
		restURL, err := f.apiHost.BaseRESTURL(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve REST base URL: %w", err)
		}
		uploadURL, err := f.apiHost.UploadURL(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve upload URL: %w", err)
		}
		client.BaseURL = restURL
		client.UploadURL = uploadURL
	}

	return client, nil
}

// GetGQLClient returns a GraphQL client authenticated as the GitHub App
// installation for the given owner.
//
// Returns an error if:
//   - No installation is configured for owner and no default is set (fail-closed).
//   - The installation transport cannot be created.
func (f *MultiOrgClientFactory) GetGQLClient(ctx context.Context, owner string) (*githubv4.Client, error) {
	installID := f.getInstallationID(owner)
	if installID == 0 {
		return nil, fmt.Errorf(
			"no GitHub App installation configured for org %q and no default installation set",
			owner,
		)
	}

	ghTransport, err := f.getOrCreateTransport(installID)
	if err != nil {
		return nil, err
	}

	// Wrap the installation transport with a user agent transport so GraphQL
	// requests carry the same UA as REST requests. The REST client sets UA via
	// gogithub.Client.UserAgent, but githubv4.Client has no equivalent field.
	httpClient := &http.Client{
		Transport: &transport.UserAgentTransport{
			Transport: ghTransport,
			Agent:     f.getUserAgent(),
		},
	}

	// Use NewEnterpriseClient unconditionally: if apiHost resolves to dotcom
	// the URL will be "https://api.github.com/graphql", which is identical to
	// what githubv4.NewClient would use. This avoids a conditional branch.
	if f.apiHost != nil {
		graphqlURL, err := f.apiHost.GraphqlURL(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve GraphQL URL: %w", err)
		}
		return githubv4.NewEnterpriseClient(graphqlURL.String(), httpClient), nil
	}

	return githubv4.NewClient(httpClient), nil
}

// GetRawClient returns a raw content client authenticated as the GitHub App
// installation for the given owner.
//
// Returns an error if:
//   - No installation is configured for owner and no default is set (fail-closed).
//   - rawURL was not provided at construction time.
//   - The installation transport cannot be created.
func (f *MultiOrgClientFactory) GetRawClient(ctx context.Context, owner string) (*raw.Client, error) {
	if f.rawURL == nil {
		return nil, fmt.Errorf("MultiOrgClientFactory: rawURL not configured; cannot create raw client")
	}

	restClient, err := f.GetRESTClient(ctx, owner)
	if err != nil {
		return nil, err
	}

	return raw.NewClient(restClient, f.rawURL), nil
}
