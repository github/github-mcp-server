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

> For non-interactive stdio deployments, see
> [GitHub App authentication](github-app-auth.md).

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

go build -ldflags "\
  -X github.com/github/github-mcp-server/internal/buildinfo.OAuthClientID=<CLIENT_ID> \
  -X github.com/github/github-mcp-server/internal/buildinfo.OAuthClientSecret=<CLIENT_SECRET>" \
  ./cmd/github-mcp-servergithub.com/github/github-mcp-server/internal/buildinfo.OAuthClientID=github.com/github/github-mcp-server/internal/buildinfo.OAuthClientSecret=# GitHub Pre-release License Terms

## GitHub Pre-release License Terms

These terms apply to the pre-release software made available to you by GitHub. To the extent there is a conflict between these terms and any other Agreement you have with us, these terms govern.

## 1. Pre-Release Software.

  The software provided is a pre-release version. “Pre-release” means software, online services, and additional products and features that are not yet generally available, such as private preview, public preview, early access, technical preview, or similar versions.

  Pre-release software may not operate correctly. It may delete your data, corrupt your data, or have other bugs. It also may not work the way a final commercial version of the software will.

  GitHub may change or discontinue pre-release software at any time, for any reason, without notice to you. GitHub may change the software for the final commercial version, or may not release a commercial version at all. GitHub is not obligated to provide to you any maintenance, technical support, or updates for the software.

## 2. Installation and Use Rights.

  a. **General**. GitHub grants you a limited right to use a non-production instance of the software for evaluation and testing. This means you may use any number of copies of the software to evaluate its functionality and internally develop and test your applications, including deployment of the software within your internal corporate network for evaluation but not external distribution. You may also use the software in internally demonstrating your applications, but may not use the pre-release software in any active production environment, including any use to process live customer data.

  b. **Inclusion of Third-Party Components.** The software may include third-party components with separate legal notices or governed by other agreements, as may be described in a license file accompanying the software.

  c. **Optional Extensions.** The software may give you the option to download other GitHub and third-party software packages. Any third-party software packages are provided for your convenience only, and are governed by any applicable agreements between you and the third party. GitHub is not responsible or liable for any third-party software.

## 3. Scope of License.

  GitHub reserves all rights not expressly granted to you in these terms, including retaining ownership of all aspects of the pre-release software as well as all related intellectual property rights.

  Unless applicable law gives you more rights despite this limitation, you may use the software only as expressly permitted in this Agreement. In doing so, you must comply with any technical limitations in the software that only allow you to use it in certain ways. You may not:

  a. work around any technical limitations in the software;

  b. reverse engineer, decompile, or disassemble the software, or otherwise attempt to derive the source code for the software, except to the extent required by applicable third party licensing terms governing use of certain open source components that may be included in the software;

  c. remove, minimize, block, or modify any notices of GitHub or its suppliers in the software;

  d. share, publish, or lease the software;

  e. provide the software as a stand-alone offering or combine it with any of your applications for others to use;

  f. transfer the software or these terms to any third party; or

  g. use the software to create or propagate malware, or in any way that is against the law

## 4. Data Collection and Usage.

  a. **Consent to Data Collection.** The pre-release software may collect telemetry information about you and your use of the software, and send that information to GitHub. Subject to the limitations in Section 4(b) below, GitHub may use this information to provide services, to improve our products and services, or for any other purpose permitted under the GitHub Data Protection Agreement or GitHub Privacy Statement. Your use of the pre-release software operates as your consent to these practices.

  b. **Use of Collected Data.**
  * GitHub will use collected data for analytics and measurement to understand how our pre-release software and related products are used.
  * The software will collect data and usage information about events generated when interacting with it. These events help us analyze and measure performance and features used. This usage information is used by GitHub and may be shared with affiliates and other third parties to help deliver, develop, evaluate, and improve the software and related products.
  * We analyze data to ensure the pre-release software is working as intended, to evaluate the safety, reliability, and user experience of the software, and to investigate and detect potential abuse.
  * We may combine the information we collect from the pre-release software with other data.
  * For pre-release software that uses AI:
      * You retain ownership of the code that you input to the software.
      * GitHub does not own the output sent to you by the software.
      * GitHub will not use Copilot Business or Copilot Enterprise Inputs or the Outputs generated to train AI language models, unless you have instructed us in writing to do so.

  c. **Processing of Personal Data.** GitHub is the data controller in relation to the Personal Data processed in connection with the pre-release software.

  d. **Data Collection by You.** There may be some features in the pre-release software that enable you and GitHub to collect data from users of your applications. If you use these features, you must comply with all applicable laws on data collection, including providing appropriate notices to users of your applications as well as a copy of GitHub’s Privacy Statement. You can learn more about data collection and use in the software documentation.

  e. **Revocation of Consent to Data Collection.** You may revoke your consent to data collection by the prerelease software by contacting GitHub and requesting removal from the technical preview. Please note that, if you request removal from the preview, you will no longer be able to use the pre-release software.

