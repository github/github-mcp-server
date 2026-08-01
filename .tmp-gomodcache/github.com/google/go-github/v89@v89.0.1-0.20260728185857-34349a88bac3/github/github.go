// Copyright 2013 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run gen-accessors.go
//go:generate go run gen-iterators.go
//go:generate go run gen-stringify-test.go
//go:generate sh ../script/metadata.sh update-go

package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/go-querystring/query"
)

const (
	Version = "v89.0.0"

	HeaderRateLimit     = "X-Ratelimit-Limit"
	HeaderRateRemaining = "X-Ratelimit-Remaining"
	HeaderRateReset     = "X-Ratelimit-Reset"
	HeaderRateResource  = "X-Ratelimit-Resource"
	HeaderRateUsed      = "X-Ratelimit-Used"
	HeaderRequestID     = "X-Github-Request-Id"

	// https://docs.github.com/en/rest/about-the-rest-api/api-versions#about-api-versioning
	api20221128 = "2022-11-28"
	api20260310 = "2026-03-10"

	defaultBaseURL   = "https://api.github.com/"
	defaultUserAgent = "go-github" + "/" + Version
	uploadBaseURL    = "https://uploads.github.com/"

	headerAPIVersion = "X-Github-Api-Version"
	headerOTP        = "X-Github-Otp"
	headerRetryAfter = "Retry-After"

	headerTokenExpiration = "Github-Authentication-Token-Expiration"

	mediaTypeV3                = "application/vnd.github.v3+json"
	defaultMediaType           = "application/octet-stream"
	mediaTypeV3SHA             = "application/vnd.github.v3.sha"
	mediaTypeV3Diff            = "application/vnd.github.v3.diff"
	mediaTypeV3Patch           = "application/vnd.github.v3.patch"
	mediaTypeOrgPermissionRepo = "application/vnd.github.v3.repository+json"
	mediaTypeIssueImportAPI    = "application/vnd.github.golden-comet-preview+json"
	mediaTypeStarring          = "application/vnd.github.star+json"
	mediaTypeSCIM              = "application/scim+json"

	// Media Type values to access preview APIs.
	// These media types will be added to the API request as headers
	// and used to enable particular features on GitHub API that are still in preview.
	// After some time, specific media types will be promoted (to a "stable" state).
	// From then on, the preview headers are not required anymore to activate the additional
	// feature on GitHub.com's API. However, this API header might still be needed for users
	// to run a GitHub Enterprise Server on-premise.
	// It's not uncommon for GitHub Enterprise Server customers to run older versions which
	// would probably rely on the preview headers for some time.
	// While the header promotion is going out for GitHub.com, it may be some time before it
	// even arrives in GitHub Enterprise Server.
	// We keep those preview headers around to avoid breaking older GitHub Enterprise Server
	// versions. Additionally, non-functional (preview) headers don't create any side effects
	// on GitHub Cloud version.
	//
	// See https://github.com/google/go-github/pull/2125 and https://github.com/google/go-github/pull/2188 for full context.

	// https://help.github.com/enterprise/2.4/admin/guides/migrations/exporting-the-github-com-organization-s-repositories/
	mediaTypeMigrationsPreview = "application/vnd.github.wyandotte-preview+json"

	// https://developer.github.com/changes/2016-04-06-deployment-and-deployment-status-enhancements/
	mediaTypeDeploymentStatusPreview = "application/vnd.github.ant-man-preview+json"

	// https://developer.github.com/changes/2018-10-16-deployments-environments-states-and-auto-inactive-updates/
	mediaTypeExpandDeploymentStatusPreview = "application/vnd.github.flash-preview+json"

	// https://developer.github.com/changes/2016-05-12-reactions-api-preview/
	mediaTypeReactionsPreview = "application/vnd.github.squirrel-girl-preview"

	// https://developer.github.com/changes/2016-05-23-timeline-preview-api/
	mediaTypeTimelinePreview = "application/vnd.github.mockingbird-preview+json"

	// https://developer.github.com/changes/2016-09-14-projects-api/
	mediaTypeProjectsPreview = "application/vnd.github.inertia-preview+json"

	// https://developer.github.com/changes/2017-01-05-commit-search-api/
	mediaTypeCommitSearchPreview = "application/vnd.github.cloak-preview+json"

	// https://developer.github.com/changes/2017-02-28-user-blocking-apis-and-webhook/
	mediaTypeBlockUsersPreview = "application/vnd.github.giant-sentry-fist-preview+json"

	// https://developer.github.com/changes/2017-05-23-coc-api/
	mediaTypeCodesOfConductPreview = "application/vnd.github.scarlet-witch-preview+json"

	// https://developer.github.com/changes/2017-07-17-update-topics-on-repositories/
	mediaTypeTopicsPreview = "application/vnd.github.mercy-preview+json"

	// https://developer.github.com/changes/2018-03-16-protected-branches-required-approving-reviews/
	mediaTypeRequiredApprovingReviewsPreview = "application/vnd.github.luke-cage-preview+json"

	// https://developer.github.com/changes/2018-05-07-new-checks-api-public-beta/
	mediaTypeCheckRunsPreview = "application/vnd.github.antiope-preview+json"

	// https://developer.github.com/enterprise/2.13/v3/repos/pre_receive_hooks/
	mediaTypePreReceiveHooksPreview = "application/vnd.github.eye-scream-preview"

	// https://developer.github.com/changes/2018-02-22-protected-branches-required-signatures/
	mediaTypeSignaturePreview = "application/vnd.github.zzzax-preview+json"

	// https://developer.github.com/changes/2018-09-05-project-card-events/
	mediaTypeProjectCardDetailsPreview = "application/vnd.github.starfox-preview+json"

	// https://developer.github.com/changes/2018-12-18-interactions-preview/
	mediaTypeInteractionRestrictionsPreview = "application/vnd.github.sombra-preview+json"

	// https://developer.github.com/changes/2019-03-14-enabling-disabling-pages/
	mediaTypeEnablePagesAPIPreview = "application/vnd.github.switcheroo-preview+json"

	// https://developer.github.com/changes/2019-04-24-vulnerability-alerts/
	mediaTypeRequiredVulnerabilityAlertsPreview = "application/vnd.github.dorian-preview+json"

	// https://developer.github.com/changes/2019-05-29-update-branch-api/
	mediaTypeUpdatePullRequestBranchPreview = "application/vnd.github.lydian-preview+json"

	// https://developer.github.com/changes/2019-04-11-pulls-branches-for-commit/
	mediaTypeListPullsOrBranchesForCommitPreview = "application/vnd.github.groot-preview+json"

	// https://docs.github.com/rest/previews/#repository-creation-permissions
	mediaTypeMemberAllowedRepoCreationTypePreview = "application/vnd.github.surtur-preview+json"

	// https://docs.github.com/rest/previews/#create-and-use-repository-templates
	mediaTypeRepositoryTemplatePreview = "application/vnd.github.baptiste-preview+json"

	// https://developer.github.com/changes/2019-10-03-multi-line-comments/
	mediaTypeMultiLineCommentsPreview = "application/vnd.github.comfort-fade-preview+json"

	// https://developer.github.com/changes/2019-11-05-deprecated-passwords-and-authorizations-api/
	mediaTypeOAuthAppPreview = "application/vnd.github.doctor-strange-preview+json"

	// https://developer.github.com/changes/2019-12-03-internal-visibility-changes/
	mediaTypeRepositoryVisibilityPreview = "application/vnd.github.nebula-preview+json"

	// https://developer.github.com/changes/2018-12-10-content-attachments-api/
	mediaTypeContentAttachmentsPreview = "application/vnd.github.corsair-preview+json"
)

// ErrPathForbidden is returned when a URL path contains ".." as a path
// segment, which could allow path traversal attacks.
var ErrPathForbidden = errors.New("path must not contain '..' due to auth vulnerability issue")

// A Client manages communication with the GitHub API.
type Client struct {
	client                *http.Client // HTTP client used to communicate with the API.
	clientIgnoreRedirects *http.Client // HTTP client used to communicate with the API on endpoints where we don't want to follow redirects.

	// Base URL for API requests. Defaults to the public GitHub API, but can be
	// set to a domain endpoint to use with GitHub Enterprise. baseURL should
	// always be specified with a trailing slash.
	baseURL *url.URL

	// Base URL for uploading files.
	uploadURL *url.URL

	// Default API version to set in the X-Github-Api-Version header.
	apiVersionDefault string
	// Minimum API version that the client can use.
	apiVersionMin string
	// Maximum API version that the client can use.
	apiVersionMax string

	// User agent used when communicating with the GitHub API.
	userAgent string

	// disableRateLimitCheck stops the client checking for rate limits or tracking
	// them. This is different to setting BypassRateLimitCheck in the context,
	// as that still tracks the rate limits.
	disableRateLimitCheck bool

	rateMu                  sync.Mutex
	rateLimits              [Categories]Rate // Rate limits for the client as determined by the most recent API calls.
	secondaryRateLimitReset time.Time        // Secondary rate limit reset for the client as determined by the most recent API calls.

	// If specified, Client will block requests for at most this duration in case of reaching a secondary
	// rate limit
	maxSecondaryRateLimitRetryAfterDuration time.Duration

	// Whether to respect rate limit headers on endpoints that return 302 redirections to artifacts
	rateLimitRedirectionalEndpoints bool

	common service // Reuse a single struct instead of allocating one for each service on the heap.

	// Services used for talking to different parts of the GitHub API.
	Actions            *ActionsService
	Activity           *ActivityService
	Admin              *AdminService
	Apps               *AppsService
	Authorizations     *AuthorizationsService
	Billing            *BillingService
	Checks             *ChecksService
	Classroom          *ClassroomService
	CodeQuality        *CodeQualityService
	CodeScanning       *CodeScanningService
	CodesOfConduct     *CodesOfConductService
	Codespaces         *CodespacesService
	Copilot            *CopilotService
	Credentials        *CredentialsService
	Dependabot         *DependabotService
	DependencyGraph    *DependencyGraphService
	Emojis             *EmojisService
	Enterprise         *EnterpriseService
	Gists              *GistsService
	Git                *GitService
	Gitignores         *GitignoresService
	Interactions       *InteractionsService
	IssueImport        *IssueImportService
	Issues             *IssuesService
	Licenses           *LicensesService
	Markdown           *MarkdownService
	Marketplace        *MarketplaceService
	Meta               *MetaService
	Migrations         *MigrationService
	Organizations      *OrganizationsService
	PrivateRegistries  *PrivateRegistriesService
	Projects           *ProjectsService
	PullRequests       *PullRequestsService
	RateLimit          *RateLimitService
	Reactions          *ReactionsService
	Repositories       *RepositoriesService
	SCIM               *SCIMService
	Search             *SearchService
	SecretScanning     *SecretScanningService
	SecurityAdvisories *SecurityAdvisoriesService
	SubIssue           *SubIssueService
	Teams              *TeamsService
	Users              *UsersService
}

