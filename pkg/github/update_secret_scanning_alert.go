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

	ghErrors "github.com/github/github-mcp-server/v2/pkg/errors"
	"github.com/github/github-mcp-server/v2/pkg/ifc"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type UpdateSecretScanningAlertInput struct {
	SecretScanningAlertPatch
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	AlertNumber int64  `json:"alertNumber"`
}

// RawMessage distinguishes an omitted field from an explicit JSON null.
type SecretScanningAlertPatch struct {
	State             *string         `json:"state,omitempty"`
	Resolution        json.RawMessage `json:"resolution,omitempty"`
	ResolutionComment json.RawMessage `json:"resolution_comment,omitempty"`
	Assignee          json.RawMessage `json:"assignee,omitempty"`
	Validity          json.RawMessage `json:"validity,omitempty"`
}

type SecretScanningAlertUpdateOutput struct {
	Number     string                       `json:"number" jsonschema:"Exact alert number as a decimal string to preserve integer precision."`
	State      string                       `json:"state"`
	Resolution *string                      `json:"resolution"`
	AssignedTo *SecretScanningAlertAssignee `json:"assigned_to"`
	Validity   *string                      `json:"validity,omitempty"`
}

type SecretScanningAlertAssignee struct {
	Login string `json:"login"`
}

func UpdateSecretScanningAlert(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataSecretProtection,
		mcp.Tool{
			Name: "update_secret_scanning_alert",
			Description: t("TOOL_UPDATE_SECRET_SCANNING_ALERT_DESCRIPTION",
				"Update a secret scanning alert's state, assignee, or validity override. "+
					"Provide at least one of state, assignee, or validity. Resolving requires a non-null resolution. "+
					"Omitted fields are unchanged; null unassigns the assignee or clears the validity override. "+
					"Resolution comments accompany closing or reopening and cannot later be edited or deleted. "+
					"Resolution revoked does not revoke credentials; validity is a manual override for custom-pattern alerts only, not a validation request. "+
					"GitHub-supported patterns retain validator-managed validity and cannot be overridden through this endpoint. "+
					"Requires repository secret scanning write access. Delegated dismissal may require Secret scanning alert dismissal requests write permission and an authorized reviewer; API policy failures are returned without bypass."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_UPDATE_SECRET_SCANNING_ALERT_USER_TITLE", "Update secret scanning alert"),
				ReadOnlyHint:    false,
				DestructiveHint: new(true),
				OpenWorldHint:   new(true),
			},
			InputSchema: &jsonschema.Schema{
				Type:                 "object",
				AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
				Properties: map[string]*jsonschema.Schema{
					"owner":              {Type: "string", MinLength: new(1), Description: "Repository owner."},
					"repo":               {Type: "string", MinLength: new(1), Description: "Repository name."},
					"alertNumber":        {Type: "integer", Minimum: new(1.0), Description: "Secret scanning alert number."},
					"state":              {Type: "string", Enum: []any{"open", "resolved"}, Description: "Reopen or resolve the alert."},
					"resolution":         {Types: []string{"string", "null"}, Enum: []any{"false_positive", "wont_fix", "revoked", "used_in_tests", nil}, Description: "Reason for resolving. Non-null when state is resolved."},
					"resolution_comment": {Types: []string{"string", "null"}, Description: "Optional comment when closing or reopening; cannot later be updated or deleted."},
					"assignee":           {Types: []string{"string", "null"}, Description: "Username with repository write access; null unassigns."},
					"validity":           {Types: []string{"string", "null"}, Enum: []any{"active", "inactive", nil}, Description: "Manual validity override for custom-pattern alerts only; null clears the override. GitHub-supported patterns retain validator-managed validity."},
				},
				Required: []string{"owner", "repo", "alertNumber"},
			},
		},
		scopes.RequireAll(scopes.SecurityEvents),
		func(ctx context.Context, deps ToolDependencies, call *mcp.CallToolRequest, _ json.RawMessage) (*mcp.CallToolResult, *SecretScanningAlertUpdateOutput, error) {
			// Typed schema validation passes through float64. Decode the original
			// JSON so large IDs and fractional inputs cannot change the write target.
			args, err := decodeSecretScanningAlertUpdate(call.Params.Arguments)
			if err != nil {
				return utils.NewToolResultErrorFromErr("invalid secret scanning alert update arguments", err), nil, nil
			}
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, err
			}
			path := fmt.Sprintf("repos/%s/%s/secret-scanning/alerts/%d", url.PathEscape(args.Owner), url.PathEscape(args.Repo), args.AlertNumber)
			request, err := client.NewRequest(ctx, http.MethodPatch, path, args.SecretScanningAlertPatch)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create secret scanning alert update request", err), nil, err
			}
			var alert struct {
				Number     int64                        `json:"number"`
				State      string                       `json:"state"`
				Resolution *string                      `json:"resolution"`
				AssignedTo *SecretScanningAlertAssignee `json:"assigned_to"`
				Validity   *string                      `json:"validity"`
			}
			resp, err := client.Do(request, &alert)
			if resp != nil && resp.Body != nil {
				defer func() { _ = resp.Body.Close() }()
			}
			const uncertainOutcome = "The secret scanning alert update may already have been applied; verify the alert with get_secret_scanning_alert before retrying"
			if err != nil {
				if resp != nil && resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
					return utils.NewToolResultErrorFromErr(uncertainOutcome, err), nil, nil
				}
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update secret scanning alert", resp, err), nil, nil
			}
			if resp.StatusCode != http.StatusOK || alert.Number != args.AlertNumber || !slices.Contains([]string{"open", "resolved"}, alert.State) {
				return utils.NewToolResultError("unexpected secret scanning alert update response. " + uncertainOutcome + "."), nil, nil
			}
			output := SecretScanningAlertUpdateOutput{
				Number: strconv.FormatInt(alert.Number, 10), State: alert.State,
				Resolution: alert.Resolution, AssignedTo: alert.AssignedTo, Validity: alert.Validity,
			}
			encoded, err := json.Marshal(output)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal secret scanning alert update", err), nil, err
			}
			result := attachStaticIFCLabel(ctx, deps, utils.NewToolResultText(string(encoded)), ifc.LabelSecurityAlert())
			return result, &output, nil
		},
		normalizeSecretScanningAlertUpdate,
	)
}

