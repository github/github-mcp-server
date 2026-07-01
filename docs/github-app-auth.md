# GitHub App Server-to-Server Authentication (stdio)

The local (stdio) GitHub MCP Server can authenticate as a **GitHub App
installation** instead of as a user. This is a **server-to-server** (s2s) flow:
the server signs a short-lived JSON Web Token (JWT) with your app's private key,
exchanges it for an installation access token, and refreshes that token
automatically. There is **no browser, no device code, and no elicitation**, so
it works in fully non-interactive environments — CI, Kubernetes, and background
agents such as Copilot's cloud agent.

> [!WARNING]
> **Read this before you enable it.** This mode was added by popular demand, but
> it is **dangerous** and is **not recommended without an independent security
> review** of your deployment and of this implementation.
>
> - It places a **long-lived, high-privilege credential** (your app's private
>   key) in the same environment as an AI agent. Anyone or anything that can read
>   that environment can mint tokens that act as your app.
> - Installation access tokens minted here can act across **every repository the
>   app is installed on**, with the app's full set of permissions.
> - Exposing credentials to agents — and **especially in the cloud** — is
>   inherently risky. Treat this as a break-glass capability and proceed with
>   **extreme caution**.
>
> If an interactive login is at all possible for your use case, prefer
> [OAuth login](oauth-login.md) instead, which keeps no long-lived secret next to
> the agent.

## Contents