type service struct {
	client *Client
}

// Client returns the http.Client used by this GitHub client.
// This should only be used for requests to the GitHub API because
// request headers will contain an authorization token.
func (c *Client) Client() *http.Client {
	clientCopy := *c.client
	return &clientCopy
}

// ListOptions specifies the optional parameters to various List methods that
// support offset pagination.
type ListOptions struct {
	// For paginated result sets, page of results to retrieve.
	Page int `url:"page,omitempty"`

	// For paginated result sets, the number of results to include per page.
	PerPage int `url:"per_page,omitempty"`
}

// ListCursorOptions specifies the optional parameters to various List methods that
// support cursor pagination.
type ListCursorOptions struct {
	// For paginated result sets, page of results to retrieve.
	Page string `url:"page,omitempty"`

	// For paginated result sets, the number of results to include per page.
	PerPage int `url:"per_page,omitempty"`

	// For paginated result sets, the number of results per page (max 100), starting from the first matching result.
	// This parameter must not be used in combination with last.
	First int `url:"first,omitempty"`

	// For paginated result sets, the number of results per page (max 100), starting from the last matching result.
	// This parameter must not be used in combination with first.
	Last int `url:"last,omitempty"`

	// A cursor, as given in the Link header. If specified, the query only searches for events after this cursor.
	After string `url:"after,omitempty"`

	// A cursor, as given in the Link header. If specified, the query only searches for events before this cursor.
	Before string `url:"before,omitempty"`

	// A cursor, as given in the Link header. If specified, the query continues the search using this cursor.
	Cursor string `url:"cursor,omitempty"`
}

// UploadOptions specifies the parameters to methods that support uploads.
type UploadOptions struct {
	Name      string `url:"name,omitempty"`
	Label     string `url:"label,omitempty"`
	MediaType string `url:"-"`
}

// RawType represents type of raw format of a request instead of JSON.
type RawType uint8

const (
	// Diff format.
	Diff RawType = 1 + iota
	// Patch format.
	Patch
)

// RawOptions specifies parameters when user wants to get raw format of
// a response instead of JSON.
type RawOptions struct {
	Type RawType
}

type structPtr[T any] interface{ *T }

// addOptions adds the parameters in opts as URL query parameters to s. opts
// must be a struct whose fields may contain "url" tags.
func addOptions[P structPtr[T], T any](s string, opts P) (string, error) {
	if opts == nil {
		return s, nil
	}

	u, err := url.Parse(s)
	if err != nil {
		return s, err
	}

	qs, err := query.Values(opts)
	if err != nil {
		return s, err
	}

	u.RawQuery = qs.Encode()
	return u.String(), nil
}

// errUninitialized is returned when an uninitialized Client is used.
var errUninitialized = errors.New("client is not initialized")

// clientOptions holds the configuration options for a Client.
type clientOptions struct {
	httpClient                              *http.Client
	transport                               http.RoundTripper
	timeout                                 *time.Duration
	apiVersionMin                           *string
	apiVersionMax                           *string
	userAgent                               *string
	envProxy                                bool
	token                                   *string
	baseURL                                 *url.URL
	uploadURL                               *url.URL
	disableRateLimitCheck                   bool
	rateLimitRedirectionalEndpoints         bool
	maxSecondaryRateLimitRetryAfterDuration *time.Duration
	marketplaceStubbed                      bool
}

// ClientOptionsFunc is a functional option for providing configuration options
// to a Client.
type ClientOptionsFunc func(*clientOptions) error

// WithHTTPClient returns a ClientOptionsFunc that sets the http.Client
// for a Client. If not set, a default http.Client will be used.
func WithHTTPClient(httpClient *http.Client) ClientOptionsFunc {
	return func(o *clientOptions) error {
		if httpClient == nil {
			return errors.New("http client must not be nil")
		}

		httpClient := *httpClient
		o.httpClient = &httpClient
		return nil
	}
}

// WithTransport returns a ClientOptionsFunc that sets the http.RoundTripper
// for a Client. This overrides the transport set by [WithHTTPClient]. If not
// set and no HTTP client is provided, the default http.RoundTripper will be used.
func WithTransport(transport http.RoundTripper) ClientOptionsFunc {
	return func(o *clientOptions) error {
		if transport == nil {
			return errors.New("transport must not be nil")
		}

		o.transport = transport
		return nil
	}
}

// WithTimeout returns a ClientOptionsFunc that sets the timeout for a Client.
// This overrides the timeout set by [WithHTTPClient]. If not set and no HTTP
// client is provided, the default http.Client with no timeout will be used.
// It is recommended to provide a timeout for production environments.
func WithTimeout(timeout time.Duration) ClientOptionsFunc {
	return func(o *clientOptions) error {
		if timeout < 0 {
			return errors.New("timeout must not be negative")
		}

		o.timeout = &timeout
		return nil
	}
}

// WithUserAgent returns a ClientOptionsFunc that sets the User-Agent header
// for a Client. If not set, a default User-Agent will be used.
func WithUserAgent(userAgent string) ClientOptionsFunc {
	return func(o *clientOptions) error {
		o.userAgent = &userAgent
		return nil
	}
}

// WithEnvProxy returns a ClientOptionsFunc that configures the Client to use
// the HTTP proxy settings from the environment variables
// (e.g., HTTP_PROXY, HTTPS_PROXY, NO_PROXY).
// If not set, the client will not use environment proxy settings.
func WithEnvProxy() ClientOptionsFunc {
	return func(o *clientOptions) error {
		o.envProxy = true
		return nil
	}
}

// WithAuthToken returns a ClientOptionsFunc that sets the authentication token
// for a Client. If not set, the client will make unauthenticated requests.
func WithAuthToken(token string) ClientOptionsFunc {
	return func(o *clientOptions) error {
		if token == "" {
			return errors.New("token must not be empty")
		}

		o.token = &token
		return nil
	}
}

// WithURLs returns a ClientOptionsFunc that sets the base and upload URLs
// while only validating the URL format. Nil values will be ignored and default
// URLs will be used.
func WithURLs(baseURL, uploadURL *string) ClientOptionsFunc {
	return func(o *clientOptions) error {
		if baseURL != nil {
			b, err := parseURL(*baseURL)
			if err != nil {
				return fmt.Errorf("invalid base url: %w", err)
			}

			o.baseURL = b
		}

		if uploadURL != nil {
			u, err := parseURL(*uploadURL)
			if err != nil {
				return fmt.Errorf("invalid upload url: %w", err)
			}

			o.uploadURL = u
		}

		return nil
	}
}

// WithEnterpriseURLs returns a ClientOptionsFunc that sets the base and upload
// URLs for a Client.
func WithEnterpriseURLs(baseURL, uploadURL string) ClientOptionsFunc {
	return func(o *clientOptions) error {
		b, err := parseURL(baseURL)
		if err != nil {
			return fmt.Errorf("invalid base url: %w", err)
		}

		if !strings.HasSuffix(b.Path, "/api/v3/") &&
			!strings.HasPrefix(b.Host, "api.") &&
			!strings.Contains(b.Host, ".api.") {
			b.Path += "api/v3/"
		}

		o.baseURL = b

		u, err := parseURL(uploadURL)
		if err != nil {
			return fmt.Errorf("invalid upload url: %w", err)
		}

		if !strings.HasSuffix(u.Path, "/api/uploads/") &&
			!strings.HasPrefix(u.Host, "api.") &&
			!strings.Contains(u.Host, ".api.") &&
			!strings.HasPrefix(u.Host, "uploads.") {
			u.Path += "api/uploads/"
		}

		o.uploadURL = u

		return nil
	}
}

// WithDisableRateLimitCheck returns a ClientOptionsFunc that disables rate
// limit checking for a Client. If not set, the client will check for rate
// limits and track them.
func WithDisableRateLimitCheck() ClientOptionsFunc {
	return func(o *clientOptions) error {
		o.disableRateLimitCheck = true
		return nil
	}
}

// WithRateLimitRedirectionalEndpoints returns a ClientOptionsFunc that
// configures the Client to respect rate limit headers on endpoints that
// return 302 redirection to artifacts. If not set, the client will not
// respect rate limit headers on these endpoints.
func WithRateLimitRedirectionalEndpoints() ClientOptionsFunc {
	return func(o *clientOptions) error {
		o.rateLimitRedirectionalEndpoints = true
		return nil
	}
}

