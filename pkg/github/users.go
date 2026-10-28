package github

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"time"

	ghErrors "github.com/github/github-mcp-server/v2/pkg/errors"
	"github.com/github/github-mcp-server/v2/pkg/ifc"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/sanitize"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MinimalUserEvent retains event-specific data without repeated user and repository metadata.
type MinimalUserEvent struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"`
	Public    *bool      `json:"public,omitempty"`
	Actor     string     `json:"actor,omitempty"`
	Repo      string     `json:"repo,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	Payload   any        `json:"payload,omitempty"`
}

type ListUserEventsInput struct {
	Username string `json:"username"`
	Page     *int   `json:"page,omitempty"`
	PerPage  *int   `json:"perPage,omitempty"`
}

// ListUserEvents creates a read-only tool for activity visible to the current GitHub credentials.
func ListUserEvents(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := WithPagination(&jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"username": {
				Type:        "string",
				Description: "GitHub username whose activity to list",
			},
		},
		Required: []string{"username"},
	})
	schema.Properties["page"].Type = "integer"
	schema.Properties["perPage"].Type = "integer"
	return NewTool[ListUserEventsInput, []MinimalUserEvent](
		ToolsetMetadataUsers,
		mcp.Tool{
			Name:        "list_user_events",
			Description: t("TOOL_LIST_USER_EVENTS_DESCRIPTION", "List recent GitHub activity performed by a user, such as pushes, pull requests, issues, and stars. GitHub returns public events for other users and may include private events when querying the authenticated user's own activity, subject to token permissions. Returns event metadata, visibility, and event-specific payloads. GitHub limits the feed to up to 300 events from the past 30 days; events may be delayed from 30 seconds to 6 hours. This is not a complete or real-time contribution history."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_USER_EVENTS_USER_TITLE", "List user activity"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		scopes.PublicRead(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input ListUserEventsInput) (*mcp.CallToolResult, []MinimalUserEvent, error) {
			if input.Username == "" {
				return utils.NewToolResultError("missing required parameter: username"), nil, nil
			}
			page, perPage := 1, 30
			if input.Page != nil {
				page = *input.Page
			}
			if input.PerPage != nil {
				perPage = *input.PerPage
			}
			if page < 1 {
				return utils.NewToolResultError("page must be at least 1"), nil, nil
			}
			if perPage < 1 || perPage > 100 {
				return utils.NewToolResultError("perPage must be between 1 and 100"), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}
			events, resp, err := client.Activity.ListEventsPerformedByUser(ctx, url.PathEscape(input.Username), false, &github.ListOptions{
				Page: page, PerPage: perPage,
			})
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list user events", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			minimalEvents := make([]MinimalUserEvent, 0, len(events))
			label := ifc.PublicUntrusted()
			for _, event := range events {
				if event == nil {
					continue
				}
				// Missing visibility is conservatively private. A mixed page may
				// also contain untrusted public content, so integrity stays untrusted.
				if !event.GetPublic() {
					label = ifc.PrivateUntrusted()
				}
				var payload any
				if raw := event.GetRawPayload(); len(raw) > 0 {
					decoder := json.NewDecoder(bytes.NewReader(raw))
					// Event payloads can contain IDs larger than float64's exact integer range.
					decoder.UseNumber()
					if err := decoder.Decode(&payload); err != nil {
						return utils.NewToolResultErrorFromErr("failed to decode event payload", err), nil, nil
					}
					payload = sanitizeUserEventPayload(payload)
				}
				var createdAt *time.Time
				if event.CreatedAt != nil {
					createdAt = &event.CreatedAt.Time
				}
				minimalEvents = append(minimalEvents, MinimalUserEvent{
					ID: event.GetID(), Type: event.GetType(), Public: event.Public,
					Actor: event.Actor.GetLogin(), Repo: event.Repo.GetName(),
					CreatedAt: createdAt, Payload: payload,
				})
			}
			result := MarshalledTextResult(minimalEvents)
			return attachStaticIFCLabel(ctx, deps, result, label), minimalEvents, nil
		},
	)
}

// Payloads vary by event type. Preserve unknown fields while filtering user-authored text.
func sanitizeUserEventPayload(value any) any {
	switch value := value.(type) {
	case string:
		return sanitize.Content(value)
	case map[string]any:
		for key, item := range value {
			value[key] = sanitizeUserEventPayload(item)
		}
		return value
	case []any:
		for i, item := range value {
			value[i] = sanitizeUserEventPayload(item)
		}
		return value
	default:
		return value
	}
}
