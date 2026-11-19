package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	ghErrors "github.com/github/github-mcp-server/v2/pkg/errors"
	"github.com/github/github-mcp-server/v2/pkg/ifc"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type UpdateDependabotAlertInput struct {
	DependabotAlertPatch
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	AlertNumber int64  `json:"alertNumber"`
}

// Pointers distinguish untouched fields from an empty comment or assignee set.
type DependabotAlertPatch struct {
	State            *string   `json:"state,omitempty"`
	DismissedReason  *string   `json:"dismissed_reason,omitempty"`
	DismissedComment *string   `json:"dismissed_comment,omitempty"`
	Assignees        *[]string `json:"assignees,omitempty"`
}

type DependabotAlertUpdateOutput struct {
	Number           string                     `json:"number" jsonschema:"Exact alert number as a decimal string to preserve integer precision."`
	State            string                     `json:"state"`
	HTMLURL          string                     `json:"html_url"`
	DismissedReason  *string                    `json:"dismissed_reason"`
	DismissedComment *string                    `json:"dismissed_comment"`
	Assignees        *[]DependabotAlertAssignee `json:"assignees,omitempty"`
}

type DependabotAlertAssignee struct {
	Login string `json:"login"`
}

func UpdateDependabotAlert(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataDependabot,
		mcp.Tool{
			Name: "update_dependabot_alert",
			Description: t("TOOL_UPDATE_DEPENDABOT_ALERT_DESCRIPTION",
				"Update a Dependabot alert's state or assignees. Provide state or assignees; dismissal requires dismissed_reason. "+
					"Assigning copilot-swe-agent[bot] requests remediation that attempts a fix; a pull request is not guaranteed. "+
					"Requires Dependabot alerts write permission or the security_events OAuth scope. "+
					"GitHub enforces authorization, licensing, repository policies and delegated-dismissal review requirements. "+
					"Agent configuration (agent_assignment) is not supported."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_UPDATE_DEPENDABOT_ALERT_USER_TITLE", "Update Dependabot alert"),
				ReadOnlyHint:    false,
				DestructiveHint: new(true),
				IdempotentHint:  false,
				OpenWorldHint:   new(true),
			},
			InputSchema: &jsonschema.Schema{
				Type:                 "object",
				AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
				Properties: map[string]*jsonschema.Schema{
					"owner":       {Type: "string", MinLength: new(1), Description: "Repository owner (username or organization)."},
					"repo":        {Type: "string", MinLength: new(1), Description: "Repository name."},
					"alertNumber": {Type: "integer", Minimum: new(1.0), Description: "The repository's Dependabot alert number."},
					"state": {
						Type: "string", Enum: []any{"open", "dismissed"},
						Description: "Reopen or dismiss the alert. Omission leaves state unchanged.",
					},
					"dismissed_reason": {
						Type: "string", Enum: []any{"fix_started", "inaccurate", "no_bandwidth", "not_used", "tolerable_risk"},
						Description: "Required when state is dismissed.",
					},
					"dismissed_comment": {
						Type: "string", MaxLength: new(280),
						Description: "Optional dismissal comment, up to 280 characters; not a standalone comment API.",
					},
					"assignees": {
						Type: "array", Items: &jsonschema.Schema{Type: "string"},
						Description: "Usernames replacing the entire assignee set; [] clears all, omission leaves it unchanged. " +
							"Including copilot-swe-agent[bot] requests remediation, not just ownership bookkeeping.",
					},
				},
				Required: []string{"owner", "repo", "alertNumber"},
			},
		},
		scopes.RequireAll(scopes.SecurityEvents),
		func(ctx context.Context, deps ToolDependencies, call *mcp.CallToolRequest, _ json.RawMessage) (*mcp.CallToolResult, *DependabotAlertUpdateOutput, error) {
			// Typed schema validation passes through float64. Decode the original
			// JSON so large IDs and fractional inputs cannot change the write target.
			args, err := decodeDependabotAlertUpdate(call.Params.Arguments)
			if err != nil {
				return utils.NewToolResultErrorFromErr("invalid Dependabot alert update arguments", err), nil, nil
			}
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, err
			}
			path := fmt.Sprintf("repos/%s/%s/dependabot/alerts/%d", url.PathEscape(args.Owner), url.PathEscape(args.Repo), args.AlertNumber)
			// The SDK's state update method requires state and lacks assignees.
			request, err := client.NewRequest(ctx, http.MethodPatch, path, args.DependabotAlertPatch)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create Dependabot alert update request", err), nil, err
			}
			var alert struct {
				DependabotAlertUpdateOutput
				Number int64 `json:"number"`
			}
			resp, err := client.Do(request, &alert)
			if resp != nil && resp.Body != nil {
				defer func() { _ = resp.Body.Close() }()
			}
			const uncertainUpdate = "The Dependabot alert update may already have been applied; verify the alert with get_dependabot_alert before retrying"
			if err != nil {
				if resp != nil && resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
					return utils.NewToolResultErrorFromErr(uncertainUpdate, err), nil, nil
				}
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update Dependabot alert", resp, err), nil, nil
			}
			if resp.StatusCode != http.StatusOK || alert.Number != args.AlertNumber || alert.State == "" || alert.HTMLURL == "" {
				return utils.NewToolResultError("unexpected Dependabot alert update response. " + uncertainUpdate + "."), nil, nil
			}
			output := alert.DependabotAlertUpdateOutput
			output.Number = strconv.FormatInt(alert.Number, 10)
			encoded, err := json.Marshal(output)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal Dependabot alert update", err), nil, err
			}
			result := attachStaticIFCLabel(ctx, deps, utils.NewToolResultText(string(encoded)), ifc.LabelSecurityAlert())
			return result, &output, nil
		},
		normalizeDependabotAlertUpdate,
	)
}