// WithMaxSecondaryRateLimitRetryAfterDuration returns a ClientOptionsFunc
// that configures the Client secondary rate limit max retry after duration.
func WithMaxSecondaryRateLimitRetryAfterDuration(duration time.Duration) ClientOptionsFunc {
	return func(o *clientOptions) error {
		o.maxSecondaryRateLimitRetryAfterDuration = &duration
		return nil
	}
}

// NewClient returns a new GitHub API client configured with the provided
// options. The default configuration is suitable for making unauthenticated
// requests to the public GitHub API.
//
// For GitHub Enterprise, use [WithEnterpriseURLs] to set the base and upload
// URLs.
//
// To make authenticated requests, use [WithAuthToken] or [WithHTTPClient] to
// pass in a [http.Client] that performs authentication
// (e.g., using golang.org/x/oauth2).
//
// For production usage it is recommended to provide a timeout using
// [WithTimeout] or by providing a custom [http.Client] with an appropriate
// timeout using [WithHTTPClient].
func NewClient(opts ...ClientOptionsFunc) (*Client, error) {
	o := clientOptions{}
	for _, opt := range opts {
		if err := opt(&o); err != nil {
			return nil, err
		}
	}

	return newClient(o)
}

// newClient creates a new Client with the provided options. This is an internal
// helper function that is called by [NewClient] and [Client.Clone].
func newClient(opts clientOptions) (*Client, error) {
	c := &Client{
		apiVersionDefault: api20221128,
		apiVersionMin:     api20221128,
		apiVersionMax:     api20260310,
	}

	if opts.httpClient != nil {
		c.client = opts.httpClient
	} else {
		c.client = &http.Client{}
	}

	if opts.transport != nil {
		c.client.Transport = opts.transport
	}

	if opts.timeout != nil {
		c.client.Timeout = *opts.timeout
	}

	if opts.envProxy {
		transport := c.client.Transport
		if transport == nil {
			transport = http.DefaultTransport
		}

		t, ok := transport.(*http.Transport)
		if !ok {
			return nil, errors.New("cannot set environment proxy on non-http transport")
		}

		t2 := t.Clone()
		t2.Proxy = http.ProxyFromEnvironment
		c.client.Transport = t2
	}

	if opts.token != nil {
		transport := c.client.Transport
		if transport == nil {
			transport = http.DefaultTransport
		}
		c.client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			req = req.Clone(req.Context())
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", *opts.token))
			return transport.RoundTrip(req)
		})
	}

	c.clientIgnoreRedirects = &http.Client{
		Transport:     c.client.Transport,
		Timeout:       c.client.Timeout,
		Jar:           c.client.Jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	if opts.apiVersionMin != nil {
		c.apiVersionMin = *opts.apiVersionMin
	}

	if opts.apiVersionMax != nil {
		c.apiVersionMax = *opts.apiVersionMax
	}

	if opts.userAgent != nil {
		c.userAgent = *opts.userAgent
	} else {
		c.userAgent = defaultUserAgent
	}

	if opts.baseURL != nil {
		c.baseURL = opts.baseURL
	} else {
		c.baseURL, _ = url.Parse(defaultBaseURL)
	}

	if opts.uploadURL != nil {
		c.uploadURL = opts.uploadURL
	} else {
		c.uploadURL, _ = url.Parse(uploadBaseURL)
	}

	c.disableRateLimitCheck = opts.disableRateLimitCheck

	if !c.disableRateLimitCheck {
		c.rateLimitRedirectionalEndpoints = opts.rateLimitRedirectionalEndpoints

		if opts.maxSecondaryRateLimitRetryAfterDuration != nil {
			c.maxSecondaryRateLimitRetryAfterDuration = *opts.maxSecondaryRateLimitRetryAfterDuration
		}
	}

	c.common.client = c
	c.Actions = (*ActionsService)(&c.common)
	c.Activity = (*ActivityService)(&c.common)
	c.Admin = (*AdminService)(&c.common)
	c.Apps = (*AppsService)(&c.common)
	c.Authorizations = (*AuthorizationsService)(&c.common)
	c.Billing = (*BillingService)(&c.common)
	c.Checks = (*ChecksService)(&c.common)
	c.Classroom = (*ClassroomService)(&c.common)
	c.CodeQuality = (*CodeQualityService)(&c.common)
	c.CodeScanning = (*CodeScanningService)(&c.common)
	c.Codespaces = (*CodespacesService)(&c.common)
	c.CodesOfConduct = (*CodesOfConductService)(&c.common)
	c.Copilot = (*CopilotService)(&c.common)
	c.Credentials = (*CredentialsService)(&c.common)
	c.Dependabot = (*DependabotService)(&c.common)
	c.DependencyGraph = (*DependencyGraphService)(&c.common)
	c.Emojis = (*EmojisService)(&c.common)
	c.Enterprise = (*EnterpriseService)(&c.common)
	c.Gists = (*GistsService)(&c.common)
	c.Git = (*GitService)(&c.common)
	c.Gitignores = (*GitignoresService)(&c.common)
	c.Interactions = (*InteractionsService)(&c.common)
	c.IssueImport = (*IssueImportService)(&c.common)
	c.Issues = (*IssuesService)(&c.common)
	c.Licenses = (*LicensesService)(&c.common)
	c.Markdown = (*MarkdownService)(&c.common)
	c.Marketplace = &MarketplaceService{client: c, Stubbed: opts.marketplaceStubbed}
	c.Meta = (*MetaService)(&c.common)
	c.Migrations = (*MigrationService)(&c.common)
	c.Organizations = (*OrganizationsService)(&c.common)
	c.PrivateRegistries = (*PrivateRegistriesService)(&c.common)
	c.Projects = (*ProjectsService)(&c.common)
	c.PullRequests = (*PullRequestsService)(&c.common)
	c.RateLimit = (*RateLimitService)(&c.common)
	c.Reactions = (*ReactionsService)(&c.common)
	c.Repositories = (*RepositoriesService)(&c.common)
	c.SCIM = (*SCIMService)(&c.common)
	c.Search = (*SearchService)(&c.common)
	c.SecretScanning = (*SecretScanningService)(&c.common)
	c.SecurityAdvisories = (*SecurityAdvisoriesService)(&c.common)
	c.SubIssue = (*SubIssueService)(&c.common)
	c.Teams = (*TeamsService)(&c.common)
	c.Users = (*UsersService)(&c.common)

	return c, nil
}

// UserAgent returns the User-Agent header value for the client.
func (c *Client) UserAgent() string {
	return c.userAgent
}

// BaseURL returns the base URL for API requests.
func (c *Client) BaseURL() string {
	if c.baseURL == nil {
		return ""
	}
	return c.baseURL.String()
}

// UploadURL returns the base URL for upload API requests.
func (c *Client) UploadURL() string {
	if c.uploadURL == nil {
		return ""
	}
	return c.uploadURL.String()
}

// Clone returns a copy of the client with the same configuration and services.
// The returned client has its own http.Client but shares the client
// configuration such as transport and timeout. The returned client starts with
// the same rate limit information as the original client, but it is not
// updated when the original client's rate limit information is updated.
// The returned client is independent of the original client and can be
// modified without affecting the original client.
func (c *Client) Clone(opts ...ClientOptionsFunc) (*Client, error) {
	if c.client == nil {
		return nil, errUninitialized
	}

	o := clientOptions{
		apiVersionMin:                           &c.apiVersionMin,
		apiVersionMax:                           &c.apiVersionMax,
		userAgent:                               &c.userAgent,
		baseURL:                                 Ptr(*c.baseURL),
		uploadURL:                               Ptr(*c.uploadURL),
		disableRateLimitCheck:                   c.disableRateLimitCheck,
		rateLimitRedirectionalEndpoints:         c.rateLimitRedirectionalEndpoints,
		maxSecondaryRateLimitRetryAfterDuration: &c.maxSecondaryRateLimitRetryAfterDuration,
	}

	if c.Marketplace != nil {
		o.marketplaceStubbed = c.Marketplace.Stubbed
	}

	for _, opt := range opts {
		if err := opt(&o); err != nil {
			return nil, err
		}
	}

	if o.httpClient == nil {
		o.httpClient = &http.Client{
			Transport:     c.client.Transport,
			CheckRedirect: c.client.CheckRedirect,
			Jar:           c.client.Jar,
			Timeout:       c.client.Timeout,
		}
	}

	clone, err := newClient(o)
	if err != nil {
		return nil, err
	}

	if !clone.disableRateLimitCheck {
		c.rateMu.Lock()
		clone.rateLimits = c.rateLimits
		clone.secondaryRateLimitReset = c.secondaryRateLimitReset
		c.rateMu.Unlock()
	}

	return clone, nil
}

// RequestOption represents an option that can modify an http.Request.
type RequestOption func(req *http.Request)

// WithVersion overrides the GitHub v3 API version for this individual request.
// For more information, see:
// https://github.blog/2022-11-28-to-infinity-and-beyond-enabling-the-future-of-githubs-rest-api-with-api-versioning/
func WithVersion(version string) RequestOption {
	return func(req *http.Request) {
		req.Header.Set(headerAPIVersion, version)
	}
}

// NewRequest creates an API request. A relative URL can be provided in urlStr,
// in which case it is resolved relative to the BaseURL of the Client.
// Relative URLs should always be specified without a preceding slash. If
// specified, the value pointed to by body is JSON encoded and included as the
// request body.
func (c *Client) NewRequest(ctx context.Context, method, urlStr string, body any, opts ...RequestOption) (*http.Request, error) {
	if !strings.HasSuffix(c.baseURL.Path, "/") {
		return nil, fmt.Errorf("baseURL must have a trailing slash, but %q does not", c.baseURL)
	}

	if err := checkURLPathTraversal(urlStr); err != nil {
		return nil, err
	}

	u, err := c.baseURL.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	var buf io.ReadWriter
	if body != nil {
		buf = &bytes.Buffer{}
		enc := json.NewEncoder(buf)
		enc.SetEscapeHTML(false)
		err := enc.Encode(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), buf)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", mediaTypeV3)
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	req.Header.Set(headerAPIVersion, c.apiVersionDefault)

	for _, opt := range opts {
		opt(req)
	}

	return req, nil
}

