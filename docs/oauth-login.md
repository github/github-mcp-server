# Local Server OAuth Login (stdio)

The local (stdio) GitHub MCP Server can log you in with OAuth instead of a
Personal Access Token (PAT). On first use it walks you through GitHub's
authorization flow in your browser and keeps the resulting token **in memory
only** — nothing is written to disk.

Official released binaries and the `ghcr.io/github/github-mcp-server` image ship
with a registered GitHub OAuth application baked in, so on **github.com** you can
start the server with no token and no client ID at all. To target a different
host (GitHub Enterprise Server or `ghe.com`), or to use your own application,
pass `--oauth-client-id` (see [Bring your own app](#bring-your-own-app)).

> OAuth login applies to the **stdio** server only. The remote server and the
> `http` command have their own authentication; see
> [Remote Server](remote-server.md).

## Contents

- [How it works](#how-it-works)
- [Quick start](#quick-start)
- [Configuration reference](#configuration-reference)
- [Scope filtering](#scope-filtering)
- [Running in Docker](#running-in-docker)
- [Headless and device-code fallback](#headless-and-device-code-fallback)
- [URL elicitation and the security advisory](#url-elicitation-and-the-security-advisory)
- [Bring your own app](#bring-your-own-app)
- [GitHub Enterprise Server and ghe.com](#github-enterprise-server-and-ghecom)
- [Building from source with baked-in credentials](#building-from-source-with-baked-in-credentials)

## How it works

The server prefers the **authorization code flow with PKCE**: it starts a
loopback callback server on your machine, opens GitHub's authorization page, and
exchanges the returned code for a token. GitHub requires a client secret at the
token endpoint (for both OAuth Apps and GitHub Apps), so the exchange sends it
together with the PKCE verifier. Because this is a public, distributed client,
that secret is baked into the binary and is **not truly confidential** — PKCE is
what secures the flow: it binds the authorization code to this one login attempt,
so a code intercepted on the loopback redirect can't be redeemed anywhere else.

To present the authorization URL, the server uses the most secure channel your
MCP client offers, in order:

1. **Open your browser automatically** (native runs).
2. **URL elicitation** — the client prompts you with the link out of band, so the
   URL never enters the model's context. Requires a client that supports MCP
   elicitation (e.g. VS Code 1.101+).
3. **A message in the first tool response** — a last resort for clients without
   elicitation. This includes a [security advisory](#url-elicitation-and-the-security-advisory).

If the authorization-code flow can't be used — for example, a container with no
published callback port — the server falls back to the
[device-code flow](#headless-and-device-code-fallback).

GitHub App tokens that expire are refreshed transparently using the refresh
token, so long-running sessions keep working without re-authorizing.

## Quick start

**Native binary (recommended).** Best experience: a random loopback port is
used and your browser opens automatically. On github.com with an official build,
no flags are needed:

```bash
github-mcp-server stdio
```

With your own application:

```bash
github-mcp-server stdio --oauth-client-id <YOUR_CLIENT_ID>
```

VS Code (`.vscode/mcp.json`), using your own app:

```json
{
  "servers": {
    "github": {
      "command": "/path/to/github-mcp-server",
      "args": ["stdio", "--oauth-client-id", "<YOUR_CLIENT_ID>"]
    }
  }
}
```

For Docker, see [Running in Docker](#running-in-docker) — containers need a fixed
callback port.

## Configuration reference

OAuth login is configured with these stdio flags (each has an environment
variable equivalent). Flags apply only to the `stdio` command.

| Flag | Environment variable | Description |
|------|----------------------|-------------|
| `--oauth-client-id` | `GITHUB_OAUTH_CLIENT_ID` | OAuth App or GitHub App client ID. Enables OAuth login when no token is set. Defaults to the baked-in app on github.com for official builds. |
| `--oauth-client-secret` | `GITHUB_OAUTH_CLIENT_SECRET` | Client secret, **if your app requires one**. For distributed clients this is a public, non-confidential credential. |
| `--oauth-scopes` | `GITHUB_OAUTH_SCOPES` | Comma-separated scopes to request. Also [filters tools](#scope-filtering) to those scopes. Defaults to the full supported set. |
| `--oauth-callback-port` | `GITHUB_OAUTH_CALLBACK_PORT` | Fixed local port for the callback server. Defaults to a random port; set a fixed port when mapping it through Docker. |

A static token still takes precedence: if `GITHUB_PERSONAL_ACCESS_TOKEN` is set,
the server uses it and skips OAuth entirely.

## Scope filtering

The scopes you request determine which tools are exposed. Requesting the full
supported set (the default) hides no tools. Narrowing `--oauth-scopes` both
narrows the token's grant **and** filters out tools that would need a scope you
didn't request, so the tool list reflects what the token can actually do.

For example, requesting only `repo,read:org` hides tools that require `gist`,
`workflow`, `notifications`, and so on.

## Running in Docker

A container can't reach a random loopback port on your host, so Docker OAuth
needs a **fixed** callback port that you publish into the container. Use port
**8085** to match the official app's registered callback URL.

```bash
docker run -i --rm \
  -p 127.0.0.1:8085:8085 \
  -e GITHUB_OAUTH_CALLBACK_PORT=8085 \
  ghcr.io/github/github-mcp-server
```

VS Code (`.vscode/mcp.json`):

```json
{
  "servers": {
    "github": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-p", "127.0.0.1:8085:8085",
        "-e", "GITHUB_OAUTH_CALLBACK_PORT",
        "ghcr.io/github/github-mcp-server"
      ],
      "env": { "GITHUB_OAUTH_CALLBACK_PORT": "8085" }
    }
  }
}
```

Because the container can't open your host browser, the authorization URL
arrives via [URL elicitation](#url-elicitation-and-the-security-advisory) or the
tool-response message. After you authorize, your browser hits
`localhost:8085`, which Docker forwards into the container's callback.

If you bring your own app for Docker, register its callback URL as exactly
`http://localhost:8085/callback`.

> **Two safety properties to be aware of with a fixed port:**
>
> - **Publish to loopback only** (`-p 127.0.0.1:8085:8085`, not `-p 8085:8085`).
>   Inside a container the callback necessarily listens on all interfaces, so a
>   plain publish would expose the authorization code to your network. The
>   server logs a warning reminding you of this when it binds inside a container.
> - **A busy port is fatal, by design.** With a fixed port, if the server can't
>   bind it (another process already holds it), it **stops with an error** rather
>   than silently falling back to the device flow. A port you didn't get could
>   belong to another user's process positioned to receive the redirect, so the
>   server refuses to continue. Free the port or choose a different
>   `--oauth-callback-port`.

## Headless and device-code fallback

When there's no usable browser or callback — a remote shell, CI, or a container
started without a published port — the server uses GitHub's **device-code
flow**. You'll get a short code and a verification URL to open on any device:

```
Visit https://github.com/login/device and enter the code WDJB-MJHT to authorize
the GitHub MCP Server.
```

The server polls GitHub until you finish authorizing, then continues. No
callback port is involved, so this works anywhere.

## URL elicitation and the security advisory

URL elicitation lets your MCP client present the authorization URL to you
directly, keeping it **out of the model's context** — the model never sees the
link or any code embedded in it. This is the most secure way to hand off the
authorization step.

If your client doesn't support elicitation, the server falls back to placing the
URL in a tool response and appends a short advisory:

> Note: your MCP client does not appear to support secure URL elicitation. For
> improved security, consider asking your agent, CLI, or IDE to add it (for
> example, by opening an issue).

If you see this, your authorization still works — but consider asking your client
vendor to add elicitation support.

## Bring your own app

You need your own application when targeting a non-github.com host, or when you'd
rather not use the baked-in app. Either application type works:

- **[Create an OAuth App](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/creating-an-oauth-app)** —
  simplest to set up. Grants the scopes you request.
- **[Register a GitHub App](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/registering-a-github-app)** —
  finer-grained, per-resource permissions and short-lived tokens that refresh
  automatically. Enable **Device Flow** in the app settings if you want the
  [headless fallback](#headless-and-device-code-fallback).

When registering, set the authorization callback URL:

- **Native runs** use a random loopback port. For loopback redirects GitHub does
  not require the callback port to match, so registering
  `http://localhost/callback` is sufficient.
- **Docker / fixed port** must match exactly: register
  `http://localhost:8085/callback` (or whichever port you publish).

Then pass the client ID (and secret, only if your app requires one):

```bash
github-mcp-server stdio \
  --oauth-client-id <YOUR_CLIENT_ID> \
  --oauth-client-secret <YOUR_CLIENT_SECRET>
```

## GitHub Enterprise Server and ghe.com

The baked-in app is registered on github.com only, so it is **not** used when you
set a custom host. GitHub Enterprise Server and `ghe.com` (Enterprise Cloud with
data residency) users must **bring their own app** registered on that host and
pass `--oauth-client-id`.

Set the host with `--gh-host` / `GITHUB_HOST`; the server derives the OAuth
authorization, token, and device endpoints from it, so login is directed at your
instance's authorization server rather than github.com:

```bash
github-mcp-server stdio \
  --gh-host https://github.example.com \
  --oauth-client-id <YOUR_CLIENT_ID>
```

- For GitHub Enterprise Server, prefix the host with `https://`.
- For `ghe.com`, use `https://YOURSUBDOMAIN.ghe.com`.

Register the app's callback URL on the same host (e.g.
`http://localhost/callback` for native runs, or `http://localhost:8085/callback`
for Docker).

## Building from source with baked-in credentials

Official builds embed the default OAuth client via linker flags at build time, so
they are not present in the source tree. To produce your own build with embedded
credentials, set them with `-ldflags`:

```bash
go build -ldflags "\
  -X github.com/github/github-mcp-server/internal/buildinfo.OAuthClientID=<CLIENT_ID> \
  -X github.com/github/github-mcp-server/internal/buildinfo.OAuthClientSecret=<CLIENT_SECRET>" \
  ./cmd/github-mcp-server
```

Without these, a source build simply has no baked-in app and expects
`--oauth-client-id` (or a PAT) at runtime.
