package githubapp

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// installationsRefreshInterval bounds how often the installation directory is
// re-listed when a lookup misses. Installing the app on a new organization is
// rare, so a miss is far more likely to be a repository the app cannot see than
// a stale directory.
const installationsRefreshInterval = 10 * time.Minute

// maxInstallationPages caps directory pagination so a misbehaving or very large
// deployment cannot spin here indefinitely.
const maxInstallationPages = 100

// MultiConfig describes a GitHub App that is installed on more than one account.
// Unlike Config it carries no installation ID: installations are discovered from
// the App itself and a token is minted per installation on demand.
type MultiConfig struct {
	// AppID is used as the JWT issuer. GitHub accepts an app ID or client ID.
	AppID string

	// PrivateKeyPEM is the RSA key used to sign app JWTs.
	PrivateKeyPEM []byte

	// BaseRESTURL is the REST API base, e.g. https://api.github.com/ for
	// github.com or https://HOST/api/v3/ for GitHub Enterprise Server.
	BaseRESTURL string
}

func (c MultiConfig) validate() error {
	switch {
	case c.AppID == "":
		return errors.New("GitHub App ID or client ID is required (GITHUB_APP_ID)")
	case len(c.PrivateKeyPEM) == 0:
		return errors.New("GitHub App private key is required (GITHUB_APP_PRIVATE_KEY_PATH or GITHUB_APP_PRIVATE_KEY)")
	case c.BaseRESTURL == "":
		return errors.New("GitHub App REST base URL is required")
	}
	return nil
}

// MultiProvider mints installation access tokens for every account a GitHub App
// is installed on, routing each request to the installation that owns the
// resource it addresses.
//
// It keeps a directory of account login to installation ID, refreshed from
// GET /app/installations, and one cached token per installation. Both the
// directory and the tokens are shared across goroutines.
type MultiProvider struct {
	cfg        MultiConfig
	privateKey *rsa.PrivateKey
	httpClient *http.Client
	logger     *slog.Logger

	mu sync.Mutex
	// byAccount maps a lowercased account login to its installation ID.
	byAccount map[string]string
	// listedAt is when byAccount was last refreshed; zero means never.
	listedAt time.Time
	// providers caches a token provider per installation ID.
	providers map[string]*Provider
	// warnedOwners records owners we have already logged a miss for, so a
	// repeatedly failing tool call does not flood the log.
	warnedOwners map[string]bool
}