// NewFormRequest creates an API request. A relative URL can be provided in urlStr,
// in which case it is resolved relative to the BaseURL of the Client.
// Relative URLs should always be specified without a preceding slash.
// Body is sent with Content-Type: application/x-www-form-urlencoded.
func (c *Client) NewFormRequest(ctx context.Context, urlStr string, body io.Reader, opts ...RequestOption) (*http.Request, error) {
	if !strings.HasSuffix(c.baseURL.Path, "/") {
		return nil, fmt.Errorf("baseURL must have a trailing slash, but %q does not", c.baseURL)
	}

	if err := checkURLPathTraversal(urlStr); err != nil {
		return nil, err
	}

	u, err := c.baseURL.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", u.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", mediaTypeV3)
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	req.Header.Set(headerAPIVersion, c.apiVersionDefault)

	for _, opt := range opts {
		opt(req)
	}

	return req, nil
}

// checkURLPathTraversal returns ErrPathForbidden if urlStr contains ".." as a
// path segment (e.g. "a/../b"), preventing path traversal attacks. Percent-
// encoded equivalents such as "%2e%2e" are also rejected because url.Parse
// decodes them to ".." before the check runs. It does not match ".." embedded
// within a segment (e.g. "file..txt"). The check is performed only on the path
// portion of the URL, ignoring any query string or fragment.
func checkURLPathTraversal(urlStr string) error {
	u, err := url.Parse(urlStr)
	if err != nil {
		return err
	}
	if slices.Contains(strings.Split(u.Path, "/"), "..") {
		return ErrPathForbidden
	}
	return nil
}

// NewUploadRequest creates an upload request. A relative URL can be provided in
// urlStr, in which case it is resolved relative to the UploadURL of the Client.
// Relative URLs should always be specified without a preceding slash.
func (c *Client) NewUploadRequest(ctx context.Context, urlStr string, reader io.Reader, size int64, mediaType string, opts ...RequestOption) (*http.Request, error) {
	if !strings.HasSuffix(c.uploadURL.Path, "/") {
		return nil, fmt.Errorf("uploadURL must have a trailing slash, but %q does not", c.uploadURL)
	}

	if err := checkURLPathTraversal(urlStr); err != nil {
		return nil, err
	}

	u, err := c.uploadURL.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	requestBody := reader
	if reader != nil {
		// Wrap the provided reader so transport code does not observe concrete body types
		// (for example *os.File) and switch to platform-specific sendfile fast paths.
		//
		// Why this exists:
		// race-enabled test runs on Windows have surfaced data races in the sendfile path
		// while request read/write loops run concurrently. Hiding concrete type information
		// keeps uploads on the generic io.Reader copy path, which is race-stable and preserves
		// request semantics (same bytes, same headers, same content length).
		requestBody = uploadRequestBodyReader{Reader: reader}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", u.String(), requestBody)
	if err != nil {
		return nil, err
	}

	req.ContentLength = size

	// Set GetBody so HTTP/2 transports can retry the request when a stream is
	// refused (REFUSED_STREAM / GOAWAY) after the body has been written. Without
	// GetBody, net/http2 fails with "cannot retry ... after Request.Body was
	// written". See https://github.com/google/go-github/issues/2113.
	//
	// http.Request.GetBody must return an independent copy of the body, so we
	// require io.ReaderAt (to read without moving the original body's position)
	// in addition to io.Seeker (to record the body's starting offset). This is
	// satisfied by *os.File, *bytes.Reader and *strings.Reader. Readers that
	// cannot provide an independent, rewindable view are left without GetBody so
	// that large uploads are never buffered in memory.
	seeker, isSeeker := reader.(io.Seeker)
	readerAt, isReaderAt := reader.(io.ReaderAt)
	if isSeeker && isReaderAt {
		offset, err := seeker.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, err
		}
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(uploadRequestBodyReader{Reader: io.NewSectionReader(readerAt, offset, size)}), nil
		}
	}

	if mediaType == "" {
		mediaType = defaultMediaType
	}
	req.Header.Set("Content-Type", mediaType)
	req.Header.Set("Accept", mediaTypeV3)
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set(headerAPIVersion, c.apiVersionDefault)

	for _, opt := range opts {
		opt(req)
	}

	return req, nil
}

// uploadRequestBodyReader intentionally wraps an io.Reader to hide concrete reader types.
// See NewUploadRequest for why this prevents race-prone transport optimizations.
type uploadRequestBodyReader struct {
	io.Reader
}

// Response is a GitHub API response. This wraps the standard http.Response
// returned from GitHub and provides convenient access to things like
// pagination links.
type Response struct {
	*http.Response

	// These fields provide the page values for paginating through a set of
	// results. Any or all of these may be set to the zero value for
	// responses that are not part of a paginated set, or for which there
	// are no additional pages.
	//
	// These fields support what is called "offset pagination" and should
	// be used with the ListOptions struct.
	NextPage  int
	PrevPage  int
	FirstPage int
	LastPage  int

	// Additionally, some APIs support "cursor pagination" instead of offset.
	// This means that a token points directly to the next record which
	// can lead to O(1) performance compared to O(n) performance provided
	// by offset pagination.
	//
	// For APIs that support cursor pagination (such as
	// TeamsService.ListIDPGroupsInOrganization), the following field
	// will be populated to point to the next page.
	//
	// To use this token, set ListCursorOptions.Page to this value before
	// calling the endpoint again.
	NextPageToken string

	// For APIs that support cursor pagination, such as RepositoriesService.ListHookDeliveries,
	// the following field will be populated to point to the next page.
	// Set ListCursorOptions.Cursor to this value when calling the endpoint again.
	Cursor string

	// For APIs that support before/after pagination, such as OrganizationsService.AuditLog.
	Before string
	After  string

	// Explicitly specify the Rate type so Rate's String() receiver doesn't
	// propagate to Response.
	Rate Rate

	// token's expiration date. Timestamp is 0001-01-01 when token doesn't expire.
	// So it is valid for TokenExpiration.Equal(Timestamp{}) or TokenExpiration.Time.After(time.Now())
	TokenExpiration Timestamp
}

// newResponse creates a new Response for the provided http.Response.
// r must not be nil.
func newResponse(r *http.Response) *Response {
	response := &Response{Response: r}
	response.populatePageValues()
	response.Rate = parseRate(r)
	response.TokenExpiration = parseTokenExpiration(r)
	return response
}

// populatePageValues parses the HTTP Link response headers and populates the
// various pagination link values in the Response.
func (r *Response) populatePageValues() {
	if links, ok := r.Response.Header["Link"]; ok && len(links) > 0 {
		for link := range strings.SplitSeq(links[0], ",") {
			segments := strings.Split(strings.TrimSpace(link), ";")

			// link must at least have href and rel
			if len(segments) < 2 {
				continue
			}

			// ensure href is properly formatted
			if !strings.HasPrefix(segments[0], "<") || !strings.HasSuffix(segments[0], ">") {
				continue
			}

			// try to pull out page parameter
			url, err := url.Parse(segments[0][1 : len(segments[0])-1])
			if err != nil {
				continue
			}

			q := url.Query()

			if cursor := q.Get("cursor"); cursor != "" {
				for _, segment := range segments[1:] {
					if strings.TrimSpace(segment) == `rel="next"` {
						r.Cursor = cursor
					}
				}

				continue
			}

			page := q.Get("page")
			since := q.Get("since")
			before := q.Get("before")
			after := q.Get("after")

			if page == "" && before == "" && after == "" && since == "" {
				continue
			}

			if since != "" && page == "" {
				page = since
			}

			for _, segment := range segments[1:] {
				switch strings.TrimSpace(segment) {
				case `rel="next"`:
					if r.NextPage, err = strconv.Atoi(page); err != nil {
						r.NextPageToken = page
					}
					r.After = after
				case `rel="prev"`:
					r.PrevPage, _ = strconv.Atoi(page)
					r.Before = before
				case `rel="first"`:
					r.FirstPage, _ = strconv.Atoi(page)
				case `rel="last"`:
					r.LastPage, _ = strconv.Atoi(page)
				}
			}
		}
	}
}

// parseRate parses the rate related headers.
func parseRate(r *http.Response) Rate {
	var rate Rate
	if limit := r.Header.Get(HeaderRateLimit); limit != "" {
		rate.Limit, _ = strconv.Atoi(limit)
	}
	if remaining := r.Header.Get(HeaderRateRemaining); remaining != "" {
		rate.Remaining, _ = strconv.Atoi(remaining)
	}
	if used := r.Header.Get(HeaderRateUsed); used != "" {
		rate.Used, _ = strconv.Atoi(used)
	}
	if reset := r.Header.Get(HeaderRateReset); reset != "" {
		if v, _ := strconv.ParseInt(reset, 10, 64); v != 0 {
			rate.Reset = Timestamp{time.Unix(v, 0)}
		}
	}
	if resource := r.Header.Get(HeaderRateResource); resource != "" {
		rate.Resource = resource
	}
	return rate
}

