# GitHub MCP Server 的策略与治理

组织和企业目前可通过 GitHub.com 上的多种控制机制管理 GitHub MCP Server：
- Copilot 中的 MCP Server 策略
- Copilot 编辑器预览策略（临时）
- OAuth App 访问策略
- GitHub App 安装
- Personal Access Token（PAT）策略
- SSO 强制执行

本文档说明这些策略如何应用于不同的部署模式、身份认证方式和 Host 应用，同时为组织范围内的 GitHub MCP Server 访问管理提供指导。

## GitHub MCP Server 的工作方式

GitHub MCP Server 通过标准化协议提供对 GitHub 资源和能力的访问，并针对不同使用场景提供灵活的部署和身份认证选项。它支持两种部署模式，二者均基于同一套底层代码库构建。

### 1. Local GitHub MCP Server
* **运行方式：** 在本地与 IDE 或应用一同运行
* **身份认证与控制：** 需要使用 Personal Access Token（PAT）。用户必须生成并配置 PAT 才能连接。通过 [PAT 策略](https://docs.github.com/organizations/managing-programmatic-access-to-your-organization/setting-a-personal-access-token-policy-for-your-organization#restricting-access-by-personal-access-tokens)进行管理。
  * 当嵌入基于 GitHub App 的工具时，也可以选择使用 GitHub App installation token（较少见）。
 
**支持的 SKU：** 可用于 GitHub Enterprise Server（GHES）和 GitHub Enterprise Cloud（GHEC）。

### 2. Remote GitHub MCP Server
* **运行方式：** 作为通过互联网访问的托管服务运行
* **身份认证与控制：**（由所选择的身份认证方式决定）
  * **GitHub App Installation Token：** 使用签名 JWT 请求 installation access token（类似于 OAuth 2.0 client credentials flow），从而以应用自身的身份执行操作。可通过[安装](https://docs.github.com/apps/using-github-apps/installing-a-github-app-from-a-third-party#requirements-to-install-a-github-app)、[权限](https://docs.github.com/apps/creating-github-apps/registering-a-github-app/choosing-permissions-for-a-github-app)和[仓库访问控制](https://docs.github.com/apps/using-github-apps/reviewing-and-modifying-installed-github-apps#modifying-repository-access)实现细粒度控制。
  * **OAuth Authorization Code Flow：** 使用标准的 OAuth 2.0 Authorization Code flow。对于 OAuth App，可通过 [OAuth App 访问策略](https://docs.github.com/organizations/managing-oauth-access-to-your-organizations-data/about-oauth-app-access-restrictions)进行控制。对于以用户身份登录（[获得用户授权](https://docs.github.com/apps/using-github-apps/authorizing-github-apps)）的 GitHub App，可通过[安装](https://docs.github.com/apps/using-github-apps/installing-a-github-app-from-a-third-party#requirements-to-install-a-github-app)控制其对组织的访问。
  * **Personal Access Token（PAT）：** 通过 [PAT 策略](https://docs.github.com/organizations/managing-programmatic-access-to-your-organization/setting-a-personal-access-token-policy-for-your-organization#restricting-access-by-personal-access-tokens)进行管理。
  * **SSO 强制执行：** 当使用 OAuth App、GitHub App 和 PAT 访问已启用 SSO 的组织和企业资源时适用。该机制作为一层叠加控制。用户在登录应用或创建 token 时，必须具有组织或企业的有效 SSO 会话，token 才能访问相应资源。有关更多信息，请参阅 [SSO 文档](https://docs.github.com/enterprise-cloud@latest/authentication/authenticating-with-single-sign-on/about-authentication-with-single-sign-on#about-oauth-apps-github-apps-and-sso)。

**支持的平台：** 当前仅适用于 GitHub Enterprise Cloud（GHEC）。目前不支持 GHES 的远程托管。

> **注意：** 这不适用于 Local GitHub MCP Server，因为 Local GitHub MCP Server 使用 PAT，不依赖 GitHub App 安装。

#### 企业安装注意事项

- 使用 Remote GitHub MCP Server 时，如果使用 OAuth 而不是 PAT 进行身份认证，则每个 Host 应用都必须注册 GitHub App（或 OAuth App），以代表用户完成身份认证。
- 企业可以选择将这些应用安装到多个组织中（例如按团队或部门划分），以缩小访问范围；也可以在企业层级安装，从而集中控制所有子组织的访问。 
- 企业级安装仅支持 GitHub App。对于包含多个组织的企业，OAuth App 只能按组织分别安装。

### 两种模式共同遵循的安全原则
* **身份认证：** 所有操作都必须经过身份认证，不允许匿名访问
* **授权：** 访问权限由 GitHub 原生权限模型强制执行。用户和应用无法通过 MCP Server 访问其原本无法通过 API 正常访问的更多资源。
* **通信：** 所有数据均通过 HTTPS 传输，并可选择使用 SSE 获取实时更新
* **速率限制：** 根据身份认证方式受到 GitHub API 速率限制
* **Token 存储：** 应使用适合目标平台的凭证存储方式安全存储 token
* **审计记录：** 在支持的情况下，所有底层 API 调用都会记录在 GitHub 审计日志中

有关集成架构和实现细节，请参阅 [Host 集成指南](https://github.com/github/github-mcp-server/blob/main/docs/host-integration.md)。

## 使用场景

GitHub MCP Server 可在多种环境中访问，这些环境称为“Host”应用：
* **第一方 Host：** 在 VS Code、Visual Studio、JetBrains、Eclipse 和 Xcode 中集成了 MCP 支持的 GitHub Copilot，以及 Copilot Coding Agent。
* **第三方 Host：** GitHub 生态之外支持连接 MCP Server 的编辑器，例如 Claude、Cursor、Windsurf 和 Cline；还包括 Claude Desktop 等 AI 聊天应用，以及其他连接 MCP Server 以获取 GitHub 上下文或执行写操作的 AI 助手。

## 可以访问的内容

MCP Server 根据所选身份认证方式（PAT、OAuth 或 GitHub App）授予的权限访问 GitHub 资源。这些资源可能包括：
* 仓库内容（文件、分支、提交）
* Issue 和 pull request
* 组织和团队元数据
* 用户个人资料信息
* Actions workflow 的运行记录、日志和状态
* 安全和漏洞警报（如果已明确授权）

访问权限始终受 GitHub 公开 API 权限模型和已认证用户自身权限的约束。

## 控制机制

### 1. Copilot 编辑器（第一方）→ Copilot 中的 MCP Server 策略

* **策略：** Copilot 中的 MCP Server
* **位置：** Enterprise/Org → Policies → Copilot
* **控制内容：** 禁用后，将为受影响的 Copilot 编辑器**完全阻止所有 GitHub MCP Server 访问**，包括远程和本地模式。当前适用于 VS Code 和 Copilot Coding Agent，预计后续会有更多 Copilot 编辑器逐步迁移到该策略。
* **禁用后的影响：** 受该策略管控的 Host 应用无法通过任何身份认证方式（OAuth、PAT 或 GitHub App）连接 GitHub MCP Server。
* **不受影响的内容：**
  * 仍处于 public preview 阶段的 IDE 中的 Copilot MCP 支持（Visual Studio、JetBrains、Xcode、Eclipse）
  * 不受 GitHub Copilot 策略管控的第三方 IDE 或 Host 应用（例如 Claude、Cursor、Windsurf）
  * 使用 GitHub 公开 API、由社区开发的 MCP Server

> **重要：** 该策略能够对 Copilot 编辑器中的 GitHub MCP Server 访问实施全面控制。禁用后，无论采用何种部署模式（远程或本地）或身份认证方式，受影响应用中的用户都无法使用 GitHub MCP Server。

#### 临时策略：Copilot 编辑器预览策略

* **策略：** Editor Preview Features  
* **状态：** 随着编辑器迁移到上述“Copilot 中的 MCP Server”策略，并且 Remote GitHub MCP Server 正式 GA，该策略将逐步淘汰
* **控制内容：** 禁用后，将阻止尚未迁移的 Copilot 编辑器在所有第一方和第三方 Host 应用中通过 OAuth 连接使用 Remote GitHub MCP Server，但不会影响本地部署或 PAT 身份认证

> **注意：** 随着 Copilot 编辑器从“Copilot Editor Preview”策略迁移到“Copilot 中的 MCP Server”策略，控制范围会更加集中；禁用后会同时阻止远程和本地 GitHub MCP Server 访问。第三方 Host 中的访问则分别由 OAuth App、GitHub App 和 PAT 策略管理。

### 2. 第三方 Host 应用（例如 Claude、Cursor、Windsurf）→ OAuth App 或 GitHub App 控制

#### a. OAuth App 访问策略
* **控制机制：** OAuth App 访问限制
* **位置：** Org → Settings → Third-party Access → OAuth app policy
* **工作方式：**
  * 在 Host 应用访问组织数据之前，组织管理员必须批准 OAuth App 请求
  * 仅当 Host 注册了 OAuth App，并且用户通过 OAuth 2.0 flow 连接时适用

#### b. GitHub App 安装
* **控制机制：** GitHub App 安装和权限
* **位置：** Org → Settings → Third-party Access → GitHub Apps
* **控制内容：** 在应用能够通过 Remote GitHub Server 访问组织拥有的数据或资源之前，组织管理员必须安装该应用、选择仓库并授予权限。
* **工作方式：**
  * 组织管理员必须安装应用、指定仓库并批准权限
  * 仅当 Host 注册了 GitHub App，并且用户通过该流程进行身份认证时适用

> **注意：** 可用的身份认证方式取决于 Host 应用所支持的能力。PAT 可用于任何兼容 Remote MCP 的 Host，而 OAuth 和 GitHub App 身份认证仅在 Host 已向 GitHub 注册应用时可用。有关更多信息，请查阅 Host 应用的文档或支持说明。

### 3. 任意 Host 通过 PAT 访问 → PAT 限制

* **类型：** Fine-grained PAT（推荐）和 Classic token（旧版）
* **位置：**
  * 用户层级：[Personal Settings → Developer Settings → Personal Access Tokens](https://docs.github.com/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens#fine-grained-personal-access-tokens)
  * 企业/组织层级：[Enterprise/Organization → Settings → Personal Access Tokens](https://docs.github.com/organizations/managing-programmatic-access-to-your-organization/setting-a-personal-access-token-policy-for-your-organization)（用于控制 PAT 创建和访问策略）
* **控制内容：** 当用户通过 PAT 进行身份认证时，该机制适用于所有 Host 应用，以及本地和远程 GitHub MCP Server。
* **工作方式：** 访问权限仅限于 token 中选择的仓库和 scope。
* **限制：** PAT 不受 OAuth App 策略和 GitHub App 安装控制约束。PAT 的作用域基于用户，因此不建议用于生产自动化。
* **组织控制：**
  * Classic PAT：可在整个组织范围内完全禁用
  * Fine-grained PAT：无法禁用，但访问组织资源需要明确批准

> **建议：** 建议优先使用 Fine-grained PAT，而不是 Classic token。Classic token 的 scope 范围更广，并且可以在组织设置中被禁用。

### 4. SSO 强制执行（叠加控制）

* **位置：** Enterprise/Organization → SSO settings
* **控制内容：** OAuth token 和 PAT 必须关联近期的 SSO 登录，才能访问受 SSO 保护的组织数据。
* **工作方式：** 使用 OAuth 或 PAT 时，适用于所有 Host 应用。

> **例外：** 不适用于 GitHub App installation token，因为这类 token 的作用域基于 installation，而不是用户

## 当前限制

虽然 GitHub MCP Server 提供了动态工具和能力，但目前尚不具备以下企业治理功能：

### 企业/组织层级的统一开关

GitHub 当前不提供一个可为所有用户阻止全部 GitHub MCP Server 流量的统一开关。管理员可以组合使用以下控制机制，实现相同的覆盖范围：
* **第一方 Copilot 编辑器（VS Code、Visual Studio、JetBrains、Eclipse 中的 GitHub Copilot）：**
  * 禁用“Copilot 中的 MCP Server”策略，以实现全面控制
  * 或禁用 Editor Preview Features 策略（适用于仍在使用旧版策略的编辑器）
* **第三方 Host 应用：**
  * 配置 OAuth App 限制
  * 管理 GitHub App 安装
* **所有 Host 应用中的 PAT 访问：**
  * 实施 Fine-grained PAT 策略（同时适用于远程和本地部署）

### MCP 专用审计日志

目前，MCP 流量会作为普通 API 调用出现在标准 GitHub 审计日志中。专门为 MCP 构建的日志功能已列入规划，但目前尚不支持以下视图：
* 当前活跃 MCP 连接的实时列表
* 展示工具或 Host 应用等细粒度 MCP 使用数据的仪表板
* 逐操作、细粒度的审计日志

在这些能力上线之前，团队仍可通过现有 API 日志条目和 OAuth/GitHub App 事件监控 MCP 活动。

## 安全最佳实践

### 面向组织

**GitHub App 管理**
* 定期检查 [GitHub App 安装](https://docs.github.com/apps/using-github-apps/reviewing-and-modifying-installed-github-apps)
* 审计权限和仓库访问范围
* 在审计日志中监控安装事件
* 记录已批准的 GitHub App 及其业务用途

**OAuth App 治理**
* 管理 [OAuth App 访问策略](https://docs.github.com/organizations/managing-oauth-access-to-your-organizations-data/about-oauth-app-access-restrictions)
* 为已批准的应用建立审核流程
* 监控正在申请访问权限的第三方应用
* 维护已批准 OAuth 应用的 allowlist

**Token 管理**
* 强制使用 Fine-grained Personal Access Token，而不是 Classic token
* 制定 token 过期策略（建议最长为 90 天）
* 实现自动 token 轮换提醒
* 在适当层级检查并强制执行 [PAT 限制](https://docs.github.com/organizations/managing-programmatic-access-to-your-organization/setting-a-personal-access-token-policy-for-your-organization)

### 面向开发者和用户

**身份认证安全**
* 优先使用 OAuth 2.0 flow，而不是长期有效的 token
* 优先使用 Fine-grained PAT，而不是 PAT（Classic）
* 使用适合目标平台的凭证管理方式安全存储 token
* 将凭证存储在 secret 管理系统中，而不是源代码中

**最小化 Scope**
* 仅请求当前使用场景所需的最小 scope
* 定期检查并撤销未使用的 token 权限
* 使用仓库级访问，而不是组织级访问
* 记录集成所需每项权限的原因

## 资源

**MCP：**
* [Model Context Protocol 规范](https://modelcontextprotocol.io/specification/2025-03-26)
* [Model Context Protocol 授权](https://modelcontextprotocol.io/specification/draft/basic/authorization)

**GitHub 治理与控制：**
* [管理 OAuth App 访问](https://docs.github.com/organizations/managing-oauth-access-to-your-organizations-data/about-oauth-app-access-restrictions)
* [GitHub App 权限](https://docs.github.com/apps/creating-github-apps/registering-a-github-app/choosing-permissions-for-a-github-app)
* [更新 GitHub App 的权限](https://docs.github.com/apps/using-github-apps/approving-updated-permissions-for-a-github-app)
* [PAT 策略](https://docs.github.com/organizations/managing-programmatic-access-to-your-organization/setting-a-personal-access-token-policy-for-your-organization)
* [Fine-grained PAT](https://docs.github.com/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens#fine-grained-personal-access-tokens)
* [为组织设置 PAT 策略](https://docs.github.com/organizations/managing-oauth-access-to-your-organizations-data/about-oauth-app-access-restrictions)

---