- [When to use this](#when-to-use-this)
- [Why stdio only](#why-stdio-only)
- [How it works](#how-it-works)
- [Prerequisites](#prerequisites)
- [Configuration reference](#configuration-reference)
- [Injecting the private key safely](#injecting-the-private-key-safely)
- [Quick start](#quick-start)
- [Kubernetes](#kubernetes)
- [GitHub Enterprise Server and ghe.com](#github-enterprise-server-and-ghecom)
- [Reducing the blast radius](#reducing-the-blast-radius)
- [Troubleshooting](#troubleshooting)

## When to use this

Use GitHub App s2s auth only when **all** of the following hold:

- The server runs **non-interactively** (no human to complete a browser or
  device flow).
- The workload should act as an **organization-managed identity** (the app),
  not a single user's Personal Access Token (PAT).
- You have reviewed the security implications above and accept them.

For everything else, prefer [OAuth login](oauth-login.md) or a
[PAT](https://github.com/settings/personal-access-tokens/new).

## Why stdio only

This mode is deliberately limited to the **stdio** server, where the server runs
as a subprocess of a single trusted client and the minted token never crosses
that process boundary.

It is intentionally **not** available for the `http` server. An HTTP server that
authenticated with a server-wide app identity would let **any** client that can
reach its endpoint act as the app, with the app's full permissions — turning a
network-reachable port into ambient, unauthenticated access to your whole
installation. The `http` server therefore keeps requiring a per-request
`Authorization` token, so every caller's identity and permissions stay explicit.

If you need a hosted, networked deployment, authenticate callers at the
client/proxy layer and pass per-request tokens; don't give the server a standing
identity.

## How it works

1. The server builds a JWT and signs it with your app's private key (RS256). The
   JWT is valid for under 10 minutes (GitHub's maximum) and identifies your app.
2. It calls `POST /app/installations/{installation_id}/access_tokens` with that
   JWT to obtain an **installation access token** (prefixed `ghs_`), which is
   valid for up to one hour.
3. Every GitHub API call uses that token. The server refreshes it about five
   minutes before it expires, so long-running sessions keep working without any
   intervention.

The private key is held **in memory only**; the server never writes it or the
minted tokens to disk.

## Prerequisites

1. **Register a GitHub App** and generate a **private key** (Settings → your
   app → *Private keys* → *Generate a private key*). GitHub downloads a `.pem`
   file in PKCS#1 or PKCS#8 format — both are accepted.
2. **Install the app** on the account/organization and grant it the **minimum**
   permissions and **only the repositories** it needs (see
   [Reducing the blast radius](#reducing-the-blast-radius)).
3. Note three values:
   - the **App ID** (or the app's **client ID** — either works as the JWT issuer),
   - the **installation ID** (visible in the installation's settings URL, or via
     the [installations API](https://docs.github.com/en/rest/apps/apps#list-installations-for-the-authenticated-app)),
   - the path to the **private key** `.pem`.

## Configuration reference

App auth is enabled when **any** of these `app-*` settings is present; a
partial configuration produces a clear startup error. Settings apply only to the
`stdio` command.

| Flag | Environment variable | Description |
|------|----------------------|-------------|
| `--app-id` | `GITHUB_APP_ID` | GitHub App ID or client ID. Becomes the JWT issuer. |
| `--app-installation-id` | `GITHUB_APP_INSTALLATION_ID` | Installation ID whose token is minted. |
| `--app-private-key-path` | `GITHUB_APP_PRIVATE_KEY_PATH` | Path to the private key PEM file. **Preferred** way to supply the key. |
| _(no flag)_ | `GITHUB_APP_PRIVATE_KEY` | The PEM contents inline. Use only where a file can't be mounted. Literal `\n` sequences are accepted so the key can live in a single-line variable. |

There is intentionally **no flag** for the private key contents: a flag would
place the key in the process's command line (`ps`, `/proc/<pid>/cmdline`), where
other processes could read it.

App auth is **mutually exclusive** with a PAT (`GITHUB_PERSONAL_ACCESS_TOKEN`)
and with OAuth login (`--oauth-client-id`). Configure exactly one.

## Injecting the private key safely

The private key is the most sensitive value in this flow. In order of
preference:

1. **A mounted secret file** (recommended). Point `GITHUB_APP_PRIVATE_KEY_PATH`
   at a file your platform mounts from its secret store — a Kubernetes secret
   volume, a Docker secret, or a tmpfs file written by your secret manager. The
   key never touches the command line or the process environment.
2. **An inline environment variable** (`GITHUB_APP_PRIVATE_KEY`). Acceptable
   where files can't be mounted, but the key is then readable by anything that
   can inspect the process environment. Avoid this in shared or cloud
   environments.

Never pass the key on the command line, never bake it into an image, and never
commit it to source control.

## Quick start

Native binary, key on disk:

```bash
github-mcp-server stdio \
  --app-id 123456 \
  --app-installation-id 7891011 \
  --app-private-key-path /secrets/github-app.pem
```

Equivalently, with environment variables:

```bash
export GITHUB_APP_ID=123456
export GITHUB_APP_INSTALLATION_ID=7891011
export GITHUB_APP_PRIVATE_KEY_PATH=/secrets/github-app.pem
github-mcp-server stdio
```

Docker, mounting the key as a read-only file (preferred over passing it inline):

```bash
docker run -i --rm \
  -v /secrets/github-app.pem:/secrets/github-app.pem:ro \
  -e GITHUB_APP_ID=123456 \
  -e GITHUB_APP_INSTALLATION_ID=7891011 \
  -e GITHUB_APP_PRIVATE_KEY_PATH=/secrets/github-app.pem \
  ghcr.io/github/github-mcp-server
```

## Kubernetes

Store the key in a `Secret` and mount it as a file; pass the IDs as environment
variables. This keeps the key off the command line and out of the container's
environment.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: github-app
type: Opaque
stringData:
  private-key.pem: |
    -----BEGIN RSA PRIVATE KEY-----
    ...
    -----END RSA PRIVATE KEY-----
---
apiVersion: v1
kind: Pod
metadata:
  name: github-mcp-server
spec:
  containers:
    - name: github-mcp-server
      image: ghcr.io/github/github-mcp-server
      stdin: true
      env:
        - name: GITHUB_APP_ID
          value: "123456"
        - name: GITHUB_APP_INSTALLATION_ID
          value: "7891011"
        - name: GITHUB_APP_PRIVATE_KEY_PATH
          value: /secrets/github-app/private-key.pem
      volumeMounts:
        - name: github-app
          mountPath: /secrets/github-app
          readOnly: true
  volumes:
    - name: github-app
      secret:
        secretName: github-app
```

## GitHub Enterprise Server and ghe.com

Set the host with `--gh-host` / `GITHUB_HOST`; the server derives the correct
installation token endpoint from it, so tokens are minted against your instance
rather than github.com. Register the app and generate its key on that same host.

```bash
github-mcp-server stdio \
  --gh-host https://github.example.com \
  --app-id 123456 \
  --app-installation-id 7891011 \
  --app-private-key-path /secrets/github-app.pem
```

- For GitHub Enterprise Server, prefix the host with `https://`.
- For `ghe.com`, use `https://YOURSUBDOMAIN.ghe.com`.

## Reducing the blast radius

Because the minted token can act across the whole installation, minimize what it
can do:

- **Grant least privilege.** Enable only the app permissions the workload needs,
  and prefer read-only where possible.
- **Scope the installation to specific repositories** rather than *All
  repositories*.
- **Rotate the private key** periodically and immediately if it may have been
  exposed (Settings → your app → *Private keys*).
- **Isolate the runtime.** Run the server where only trusted code shares its
  process environment and mounted secrets.
- **Combine with `--read-only` and toolset/scoping flags** to further narrow
  what the agent can invoke. See the
  [Server Configuration Guide](server-configuration.md).

## Troubleshooting

- **`GitHub App authentication requires a private key`** — you set some `app-*`
  values but no key. Set `GITHUB_APP_PRIVATE_KEY_PATH` (preferred) or
  `GITHUB_APP_PRIVATE_KEY`.
- **`invalid GitHub App private key`** — the PEM could not be parsed. Ensure it
  is the app's RSA private key in PKCS#1 or PKCS#8 form and was not truncated
  (when inline, encode newlines as literal `\n`).
- **`installation token request failed: 401`** — usually a clock-skew problem or
  the wrong App ID/key pairing. Check the host clock and that the key belongs to
  the configured app.
- **`installation token request failed: 404`** — the installation ID is wrong,
  or the app is not installed where you think. Re-check the installation ID.
- **`... and GITHUB_PERSONAL_ACCESS_TOKEN are mutually exclusive`** — a PAT is
  also set in the environment. Unset it; choose exactly one auth mode.