// parseSecondaryRate parses the secondary rate related headers,
// and returns the time to retry after.
func parseSecondaryRate(r *http.Response) *time.Duration {
	// According to GitHub support, the "Retry-After" header value will be
	// an integer which represents the number of seconds that one should
	// wait before resuming making requests.
	if v := r.Header.Get(headerRetryAfter); v != "" {
		retryAfterSeconds, _ := strconv.ParseInt(v, 10, 64) // Error handling is noop.
		retryAfter := time.Duration(retryAfterSeconds) * time.Second
		return &retryAfter
	}

	// According to GitHub support, endpoints might return x-ratelimit-reset instead,
	// as an integer which represents the number of seconds since epoch UTC,
	// representing the time to resume making requests.
	if v := r.Header.Get(HeaderRateReset); v != "" {
		secondsSinceEpoch, _ := strconv.ParseInt(v, 10, 64) // Error handling is noop.
		retryAfter := time.Until(time.Unix(secondsSinceEpoch, 0))
		return &retryAfter
	}

	return nil
}

// parseTokenExpiration parses the TokenExpiration related headers.
// Returns 0001-01-01 if the header is not defined or could not be parsed.
func parseTokenExpiration(r *http.Response) Timestamp {
	if v := r.Header.Get(headerTokenExpiration); v != "" {
		if t, err := time.Parse("2006-01-02 15:04:05 MST", v); err == nil {
			return Timestamp{t.Local()}
		}
		// Some tokens include the timezone offset instead of the timezone.
		// https://github.com/google/go-github/issues/2649
		if t, err := time.Parse("2006-01-02 15:04:05 -0700", v); err == nil {
			return Timestamp{t.Local()}
		}
	}
	return Timestamp{} // 0001-01-01 00:00:00
}

type requestContext uint8

const (
	// BypassRateLimitCheck prevents a pre-emptive check for exceeded primary rate limits
	// Specify this by providing a context with this key, e.g.
	//   context.WithValue(context.Background(), github.BypassRateLimitCheck, true)
	BypassRateLimitCheck requestContext = iota

	SleepUntilPrimaryRateLimitResetWhenRateLimited
)

// maxErrorBodySize is the maximum number of bytes read from an HTTP error
// response body. Limits memory allocation when a server returns an
// unexpectedly large error body.
const maxErrorBodySize = 1 * 1024 * 1024 // 1 MiB

// ErrUnsupportedAPIVersion is returned when the API version specified in the
// request is not supported by the client.
var ErrUnsupportedAPIVersion = errors.New("unsupported api version")

// checkRequestAPIVersionBeforeDo checks if the API version specified in the
// request is supported by the client before making the API call. If the
// version is not supported, it returns [ErrUnsupportedAPIVersion]. If the
// version is empty it returns nil.
func (c *Client) checkRequestAPIVersionBeforeDo(req *http.Request) error {
	reqAPIVersion := req.Header.Get(headerAPIVersion)

	if reqAPIVersion == "" {
		return nil
	}

	if reqAPIVersion < c.apiVersionMin || reqAPIVersion > c.apiVersionMax {
		return ErrUnsupportedAPIVersion
	}

	return nil
}

// bareDo sends an API request using `caller` http.Client passed in the parameters
// and lets you handle the api response. If an error or API Error occurs, the error
// will contain more information. Otherwise, you are supposed to read and close the
// response's Body. If rate limit is exceeded and reset time is in the future,
// bareDo returns *RateLimitError immediately without making a network API call.
func (c *Client) bareDo(caller *http.Client, req *http.Request) (*Response, error) {
	ctx := req.Context()

	if err := c.checkRequestAPIVersionBeforeDo(req); err != nil {
		return nil, err
	}

	rateLimitCategory := CoreCategory

	if !c.disableRateLimitCheck {
		rateLimitCategory = GetRateLimitCategory(req.Method, req.URL.Path)

		if bypass := ctx.Value(BypassRateLimitCheck); bypass == nil {
			// If we've hit rate limit, don't make further requests before Reset time.
			if err := c.checkRateLimitBeforeDo(req, rateLimitCategory); err != nil {
				return &Response{
					Response: err.Response,
					Rate:     err.Rate,
				}, err
			}

			// If we've hit a secondary rate limit, don't make further requests before Retry After.
			if err := c.checkSecondaryRateLimitBeforeDo(req); err != nil {
				return &Response{
					Response: err.Response,
				}, err
			}
		}
	}

	resp, err := caller.Do(req)
	var response *Response
	if resp != nil {
		response = newResponse(resp)
	}

	if err != nil {
		// If we got an error, and the context has been canceled,
		// the context's error is probably more useful.
		select {
		case <-ctx.Done():
			return response, ctx.Err()
		default:
		}

		// If the error type is *url.Error, sanitize its URL before returning.
		var e *url.Error
		if errors.As(err, &e) {
			if url, err := url.Parse(e.URL); err == nil {
				e.URL = sanitizeURL(url).String()
				return response, e
			}
		}

		return response, err
	}

	// Don't update the rate limits if the client has rate limits disabled or if
	// this was a cached response. The X-From-Cache is set by
	// https://github.com/bartventer/httpcache if it's enabled.
	if !c.disableRateLimitCheck && response.Header.Get("X-From-Cache") == "" {
		c.rateMu.Lock()
		c.rateLimits[rateLimitCategory] = response.Rate
		c.rateMu.Unlock()
	}

	err = CheckResponse(resp)
	if err != nil {
		defer resp.Body.Close()
		// Special case for AcceptedErrors. If an AcceptedError
		// has been encountered, the response's payload will be
		// added to the AcceptedError and returned.
		//
		// Issue #1022
		var aerr *AcceptedError
		if errors.As(err, &aerr) {
			b, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))
			if readErr != nil {
				return response, readErr
			}

			aerr.Raw = b
			err = aerr
		}

		var rateLimitError *RateLimitError
		if errors.As(err, &rateLimitError) &&
			ctx.Value(SleepUntilPrimaryRateLimitResetWhenRateLimited) != nil {
			if err := sleepUntilResetWithBuffer(ctx, rateLimitError.Rate.Reset.Time); err != nil {
				return response, err
			}
			// retry the request now the rate limit should have been reset
			newReq := req.Clone(context.WithValue(ctx, SleepUntilPrimaryRateLimitResetWhenRateLimited, nil))
			return c.bareDo(caller, newReq)
		}

		// Update the secondary rate limit if we hit it.
		var rerr *AbuseRateLimitError
		if errors.As(err, &rerr) && rerr.RetryAfter != nil {
			// if a max duration is specified, make sure that we are waiting at most this duration
			if c.maxSecondaryRateLimitRetryAfterDuration > 0 && rerr.GetRetryAfter() > c.maxSecondaryRateLimitRetryAfterDuration {
				rerr.RetryAfter = &c.maxSecondaryRateLimitRetryAfterDuration
			}
			c.rateMu.Lock()
			c.secondaryRateLimitReset = time.Now().Add(*rerr.RetryAfter)
			c.rateMu.Unlock()
		}
	}
	return response, err
}

// BareDo sends an API request and lets you handle the api response. If an error
// or API Error occurs, the error will contain more information. Otherwise, you
// are supposed to read and close the response's Body. If rate limit is exceeded
// and reset time is in the future, BareDo returns *RateLimitError immediately
// without making a network API call.
func (c *Client) BareDo(req *http.Request) (*Response, error) {
	return c.bareDo(c.client, req)
}

// bareDoIgnoreRedirects has the exact same behavior as BareDo but stops at the first
// redirection code returned by the API. If a redirection is returned by the api, bareDoIgnoreRedirects
// returns a *RedirectionError.
func (c *Client) bareDoIgnoreRedirects(req *http.Request) (*Response, error) {
	return c.bareDo(c.clientIgnoreRedirects, req)
}

var errInvalidLocation = errors.New("invalid or empty Location header in redirection response")

// bareDoUntilFound has the exact same behavior as BareDo but only follows 301s, up to maxRedirects times. If it receives
// a 302, it will parse the Location header into a *url.URL and return that.
// This is useful for endpoints that return a 302 in successful cases but still might return 301s for
// permanent redirections.
func (c *Client) bareDoUntilFound(req *http.Request, maxRedirects int) (*url.URL, *Response, error) {
	response, err := c.bareDoIgnoreRedirects(req)
	if err != nil {
		var rerr *RedirectionError
		if errors.As(err, &rerr) {
			// If we receive a 302, transform potential relative locations into absolute and return it.
			if rerr.StatusCode == http.StatusFound {
				if rerr.Location == nil {
					return nil, nil, errInvalidLocation
				}
				newURL := c.baseURL.ResolveReference(rerr.Location)
				return newURL, response, nil
			}
			// If permanent redirect response is returned, follow it
			if maxRedirects > 0 && rerr.StatusCode == http.StatusMovedPermanently {
				if rerr.Location == nil {
					return nil, nil, errInvalidLocation
				}
				newURL := c.baseURL.ResolveReference(rerr.Location)
				// Refuse to follow a permanent redirect to a different host:
				// req.Clone preserves Authorization headers added by the auth
				// transport, so a cross-host target would leak credentials.
				if newURL.Host != c.baseURL.Host {
					return nil, response, fmt.Errorf("refusing to follow cross-host redirect from %q to %q", c.baseURL.Host, newURL.Host)
				}
				newRequest := req.Clone(req.Context())
				newRequest.URL = newURL
				return c.bareDoUntilFound(newRequest, maxRedirects-1)
			}
			// If we reached the maximum amount of redirections, return an error
			if maxRedirects <= 0 && rerr.StatusCode == http.StatusMovedPermanently {
				return nil, response, fmt.Errorf("reached the maximum amount of redirections: %w", err)
			}
			return nil, response, fmt.Errorf("unexpected redirection response: %w", err)
		}
	}

	// If we don't receive a redirection, forward the response and potential error
	return nil, response, err
}