// NewMultiProvider validates cfg and returns a provider. Installations are
// discovered lazily on the first token request, so construction does not
// require network access.
func NewMultiProvider(cfg MultiConfig, logger *slog.Logger) (*MultiProvider, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	privateKey, err := parsePrivateKey(cfg.PrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("invalid GitHub App private key: %w", err)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &MultiProvider{
		cfg:          cfg,
		privateKey:   privateKey,
		httpClient:   &http.Client{Timeout: httpTimeout},
		logger:       logger,
		byAccount:    map[string]string{},
		providers:    map[string]*Provider{},
		warnedOwners: map[string]bool{},
	}, nil
}

// TokenForOwner returns an installation access token for the account that owns
// the resource being addressed, or "" when the app is not installed there. An
// empty token leaves the request unauthenticated, which surfaces as a 401 or
// 404 from the API rather than as a silent call against the wrong installation.
func (p *MultiProvider) TokenForOwner(owner string) string {
	if owner == "" {
		return ""
	}
	installationID, ok := p.installationFor(owner)
	if !ok {
		p.warnOnce(owner)
		return ""
	}
	provider, err := p.providerFor(installationID)
	if err != nil {
		p.logger.Error("failed to configure GitHub App installation", "owner", owner, "installationID", installationID, "error", err)
		return ""
	}
	return provider.AccessToken()
}

// TokenForRequest routes an outbound GitHub API request to the installation
// that owns the resource it addresses. See OwnerFromRequest for how the owner
// is determined.
func (p *MultiProvider) TokenForRequest(req *http.Request) string {
	return p.TokenForOwner(OwnerFromRequest(req))
}

// installationFor resolves an account login to an installation ID, refreshing
// the directory when the login is unknown and the cached copy is stale.
func (p *MultiProvider) installationFor(owner string) (string, bool) {
	key := strings.ToLower(owner)

	p.mu.Lock()
	id, ok := p.byAccount[key]
	stale := time.Since(p.listedAt) >= installationsRefreshInterval
	p.mu.Unlock()

	if ok {
		return id, true
	}
	if !stale {
		return "", false
	}

	installations, err := p.listInstallations()
	if err != nil {
		p.logger.Error("failed to list GitHub App installations", "error", err)
		return "", false
	}

	p.mu.Lock()
	p.byAccount = installations
	p.listedAt = time.Now()
	id, ok = p.byAccount[key]
	if ok {
		delete(p.warnedOwners, key)
	}
	p.mu.Unlock()

	return id, ok
}

// providerFor returns the cached single-installation provider for id, creating
// it on first use.
func (p *MultiProvider) providerFor(installationID string) (*Provider, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if provider, ok := p.providers[installationID]; ok {
		return provider, nil
	}
	provider, err := NewProvider(Config{
		AppID:          p.cfg.AppID,
		InstallationID: installationID,
		PrivateKeyPEM:  p.cfg.PrivateKeyPEM,
		BaseRESTURL:    p.cfg.BaseRESTURL,
	}, p.logger)
	if err != nil {
		return nil, err
	}
	p.providers[installationID] = provider
	return provider, nil
}

func (p *MultiProvider) warnOnce(owner string) {
	key := strings.ToLower(owner)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.warnedOwners[key] {
		return
	}
	p.warnedOwners[key] = true
	p.logger.Warn("GitHub App is not installed on this account; the request will be unauthenticated", "owner", owner)
}

// listInstallations pages through GET /app/installations and returns a map of
// lowercased account login to installation ID.
func (p *MultiProvider) listInstallations() (map[string]string, error) {
	jwt, err := mintJWT(p.cfg.AppID, p.privateKey, time.Now())
	if err != nil {
		return nil, err
	}

	endpoint, err := url.JoinPath(p.cfg.BaseRESTURL, "app", "installations")
	if err != nil {
		return nil, fmt.Errorf("building installations URL: %w", err)
	}

	result := map[string]string{}
	const perPage = 100
	for page := 1; page <= maxInstallationPages; page++ {
		installations, err := p.listInstallationsPage(jwt, endpoint, page, perPage)
		if err != nil {
			return nil, err
		}
		for _, installation := range installations {
			if installation.Account.Login == "" {
				continue
			}
			result[strings.ToLower(installation.Account.Login)] = strconv.FormatInt(installation.ID, 10)
		}
		if len(installations) < perPage {
			return result, nil
		}
	}
	p.logger.Warn("stopped listing GitHub App installations at the page limit", "pages", maxInstallationPages)
	return result, nil
}

type appInstallation struct {
	ID      int64 `json:"id"`
	Account struct {
		Login string `json:"login"`
	} `json:"account"`
}

func (p *MultiProvider) listInstallationsPage(jwt, endpoint string, page, perPage int) ([]appInstallation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating installations request: %w", err)
	}
	query := req.URL.Query()
	query.Set("per_page", strconv.Itoa(perPage))
	query.Set("page", strconv.Itoa(page))
	req.URL.RawQuery = query.Encode()
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting installations: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		snippet, readErr := io.ReadAll(io.LimitReader(resp.Body, 512))
		if readErr != nil {
			return nil, fmt.Errorf("installations request failed: %s (reading response: %w)", resp.Status, readErr)
		}
		return nil, fmt.Errorf("installations request failed: %s: %s", resp.Status, strings.TrimSpace(string(snippet)))
	}

	var installations []appInstallation
	if err := json.NewDecoder(resp.Body).Decode(&installations); err != nil {
		return nil, fmt.Errorf("decoding installations response: %w", err)
	}
	return installations, nil
}
