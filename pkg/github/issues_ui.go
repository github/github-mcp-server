package github

import (
	"context"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// IssueWriteUIResourceURI is the URI for the create_issue_ui tool's MCP App UI resource.
const IssueWriteUIResourceURI = "ui://github-mcp-server/issue-write"

// CreateIssueUI creates a tool that shows an interactive UI for creating GitHub issues.
// This tool only displays the form - the actual issue creation happens when the user
// clicks "Create Issue" in the UI, which calls the issue_write tool.
func CreateIssueUI(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "create_issue_ui",
			Description: t("TOOL_CREATE_ISSUE_UI_DESCRIPTION", "Show an interactive UI for creating a new issue in a GitHub repository. The user will fill in the issue details and submit the form."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_CREATE_ISSUE_UI_USER_TITLE", "Create issue form"),
				ReadOnlyHint: true, // The tool itself doesn't create anything, just shows UI
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "Repository owner",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
				},
				Required: []string{"owner", "repo"},
			},
			// MCP Apps UI metadata - links this tool to its UI resource
			Meta: mcp.Meta{
				"ui": map[string]any{
					"resourceUri": IssueWriteUIResourceURI,
				},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(_ context.Context, _ ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Return a simple confirmation message
			// The UI will be rendered by the host and will handle the actual form
			return utils.NewToolResultText("Ready to create an issue in " + owner + "/" + repo), nil, nil
		},
	)
}