// Do sends an API request and returns the API response. The API response is
// JSON decoded and stored in the value pointed to by v, or returned as an
// error if an API error has occurred. If v implements the io.Writer interface,
// the raw response body will be written to v, without attempting to first
// decode it. If v is nil, and no error happens, the response is returned as is.
// If rate limit is exceeded and reset time is in the future, Do returns
// *RateLimitError immediately without making a network API call.
func (c *Client) Do(req *http.Request, v any) (*Response, error) {
	resp, err := c.BareDo(req)
	if err != nil {
		return resp, err
	}
	defer resp.Body.Close()

	switch v := v.(type) {
	case nil:
	case io.Writer:
		_, err = io.Copy(v, resp.Body)
	default:
		decErr := json.NewDecoder(resp.Body).Decode(v)
		if decErr == io.EOF {
			decErr = nil // ignore EOF errors caused by empty response body
		}
		if decErr != nil {
			err = decErr
		}
	}
	return resp, err
}

// checkRateLimitBeforeDo does not make any network calls, but uses existing knowledge from
// current client state in order to quickly check if *RateLimitError can be immediately returned
// from Client.Do, and if so, returns it so that Client.Do can skip making a network API call unnecessarily.
// Otherwise it returns nil, and Client.Do should proceed normally.
func (c *Client) checkRateLimitBeforeDo(req *http.Request, rateLimitCategory RateLimitCategory) *RateLimitError {
	ctx := req.Context()

	c.rateMu.Lock()
	rate := c.rateLimits[rateLimitCategory]
	c.rateMu.Unlock()
	if !rate.Reset.Time.IsZero() && rate.Remaining == 0 && time.Now().Before(rate.Reset.Time) {
		// Create a fake response.
		resp := &http.Response{
			Status:     http.StatusText(http.StatusForbidden),
			StatusCode: http.StatusForbidden,
			Request:    req,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("")),
		}

		if ctx.Value(SleepUntilPrimaryRateLimitResetWhenRateLimited) != nil {
			if err := sleepUntilResetWithBuffer(ctx, rate.Reset.Time); err == nil {
				return nil
			}
			return &RateLimitError{
				Rate:     rate,
				Response: resp,
				Message:  fmt.Sprintf("Context cancelled while waiting for rate limit to reset until %v, not making remote request.", rate.Reset.Time),
			}
		}

		return &RateLimitError{
			Rate:     rate,
			Response: resp,
			Message:  fmt.Sprintf("API rate limit of %v still exceeded until %v, not making remote request.", rate.Limit, rate.Reset.Time),
		}
	}

	return nil
}

// checkSecondaryRateLimitBeforeDo does not make any network calls, but uses existing knowledge from
// current client state in order to quickly check if *AbuseRateLimitError can be immediately returned
// from Client.Do, and if so, returns it so that Client.Do can skip making a network API call unnecessarily.
// Otherwise it returns nil, and Client.Do should proceed normally.
func (c *Client) checkSecondaryRateLimitBeforeDo(req *http.Request) *AbuseRateLimitError {
	c.rateMu.Lock()
	secondary := c.secondaryRateLimitReset
	c.rateMu.Unlock()
	if !secondary.IsZero() && time.Now().Before(secondary) {
		// Create a fake response.
		resp := &http.Response{
			Status:     http.StatusText(http.StatusForbidden),
			StatusCode: http.StatusForbidden,
			Request:    req,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("")),
		}

		retryAfter := time.Until(secondary)
		return &AbuseRateLimitError{
			Response:   resp,
			Message:    fmt.Sprintf("API secondary rate limit exceeded until %v, not making remote request.", secondary),
			RetryAfter: &retryAfter,
		}
	}

	return nil
}

// compareHTTPResponse returns whether two http.Response objects are equal or not.
// Currently, only StatusCode is checked. This function is used when implementing the
// Is(error) bool interface for the custom error types in this package.
func compareHTTPResponse(r1, r2 *http.Response) bool {
	if r1 == nil && r2 == nil {
		return true
	}

	if r1 != nil && r2 != nil {
		return r1.StatusCode == r2.StatusCode
	}
	return false
}

/*
An ErrorResponse reports one or more errors caused by an API request.

GitHub API docs: https://docs.github.com/rest?apiVersion=2022-11-28#client-errors
*/
type ErrorResponse struct {
	Response *http.Response `json:"-"`       // HTTP response that caused this error
	Message  string         `json:"message"` // error message
	//nolint:sliceofpointers
	Errors []Error `json:"errors"` // more detail on individual errors
	// Block is only populated on certain types of errors such as code 451.
	Block *ErrorBlock `json:"block,omitempty"`
	// Most errors will also include a documentation_url field pointing
	// to some content that might help you resolve the error, see
	// https://docs.github.com/rest?apiVersion=2022-11-28#client-errors
	DocumentationURL string `json:"documentation_url,omitempty"`
}

// ErrorBlock contains a further explanation for the reason of an error.
// See https://developer.github.com/changes/2016-03-17-the-451-status-code-is-now-supported/
// for more information.
type ErrorBlock struct {
	Reason    string     `json:"reason,omitempty"`
	CreatedAt *Timestamp `json:"created_at,omitempty"`
}

func (r *ErrorResponse) Error() string {
	if r.Response != nil && r.Response.Request != nil {
		return fmt.Sprintf("%v %v: %v %v %+v",
			r.Response.Request.Method, sanitizeURL(r.Response.Request.URL),
			r.Response.StatusCode, r.Message, r.Errors)
	}

	if r.Response != nil {
		return fmt.Sprintf("%v %v %+v", r.Response.StatusCode, r.Message, r.Errors)
	}

	return fmt.Sprintf("%v %+v", r.Message, r.Errors)
}

// Is returns whether the provided error equals this error.
func (r *ErrorResponse) Is(target error) bool {
	var v *ErrorResponse
	if !errors.As(target, &v) {
		return false
	}

	if r.Message != v.Message || (r.DocumentationURL != v.DocumentationURL) ||
		!compareHTTPResponse(r.Response, v.Response) {
		return false
	}

	// Compare Errors.
	if len(r.Errors) != len(v.Errors) {
		return false
	}
	for idx := range r.Errors {
		if r.Errors[idx] != v.Errors[idx] {
			return false
		}
	}

	// Compare Block.
	if (r.Block != nil && v.Block == nil) || (r.Block == nil && v.Block != nil) {
		return false
	}
	if r.Block != nil && v.Block != nil {
		if r.Block.Reason != v.Block.Reason {
			return false
		}
		if (r.Block.CreatedAt != nil && v.Block.CreatedAt == nil) || (r.Block.CreatedAt ==
			nil && v.Block.CreatedAt != nil) {
			return false
		}
		if r.Block.CreatedAt != nil && v.Block.CreatedAt != nil {
			if *(r.Block.CreatedAt) != *(v.Block.CreatedAt) {
				return false
			}
		}
	}

	return true
}

// TwoFactorAuthError occurs when using HTTP Basic Authentication for a user
// that has two-factor authentication enabled. The request can be reattempted
// by providing a one-time password in the request.
type TwoFactorAuthError ErrorResponse

func (r *TwoFactorAuthError) Error() string { return (*ErrorResponse)(r).Error() }

// RateLimitError occurs when GitHub returns 403 Forbidden response with a rate limit
// remaining value of 0.
type RateLimitError struct {
	Rate     Rate           // Rate specifies last known rate limit for the client
	Response *http.Response // HTTP response that caused this error
	Message  string         `json:"message"` // error message
}

func (r *RateLimitError) Error() string {
	return fmt.Sprintf("%v %v: %v %v %v",
		r.Response.Request.Method, sanitizeURL(r.Response.Request.URL),
		r.Response.StatusCode, r.Message, formatRateReset(time.Until(r.Rate.Reset.Time)))
}

// Is returns whether the provided error equals this error.
func (r *RateLimitError) Is(target error) bool {
	var v *RateLimitError
	if !errors.As(target, &v) {
		return false
	}

	return r.Rate == v.Rate &&
		r.Message == v.Message &&
		compareHTTPResponse(r.Response, v.Response)
}

// AcceptedError occurs when GitHub returns 202 Accepted response with an
// empty body, which means a job was scheduled on the GitHub side to process
// the information needed and cache it.
// Technically, 202 Accepted is not a real error, it's just used to
// indicate that results are not ready yet, but should be available soon.
// The request can be repeated after some time.
type AcceptedError struct {
	// Raw contains the response body.
	Raw []byte
}

func (*AcceptedError) Error() string {
	return "job scheduled on GitHub side; try again later"
}

// Is returns whether the provided error equals this error.
func (ae *AcceptedError) Is(target error) bool {
	var v *AcceptedError
	if !errors.As(target, &v) {
		return false
	}
	return bytes.Equal(ae.Raw, v.Raw)
}

// AbuseRateLimitError occurs when GitHub returns 403 Forbidden response with the
// "documentation_url" field value equal to "https://docs.github.com/rest/overview/rate-limits-for-the-rest-api?apiVersion=2022-11-28#about-secondary-rate-limits".
type AbuseRateLimitError struct {
	Response *http.Response // HTTP response that caused this error
	Message  string         `json:"message"` // error message

	// RetryAfter is provided with some abuse rate limit errors. If present,
	// it is the amount of time that the client should wait before retrying.
	// Otherwise, the client should try again later (after an unspecified amount of time).
	RetryAfter *time.Duration
}

func (r *AbuseRateLimitError) Error() string {
	retryInfo := ""
	if r.RetryAfter != nil && *r.RetryAfter > 0 {
		retryInfo = fmt.Sprintf(" [retry after %v]", r.RetryAfter.Round(time.Second))
	}
	return fmt.Sprintf("%v %v: %v %v%v",
		r.Response.Request.Method, sanitizeURL(r.Response.Request.URL),
		r.Response.StatusCode, r.Message, retryInfo)
}