func normalizeSecretScanningAlertUpdate(raw json.RawMessage) (json.RawMessage, error) {
	if _, err := decodeSecretScanningAlertUpdate(raw); err != nil {
		return nil, &inventory.ToolInputError{Message: err.Error()}
	}
	return raw, nil
}

func decodeSecretScanningAlertUpdate(raw json.RawMessage) (UpdateSecretScanningAlertInput, error) {
	var args UpdateSecretScanningAlertInput
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return args, err
	}
	for name, value := range fields {
		switch name {
		case "resolution", "resolution_comment", "assignee", "validity":
			continue
		case "owner", "repo", "alertNumber", "state":
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
	if args.State == nil && len(args.Assignee) == 0 && len(args.Validity) == 0 {
		return args, errors.New("provide state, assignee, or validity")
	}
	if args.State != nil && !slices.Contains([]string{"open", "resolved"}, *args.State) {
		return args, errors.New("state must be open or resolved")
	}
	var resolution *string
	if len(args.Resolution) != 0 {
		if err := json.Unmarshal(args.Resolution, &resolution); err != nil {
			return args, err
		}
		if resolution != nil && !slices.Contains([]string{"false_positive", "wont_fix", "revoked", "used_in_tests"}, *resolution) {
			return args, errors.New("invalid resolution")
		}
	}
	if args.State != nil && *args.State == "resolved" && resolution == nil {
		return args, errors.New("a non-null resolution is required when state is resolved")
	}
	for name, raw := range map[string]json.RawMessage{
		"resolution_comment": args.ResolutionComment, "assignee": args.Assignee, "validity": args.Validity,
	} {
		if len(raw) == 0 {
			continue
		}
		var value *string
		if err := json.Unmarshal(raw, &value); err != nil {
			return args, fmt.Errorf("invalid %s: %w", name, err)
		}
		if name == "validity" && value != nil && !slices.Contains([]string{"active", "inactive"}, *value) {
			return args, errors.New("validity must be active, inactive, or null")
		}
	}
	return args, nil
}
