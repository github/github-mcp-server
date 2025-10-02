# GitHub MCP Server - Organization Access Support

This document explains the enhanced organization access functionality for GitHub MCP Server, addressing Issue #153.

## Overview

GitHub MCP Server now supports accessing private organization repositories for enterprise users. This enhancement enables seamless integration with organizational workflows while providing clear guidance for authentication and permission setup.

## Features

### 1. Organization Access Validation
- **Automatic Detection**: Identifies when requests target organization repositories
- **Membership Checking**: Validates user membership in the target organization  
- **Permission Validation**: Ensures the token has appropriate scopes for organization access

### 2. Enhanced Error Messages
- **Detailed Feedback**: Provides specific error messages for access issues
- **Actionable Suggestions**: Offers step-by-step guidance to resolve problems
- **Token Guidance**: Explains required token permissions and scopes

### 3. New Diagnostic Tools
- **check_organization_access**: Validates access to specific organizations
- **check_token_permissions**: Verifies token validity and scope requirements

## Token Requirements

### Classic Personal Access Tokens (Recommended)

For organization access, ensure your token has these scopes:

- **`repo`** - Full control of private repositories
- **`read:org`** - Read organization membership and repository access

### Token Setup Steps

1. **Generate Token**: Go to GitHub Settings > Developer settings > Personal access tokens > Tokens (classic)
2. **Select Scopes**: Enable `repo` and `read:org` scopes
3. **Organization Approval**: Some organizations require token approval
4. **SSO Configuration**: If SSO is enabled, authorize your token

### Organization Requirements

#### Token Policies
Organizations can restrict personal access token usage:
- **Allow classic tokens**: Organization must allow classic PAT access
- **Require approval**: Some orgs require token approval process
- **SSO Authorization**: Tokens may need SSO authorization

#### Membership Requirements
- User must be a member of the organization
- Repository access depends on team membership and permissions
- Private repository access requires appropriate team permissions

## Usage

### Automatic Enhancement
The enhanced error handling is automatically applied to existing tools when accessing organization repositories:

- `get_file_contents`
- `create_or_update_file` 
- `list_commits`
- `search_repositories`
- And other repository-related tools

### Diagnostic Tools

#### Check Organization Access
```bash
# Check if you have access to an organization
{
  "tool": "check_organization_access",
  "arguments": {
    "organization": "your-org-name"
  }
}
```

**Response Example:**
```json
{
  "organization": "your-org-name",
  "has_access": true,
  "is_public_member": false,
  "is_private_member": true,
  "token_valid": true,
  "token_info": "Token is valid for user: your-username"
}
```

#### Check Token Permissions
```bash
# Validate your token permissions
{
  "tool": "check_token_permissions",
  "arguments": {}
}
```

**Response Example:**
```json
{
  "token_valid": true,
  "message": "Token is valid for user: your-username",
  "recommendations": [
    "For organization access, ensure your token has 'repo' and 'read:org' scopes",
    "Check if the organization allows classic personal access tokens", 
    "Verify that SSO is properly configured if required"
  ]
}
```

## Troubleshooting

### Common Issues

#### 1. "Access denied to organization"
**Cause**: Token lacks required permissions or organization restrictions
**Solutions**:
- Verify token has `repo` and `read:org` scopes
- Check if organization allows classic PATs
- Ensure you're a member of the organization
- Authorize token for SSO if required

#### 2. "Repository not found or no access"
**Cause**: Repository is private and token lacks access
**Solutions**:
- Confirm repository name and owner are correct
- Ensure you have access to the repository
- Check team membership for private repositories
- Verify token permissions

#### 3. "Organization not found"
**Cause**: Organization name incorrect or no membership
**Solutions**:
- Verify organization name spelling
- Confirm you're a member of the organization
- Check if organization exists and is accessible

### Enhanced Error Messages

When access issues occur, the server now provides detailed error messages with specific suggestions:

```
Access denied to organization 'example-org'. This may be due to insufficient token permissions or organization restrictions.

Suggestions to resolve this issue:
  1. Ensure your token has 'repo' and 'read:org' scopes
  2. Verify that the organization allows classic personal access tokens
  3. Check if the organization requires SSO authentication
  4. Confirm you are a member of the organization

For more information about token permissions, see: https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens
```

## Configuration

### Environment Variables
The server continues to use the existing `GITHUB_PERSONAL_ACCESS_TOKEN` environment variable. No additional configuration is required.

### Organization Setup
Work with your organization administrators to:
1. Enable classic personal access token usage
2. Configure appropriate repository access permissions
3. Set up SSO authorization if required

## Backward Compatibility

This enhancement maintains full backward compatibility:
- Existing functionality for personal repositories unchanged
- Public repository access continues to work as before
- No breaking changes to existing API or tool interfaces
- Enhanced error messages provide additional context without changing tool behavior

## Implementation Details

### Architecture
- **Validator Pattern**: `OrganizationAccessValidator` provides centralized validation logic
- **Enhanced Error Handling**: `ValidateRepositoryAccessWithEnhancedError` integrates with existing tools
- **Diagnostic Tools**: New tools provide proactive access validation

### Key Components
- `organization_access.go`: Core validation and error handling logic
- Enhanced repository tools with automatic access validation
- Comprehensive error messages with actionable suggestions

## Security Considerations

- **Token Scope Validation**: Verifies token permissions before API calls
- **Organization Policy Compliance**: Respects organization token policies
- **SSO Integration**: Supports organization SSO requirements
- **Graceful Degradation**: Provides helpful errors rather than silent failures

## Contributing

When adding new repository-related tools, ensure they include organization access validation by calling `ValidateRepositoryAccessWithEnhancedError` before performing GitHub API operations.

## Related Documentation

- [GitHub Personal Access Tokens](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)
- [Organization Token Policies](https://docs.github.com/en/organizations/managing-programmatic-access-to-your-organization)
- [GitHub API Authentication](https://docs.github.com/en/rest/authentication)