// Is returns whether the provided error equals this error.
func (r *AbuseRateLimitError) Is(target error) bool {
	var v *AbuseRateLimitError
	if !errors.As(target, &v) {
		return false
	}

	return r.Message == v.Message &&
		equalDurationPtr(r.RetryAfter, v.RetryAfter) &&
		compareHTTPResponse(r.Response, v.Response)
}

func equalDurationPtr(a, b *time.Duration) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// RedirectionError represents a response that returned a redirect status code:
//
//	301 (Moved Permanently)
//	302 (Found)
//	303 (See Other)
//	307 (Temporary Redirect)
//	308 (Permanent Redirect)
//
// If there was a valid Location header included, it will be parsed to a URL. You should use
// `BaseURL.ResolveReference()` to enrich it with the correct hostname where needed.
type RedirectionError struct {
	Response   *http.Response // HTTP response that caused this error
	StatusCode int
	Location   *url.URL // location header of the redirection if present
}

func (r *RedirectionError) Error() string {
	return fmt.Sprintf("%v %v: %v location %v",
		r.Response.Request.Method, sanitizeURL(r.Response.Request.URL),
		r.StatusCode, sanitizeURL(r.Location))
}

// Is returns whether the provided error equals this error.
func (r *RedirectionError) Is(target error) bool {
	var v *RedirectionError
	if !errors.As(target, &v) {
		return false
	}

	return r.StatusCode == v.StatusCode &&
		(r.Location == v.Location || // either both locations are nil or exactly the same pointer
			r.Location != nil && v.Location != nil && r.Location.String() == v.Location.String()) // or they are both not nil and marshaled identically
}

var sensitiveParams = []string{"client_secret", "access_token", "token"}

// sanitizeURL redacts sensitive parameters from the URL which may be
// exposed to the user.
func sanitizeURL(uri *url.URL) *url.URL {
	if uri == nil {
		return nil
	}
	params := uri.Query()
	var redacted bool
	for _, p := range sensitiveParams {
		if len(params.Get(p)) > 0 {
			params.Set(p, "REDACTED")
			redacted = true
		}
	}
	if redacted {
		uri.RawQuery = params.Encode()
	}
	return uri
}

/*
An Error reports more details on an individual error in an ErrorResponse.
These are the possible validation error codes:

	missing:
	    resource does not exist
	missing_field:
	    a required field on a resource has not been set
	invalid:
	    the formatting of a field is invalid
	already_exists:
	    another resource has the same valid as this field
	custom:
	    some resources return this (e.g. github.User.CreateKey()), additional
	    information is set in the Message field of the Error

GitHub error responses structure are often undocumented and inconsistent.
Sometimes error is just a simple string (Issue #540).
In such cases, Message represents an error message as a workaround.

GitHub API docs: https://docs.github.com/rest?apiVersion=2022-11-28#client-errors
*/
type Error struct {
	Resource string `json:"resource"` // resource on which the error occurred
	Field    string `json:"field"`    // field on which the error occurred
	Code     string `json:"code"`     // validation error code
	Message  string `json:"message"`  // Message describing the error. Errors with Code == "custom" will always have this set.
}