func normalizeDependabotAlertUpdate(raw json.RawMessage) (json.RawMessage, error) {
	if _, err := decodeDependabotAlertUpdate(raw); err != nil {
		return nil, &inventory.ToolInputError{Message: err.Error()}
	}
	return raw, nil
}

func decodeDependabotAlertUpdate(raw json.RawMessage) (UpdateDependabotAlertInput, error) {
	var args UpdateDependabotAlertInput
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return args, err
	}
	for name, value := range fields {
		switch name {
		case "owner", "repo", "alertNumber", "state", "dismissed_reason", "dismissed_comment", "assignees":
		default:
			return args, fmt.Errorf("unknown parameter: %s", name)
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return args, fmt.Errorf("parameter %s must not be null", name)
		}
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return args, err
	}
	for name, value := range map[string]string{"owner": args.Owner, "repo": args.Repo} {
		if value == "" || value == "." || value == ".." ||
			strings.ContainsAny(value, `/\%?#`) || strings.ContainsFunc(value, func(r rune) bool {
			return unicode.IsSpace(r) || unicode.IsControl(r)
		}) {
			return args, fmt.Errorf("%s must be a non-empty repository path segment", name)
		}
	}
	if args.AlertNumber < 1 {
		return args, errors.New("alertNumber must be a positive int64 integer")
	}
	if args.State == nil && args.Assignees == nil {
		return args, errors.New("provide state or assignees")
	}
	if args.State != nil {
		if !slices.Contains([]string{"open", "dismissed"}, *args.State) {
			return args, errors.New("state must be open or dismissed")
		}
		if *args.State == "dismissed" && args.DismissedReason == nil {
			return args, errors.New("dismissed_reason is required when state is dismissed")
		}
	}
	if args.DismissedReason != nil &&
		!slices.Contains([]string{"fix_started", "inaccurate", "no_bandwidth", "not_used", "tolerable_risk"}, *args.DismissedReason) {
		return args, errors.New("invalid dismissed_reason")
	}
	if args.DismissedComment != nil && utf8.RuneCountInString(*args.DismissedComment) > 280 {
		return args, errors.New("dismissed_comment must not exceed 280 characters")
	}
	if args.Assignees != nil {
		var assignees []*string
		if err := json.Unmarshal(fields["assignees"], &assignees); err != nil {
			return args, err
		}
		if slices.Contains(assignees, nil) {
			return args, errors.New("assignees must contain only strings")
		}
	}
	return args, nil
}
