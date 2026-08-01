# go-github #

[![go-github release (latest SemVer)](https://img.shields.io/github/v/release/google/go-github?sort=semver)](https://github.com/google/go-github/releases)
[![Go Reference](https://img.shields.io/static/v1?label=godoc&message=reference&color=blue)](https://pkg.go.dev/github.com/google/go-github/v89/github)
[![Test Status](https://github.com/google/go-github/actions/workflows/tests.yml/badge.svg?branch=master)](https://github.com/google/go-github/actions/workflows/tests.yml)
[![Test Coverage](https://codecov.io/gh/google/go-github/branch/master/graph/badge.svg)](https://codecov.io/gh/google/go-github)
[![Discuss at go-github@googlegroups.com](https://img.shields.io/badge/discuss-go--github%40googlegroups.com-blue.svg)](https://groups.google.com/group/go-github)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/796/badge)](https://bestpractices.coreinfrastructure.org/projects/796)

go-github is a Go client library for accessing the [GitHub API v3][].

go-github tracks [Go's version support policy][support-policy] supporting any
minor version of the latest two major releases of Go and the go directive in
go.mod reflects that.
We do our best not to break older versions of Go if we don't have to, but we
don't explicitly test older versions and as of Go 1.23 the go directive in
go.mod declares a hard required _minimum_ version of Go to use with this module
and this _must_ be greater than or equal to the go line of all dependencies so
go-github will require the N-1 major release of Go by default.

[support-policy]: https://golang.org/doc/devel/release.html#policy

## Development ##

If you're interested in using the [GraphQL API v4][], the recommended library is
[shurcooL/githubv4][].

## Installation ##

go-github is compatible with modern Go releases in module mode, with Go installed:

```bash
go get github.com/google/go-github/v89
```

will resolve and add the package to the current development module, along with its dependencies.

Alternatively the same can be achieved if you use import in a package:

```go
import "github.com/google/go-github/v89/github"
```

and run `go get` without parameters.

Finally, to use the top-of-trunk version of this repo, use the following command:

```bash
go get github.com/google/go-github/v89@master
```

To discover all the changes that have occurred since a prior release, you can
first clone the repo, then run (for example):

```bash
go run tools/gen-release-notes/main.go --tag v89.0.0
```

## Usage ##

```go
import "github.com/google/go-github/v89/github"
```

Construct a new GitHub client, then use the various services on the client to
access different parts of the GitHub API. For example:

```go
client, err := github.NewClient()
if err != nil {
	// Handle error.
}

// list all organizations for user "willnorris"
orgs, _, err := client.Organizations.List(context.Background(), "willnorris", nil)
```

Some API methods have optional parameters that can be passed. For example:

```go
client, err := github.NewClient()
if err != nil {
	// Handle error.
}

// list public repositories for org "github"
opt := &github.RepositoryListByOrgOptions{Type: "public"}
repos, _, err := client.Repositories.ListByOrg(context.Background(), "github", opt)
```

The services of a client divide the API into logical chunks and correspond to
the structure of the [GitHub API documentation](https://docs.github.com/en/rest).

NOTE: Using the [context](https://pkg.go.dev/context) package, one can easily
pass cancellation signals and deadlines to various services of the client for
handling a request. In case there is no context available, then `context.Background()`
can be used as a starting point.

For more sample code snippets, head over to the
[example](https://github.com/google/go-github/tree/master/example) directory.

### Authentication ###

Use the `github.WithAuthToken` options method to configure your client to authenticate using an
OAuth token (for example, a [personal access token][]). This is what is needed
for a majority of use cases aside from GitHub Apps.

```go
client, err := github.NewClient(github.WithAuthToken("... your access token ..."))
if err != nil {
	// Handle error.
}
```

To support more advanced use cases; you can use the `github.WithTransport` option to provide a
custom `http.RoundTripper` that handles authentication for you, or the `github.WithHTTPClient`
option to provide a custom `http.Client`. As an example; you can use the `oauth2.Transport`
from the [golang.org/x/oauth2](https://pkg.go.dev/golang.org/x/oauth2) package to handle OAuth
token refreshing for you.

Note that when using an authenticated Client, all calls made by the client will
include the specified OAuth token. Therefore, authenticated clients should
almost never be shared between different users.

For API methods that require HTTP Basic Authentication, use the
[`BasicAuthTransport`](https://pkg.go.dev/github.com/google/go-github/v89/github#BasicAuthTransport).

#### As a GitHub App ####

GitHub Apps authentication can be provided by different pkgs like [bradleyfalzon/ghinstallation](https://github.com/bradleyfalzon/ghinstallation)
or [jferrl/go-githubauth](https://github.com/jferrl/go-githubauth).

> **Note**: Most endpoints (ex. [`GET /rate_limit`]) require access token authentication
> while a few others (ex. [`GET /app/hook/deliveries`]) require [JWT] authentication.

[`GET /rate_limit`]: https://docs.github.com/en/rest/rate-limit#get-rate-limit-status-for-the-authenticated-user
[`GET /app/hook/deliveries`]: https://docs.github.com/en/rest/apps/webhooks#list-deliveries-for-an-app-webhook
[JWT]: https://docs.github.com/en/developers/apps/building-github-apps/authenticating-with-github-apps#authenticating-as-a-github-app

`ghinstallation` provides `Transport`, which implements `http.RoundTripper` to provide authentication as an installation for GitHub Apps.

Here is an example of how to authenticate as a GitHub App using the `ghinstallation` package:

```go
import (
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v89/github"
)

func main() {
	// Wrap the shared transport for use with the integration ID 1 authenticating with installation ID 99.
	itr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, 1, 99, "2016-10-19.private-key.pem")

	// Or for endpoints that require JWT authentication
	// itr, err := ghinstallation.NewAppsTransportKeyFromFile(http.DefaultTransport, 1, "2016-10-19.private-key.pem")

	if err != nil {
		// Handle error.
	}

	// Use installation transport with client.
	client, err := github.NewClient(github.WithTransport(itr))
	if err != nil {
		// Handle error.
	}

	// Use client...
}
```

`go-githubauth` implements a set of `oauth2.TokenSource` to be used with `oauth2.Client`. An `oauth2.Client` can be injected into the `github.Client` to authenticate requests.

Another example using `go-githubauth`:

```go
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/google/go-github/v89/github"
	"github.com/jferrl/go-githubauth"
	"golang.org/x/oauth2"
)

func main() {
	privateKey := []byte(os.Getenv("GITHUB_APP_PRIVATE_KEY"))

	appTokenSource, err := githubauth.NewApplicationTokenSource(1112, privateKey)
	if err != nil {
		fmt.Println("Error creating application token source:", err)
		return
	 }

	installationTokenSource := githubauth.NewInstallationTokenSource(1113, appTokenSource)

	// oauth2.NewClient uses oauth2.ReuseTokenSource to reuse the token until it expires.
	// The token will be automatically refreshed when it expires.
	// InstallationTokenSource has the mechanism to refresh the token when it expires.
	httpClient := oauth2.NewClient(context.Background(), installationTokenSource)

	client, err := github.NewClient(github.WithHTTPClient(httpClient))
	if err != nil {
		// Handle error.
	}
}
```

_Note_: In order to interact with certain APIs, for example writing a file to a repo, one must generate an installation token
using the installation ID of the GitHub app and authenticate with the OAuth method mentioned above. See the examples.

### Rate Limiting ###

GitHub imposes rate limits on all API clients. The [primary rate limit](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api#about-primary-rate-limits)
is the limit to the number of REST API requests that a client can make within a
specific amount of time. This limit helps prevent abuse and denial-of-service
attacks, and ensures that the API remains available for all users. Some
endpoints, like the search endpoints, have more restrictive limits.
Unauthenticated clients may request public data but have a low rate limit,
while authenticated clients have rate limits based on the client
identity.

In addition to primary rate limits, GitHub enforces [secondary rate limits](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api#about-secondary-rate-limits)
in order to prevent abuse and keep the API available for all users.
Secondary rate limits generally limit the number of concurrent requests that a
client can make.

The client returned `Response.Rate` value contains the rate limit information
from the most recent API call. If a recent enough response isn't
available, you can use the client `RateLimits` service to fetch the most
up-to-date rate limit data for the client.

To detect a primary API rate limit error, you can check if the error is a
`RateLimitError`.

```go
repos, _, err := client.Repositories.List(ctx, "", nil)
var rateErr *github.RateLimitError
if errors.As(err, &rateErr) {
	log.Printf("hit primary rate limit, used %v of %v\n", rateErr.Rate.Used, rateErr.Rate.Limit)
}
```

To detect an API secondary rate limit error, you can check if the error is an
`AbuseRateLimitError`.

```go
repos, _, err := client.Repositories.List(ctx, "", nil)
var rateErr *github.AbuseRateLimitError
if errors.As(err, &rateErr) {
	log.Printf("hit secondary rate limit, retry after %v\n", rateErr.RetryAfter)
}
```

If you hit the primary rate limit, you can use the `SleepUntilPrimaryRateLimitResetWhenRateLimited`
method to block until the rate limit is reset.

```go
repos, _, err := client.Repositories.List(context.WithValue(ctx, github.SleepUntilPrimaryRateLimitResetWhenRateLimited, true), "", nil)
```

If you need to make a request even if the rate limit has been hit you can use
the `BypassRateLimitCheck` method to bypass the rate limit check and make the
request anyway.

```go
repos, _, err := client.Repositories.List(context.WithValue(ctx, github.BypassRateLimitCheck, true), "", nil)
```

For more advanced use cases, you can use [gofri/go-github-ratelimit](https://github.com/gofri/go-github-ratelimit)
which provides a middleware (`http.RoundTripper`) that handles both the primary
rate limit and secondary rate limit for the GitHub API. In this case you can
set the client `DisableRateLimitCheck` to `true` so the client doesn't track the rate limit usage.

If the client is an [OAuth app](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api#primary-rate-limit-for-oauth-apps)
you can use the apps higher rate limit to request public data by using the
`UnauthenticatedRateLimitedTransport` to make calls as the app instead of as
the user.

### Accepted Status ###

Some endpoints may return a 202 Accepted status code, meaning that the
information required is not yet ready and was scheduled to be gathered on
the GitHub side. Methods known to behave like this are documented specifying
this behavior.

To detect this condition of error, you can check if its type is
`*github.AcceptedError`:

```go
stats, _, err := client.Repositories.ListContributorsStats(ctx, org, repo)
if errors.As(err, new(*github.AcceptedError)) {
	log.Println("scheduled on GitHub side")
}
```

### Conditional Requests ###

The GitHub REST API has good support for [conditional HTTP requests](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api?apiVersion=2022-11-28#use-conditional-requests-if-appropriate)
via the `ETag` header which will help prevent you from burning through your
rate limit, as well as help speed up your application. `go-github` does not
handle conditional requests directly, but is instead designed to work with a
caching `http.Transport`.

Typically, an [RFC 9111](https://datatracker.ietf.org/doc/html/rfc9111)
compliant HTTP cache such as [bartventer/httpcache](https://github.com/bartventer/httpcache)
is recommended, ex:

```go
import (
	"github.com/bartventer/httpcache"
	_ "github.com/bartventer/httpcache/store/memcache" // Register the in-memory backend
)

client, err := github.NewClient(github.WithHTTPClient(httpcache.NewClient("memcache://")), github.WithAuthToken(os.Getenv("GITHUB_TOKEN")))
if err != nil {
	// Handle error.
}
```

Alternatively, the [bored-engineer/github-conditional-http-transport](https://github.com/bored-engineer/github-conditional-http-transport)
package relies on (undocumented) GitHub specific cache logic and is
recommended when making requests using short-lived credentials such as a
[GitHub App installation token](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/authenticating-as-a-github-app-installation).

### Creating and Updating Resources ###

All structs for GitHub resources use pointer values for all non-repeated fields.
This allows distinguishing between unset fields and those set to a zero-value.
Helper functions have been provided to easily create these pointers for string,
bool, and int values. For example:

```go
// create a new private repository named "foo"
repo := &github.Repository{
	Name:    github.Ptr("foo"),
	Private: github.Ptr(true),
}
client.Repositories.Create(ctx, "", repo)
```

Users who have worked with protocol buffers should find this pattern familiar.

### Pagination ###

All requests for resource collections (repos, pull requests, issues, etc.)
support pagination. Pagination options using page numbers are described in the
`github.ListOptions` struct and passed to the list methods directly or as an
embedded type of a more specific list options struct (for example
`github.PullRequestListOptions`). Pages information is available via the
`github.Response` struct.

```go
client, err := github.NewClient()
if err != nil {
	// Handle error.
}

opt := &github.RepositoryListByOrgOptions{
	ListOptions: github.ListOptions{PerPage: 10},
}
// get all pages of results
var allRepos []*github.Repository
for {
	repos, resp, err := client.Repositories.ListByOrg(ctx, "github", opt)
	if err != nil {
		return err
	}
	allRepos = append(allRepos, repos...)
	if resp.NextPage == 0 {
		break
	}
	opt.Page = resp.NextPage
}
```

Pagination options using string cursors are described in the `github.ListCursorOptions`
struct and passed to the list methods directly or as an
embedded type of a more specific list cursor options struct (for example
`github.ListGlobalSecurityAdvisoriesOptions`). Similarly, cursor and pages information
is available via the `github.Response` struct.

#### Iterators ####

Go v1.23 introduces the new `iter` package.

The new `github/gen-iterators.go` file auto-generates "*Iter" methods in `github/github-iterators.go`
for all methods that support page number iteration (using the `NextPage` field in each response)
or string cursor iteration (using the `After` field in each response).
To handle rate limiting issues, make sure to use a rate-limiting transport.
(See [Rate Limiting](/#rate-limiting) above for more details.)
To use these methods, simply create an iterator and then range over it, for example:

```go
client, err := github.NewClient()
if err != nil {
	// Handle error.
}
var allRepos []*github.Repository

// create an iterator and start looping through all the results
iter := client.Repositories.ListIter(ctx, "github", nil)
for repo, err := range iter {
	if err != nil {
		log.Fatal(err)
	}
	allRepos = append(allRepos, repo)
}
```

Alternatively, if you wish to use an external package, there is `enrichman/gh-iter`.
Its iterator will handle pagination for you, looping through all the available results.

```go
client, err := github.NewClient()
if err != nil {
	// Handle error.
}
var allRepos []*github.Repository

// create an iterator and start looping through all the results
repos := ghiter.NewFromFn1(client.Repositories.ListByOrg, "github")
for repo := range repos.All() {
	allRepos = append(allRepos, repo)
}
```

For complete usage of `enrichman/gh-iter`, see the full [package docs](https://github.com/enrichman/gh-iter).

#### Middleware ####

You can use [gofri/go-github-pagination](https://github.com/gofri/go-github-pagination) to handle
pagination for you. It supports both sync and async modes, as well as customizations.
By default, the middleware automatically paginates through all pages, aggregates results, and returns them as an array.
See `example/ratelimit/main.go` for usage.

### Webhooks ###

`go-github` provides structs for almost all [GitHub webhook events][] as well as functions to validate them and unmarshal JSON payloads from `http.Request` structs.

```go
func (s *GitHubEventMonitor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, s.webhookSecretKey)
	if err != nil { ... }
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil { ... }
	switch event := event.(type) {
	case *github.CommitCommentEvent:
		processCommitCommentEvent(event)
	case *github.CreateEvent:
		processCreateEvent(event)
	...
	}
}
```

Furthermore, there are libraries like [cbrgm/githubevents][] that build upon the example above and provide functions to subscribe callbacks to specific events.

For complete usage of go-github, see the full [package docs][].

[GitHub API v3]: https://docs.github.com/en/rest
[personal access token]: https://github.com/blog/1509-personal-api-tokens
[package docs]: https://pkg.go.dev/github.com/google/go-github/v89/github
[GraphQL API v4]: https://developer.github.com/v4/
[shurcooL/githubv4]: https://github.com/shurcooL/githubv4
[GitHub webhook events]: https://docs.github.com/en/developers/webhooks-and-events/webhooks/webhook-events-and-payloads
[cbrgm/githubevents]: https://github.com/cbrgm/githubevents

### Testing code that uses `go-github` ###

The repo [migueleliasweb/go-github-mock](https://github.com/migueleliasweb/go-github-mock) provides a way to mock responses. Check the repo for more details.

### Integration Tests ###

You can run integration tests from the `test` directory. See the integration tests [README](test/README.md).

## Contributing ##

I would like to cover the entire GitHub API and contributions are of course always welcome. The
calling pattern is pretty well established, so adding new methods is relatively
straightforward. See [`CONTRIBUTING.md`](CONTRIBUTING.md) for details.

## Versioning ##

In general, go-github follows [semver](https://semver.org/) as closely as we
can for tagging releases of the package. For self-contained libraries, the
application of semantic versioning is relatively straightforward and generally
understood. But because go-github is a client library for the GitHub API, which
itself changes behavior, and because we are typically pretty aggressive about
implementing preview features of the GitHub API, we've adopted the following
versioning policy:

* We increment the **major version** with any incompatible change to
  non-preview functionality, including changes to the exported Go API surface
  or behavior of the API.

* We increment the **minor version** with any backwards-compatible changes to
  functionality, as well as any changes to preview functionality in the GitHub
  API. GitHub makes no guarantee about the stability of preview functionality,
  so neither do we consider it a stable part of the go-github API.

* We increment the **patch version** with any backwards-compatible bug fixes.

Preview functionality may take the form of entire methods or simply additional
data returned from an otherwise non-preview method. Refer to the GitHub API
documentation for details on preview functionality.

### Calendar Versioning ###

As of 2022-11-28, GitHub [has announced](https://github.blog/developer-skills/github/to-infinity-and-beyond-enabling-the-future-of-githubs-rest-api-with-api-versioning/)
that they are starting to version their v3 API based on "calendar-versioning".

In practice, our goal is to make per-method version overrides (at
least in the core library) rare and temporary.

Our understanding of the GitHub docs is that they will be revving the
entire API to each new date-based version, even if only a few methods
have breaking changes. Other methods will accept the new version with
their existing functionality. So when a new date-based version of the
GitHub API is released, we (the repo maintainers) plan to:

* update each method that had breaking changes, overriding their
  per-method API version header. This may happen in one or multiple
  commits and PRs, and is all done in the main branch.

* once all of the methods with breaking changes have been updated,
  have a final commit that bumps the default API version, and remove
  all of the per-method overrides. That would now get a major version
  bump when the next go-github release is made.

### Version Compatibility Table ###

The following table identifies which version of the GitHub API is
supported by this (and past) versions of this repo (go-github).
Versions prior to 48.2.0 are not listed.

| go-github Version | GitHub v3 API Version |
| ----------------- | --------------------- |
| 89.0.0            | 2022-11-28            |
| ...               | 2022-11-28            |
| 48.2.0            | 2022-11-28            |

## License ##

This library is distributed under the BSD-style license found in the [LICENSE](./LICENSE)
file.