func (e *Error) Error() string {
	return fmt.Sprintf("%v error caused by %v field on %v resource",
		e.Code, e.Field, e.Resource)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (e *Error) UnmarshalJSON(data []byte) error {
	type aliasError Error // avoid infinite recursion by using type alias.
	if err := json.Unmarshal(data, (*aliasError)(e)); err != nil {
		return json.Unmarshal(data, &e.Message) // data can be json string.
	}
	return nil
}

// CheckResponse checks the API response for errors, and returns them if
// present. A response is considered an error if it has a status code outside
// the 200 range or equal to 202 Accepted.
// API error responses are expected to have response
// body, and a JSON response body that maps to [ErrorResponse].
//
// The error type will be *[RateLimitError] for rate limit exceeded errors,
// *[AcceptedError] for 202 Accepted status codes,
// *[TwoFactorAuthError] for two-factor authentication errors,
// and *[RedirectionError] for redirect status codes (only happens when ignoring redirections).
func CheckResponse(r *http.Response) error {
	if r.StatusCode == http.StatusAccepted {
		return &AcceptedError{}
	}
	if c := r.StatusCode; 200 <= c && c <= 299 {
		return nil
	}

	errorResponse := &ErrorResponse{Response: r}
	data, err := io.ReadAll(io.LimitReader(r.Body, maxErrorBodySize))
	if err == nil && data != nil {
		err = json.Unmarshal(data, errorResponse)
		if err != nil {
			// reset the response as if this never happened
			errorResponse = &ErrorResponse{Response: r}
		}
	}
	// Re-populate error response body because GitHub error responses are often
	// undocumented and inconsistent.
	// Issue #1136, #540.
	r.Body = io.NopCloser(bytes.NewBuffer(data))
	switch {
	case r.StatusCode == http.StatusUnauthorized && strings.HasPrefix(r.Header.Get(headerOTP), "required"):
		return (*TwoFactorAuthError)(errorResponse)
	// Primary rate limit exceeded: GitHub returns 403 or 429 with X-RateLimit-Remaining: 0
	// See: https://docs.github.com/rest/using-the-rest-api/rate-limits-for-the-rest-api?apiVersion=2022-11-28
	case (r.StatusCode == http.StatusForbidden || r.StatusCode == http.StatusTooManyRequests) &&
		r.Header.Get(HeaderRateRemaining) == "0":
		return &RateLimitError{
			Rate:     parseRate(r),
			Response: errorResponse.Response,
			Message:  errorResponse.Message,
		}
	// Secondary rate limit exceeded: GitHub returns 403 or 429 with specific documentation_url
	// See: https://docs.github.com/rest/using-the-rest-api/rate-limits-for-the-rest-api?apiVersion=2022-11-28#about-secondary-rate-limits
	case (r.StatusCode == http.StatusForbidden || r.StatusCode == http.StatusTooManyRequests) &&
		(strings.HasSuffix(errorResponse.DocumentationURL, "#abuse-rate-limits") ||
			strings.HasSuffix(errorResponse.DocumentationURL, "secondary-rate-limits")):
		abuseRateLimitError := &AbuseRateLimitError{
			Response: errorResponse.Response,
			Message:  errorResponse.Message,
		}
		if retryAfter := parseSecondaryRate(r); retryAfter != nil {
			abuseRateLimitError.RetryAfter = retryAfter
		}
		return abuseRateLimitError
	// Check that the status code is a redirection and return a sentinel error that can be used to handle special cases
	// where 302 is considered a successful result.
	// This should never happen with the default `CheckRedirect`, because it would return a `url.Error` that should be handled upstream.
	case r.StatusCode == http.StatusMovedPermanently ||
		r.StatusCode == http.StatusFound ||
		r.StatusCode == http.StatusSeeOther ||
		r.StatusCode == http.StatusTemporaryRedirect ||
		r.StatusCode == http.StatusPermanentRedirect:

		locationStr := r.Header.Get("Location")
		var location *url.URL
		if locationStr != "" {
			location, _ = url.Parse(locationStr)
		}
		return &RedirectionError{
			Response:   errorResponse.Response,
			StatusCode: r.StatusCode,
			Location:   location,
		}
	default:
		return errorResponse
	}
}

// parseBoolResponse determines the boolean result from a GitHub API response.
// Several GitHub API methods return boolean responses indicated by the HTTP
// status code in the response (true indicated by a 204, false indicated by a
// 404). This helper function will determine that result and hide the 404
// error if present. Any other error will be returned through as-is.
func parseBoolResponse(err error) (bool, error) {
	if err == nil {
		return true, nil
	}

	var rerr *ErrorResponse
	if errors.As(err, &rerr) && rerr.Response.StatusCode == http.StatusNotFound {
		// Simply false. In this one case, we do not pass the error through.
		return false, nil
	}

	// some other real error occurred
	return false, err
}

// RateLimitCategory represents the enumeration of rate limit categories.
type RateLimitCategory uint8

const (
	CoreCategory RateLimitCategory = iota
	SearchCategory
	GraphqlCategory
	IntegrationManifestCategory
	SourceImportCategory
	CodeScanningUploadCategory
	ActionsRunnerRegistrationCategory
	ScimCategory
	DependencySnapshotsCategory
	CodeSearchCategory
	AuditLogCategory
	DependencySBOMCategory

	Categories // An array of this length will be able to contain all rate limit categories.
)

// GetRateLimitCategory returns the rate limit RateLimitCategory of the endpoint, determined by HTTP method and Request.URL.Path.
func GetRateLimitCategory(method, path string) RateLimitCategory {
	switch {
	// https://docs.github.com/rest/rate-limit?apiVersion=2022-11-28#about-rate-limits
	default:
		// NOTE: coreCategory is returned for actionsRunnerRegistrationCategory too,
		// because no API found for this category.
		return CoreCategory

	// https://docs.github.com/rest/search/search?apiVersion=2022-11-28#search-code
	case strings.HasPrefix(path, "/search/code") &&
		method == "GET":
		return CodeSearchCategory

	case strings.HasPrefix(path, "/search/"):
		return SearchCategory
	case path == "/graphql":
		return GraphqlCategory
	case strings.HasPrefix(path, "/app-manifests/") &&
		strings.HasSuffix(path, "/conversions") &&
		method == "POST":
		return IntegrationManifestCategory

	// https://docs.github.com/rest/migrations/source-imports?apiVersion=2022-11-28#start-an-import
	case strings.HasPrefix(path, "/repos/") &&
		strings.HasSuffix(path, "/import") &&
		method == "PUT":
		return SourceImportCategory

	// https://docs.github.com/rest/code-scanning?apiVersion=2022-11-28#upload-an-analysis-as-sarif-data
	case strings.HasSuffix(path, "/code-scanning/sarifs"):
		return CodeScanningUploadCategory

	// https://docs.github.com/enterprise-cloud@latest/rest/scim?apiVersion=2022-11-28
	case strings.HasPrefix(path, "/scim/"):
		return ScimCategory

	// https://docs.github.com/rest/dependency-graph/dependency-submission?apiVersion=2022-11-28#create-a-snapshot-of-dependencies-for-a-repository
	case strings.HasPrefix(path, "/repos/") &&
		strings.HasSuffix(path, "/dependency-graph/snapshots") &&
		method == "POST":
		return DependencySnapshotsCategory

	// https://docs.github.com/enterprise-cloud@latest/rest/orgs/orgs?apiVersion=2022-11-28#get-the-audit-log-for-an-organization
	case strings.HasSuffix(path, "/audit-log"):
		return AuditLogCategory

	// https://docs.github.com/rest/dependency-graph/sboms?apiVersion=2022-11-28#export-a-software-bill-of-materials-sbom-for-a-repository
	case strings.HasPrefix(path, "/repos/") &&
		strings.HasSuffix(path, "/dependency-graph/sbom"):
		return DependencySBOMCategory
	}
}

// RateLimits returns the rate limits for the current client.
//
// Deprecated: Use RateLimitService.Get instead.
func (c *Client) RateLimits(ctx context.Context) (*RateLimits, *Response, error) {
	return c.RateLimit.Get(ctx)
}

func setCredentialsAsHeaders(req *http.Request, id, secret string) *http.Request {
	// To set extra headers, we must make a copy of the Request so
	// that we don't modify the Request we were given. This is required by the
	// specification of http.RoundTripper.
	//
	// Since we are going to modify only req.Header here, we only need a deep copy
	// of req.Header.
	convertedRequest := *req
	convertedRequest.Header = make(http.Header, len(req.Header))

	for k, s := range req.Header {
		convertedRequest.Header[k] = append([]string(nil), s...)
	}
	convertedRequest.SetBasicAuth(id, secret)
	return &convertedRequest
}

/*
UnauthenticatedRateLimitedTransport allows you to make unauthenticated calls
that need to use a higher rate limit associated with your OAuth application.

	t := &github.UnauthenticatedRateLimitedTransport{
		ClientID:     "your app's client ID",
		ClientSecret: "your app's client secret",
	}
	client := github.NewClient(t.Client())

This will add the client id and secret as a base64-encoded string in the format
ClientID:ClientSecret and apply it as an "Authorization": "Basic" header.

See https://docs.github.com/rest/using-the-rest-api/rate-limits-for-the-rest-api?apiVersion=2022-11-28#primary-rate-limit-for-oauth-apps
for more information.
*/
type UnauthenticatedRateLimitedTransport struct {
	// ClientID is the GitHub OAuth client ID of the current application, which
	// can be found by selecting its entry in the list at
	// https://github.com/settings/applications.
	ClientID string

	// ClientSecret is the GitHub OAuth client secret of the current
	// application.
	ClientSecret string

	// Transport is the underlying HTTP transport to use when making requests.
	// It will default to http.DefaultTransport if nil.
	Transport http.RoundTripper
}

// RoundTrip implements the RoundTripper interface.
func (t *UnauthenticatedRateLimitedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.ClientID == "" {
		return nil, errors.New("t.ClientID is empty")
	}
	if t.ClientSecret == "" {
		return nil, errors.New("t.ClientSecret is empty")
	}

	req2 := setCredentialsAsHeaders(req, t.ClientID, t.ClientSecret)
	// Make the HTTP request.
	return t.transport().RoundTrip(req2)
}

// Client returns an *http.Client that makes requests which are subject to the
// rate limit of your OAuth application.
func (t *UnauthenticatedRateLimitedTransport) Client() *http.Client {
	return &http.Client{Transport: t}
}

func (t *UnauthenticatedRateLimitedTransport) transport() http.RoundTripper {
	if t.Transport != nil {
		return t.Transport
	}
	return http.DefaultTransport
}

// BasicAuthTransport is an http.RoundTripper that authenticates all requests
// using HTTP Basic Authentication with the provided username and password. It
// additionally supports users who have two-factor authentication enabled on
// their GitHub account.
type BasicAuthTransport struct {
	Username string // GitHub username
	Password string // GitHub password
	OTP      string // one-time password for users with two-factor auth enabled

	// Transport is the underlying HTTP transport to use when making requests.
	// It will default to http.DefaultTransport if nil.
	Transport http.RoundTripper
}

// RoundTrip implements the RoundTripper interface.
func (t *BasicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := setCredentialsAsHeaders(req, t.Username, t.Password)
	if t.OTP != "" {
		req2.Header.Set(headerOTP, t.OTP)
	}
	return t.transport().RoundTrip(req2)
}

// Client returns an *http.Client that makes requests that are authenticated
// using HTTP Basic Authentication.
func (t *BasicAuthTransport) Client() *http.Client {
	return &http.Client{Transport: t}
}

func (t *BasicAuthTransport) transport() http.RoundTripper {
	if t.Transport != nil {
		return t.Transport
	}
	return http.DefaultTransport
}

// formatRateReset formats d to look like "[rate reset in 2s]" or
// "[rate reset in 87m02s]" for the positive durations. And like "[rate limit was reset 87m02s ago]"
// for the negative cases.
func formatRateReset(d time.Duration) string {
	isNegative := d < 0
	if isNegative {
		d *= -1
	}
	secondsTotal := int(0.5 + d.Seconds())
	minutes := secondsTotal / 60
	seconds := secondsTotal - minutes*60

	var timeString string
	if minutes > 0 {
		timeString = fmt.Sprintf("%vm%02ds", minutes, seconds)
	} else {
		timeString = fmt.Sprintf("%vs", seconds)
	}

	if isNegative {
		return fmt.Sprintf("[rate limit was reset %v ago]", timeString)
	}
	return fmt.Sprintf("[rate reset in %v]", timeString)
}

func sleepUntilResetWithBuffer(ctx context.Context, reset time.Time) error {
	buffer := time.Second
	timer := time.NewTimer(time.Until(reset) + buffer)
	select {
	case <-ctx.Done():
		if !timer.Stop() {
			<-timer.C
		}
		return ctx.Err()
	case <-timer.C:
	}
	return nil
}

// When using roundTripWithOptionalFollowRedirect, note that it
// is the responsibility of the caller to close the response body.
func (c *Client) roundTripWithOptionalFollowRedirect(ctx context.Context, u string, maxRedirects int, opts ...RequestOption) (*http.Response, error) {
	req, err := c.NewRequest(ctx, "GET", u, nil, opts...)
	if err != nil {
		return nil, err
	}

	var resp *http.Response
	// Use http.DefaultTransport if no custom Transport is configured
	if c.client.Transport == nil {
		resp, err = http.DefaultTransport.RoundTrip(req)
	} else {
		resp, err = c.client.Transport.RoundTrip(req)
	}
	if err != nil {
		return nil, err
	}

	// If redirect response is returned, follow it
	if maxRedirects > 0 && resp.StatusCode == http.StatusMovedPermanently {
		_ = resp.Body.Close()
		u = resp.Header.Get("Location")
		if err := c.checkRedirectHost(u); err != nil {
			return nil, err
		}
		resp, err = c.roundTripWithOptionalFollowRedirect(ctx, u, maxRedirects-1, opts...)
	}
	return resp, err
}

// checkRedirectHost returns an error if the redirect target is on a different
// host than the client's configured BaseURL. This prevents credentials attached
// by the auth transport from being sent to an attacker-controlled host when a
// compromised or malicious API response returns a cross-origin Location header.
// An empty Location is also rejected.
func (c *Client) checkRedirectHost(location string) error {
	if location == "" {
		return errInvalidLocation
	}
	target, err := url.Parse(location)
	if err != nil {
		return fmt.Errorf("invalid redirect location %q: %w", location, err)
	}
	// Resolve relative locations against BaseURL so relative paths are allowed.
	target = c.baseURL.ResolveReference(target)
	if target.Host != c.baseURL.Host {
		return fmt.Errorf("refusing to follow cross-host redirect from %q to %q", c.baseURL.Host, target.Host)
	}
	return nil
}

// Ptr is a helper routine that allocates a new T value
// to store v and returns a pointer to it.
func Ptr[T any](v T) *T {
	return &v
}

// Bool is a helper routine that allocates a new bool value
// to store v and returns a pointer to it.
//
// Deprecated: use Ptr instead.
//
//go:fix inline
func Bool(v bool) *bool { return Ptr(v) }

// Int is a helper routine that allocates a new int value
// to store v and returns a pointer to it.
//
// Deprecated: use Ptr instead.
//
//go:fix inline
func Int(v int) *int { return Ptr(v) }

// Int64 is a helper routine that allocates a new int64 value
// to store v and returns a pointer to it.
//
// Deprecated: use Ptr instead.
//
//go:fix inline
func Int64(v int64) *int64 { return Ptr(v) }

// String is a helper routine that allocates a new string value
// to store v and returns a pointer to it.
//
// Deprecated: use Ptr instead.
//
//go:fix inline
func String(v string) *string { return Ptr(v) }

// roundTripperFunc creates a RoundTripper (transport).
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

var runIDFromURLRE = regexp.MustCompile(`repos/.*/actions/runs/(\d+)/deployment_protection_rule$`)

// GetRunID is a Helper Function used to extract the workflow RunID from the *DeploymentProtectionRuleEvent.DeploymentCallBackURL.
func (e *DeploymentProtectionRuleEvent) GetRunID() (int64, error) {
	match := runIDFromURLRE.FindStringSubmatch(*e.DeploymentCallbackURL)
	if len(match) != 2 {
		return -1, errors.New("no match")
	}
	runID, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil {
		return -1, err
	}
	return runID, nil
}
