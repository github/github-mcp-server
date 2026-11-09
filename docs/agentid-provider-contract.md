# AgentID Provider Authorization Contract Example

The GitHub MCP Server already supports several access controls, including
toolsets, individual tool allowlists, excluded tools, read-only mode, lockdown
mode, OAuth/PAT scope filtering, and GitHub's native API authorization model.

For organizations routing GitHub MCP Server traffic through an enterprise MCP
gateway, an additional pattern is to publish a provider-side authorization
contract that describes the blast radius and context requirements for each
tool. A gateway can import that contract, apply local agent/user/job policy,
and then forward high-risk tool calls with a scoped authorization receipt.

This does not replace GitHub API authorization. It gives the enterprise gateway
a machine-readable contract for deciding whether an agent-originated tool call
should be attempted before the GitHub MCP Server and GitHub API enforce their
normal permissions.

## Example Flow

```text
Agent or IDE
  -> Enterprise MCP gateway
  -> AgentID authorization check
  -> GitHub MCP Server
  -> GitHub API authorization
  -> Tool execution
```

For provider-hosted or enterprise-routed deployments, high-risk tools such as
creating pull requests, merging pull requests, updating repository contents, or
deleting files can require:

- a declared job or task boundary
- a target repository/resource binding
- a user or agent identity binding
- approval or just-in-time authority
- a short-lived authorization receipt
- an audit event that can be correlated with the eventual GitHub API action

## Example Contract

The following example uses the AgentID provider MCP authorization contract shape.
It is illustrative and intentionally scoped to a small set of repository and
pull request tools.

```yaml
$schema: https://raw.githubusercontent.com/dinpd/AgentID/main/schema/provider-mcp-contract.schema.json

provider_agentid:
  provider: github
  mcp_server: github-mcp-server
  version: example
  tools:
    get_file_contents:
      action: read
      risk: low
      resource_template: github/repo/{owner}/{repo}/contents/{path}
      data_from: github_repository
      data_to: agent_context
      requires_jit: false
      receipt_required: false
      input_schema:
        type: object
        required:
          - owner
          - repo
          - path
        properties:
          owner:
            type: string
          repo:
            type: string
          path:
            type: string

    create_pull_request:
      action: write
      risk: high
      resource_template: github/repo/{owner}/{repo}/pulls
      data_from: agent_context
      data_to: github_repository
      requires_jit: true
      approval: human_confirm
      receipt_required: true
      authorization_requirements:
        required_context:
          - tenant_id
          - agent_id
          - user_id
          - job_id
          - owner
          - repo
          - approval_id
        bind_receipt_to:
          - tenant_id
          - agent_id
          - user_id
          - tool
          - action
          - resource
          - job_id
          - owner
          - repo
          - approval_id
          - jit_grant_id
        resource_template: github/repo/{owner}/{repo}/pulls
        receipt_ttl_seconds: 300
        single_use: true

    merge_pull_request:
      action: write
      risk: high
      resource_template: github/repo/{owner}/{repo}/pull/{pull_number}/merge
      data_from: agent_context
      data_to: github_repository
      requires_jit: true
      approval: human_confirm
      receipt_required: true
      authorization_requirements:
        required_context:
          - tenant_id
          - agent_id
          - user_id
          - job_id
          - owner
          - repo
          - pull_number
          - approval_id
        bind_receipt_to:
          - tenant_id
          - agent_id
          - user_id
          - tool
          - action
          - resource
          - job_id
          - owner
          - repo
          - pull_number
          - approval_id
          - jit_grant_id
        resource_template: github/repo/{owner}/{repo}/pull/{pull_number}/merge
        receipt_ttl_seconds: 180
        single_use: true

    create_or_update_file:
      action: write
      risk: high
      resource_template: github/repo/{owner}/{repo}/contents/{path}
      data_from: agent_context
      data_to: github_repository
      requires_jit: true
      approval: human_confirm
      receipt_required: true
      authorization_requirements:
        required_context:
          - tenant_id
          - agent_id
          - user_id
          - job_id
          - owner
          - repo
          - path
          - approval_id
        bind_receipt_to:
          - tenant_id
          - agent_id
          - user_id
          - tool
          - action
          - resource
          - job_id
          - owner
          - repo
          - path
          - approval_id
          - jit_grant_id
        resource_template: github/repo/{owner}/{repo}/contents/{path}
        receipt_ttl_seconds: 300
        single_use: true

    delete_file:
      action: write
      risk: critical
      resource_template: github/repo/{owner}/{repo}/contents/{path}
      data_from: agent_context
      data_to: github_repository
      requires_jit: true
      approval: human_confirm
      receipt_required: true
      authorization_requirements:
        required_context:
          - tenant_id
          - agent_id
          - user_id
          - job_id
          - owner
          - repo
          - path
          - approval_id
        bind_receipt_to:
          - tenant_id
          - agent_id
          - user_id
          - tool
          - action
          - resource
          - job_id
          - owner
          - repo
          - path
          - approval_id
          - jit_grant_id
        resource_template: github/repo/{owner}/{repo}/contents/{path}
        receipt_ttl_seconds: 180
        single_use: true
```

## How an Enterprise Gateway Can Use This

An enterprise gateway can use the contract to:

1. Import the allowed GitHub MCP tools into a local policy manifest.
2. Bind agent authorization to a job, owner, repository, pull request, or file path.
3. Require approval or just-in-time authority for write/destructive tools.
4. Attach a scoped receipt to forwarded high-risk tool calls.
5. Correlate gateway authorization decisions with GitHub audit/API events.

This is complementary to this server's built-in configuration and GitHub's API
authorization model:

- Toolsets and read-only mode decide which tools are exposed.
- PAT, OAuth, and GitHub App permissions decide what the token can do.
- GitHub API authorization remains the final enforcement point for the actual
  operation.
- The provider authorization contract helps an enterprise gateway decide
  whether a specific agent-originated action should be attempted at all.

For a reference implementation of this contract shape, see the AgentID project:
<https://github.com/dinpd/AgentID>.