// IssueWriteUIHTML is the HTML content for the issue_write tool's MCP App UI.
// This UI provides a GitHub-like interface for creating issues.
//
// How this MCP App works:
// 1. Server registers this HTML as a resource at ui://github-mcp-server/issue-write
// 2. Server links the issue_write tool to this resource via _meta.ui.resourceUri
// 3. When host calls issue_write, it sees the resourceUri and fetches this HTML
// 4. Host renders HTML in a sandboxed iframe and communicates via postMessage
// 5. User fills in the form and clicks "Create Issue"
// 6. UI sends a tools/call request to create the issue via the MCP server
// 7. UI displays the result (success with link or error)
const IssueWriteUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>GitHub MCP Server - Create Issue</title>
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: var(--font-sans, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif);
            padding: 16px;
            margin: 0;
            background: var(--color-background-primary, #fff);
            color: var(--color-text-primary, #24292f);
            font-size: var(--font-text-md-size, 14px);
            line-height: var(--font-text-md-line-height, 1.5);
        }
        .issue-form {
            max-width: 600px;
            border: 1px solid var(--color-border-primary, #d0d7de);
            border-radius: var(--border-radius-lg, 6px);
            background: var(--color-background-secondary, #f6f8fa);
            overflow: hidden;
        }
        .form-header {
            padding: 12px 16px;
            background: var(--color-background-primary, #fff);
            border-bottom: 1px solid var(--color-border-primary, #d0d7de);
            font-weight: 600;
            font-size: var(--font-heading-xs-size, 16px);
            display: flex;
            align-items: center;
            gap: 8px;
        }
        .form-header-icon {
            color: var(--color-text-success, #1a7f37);
        }
        .form-body {
            padding: 16px;
        }
        .form-group {
            margin-bottom: 16px;
        }
        .form-group:last-child {
            margin-bottom: 0;
        }
        .form-label {
            display: block;
            font-weight: 600;
            margin-bottom: 8px;
            color: var(--color-text-primary, #24292f);
        }
        .form-input {
            width: 100%;
            padding: 8px 12px;
            font-size: var(--font-text-md-size, 14px);
            line-height: 1.5;
            border: 1px solid var(--color-border-primary, #d0d7de);
            border-radius: var(--border-radius-sm, 6px);
            background: var(--color-background-primary, #fff);
            color: var(--color-text-primary, #24292f);
        }
        .form-input:focus {
            outline: none;
            border-color: var(--color-border-info, #0969da);
            box-shadow: 0 0 0 3px rgba(9, 105, 218, 0.3);
        }
        .form-textarea {
            min-height: 200px;
            resize: vertical;
            font-family: inherit;
        }
        .form-hint {
            font-size: var(--font-text-sm-size, 12px);
            color: var(--color-text-secondary, #656d76);
            margin-top: 4px;
        }
        .form-actions {
            padding: 12px 16px;
            background: var(--color-background-primary, #fff);
            border-top: 1px solid var(--color-border-primary, #d0d7de);
            display: flex;
            justify-content: flex-end;
            gap: 8px;
        }
        .btn {
            display: inline-flex;
            align-items: center;
            justify-content: center;
            padding: 8px 16px;
            font-size: var(--font-text-md-size, 14px);
            font-weight: 600;
            line-height: 1;
            border-radius: var(--border-radius-sm, 6px);
            cursor: pointer;
            border: 1px solid transparent;
            transition: background 0.1s ease;
        }
        .btn:disabled {
            opacity: 0.6;
            cursor: not-allowed;
        }
        .btn-primary {
            color: #fff;
            background: var(--color-background-success, #1f883d);
            border-color: var(--color-border-success, #1a7f37);
        }
        .btn-primary:hover:not(:disabled) {
            background: var(--color-background-success-hover, #1a7f37);
        }
        .status-message {
            padding: 12px 16px;
            border-radius: var(--border-radius-sm, 6px);
            margin: 16px;
        }
        .status-error {
            background: var(--color-background-danger-subtle, #ffebe9);
            color: var(--color-text-danger, #cf222e);
            border: 1px solid var(--color-border-danger, #ffcecb);
        }
        .hidden {
            display: none;
        }
        .loading-spinner {
            display: inline-block;
            width: 16px;
            height: 16px;
            border: 2px solid #fff;
            border-top-color: transparent;
            border-radius: 50%;
            animation: spin 0.8s linear infinite;
            margin-right: 8px;
        }
        @keyframes spin {
            to { transform: rotate(360deg); }
        }
        .repo-info {
            font-size: var(--font-text-sm-size, 12px);
            color: var(--color-text-secondary, #656d76);
            padding: 8px 16px;
            background: var(--color-background-tertiary, #eaeef2);
            border-bottom: 1px solid var(--color-border-primary, #d0d7de);
        }
        .repo-info code {
            font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, monospace;
            background: var(--color-background-primary, #fff);
            padding: 2px 6px;
            border-radius: 3px;
            font-size: inherit;
        }
        /* Success view styles */
        .success-view {
            padding: 16px;
        }
        .success-header {
            display: flex;
            align-items: center;
            gap: 8px;
            margin-bottom: 16px;
            color: var(--color-text-success, #1a7f37);
        }
        .success-icon {
            width: 24px;
            height: 24px;
            border-radius: 50%;
            background: var(--color-background-success, #1f883d);
            color: #fff;
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 14px;
        }
        .success-title {
            font-weight: 600;
            font-size: var(--font-heading-xs-size, 16px);
        }
        .issue-card {
            border: 1px solid var(--color-border-primary, #d0d7de);
            border-radius: var(--border-radius-sm, 6px);
            background: var(--color-background-primary, #fff);
            overflow: hidden;
        }
        .issue-card-header {
            padding: 12px 16px;
            border-bottom: 1px solid var(--color-border-primary, #d0d7de);
            display: flex;
            align-items: flex-start;
            gap: 8px;
        }
        .issue-state-icon {
            color: var(--color-text-success, #1a7f37);
            margin-top: 2px;
        }
        .issue-title-link {
            font-weight: 600;
            color: var(--color-text-primary, #24292f);
            text-decoration: none;
        }
        .issue-title-link:hover {
            color: var(--color-text-info, #0969da);
            text-decoration: underline;
        }
        .issue-number {
            color: var(--color-text-secondary, #656d76);
            font-weight: 400;
        }
        .issue-card-body {
            padding: 12px 16px;
            color: var(--color-text-secondary, #656d76);
            font-size: var(--font-text-sm-size, 13px);
        }
        .issue-card-body p {
            margin: 0;
            white-space: pre-wrap;
            word-break: break-word;
        }
        .issue-card-footer {
            padding: 8px 16px;
            background: var(--color-background-tertiary, #f6f8fa);
            border-top: 1px solid var(--color-border-primary, #d0d7de);
            font-size: var(--font-text-sm-size, 12px);
            color: var(--color-text-secondary, #656d76);
        }
        .issue-card-footer a {
            color: var(--color-text-info, #0969da);
            text-decoration: none;
        }
        .issue-card-footer a:hover {
            text-decoration: underline;
        }
    </style>
</head>
<body>
    <div id="content">
        <!-- Form View -->
        <div id="form-view" class="issue-form">
            <div class="form-header">
                <span class="form-header-icon">●</span>
                <span>New Issue</span>
            </div>
            <div id="repo-info" class="repo-info hidden"></div>
            <div id="error-container"></div>
            <div class="form-body" id="form-body">
                <div class="form-group">
                    <label class="form-label" for="issue-title">Title</label>
                    <input type="text" id="issue-title" class="form-input" placeholder="Issue title" />
                </div>
                <div class="form-group">
                    <label class="form-label" for="issue-body">Description</label>
                    <textarea id="issue-body" class="form-input form-textarea" placeholder="Describe the issue..."></textarea>
                    <p class="form-hint">Markdown is not supported</p>
                </div>
            </div>
            <div class="form-actions" id="form-actions">
                <button type="button" id="submit-btn" class="btn btn-primary" onclick="handleSubmit()">
                    Create Issue
                </button>
            </div>
        </div>

        <!-- Success View (hidden by default) -->
        <div id="success-view" class="issue-form hidden">
            <div class="success-view">
                <div class="success-header">
                    <span class="success-icon">✓</span>
                    <span class="success-title">Issue created</span>
                </div>
                <div class="issue-card">
                    <div class="issue-card-header">
                        <span class="issue-state-icon">●</span>
                        <div>
                            <a id="success-issue-link" class="issue-title-link" href="#" onclick="openLink(this.href); return false;">
                                <span id="success-issue-title"></span>
                                <span id="success-issue-number" class="issue-number"></span>
                            </a>
                        </div>
                    </div>
                    <div id="success-issue-body" class="issue-card-body"></div>
                    <div class="issue-card-footer">
                        <a id="success-view-link" href="#" onclick="openLink(this.href); return false;">View on GitHub →</a>
                    </div>
                </div>
            </div>
        </div>
    </div>
    <script>
        // ============================================================
        // MCP Apps Protocol Implementation
        // ============================================================
        
        const pendingRequests = new Map();
        let requestId = 0;
        
        // Context from tool input
        let toolInput = null;

        // Send a JSON-RPC request to the host and wait for response
        function sendRequest(method, params) {
            const id = ++requestId;
            return new Promise((resolve, reject) => {
                pendingRequests.set(id, { resolve, reject });
                window.parent.postMessage({ jsonrpc: '2.0', id, method, params }, '*');
            });
        }

        // Send a notification (no response expected)
        function sendNotification(method, params) {
            window.parent.postMessage({ jsonrpc: '2.0', method, params }, '*');
        }

        // Handle all messages from the host
        function handleMessage(event) {
            const message = event.data;
            if (!message || typeof message !== 'object') return;

            // Handle responses to our requests
            if (message.id !== undefined && pendingRequests.has(message.id)) {
                const { resolve, reject } = pendingRequests.get(message.id);
                pendingRequests.delete(message.id);
                if (message.error) {
                    reject(new Error(message.error.message || JSON.stringify(message.error)));
                } else {
                    resolve(message.result);
                }
                return;
            }

            // Handle notifications from the host
            if (message.method === 'ui/notifications/tool-input') {
                handleToolInput(message.params);
            } else if (message.method === 'ui/notifications/tool-result') {
                handleToolResult(message.params);
            }
        }

        // Process tool input - pre-populate form if data provided
        function handleToolInput(params) {
            toolInput = params?.arguments || {};
            
            // Show repository info if available
            if (toolInput.owner && toolInput.repo) {
                const repoInfo = document.getElementById('repo-info');
                repoInfo.innerHTML = 'Creating issue in <code>' + escapeHtml(toolInput.owner) + '/' + escapeHtml(toolInput.repo) + '</code>';
                repoInfo.classList.remove('hidden');
            }
            
            // Pre-fill title if provided
            if (toolInput.title) {
                document.getElementById('issue-title').value = toolInput.title;
            }
            
            // Pre-fill body if provided
            if (toolInput.body) {
                document.getElementById('issue-body').value = toolInput.body;
            }
            
            notifySize();
        }

        // Handle tool result (when issue is created)
        function handleToolResult(result) {
            setLoading(false);
            
            if (!result || !result.content) {
                // Initial tool load - no content yet, just ignore
                return;
            }

            const textContent = result.content.find(c => c.type === 'text');
            if (!textContent || !textContent.text) {
                return;
            }

            // Ignore the initial "Ready to create an issue" message
            if (textContent.text.startsWith('Ready to create an issue')) {
                return;
            }

            try {
                // Check if it's an error response
                if (result.isError) {
                    showError(textContent.text);
                    return;
                }

                const issueData = JSON.parse(textContent.text);
                showSuccess(issueData);
            } catch (e) {
                // If we can't parse it, it might be a plain text error
                showError(textContent.text);
            }
        }

        // Submit the form - call the issue_write tool
        async function handleSubmit() {
            const title = document.getElementById('issue-title').value.trim();
            const body = document.getElementById('issue-body').value.trim();
            
            if (!title) {
                showError('Please enter a title for the issue');
                return;
            }

            // We need owner and repo from the initial tool input
            if (!toolInput?.owner || !toolInput?.repo) {
                showError('Repository information not available. Please specify owner and repo.');
                return;
            }

            setLoading(true);
            clearError();

            try {
                const result = await sendRequest('tools/call', {
                    name: 'issue_write',
                    arguments: {
                        method: 'create',
                        owner: toolInput.owner,
                        repo: toolInput.repo,
                        title: title,
                        body: body
                    }
                });

                // Process the result
                if (result.isError) {
                    const textContent = result.content?.find(c => c.type === 'text');
                    showError(textContent?.text || 'Failed to create issue');
                } else {
                    const textContent = result.content?.find(c => c.type === 'text');
                    if (textContent?.text) {
                        try {
                            const issueData = JSON.parse(textContent.text);
                            // Store the title and body we submitted for the success view
                            issueData._submittedTitle = title;
                            issueData._submittedBody = body;
                            showSuccess(issueData);
                        } catch {
                            showSuccess({ url: textContent.text, _submittedTitle: title, _submittedBody: body });
                        }
                    } else {
                        showSuccess({ _submittedTitle: title, _submittedBody: body });
                    }
                }
            } catch (error) {
                showError('Error creating issue: ' + error.message);
            } finally {
                setLoading(false);
            }
        }

        function setLoading(loading) {
            const btn = document.getElementById('submit-btn');
            btn.disabled = loading;
            if (loading) {
                btn.innerHTML = '<span class="loading-spinner"></span>Creating...';
            } else {
                btn.innerHTML = 'Create Issue';
            }
        }

        function clearError() {
            document.getElementById('error-container').innerHTML = '';
        }

        function showSuccess(data) {
            // Hide the form view
            document.getElementById('form-view').classList.add('hidden');
            
            // Populate and show the success view
            const successView = document.getElementById('success-view');
            const issueUrl = data.url || data.URL || data.html_url || '#';
            const repoIssuesUrl = 'https://github.com/' + encodeURIComponent(toolInput.owner) + '/' + encodeURIComponent(toolInput.repo) + '/issues';
            const title = data.title || data._submittedTitle || 'Issue';
            const body = data.body || data._submittedBody || '';
            const number = data.number ? '#' + data.number : '';
            
            document.getElementById('success-issue-title').textContent = title;
            document.getElementById('success-issue-number').textContent = number ? ' ' + number : '';
            document.getElementById('success-issue-link').href = issueUrl;
            document.getElementById('success-view-link').href = repoIssuesUrl;
            
            const bodyEl = document.getElementById('success-issue-body');
            if (body) {
                bodyEl.innerHTML = '<p>' + escapeHtml(body) + '</p>';
            } else {
                bodyEl.innerHTML = '<p><em>No description provided.</em></p>';
            }
            
            successView.classList.remove('hidden');
            notifySize();
        }

        function showError(message) {
            const container = document.getElementById('error-container');
            container.innerHTML = '<div class="status-message status-error">' + escapeHtml(message) + '</div>';
            notifySize();
        }

        function notifySize() {
            sendNotification('ui/notifications/size-changed', {
                height: document.body.scrollHeight + 20
            });
        }

        function escapeHtml(text) {
            if (text == null) return '';
            const div = document.createElement('div');
            div.textContent = String(text);
            return div.innerHTML;
        }

        function openLink(url) {
            if (!url || url === '#') return;
            // Try window.open first, then fall back to creating a link
            const opened = window.open(url, '_blank');
            if (!opened) {
                // If popup blocked, try navigation
                window.location.href = url;
            }
        }

        // Listen for messages from the host
        window.addEventListener('message', handleMessage);

        // Initialize the MCP App connection
        sendRequest('ui/initialize', {
            appInfo: { name: 'github-mcp-server-issue-write', version: '1.0.0' },
            appCapabilities: {},
            protocolVersion: '2025-11-21'
        }).then(() => {
            notifySize();
        });
    </script>
</body>
</html>`
