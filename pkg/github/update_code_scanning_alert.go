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

type UpdateCodeScanningAlertInput struct {
	CodeScanningAlertPatch
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	AlertNumber int64  `json:"alertNumber"`
}

// RawMessage distinguishes an omitted field from an explicit JSON null.
type CodeScanningAlertPatch struct {
	State            *string         `json:"state,omitempty"`
	DismissedReason  json.RawMessage `json:"dismissed_reason,omitempty"`
	DismissedComment json.RawMessage `json:"dismissed_comment,omitempty"`
	CreateRequest    *bool           `json:"create_request,omitempty"`
	Assignees        *[]string       `json:"assignees,omitempty"`
}

type CodeScanningAlertAssignee struct {
	Login string `json:"login"`
}

type CodeScanningAlertUpdateOutput struct {
	Number          string                       `json:"number" jsonschema:"Exact alert number as a decimal string to preserve integer precision."`
	State           *string                      `json:"state" jsonschema:"Actual API state, not the requested state. An open alert is not dismissed even when a dismissal request was attempted."`
	HTMLURL         string                       `json:"html_url"`
	DismissedReason *string                      `json:"dismissed_reason,omitempty"`
	Assignees       *[]CodeScanningAlertAssignee `json:"assignees,omitempty" jsonschema:"Actual API assignments, when returned. Empty means unassigned; omitted means the API did not report assignments."`
}

func UpdateCodeScanningAlert(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataCodeSecurity,
		mcp.Tool{
			Name: "update_code_scanning_alert",
			Description: t("TOOL_UPDATE_CODE_SCANNING_ALERT_DESCRIPTION",
				"Reopen, dismiss, or replace assignees on a code scanning alert. Requires state or assignees. "+
					"Dismissal requires a non-null dismissed_reason. Omitted fields are unchanged; [] clears all assignees. "+
					"create_request:true requires state:dismissed and attempts a delegated dismissal request, not necessarily dismissal. "+
					"The returned state and assignees are the actual API result; no request approval or agent session is inferred. "+
					"Assigning copilot-swe-agent[bot] starts agentic remediation and may open a draft pull request, consuming Code Security AI Credits and Actions minutes. "+
					"Repository permissions, delegated dismissal policy, and agentic Autofix eligibility are enforced by GitHub; errors are returned without fallback."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_UPDATE_CODE_SCANNING_ALERT_USER_TITLE", "Update code scanning alert"),
				ReadOnlyHint:    false,
				DestructiveHint: new(true),
				IdempotentHint:  false,
				OpenWorldHint:   new(true),
			},
			InputSchema: &jsonschema.Schema{
				Type:                 "object",
				AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
				Properties: map[string]*jsonschema.Schema{
					"owner":             {Type: "string", MinLength: new(1), Description: "Repository owner."},
					"repo":              {Type: "string", MinLength: new(1), Description: "Repository name."},
					"alertNumber":       {Type: "integer", Minimum: new(1.0), Description: "Code scanning alert number."},
					"state":             {Type: "string", Enum: []any{"open", "dismissed"}, Description: "Reopen or dismiss the alert. Omission leaves state unchanged."},
					"dismissed_reason":  {Types: []string{"string", "null"}, Enum: []any{"false positive", "won't fix", "used in tests", "mitigated", nil}, Description: "Required and non-null when dismissing."},
					"dismissed_comment": {Types: []string{"string", "null"}, MaxLength: new(280), Description: "Optional dismissal comment, up to 280 characters, or null. Requires a state or assignee update; not a standalone comment API."},
					"create_request":    {Type: "boolean", Description: "Attempt a delegated dismissal request with state:dismissed and a reason. Does not bypass policy."},
					"assignees":         {Type: "array", Items: &jsonschema.Schema{Type: "string"}, Description: "Replace assignees with human usernames or copilot-swe-agent[bot]. [] unassigns everyone. Copilot assignment starts remediation, not just ownership."},
				},
				Required: []string{"owner", "repo", "alertNumber"},
			},
		},
		scopes.RequireAll(scopes.SecurityEvents),
		func(ctx context.Context, deps ToolDependencies, call *mcp.CallToolRequest, _ json.RawMessage) (*mcp.CallToolResult, *CodeScanningAlertUpdateOutput, error) {
			// Typed schema validation passes through float64. Decode the original
			// JSON so large IDs and fractional inputs cannot change the write target.
			args, err := decodeCodeScanningAlertUpdate(call.Params.Arguments)
			if err != nil {
				return utils.NewToolResultErrorFromErr("invalid code scanning alert update arguments", err), nil, nil
			}
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, err
			}
			path := fmt.Sprintf("repos/%s/%s/code-scanning/alerts/%d", url.PathEscape(args.Owner), url.PathEscape(args.Repo), args.AlertNumber)
			// The SDK's update type requires state and lacks assignees and create_request.
			request, err := client.NewRequest(ctx, http.MethodPatch, path, args.CodeScanningAlertPatch)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create code scanning alert update request", err), nil, err
			}
			var alert struct {
				Number          int64                        `json:"number"`
				State           json.RawMessage              `json:"state"`
				HTMLURL         string                       `json:"html_url"`
				DismissedReason *string                      `json:"dismissed_reason"`
				Assignees       *[]CodeScanningAlertAssignee `json:"assignees"`
			}
			resp, err := client.Do(request, &alert)
			if resp != nil && resp.Body != nil {
				defer func() { _ = resp.Body.Close() }()
			}
			const uncertainUpdate = "The code scanning alert update may already have been applied; verify the alert with get_code_scanning_alert before retrying"
			if err != nil {
				if resp != nil && resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
					return utils.NewToolResultErrorFromErr(uncertainUpdate, err), nil, nil
				}
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update code scanning alert", resp, err), nil, nil
			}
			if resp.StatusCode != http.StatusOK || alert.Number != args.AlertNumber || alert.HTMLURL == "" || len(alert.State) == 0 {
				return utils.NewToolResultError("invalid code scanning alert update response. " + uncertainUpdate + "."), nil, nil
			}
			output := CodeScanningAlertUpdateOutput{
				Number: strconv.FormatInt(alert.Number, 10), HTMLURL: alert.HTMLURL,
				DismissedReason: alert.DismissedReason, Assignees: alert.Assignees,
			}
			if err := json.Unmarshal(alert.State, &output.State); err != nil {
				return utils.NewToolResultErrorFromErr("invalid code scanning alert state. "+uncertainUpdate, err), nil, nil
			}
			encoded, err := json.Marshal(output)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal code scanning alert update", err), nil, err
			}
			result := attachStaticIFCLabel(ctx, deps, utils.NewToolResultText(string(encoded)), ifc.LabelSecurityAlert())
			return result, &output, nil
		},
		normalizeCodeScanningAlertUpdate,
	)
}

