# GitHub App authentication

The local stdio server can authenticate as a GitHub App installation without a
browser, device flow, or elicitation. It signs a short-lived JWT with the app's
private key, exchanges it for an installation access token, and refreshes the
token before it expires.

This authentication mode is not available for the `http` command. HTTP clients
must continue to provide their own `Authorization` token.

> [!WARNING]
> The private key can mint tokens for every repository and permission granted to
> the installation. Keep it out of source control, restrict access to the server
> process, and install the app only on the repositories it needs.

## Configuration

Configure exactly one of a Personal Access Token, OAuth login, or GitHub App
authentication.

| Flag | Environment variable | Description |
|------|----------------------|-------------|
| `--app-id` | `GITHUB_APP_ID` | App ID or client ID used as the JWT issuer |
| `--app-installation-id` | `GITHUB_APP_INSTALLATION_ID` | Installation whose access token is used. Omit to use every installation of the app (see [Multiple organizations](#multiple-organizations)) |
| `--app-private-key-path` | `GITHUB_APP_PRIVATE_KEY_PATH` | Path to the private key PEM |
| _(none)_ | `GITHUB_APP_PRIVATE_KEY` | PEM contents, optionally with literal `\n` escapes |

A mounted private-key file is preferred. There is no flag for inline PEM
contents because command-line arguments may be visible to other processes.

## Usage

```bash
github-mcp-server stdio \
  --app-id 123456 \
  --app-installation-id 7891011 \
  --app-private-key-path /secrets/github-app.pem
```

The equivalent environment configuration is:

```bash
export GITHUB_APP_ID=123456
export GITHUB_APP_INSTALLATION_ID=7891011
export GITHUB_APP_PRIVATE_KEY_PATH=/secrets/github-app.pem
github-mcp-server stdio
```

For Docker, mount the key read-only:

```bash
docker run -i --rm \
  -v /secrets/github-app.pem:/secrets/github-app.pem:ro \
  -e GITHUB_APP_ID=123456 \
  -e GITHUB_APP_INSTALLATION_ID=7891011 \
  -e GITHUB_APP_PRIVATE_KEY_PATH=/secrets/github-app.pem \
  ghcr.io/github/github-mcp-server
```

## Multiple organizations

A GitHub App can be installed on several accounts, and each installation has its
own ID and its own access token. Omit `--app-installation-id` to work across all
of them from a single app ID and private key:

```bash
github-mcp-server stdio \
  --app-id 123456 \
  --app-private-key-path /secrets/github-app.pem
```

The server then lists the app's installations, caches the map of account to
installation, and mints a token per installation on demand. Each API request is
routed to the installation that owns the resource it addresses: REST requests by
the owner in the path (`/repos/{owner}/...`, `/orgs/{org}/...`,
`/users/{user}/...`), and GraphQL requests by the `owner` or `login` variable in
the query. The installation directory is refreshed at most every 10 minutes,
when a lookup misses, so installing the app on a new organization is picked up
without a restart.

Requests that name no owner are sent unauthenticated, and so are requests for an
account the app is not installed on — the server does not fall back to another
installation's token. Endpoints that are not owner-scoped (`/user`,
`/rate_limit`, `/repositories/{id}`) therefore do not work in this mode; set
`--app-installation-id` to authenticate as one specific installation instead.

For GitHub Enterprise Server or `ghe.com`, also set `--gh-host` or
`GITHUB_HOST`. The server derives the installation-token endpoint from that
host.

## Troubleshooting

- **Private key required**: set `GITHUB_APP_PRIVATE_KEY_PATH` or
  `GITHUB_APP_PRIVATE_KEY`.
- **Invalid private key**: provide the RSA PEM generated in the GitHub App
  settings. PKCS#1 and PKCS#8 keys are supported.
- **401 from the installation-token endpoint**: verify the app ID or client ID,
  private key, target host, and system clock.
- **404 from the installation-token endpoint**: verify the installation ID and
  that the app is installed on the target host.
- **401 or 404 for one organization only** (multi-installation mode): the app is
  not installed on that account, or the tool call named an owner that does not
  match the account login. The server logs `GitHub App is not installed on this
  account` once per owner.