## 5. Updates.

  You may obtain updates to the pre-release software only from GitHub or GitHub-authorized sources. The software may install automatic updates and download and install them for you. You agree to these automatic updates without any additional notice. Software updates may not include or support all existing software features, platforms, services, or peripheral devices. These updates are generally meant to improve and evolve the software, but they may also change or disable any part of the software, including potentially removing features and services, or revoking support for certain platforms or hardware.

## 6. Time-Bound Software.

  Your use of the pre-release software will end upon any of _(i)_ commercial release of the software, or _(ii)_ at the discretion of GitHub to discontinue the support or development of the software, or _(iii)_ termination of the technical preview by either party. You may not be able to access data used in the software when it stops running.

## 7. Feedback.

  If you give feedback about the pre-release software to GitHub, you give to GitHub the right to use, share, and commercialize your feedback in any way and for any purpose, without payment to you. You agree that you will not give feedback that is subject to any license that would require GitHub to license its software or documentation to third parties if we included your feedback in them.

## 8. Communications.

  By using pre-release software, you agree to be contacted by GitHub and Microsoft regarding your participation in the technical preview, including email request(s) for feedback about the software.

## 9. No Warranties.

You bear the sole risk of using the pre-release software.

  The pre-release software is licensed “as is” without any warranty of any kind or sort, whether such warranty would be express, implied, or statutory. To the extent permitted under your local laws, GitHub and Microsoft disclaim all warranties in the pre-release software, including implied warranties of merchantability, fitness for a particular purpose, title, quiet enjoyment, accuracy, course of dealing, usage of trade, and non-infringement.

  Neither GitHub nor Microsoft give any express warranties, guarantees, or commitments about the pre-release software or its quality, reliability, availability, security, or function. The software may contain errors, may delete or corrupt your data, and may have defects or other bugs.

## 10. Defense of Third Party Claims.

  If your Agreement provides for the defense of third party claims, that provision will apply to your use of the pre-release software and the outputs you receive from it. For software that uses artificial intelligence, you must have complied with (a) the Acceptable Use Policies in your Agreement, and (b) the [Microsoft Enterprise AI Services Code of Conduct](https://aka.ms/AIcode), and (c) the [Microsoft Customer Copyright Commitment Required Mitigations](https://aka.ms/AIfilters).

## 11. No Uptime Guarantees.

  The pre-release software is not subject to an uptime guarantee or similar service level agreement. The software may be unavailable or stop working entirely at any time for any reason.

## 12. Limitation of Liability.

  GitHub’s maximum liability for any claim related to your use of the pre-release software is limited to direct damages up to five hundred dollars ($500.00 USD). This limit will not apply to the defense obligations in Section 10.

## 13. Compliance with Export Restrictions.

  You must comply with all domestic and international export laws and regulations that apply to the pre-release software, including any applicable restrictions on destinations, end users, and end use.

## 14. Confidentiality.

  The pre-release software is non-public, confidential information of GitHub. Your use is subject to the confidentiality obligations between you and GitHub in the Agreement.

  Please do not _(i)_ disclose or share the software with anyone who is not subject to these terms and a non-disclosure agreement; _(ii)_ post or allow others to post any photos or videos of the pre-release software on or via any online platform, including personal social media websites; or _(iii)_ describe or discuss any part of the software on or via any online platform, unless given advance and express permission by GitHub to do so.

## 15. Nature of Terms for Microsoft Customers.

  If you license GitHub through Microsoft, these terms shall be considered an amendment to the Microsoft Product Terms for GitHub Offerings for the duration of your use of the pre-release software.content/site-policy/github-terms/github-pre-release-license-terms.md