func normalizeCodeScanningAlertUpdate(raw json.RawMessage) (json.RawMessage, error) {
	if _, err := decodeCodeScanningAlertUpdate(raw); err != nil {
		return nil, &inventory.ToolInputError{Message: err.Error()}
	}
	return raw, nil
}

func decodeCodeScanningAlertUpdate(raw json.RawMessage) (UpdateCodeScanningAlertInput, error) {
	var args UpdateCodeScanningAlertInput
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return args, err
	}
	for name, value := range fields {
		switch name {
		case "dismissed_reason", "dismissed_comment":
			continue
		case "owner", "repo", "alertNumber", "state", "create_request", "assignees":
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
	if args.State != nil && !slices.Contains([]string{"open", "dismissed"}, *args.State) {
		return args, errors.New("state must be open or dismissed")
	}
	var reason *string
	if len(args.DismissedReason) != 0 {
		if err := json.Unmarshal(args.DismissedReason, &reason); err != nil {
			return args, fmt.Errorf("invalid dismissed_reason: %w", err)
		}
		if reason != nil && !slices.Contains([]string{"false positive", "won't fix", "used in tests", "mitigated"}, *reason) {
			return args, errors.New("invalid dismissed_reason")
		}
	}
	if args.State != nil && *args.State == "dismissed" && reason == nil {
		return args, errors.New("a non-null dismissed_reason is required when state is dismissed")
	}
	if args.CreateRequest != nil && *args.CreateRequest && (args.State == nil || *args.State != "dismissed") {
		return args, errors.New("create_request:true requires state:dismissed and dismissed_reason")
	}
	if len(args.DismissedComment) != 0 {
		var comment *string
		if err := json.Unmarshal(args.DismissedComment, &comment); err != nil {
			return args, fmt.Errorf("invalid dismissed_comment: %w", err)
		}
		if comment != nil && utf8.RuneCountInString(*comment) > 280 {
			return args, errors.New("dismissed_comment must not exceed 280 characters")
		}
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
