# Policies & Governance for the GitHub MCP Server

Organizations and enterprises have several existing control mechanisms for the GitHub MCP server on GitHub.com:
- MCP servers in Copilot Policy
- Copilot Editor Preview Policy (temporary)
- OAuth App Access Policies
- GitHub App Installation
- Personal Access Token (PAT) policies
- SSO Enforcement

This document outlines how these policies apply to different deployment modes, authentication methods, and host applications – while providing guidance for managing GitHub MCP Server access across your organization.

## How the GitHub MCP Server Works

The GitHub MCP Server provides access to GitHub resources and capabilities through a standardized protocol, with flexible deployment and authentication options tailored to different use cases. It supports two deployment modes, both built on the same underlying codebase.

### 1. Local GitHub MCP Server
* **Runs:** Locally alongside your IDE or application
* **Authentication & Controls:** Requires Personal Access Tokens (PATs). Users must generate and configure a PAT to connect. Managed via [PAT policies](https://docs.github.com/organizations/managing-programmatic-access-to-your-organization/setting-a-personal-access-token-policy-for-your-organization#restricting-access-by-personal-access-tokens).
  * Can optionally use GitHub App installation tokens when embedded in a GitHub App-based tool (rare).
 
**Supported SKUs:** Can be used with GitHub Enterprise Server (GHES) and GitHub Enterprise Cloud (GHEC).

### 2. Remote GitHub MCP Server
* **Runs:** As a hosted service accessed over the internet
* **Authentication & Controls:** (determined by the chosen authentication method)
  * **GitHub App Installation Tokens:** Uses a signed JWT to request installation access tokens (similar to the OAuth 2.0 client credentials flow) to operate as the application itself. Provides granular control via [installation](https://docs.github.com/apps/using-github-apps/installing-a-github-app-from-a-third-party#requirements-to-install-a-github-app), [permissions](https://docs.github.com/apps/creating-github-apps/registering-a-github-app/choosing-permissions-for-a-github-app) and [repository access controls](https://docs.github.com/apps/using-github-apps/reviewing-and-modifying-installed-github-apps#modifying-repository-access).
  * **OAuth Authorization Code Flow:** Uses the standard OAuth 2.0 Authorization Code flow. Controlled via [OAuth App access policies](https://docs.github.com/organizations/managing-oauth-access-to-your-organizations-data/about-oauth-app-access-restrictions) for OAuth apps. For GitHub Apps that sign in ([are authorized by](https://docs.github.com/apps/using-github-apps/authorizing-github-apps)) a user, control access to your organization via [installation](https://docs.github.com/apps/using-github-apps/installing-a-github-app-from-a-third-party#requirements-to-install-a-github-app).
  * **Personal Access Tokens (PATs):** Managed via [PAT policies](https://docs.github.com/organizations/managing-programmatic-access-to-your-organization/setting-a-personal-access-token-policy-for-your-organization#restricting-access-by-personal-access-tokens).
  * **SSO enforcement:** Applies when using OAuth Apps, GitHub Apps, and PATs to access resources in organizations and enterprises with SSO enabled. Acts as an overlay control. Users must have a valid SSO session for your organization or enterprise when signing into the app or creating the token in order for the token to access your resources. Learn more in the [SSO documentation](https://docs.github.com/enterprise-cloud@latest/authentication/authenticating-with-single-sign-on/about-authentication-with-single-sign-on#about-oauth-apps-github-apps-and-sso).

**Supported Platforms:** Currently available only on GitHub Enterprise Cloud (GHEC). Remote hosting for GHES is not supported at this time.

> **Note:** This does not apply to the Local GitHub MCP Server, which uses PATs and does not rely on GitHub App installations.

#### Enterprise Install Considerations

- When using the Remote GitHub MCP Server, if authenticating with OAuth instead of PAT, each host application must have a registered GitHub App (or OAuth App) to authenticate on behalf of the user.
- Enterprises may choose to install these apps in multiple organizations (e.g., per team or department) to scope access narrowly, or at the enterprise level to centralize access control across all child organizations. 
- Enterprise installation is only supported for GitHub Apps. OAuth Apps can only be installed on a per organization basis in multi-org enterprises.

### Security Principles for Both Modes
* **Authentication:** Required for all operations, no anonymous access
* **Authorization:** Access enforced by GitHub's native permission model. Users and apps cannot use an MCP server to access more resources than they could otherwise access normally via the API.
* **Communication:** All data transmitted over HTTPS with optional SSE for real-time updates
* **Rate Limiting:** Subject to GitHub API rate limits based on authentication method
* **Token Storage:** Tokens should be stored securely using platform-appropriate credential storage
* **Audit Trail:** All underlying API calls are logged in GitHub's audit log when available

For integration architecture and implementation details, see the [Host Integration Guide](https://github.com/github/github-mcp-server/blob/main/docs/host-integration.md).

## Where It's Used

The GitHub MCP server can be accessed in various environments (referred to as "host" applications):
* **First-party Hosts:** GitHub Copilot in VS Code, Visual Studio, JetBrains, Eclipse, and Xcode with integrated MCP support, as well as Copilot Coding Agent.
* **Third-party Hosts:** Editors outside the GitHub ecosystem, such as Claude, Cursor, Windsurf, and Cline, that support connecting to MCP servers, as well as AI chat applications like Claude Desktop and other AI assistants that connect to MCP servers to fetch GitHub context or execute write actions.

## What It Can Access

The MCP server accesses GitHub resources based on the permissions granted through the chosen authentication method (PAT, OAuth, or GitHub App). These may include:
* Repository contents (files, branches, commits)
* Issues and pull requests
* Organization and team metadata
* User profile information
* Actions workflow runs, logs, and statuses
* Security and vulnerability alerts (if explicitly granted)

Access is always constrained by GitHub's public API permission model and the authenticated user's privileges.

## Control Mechanisms

### 1. Copilot Editors (first-party) → MCP Servers in Copilot Policy

* **Policy:** MCP servers in Copilot
* **Location:** Enterprise/Org → Policies → Copilot
* **What it controls:** When disabled, **completely blocks all GitHub MCP Server access** (both remote and local) for affected Copilot editors. Currently applies to VS Code and Copilot Coding Agent, with more Copilot editors expected to migrate to this policy over time.
* **Impact when disabled:** Host applications governed by this policy cannot connect to the GitHub MCP Server through any authentication method (OAuth, PAT, or GitHub App).
* **What it does NOT affect:**
  * MCP support in Copilot on IDEs that are still in public preview (Visual Studio, JetBrains, Xcode, Eclipse)
  * Third-party IDE or host apps (like Claude, Cursor, Windsurf) not governed by GitHub's Copilot policies
  * Community-authored MCP servers using GitHub's public APIs

> **Important:** This policy provides comprehensive control over GitHub MCP Server access in Copilot editors. When disabled, users in affected applications will not be able to use the GitHub MCP Server regardless of deployment mode (remote or local) or authentication method.

#### Temporary: Copilot Editor Preview Policy

* **Policy:** Editor Preview Features  
* **Status:** Being phased out as editors migrate to the "MCP servers in Copilot" policy above, and once the Remote GitHub MCP server goes GA
* **What it controls:** When disabled, prevents remaining Copilot editors from using the Remote GitHub MCP Server through OAuth connections in all first-party and third-party host applications (does not affect local deployments or PAT authentication)

> **Note:** As Copilot editors migrate from the "Copilot Editor Preview" policy to the "MCP servers in Copilot" policy, the scope of control becomes more centralized, blocking both remote and local GitHub MCP Server access when disabled. Access in third-party hosts is governed separately by OAuth App, GitHub App, and PAT policies.

### 2. Third-Party Host Apps (e.g., Claude, Cursor, Windsurf) → OAuth App or GitHub App Controls

#### a. OAuth App Access Policies
* **Control Mechanism:** OAuth App access restrictions
* **Location:** Org → Settings → Third-party Access → OAuth app policy
* **How it works:**
  * Organization admins must approve OAuth App requests before host apps can access organization data
  * Only applies when the host registers an OAuth App AND the user connects via OAuth 2.0 flow

#### b. GitHub App Installation
* **Control Mechanism:** GitHub App installation and permissions
* **Location:** Org → Settings → Third-party Access → GitHub Apps
* **What it controls:** Organization admins must install the app, select repositories, and grant permissions before the app can access organization-owned data or resources through the Remote GitHub Server.
* **How it works:**
  * Organization admins must install the app, specify repositories, and approve permissions
  * Only applies when the host registers a GitHub App AND the user authenticates through that flow

> **Note:** The authentication methods available depend on what your host application supports. While PATs work with any remote MCP-compatible host, OAuth and GitHub App authentication are only available if the host has registered an app with GitHub. Check your host application's documentation or support for more info.

### 3. PAT Access from Any Host → PAT Restrictions

* **Types:** Fine-grained PATs (recommended) and Classic tokens (legacy)
* **Location:**
  * User level: [Personal Settings → Developer Settings → Personal Access Tokens](https://docs.github.com/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens#fine-grained-personal-access-tokens)
  * Enterprise/Organization level: [Enterprise/Organization → Settings → Personal Access Tokens](https://docs.github.com/organizations/managing-programmatic-access-to-your-organization/setting-a-personal-access-token-policy-for-your-organization) (to control PAT creation/access policies)
* **What it controls:** Applies to all host apps and both local & remote GitHub MCP servers when users authenticate via PAT.
* **How it works:** Access limited to the repositories and scopes selected on the token.
* **Limitations:** PATs do not adhere to OAuth App policies and GitHub App installation controls. They are user-scoped and not recommended for production automation.
* **Organization controls:**
  * Classic PATs: Can be completely disabled organization-wide
  * Fine-grained PATs: Cannot be disabled but require explicit approval for organization access

> **Recommendation:** We recommend using fine-grained PATs over classic tokens. Classic tokens have broader scopes and can be disabled in organization settings.

### 4. SSO Enforcement (overlay control)

* **Location:** Enterprise/Organization → SSO settings
* **What it controls:** OAuth tokens and PATs must map to a recent SSO login to access SSO-protected organization data.
* **How it works:** Applies to ALL host apps when using OAuth or PATs.

> **Exception:** Does NOT apply to GitHub App installation tokens (these are installation-scoped, not user-scoped)

## Current Limitations

While the GitHub MCP Server provides dynamic tooling and capabilities, the following enterprise governance features are not yet available:

### Single Enterprise/Organization-Level Toggle

GitHub does not provide a single toggle that blocks all GitHub MCP server traffic for every user. Admins can achieve equivalent coverage by combining the controls shown here:
* **First-party Copilot Editors (GitHub Copilot in VS Code, Visual Studio, JetBrains, Eclipse):**
  * Disable the "MCP servers in Copilot" policy for comprehensive control
  * Or disable the Editor Preview Features policy (for editors still using the legacy policy)
* **Third-party Host Applications:**
  * Configure OAuth app restrictions
  * Manage GitHub App installations
* **PAT Access in All Host Applications:**
  * Implement fine-grained PAT policies (applies to both remote and local deployments)

### MCP-Specific Audit Logging

At present, MCP traffic appears in standard GitHub audit logs as normal API calls. Purpose-built logging for MCP is on the roadmap, but the following views are not yet available:
* Real-time list of active MCP connections
* Dashboards showing granular MCP usage data, like tools or host apps
* Granular, action-by-action audit logs

Until those arrive, teams can continue to monitor MCP activity through existing API log entries and OAuth/GitHub App events.

## Security Best Practices

### For Organizations

**GitHub App Management**
* Review [GitHub App installations](https://docs.github.com/apps/using-github-apps/reviewing-and-modifying-installed-github-apps) regularly
* Audit permissions and repository access
* Monitor installation events in audit logs
* Document approved GitHub Apps and their business purposes

**OAuth App Governance**
* Manage [OAuth App access policies](https://docs.github.com/organizations/managing-oauth-access-to-your-organizations-data/about-oauth-app-access-restrictions)
* Establish review processes for approved applications
* Monitor which third-party applications are requesting access
* Maintain an allowlist of approved OAuth applications

**Token Management**
* Mandate fine-grained Personal Access Tokens over classic tokens
* Establish token expiration policies (90 days maximum recommended)
* Implement automated token rotation reminders
* Review and enforce [PAT restrictions](https://docs.github.com/organizations/managing-programmatic-access-to-your-organization/setting-a-personal-access-token-policy-for-your-organization) at the appropriate level

### For Developers and Users

**Authentication Security**
* Prioritize OAuth 2.0 flows over long-lived tokens
* Prefer fine-grained PATs to PATs (Classic)
* Store tokens securely using platform-appropriate credential management
* Store credentials in secret management systems, not source code

**Scope Minimization**
* Request only the minimum required scopes for your use case
* Regularly review and revoke unused token permissions
* Use repository-specific access instead of organization-wide access
* Document why each permission is needed for your integration

## Resources

**MCP:**
* [Model Context Protocol Specification](https://modelcontextprotocol.io/specification/2025-03-26)
* [Model Context Protocol Authorization](https://modelcontextprotocol.io/specification/draft/basic/authorization)

**GitHub Governance & Controls:**
* [Managing OAuth App Access](https://docs.github.com/organizations/managing-oauth-access-to-your-organizations-data/about-oauth-app-access-restrictions)
* [GitHub App Permissions](https://docs.github.com/apps/creating-github-apps/registering-a-github-app/choosing-permissions-for-a-github-app)
* [Updating permissions for a GitHub App](https://docs.github.com/apps/using-github-apps/approving-updated-permissions-for-a-github-app)
* [PAT Policies](https://docs.github.com/organizations/managing-programmatic-access-to-your-organization/setting-a-personal-access-token-policy-for-your-organization)
* [Fine-grained PATs](https://docs.github.com/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens#fine-grained-personal-access-tokens)
* [Setting a PAT policy for your organization](https://docs.github.com/organizations/managing-oauth-access-to-your-organizations-data/about-oauth-app-access-restrictions)

---

**Questions or Feedback?**

Open an [issue in the github-mcp-server repository](https://github.com/github/github-mcp-server/issues) with the label "policies & governance" attached.

This document reflects GitHub MCP Server policies as of July 2025. Policies and capabilities continue to evolve based on customer feedback and security best practices.
