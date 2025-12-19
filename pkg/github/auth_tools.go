package github

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AuthToolset is the toolset for authentication tools.
// This is a special toolset that's only available when unauthenticated.
var ToolsetMetadataAuth = inventory.ToolsetMetadata{
	ID:          "auth",
	Description: "Authentication tools for logging into GitHub",
	Icon:        "key",
}

// AuthToolDependencies contains dependencies for auth tools.
type AuthToolDependencies struct {
	AuthManager *AuthManager
	T           translations.TranslationHelperFunc
	// Server is the MCP server, used to access sessions for notifications
	Server *mcp.Server
	// Logger for debug logging
	Logger *slog.Logger
	// OnAuthenticated is called when authentication completes successfully.
	// It should initialize GitHub clients and register tools.
	OnAuthenticated func(ctx context.Context, token string) error
	// OnAuthComplete is called after authentication flow completes (success or failure).
	// It can be used to clean up auth tools after they're no longer needed.
	OnAuthComplete func()
}

// AuthTools returns the authentication tools.
// These are available when the server starts without a token.
func AuthTools(t translations.TranslationHelperFunc) []inventory.ServerTool {
	return []inventory.ServerTool{
		AuthLogin(t),
	}
}

// AuthLogin creates a tool that initiates the OAuth device flow.
// It uses URL elicitation to show the user the authorization URL and code,
// then blocks while polling until the user completes authorization.
func AuthLogin(t translations.TranslationHelperFunc) inventory.ServerTool {
	return inventory.ServerTool{
		Tool: mcp.Tool{
			Name:        "auth_login",
			Description: t("auth_login_description", "Initiate GitHub authentication using OAuth device flow. This will provide a URL and code that you can use to authenticate with GitHub. After visiting the URL and entering the code, authentication will complete automatically."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("auth_login_title", "Login to GitHub"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type:       "object",
				Properties: map[string]*jsonschema.Schema{},
			},
		},
		Toolset: ToolsetMetadataAuth,
		HandlerFunc: func(deps any) mcp.ToolHandler {
			return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				authDeps := deps.(AuthToolDependencies)
				authMgr := authDeps.AuthManager

				if authMgr.IsAuthenticated() {
					return utils.NewToolResultText("Already authenticated with GitHub."), nil
				}

				// Reset any pending flow before starting a new one
				authMgr.Reset()

				deviceResp, err := authMgr.StartDeviceFlow(ctx)
				if err != nil {
					return utils.NewToolResultError(fmt.Sprintf("Failed to start authentication: %v", err)), nil
				}

				if authDeps.Logger != nil {
					authDeps.Logger.Info("starting auth flow", "expiresIn", deviceResp.ExpiresIn)
				}

				// Use URL elicitation to show the auth URL to the user
				// This creates a nice UI in the client for the user to click
				elicitResult, err := req.Session.Elicit(ctx, &mcp.ElicitParams{
					Mode:    "url",
					Message: fmt.Sprintf("🔐 GitHub Authentication\n\nEnter code: %s", deviceResp.UserCode),
					URL:     deviceResp.VerificationURI,
				})
				if err != nil {
					if authDeps.Logger != nil {
						authDeps.Logger.Error("elicitation failed", "error", err)
					}
					// Elicitation not supported or failed - fall back to polling
					return pollAndComplete(ctx, req.Session, authDeps, authMgr, deviceResp)
				}

				// Check if user cancelled
				if elicitResult.Action == "cancel" || elicitResult.Action == "decline" {
					authMgr.Reset()
					return utils.NewToolResultText("Authentication cancelled."), nil
				}

				// User clicked the link - now poll for completion with progress
				return pollAndComplete(ctx, req.Session, authDeps, authMgr, deviceResp)
			}
		},
	}
}

// pollAndComplete polls for the auth token and completes the flow.
// It sends progress notifications during polling so the user knows it's working.
func pollAndComplete(ctx context.Context, session *mcp.ServerSession, authDeps AuthToolDependencies, authMgr *AuthManager, _ *DeviceCodeResponse) (*mcp.CallToolResult, error) {
	// Poll for the token with progress updates
	err := authMgr.CompleteDeviceFlowWithProgress(ctx, func(elapsed, total int, _ string) {
		if authDeps.Logger != nil {
			authDeps.Logger.Debug("auth polling", "elapsed", elapsed, "total", total)
		}
		// Send progress notification so user sees we're waiting
		if session != nil {
			_ = session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
				ProgressToken: "auth-polling",
				Progress:      float64(elapsed),
				Total:         float64(total),
				Message:       "⏳ Waiting for GitHub authorization...",
			})
		}
	})
	if err != nil {
		if authDeps.Logger != nil {
			authDeps.Logger.Error("auth polling failed", "error", err)
		}
		return utils.NewToolResultError(fmt.Sprintf("Authentication failed: %v", err)), nil
	}

	if authDeps.Logger != nil {
		authDeps.Logger.Info("auth polling succeeded, registering tools")
	}

	// Call the OnAuthenticated callback to initialize clients and register tools
	if authDeps.OnAuthenticated != nil {
		if err := authDeps.OnAuthenticated(ctx, authMgr.Token()); err != nil {
			if authDeps.Logger != nil {
				authDeps.Logger.Error("failed to initialize after auth", "error", err)
			}
			return nil, fmt.Errorf("authentication succeeded but failed to initialize: %w", err)
		}
	}

	// Send a user-visible notification about successful authentication
	if session != nil {
		_ = session.Log(ctx, &mcp.LoggingMessageParams{
			Level:  "notice",
			Logger: "github-mcp-server",
			Data:   "✅ Successfully authenticated with GitHub! All GitHub tools are now available.",
		})
	}

	// Clean up auth tools now that we're authenticated
	if authDeps.OnAuthComplete != nil {
		authDeps.OnAuthComplete()
	}

	return utils.NewToolResultText(`✅ Successfully authenticated with GitHub!

All GitHub tools are now available. You can now most likely:
- Create and manage repositories
- Work with issues and pull requests
- Access your organizations and teams
- And much more, depending on configuration (mention the success and new tools to the user!

Call get_me to see who you're logged in as.`), nil
}
