package github

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	ghErrors "github.com/github/github-mcp-server/v2/pkg/errors"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/sanitize"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const agentAutomationAPIVersion = "2026-03-10"
const agentAutomationConfirmationIDPrefix = "agent_automation_confirmation_"
const agentAutomationConfirmationField = "confirmation"
const agentAutomationConfirmationTTL = 10 * time.Minute

type ListAutomationsInput struct {
	Owner   string `json:"owner"`
	Repo    string `json:"repo"`
	Page    int    `json:"page,omitempty"`
	PerPage int    `json:"perPage,omitempty"`
	Type    string `json:"type,omitempty"`
}

type GetAutomationInput struct {
	Owner        string `json:"owner"`
	Repo         string `json:"repo"`
	AutomationID string `json:"automation_id"`
}

type ListAutomationRevisionsInput struct {
	Owner        string `json:"owner"`
	Repo         string `json:"repo"`
	AutomationID string `json:"automation_id"`
	Cursor       string `json:"cursor,omitempty"`
	PerPage      int    `json:"perPage,omitempty"`
}

type GetAutomationRevisionInput struct {
	Owner        string `json:"owner"`
	Repo         string `json:"repo"`
	AutomationID string `json:"automation_id"`
	RevisionID   string `json:"revision_id"`
}

type AutomationCatalogInput struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

type CreateAutomationInput struct {
	Owner                       string         `json:"owner"`
	Repo                        string         `json:"repo"`
	Name                        string         `json:"name"`
	Prompt                      string         `json:"prompt"`
	Triggers                    map[string]any `json:"triggers,omitempty"`
	Model                       *string        `json:"model,omitempty"`
	ReasoningEffort             *string        `json:"reasoning_effort,omitempty"`
	Tools                       []string       `json:"tools,omitempty"`
	Enabled                     *bool          `json:"enabled,omitempty"`
	Type                        *string        `json:"type,omitempty"`
	RequireActorWritePermission *bool          `json:"require_actor_write_permission,omitempty"`
	ContinuousHistory           *bool          `json:"continuous_history,omitempty"`
}

type UpdateAutomationInput struct {
	Owner                       string         `json:"owner"`
	Repo                        string         `json:"repo"`
	AutomationID                string         `json:"automation_id"`
	Name                        *string        `json:"name,omitempty"`
	Enabled                     *bool          `json:"enabled,omitempty"`
	RequireActorWritePermission *bool          `json:"require_actor_write_permission,omitempty"`
	ContinuousHistory           *bool          `json:"continuous_history,omitempty"`
	Model                       *string        `json:"model,omitempty"`
	ReasoningEffort             *string        `json:"reasoning_effort,omitempty"`
	Triggers                    map[string]any `json:"triggers,omitempty"`
	Prompt                      *string        `json:"prompt,omitempty"`
	Tools                       []string       `json:"tools,omitempty"`
}

type DeleteAutomationInput struct {
	Owner        string `json:"owner"`
	Repo         string `json:"repo"`
	AutomationID string `json:"automation_id"`
}

type agentAutomationRequest struct {
	method         string
	path           string
	query          url.Values
	body           any
	successStatus  int
	noBody         bool
	successMessage string
	confirmation   *agentAutomationConfirmation
}

type agentAutomationConfirmation struct {
	id      string
	value   string
	message string
}

type agentAutomationConfirmationState struct {
	ToolName          string `json:"tool_name"`
	RequestDigest     string `json:"request_digest"`
	ConfirmationID    string `json:"confirmation_id"`
	ConfirmationValue string `json:"confirmation_value"`
	ExpiresAt         int64  `json:"expires_at"`
}

type agentAutomationRequestBuilder[Input any] func(Input) (agentAutomationRequest, error)

func newAgentAutomationTool[Input any](
	t translations.TranslationHelperFunc,
	name, title, description string,
	inputSchema *jsonschema.Schema,
	outputSchema json.RawMessage,
	annotations mcp.ToolAnnotations,
	build agentAutomationRequestBuilder[Input],
) inventory.ServerTool {
	annotations.Title = t("TOOL_"+name+"_USER_TITLE", title)
	tool := NewTool[Input, json.RawMessage](
		ToolsetMetadataCopilot,
		mcp.Tool{
			Name:         name,
			Description:  t("TOOL_"+name+"_DESCRIPTION", description),
			InputSchema:  inputSchema,
			OutputSchema: outputSchema,
			Annotations: &mcp.ToolAnnotations{
				Title:           annotations.Title,
				ReadOnlyHint:    annotations.ReadOnlyHint,
				DestructiveHint: annotations.DestructiveHint,
				IdempotentHint:  annotations.IdempotentHint,
				OpenWorldHint:   annotations.OpenWorldHint,
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, req *mcp.CallToolRequest, args Input) (*mcp.CallToolResult, json.RawMessage, error) {
			request, err := build(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if request.confirmation != nil {
				result, err := agentAutomationConfirmationResult(ctx, deps, req, name, request)
				if err != nil || result != nil {
					return result, nil, err
				}
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, err
			}

			if len(request.query) > 0 {
				request.path += "?" + request.query.Encode()
			}
			httpRequest, err := client.NewRequest(ctx, request.method, request.path, request.body, github.WithVersion(agentAutomationAPIVersion))
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create GitHub request", err), nil, nil
			}

			var response json.RawMessage
			var responseTarget any = &response
			if request.noBody {
				responseTarget = nil
			}
			httpResponse, err := client.Do(httpRequest, responseTarget)
			if httpResponse != nil {
				defer func() { _ = httpResponse.Body.Close() }()
			}
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to access GitHub agent automations", httpResponse, err), nil, nil
			}
			successStatus := request.successStatus
			if successStatus == 0 {
				successStatus = http.StatusOK
			}
			if httpResponse.StatusCode != successStatus {
				return utils.NewToolResultError(fmt.Sprintf("GitHub agent automations returned unexpected status %d", httpResponse.StatusCode)), nil, nil
			}

			if request.successMessage != "" {
				response, err = json.Marshal(map[string]string{"message": request.successMessage})
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to marshal success response", err), nil, err
				}
			} else if len(response) == 0 {
				response = json.RawMessage(`{}`)
			}
			sanitizedResponse, err := sanitizeAutomationNames(response)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to sanitize GitHub agent automation response", err), nil, nil
			}
			return utils.NewToolResultText(string(sanitizedResponse)), sanitizedResponse, nil
		},
	)
	tool.FeatureRule = featureEnabledRule(FeatureFlagAgentAutomations)
	return tool
}

func agentAutomationConfirmationResult(
	ctx context.Context,
	deps ToolDependencies,
	req *mcp.CallToolRequest,
	toolName string,
	request agentAutomationRequest,
) (*mcp.CallToolResult, error) {
	if req == nil || req.Params == nil {
		return utils.NewToolResultError("Automation confirmation is unavailable without an MCP request."), nil
	}
	sealer := requestStateSealerFromDeps(deps)
	if sealer == nil {
		return utils.NewToolResultError("Automation confirmation is unavailable because request-state protection is not configured."), nil
	}
	digest, err := agentAutomationRequestDigest(request)
	if err != nil {
		return nil, fmt.Errorf("failed to fingerprint automation request: %w", err)
	}

	response, hasResponse := req.Params.InputResponses[request.confirmation.id]
	if !hasResponse {
		state, err := json.Marshal(agentAutomationConfirmationState{
			ToolName:          toolName,
			RequestDigest:     digest,
			ConfirmationID:    request.confirmation.id,
			ConfirmationValue: request.confirmation.value,
			ExpiresAt:         time.Now().Add(agentAutomationConfirmationTTL).Unix(),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal automation confirmation state: %w", err)
		}
		requestState, err := sealer.Seal(ctx, state)
		if err != nil {
			return nil, fmt.Errorf("failed to seal automation confirmation state: %w", err)
		}
		return &mcp.CallToolResult{
			InputRequests: mcp.InputRequestMap{
				request.confirmation.id: &mcp.ElicitParams{
					Mode:    "form",
					Message: request.confirmation.message,
					RequestedSchema: &jsonschema.Schema{
						Type: "object",
						Properties: map[string]*jsonschema.Schema{
							agentAutomationConfirmationField: {
								Type:        "string",
								Title:       "Confirmation",
								Description: fmt.Sprintf("Enter %q exactly to confirm this operation.", request.confirmation.value),
							},
						},
						Required: []string{agentAutomationConfirmationField},
					},
				},
			},
			RequestState: requestState,
		}, nil
	}

	if req.Params.RequestState == "" {
		return utils.NewToolResultError("Automation confirmation state is missing. The operation was not performed."), nil
	}
	stateJSON, err := sealer.Open(req.Params.RequestState)
	if err != nil {
		return utils.NewToolResultError("Automation confirmation state is invalid. The operation was not performed."), nil
	}
	var state agentAutomationConfirmationState
	if err := json.Unmarshal(stateJSON, &state); err != nil {
		return utils.NewToolResultError("Automation confirmation state is invalid. The operation was not performed."), nil
	}
	if state.ToolName != toolName ||
		state.RequestDigest != digest ||
		state.ConfirmationID != request.confirmation.id ||
		state.ConfirmationValue != request.confirmation.value {
		return utils.NewToolResultError("Automation request changed after confirmation was requested. The operation was not performed."), nil
	}
	if state.ExpiresAt <= time.Now().Unix() {
		return utils.NewToolResultError("Automation confirmation expired. The operation was not performed."), nil
	}

	confirmation, ok := response.(*mcp.ElicitResult)
	if !ok {
		return utils.NewToolResultError("Automation confirmation response is invalid. The operation was not performed."), nil
	}
	if confirmation.Action != "accept" {
		return utils.NewToolResultError("Automation operation was not confirmed."), nil
	}
	value, ok := confirmation.Content[agentAutomationConfirmationField].(string)
	if !ok || value != request.confirmation.value {
		return utils.NewToolResultError("Automation confirmation did not match the requested operation."), nil
	}
	return nil, nil
}

func agentAutomationRequestDigest(request agentAutomationRequest) (string, error) {
	body, err := json.Marshal(request.body)
	if err != nil {
		return "", err
	}
	payload := strings.Join([]string{
		request.method,
		request.path,
		request.query.Encode(),
		string(body),
	}, "\n")
	digest := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(digest[:]), nil
}

func agentAutomationConfirmationRequired(tool inventory.ServerTool) inventory.ServerTool {
	tool.MinimumProtocolVersion = inventory.ProtocolVersionMultiRoundTrip
	tool.RequiredElicitationMode = inventory.ElicitationModeForm
	return tool
}

func agentAutomationMutationConfirmation(action, target string, body map[string]any) *agentAutomationConfirmation {
	fields := make([]string, 0, len(body))
	for field := range body {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	value := strings.ToUpper(action) + " " + target
	return &agentAutomationConfirmation{
		id:      agentAutomationConfirmationIDPrefix + action,
		value:   value,
		message: fmt.Sprintf("Review the proposed automation %s for %s. The request changes these fields: %s. Values are not shown here. Type %q to confirm.", action, target, strings.Join(fields, ", "), value),
	}
}

func agentAutomationTarget(owner, repo, automationID string) string {
	target := url.PathEscape(owner) + "/" + url.PathEscape(repo)
	if automationID != "" {
		target += "/" + url.PathEscape(automationID)
	}
	return target
}

func agentAutomationObjectOutputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","additionalProperties":true}`)
}

func agentAutomationArrayOutputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"array","items":{"type":"object","additionalProperties":true}}`)
}

func agentAutomationConfigurationOutputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"automations_enabled":{"type":"boolean"},
			"can_manage_automations":{"type":"boolean"},
			"allow_read_access_triggers":{"type":"boolean"},
			"require_trigger_scoped_writes":{"type":"boolean"}
		},
		"required":[
			"automations_enabled",
			"can_manage_automations",
			"allow_read_access_triggers",
			"require_trigger_scoped_writes"
		],
		"additionalProperties":true
	}`)
}

func agentAutomationInputSchema(properties map[string]*jsonschema.Schema, required ...string) *jsonschema.Schema {
	return &jsonschema.Schema{Type: "object", Properties: properties, Required: required}
}

func automationStringProperty(description string) *jsonschema.Schema {
	return &jsonschema.Schema{Type: "string", Description: description}
}

func automationIntegerProperty(description string, minimum float64, maximum ...float64) *jsonschema.Schema {
	property := &jsonschema.Schema{Type: "integer", Description: description, Minimum: &minimum}
	if len(maximum) > 0 {
		property.Maximum = &maximum[0]
	}
	return property
}

func automationBooleanProperty(description string) *jsonschema.Schema {
	return &jsonschema.Schema{Type: "boolean", Description: description}
}

func automationStringArrayProperty(description string) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        "array",
		Description: description,
		Items:       &jsonschema.Schema{Type: "string"},
	}
}

func automationConfigurationProperties() map[string]*jsonschema.Schema {
	return map[string]*jsonschema.Schema{
		"name":                           automationStringProperty("The name of the automation."),
		"triggers":                       {Type: "object", Description: "Trigger configuration for the automation."},
		"prompt":                         automationStringProperty("The prompt used when the automation runs."),
		"model":                          automationStringProperty("The model used by the automation."),
		"reasoning_effort":               automationStringProperty("The reasoning effort used by the automation."),
		"tools":                          automationStringArrayProperty("The tools enabled for the automation."),
		"enabled":                        automationBooleanProperty("Whether the automation is enabled."),
		"type":                           {Type: "string", Description: "The automation type.", Enum: []any{"personal", "shared"}},
		"require_actor_write_permission": automationBooleanProperty("Whether the triggering actor must have write permission."),
		"continuous_history":             automationBooleanProperty("Whether the automation keeps a continuous conversation history."),
	}
}

func requireAutomationRepository(owner, repo string) error {
	if owner == "" {
		return errors.New("missing required parameter: owner")
	}
	if repo == "" {
		return errors.New("missing required parameter: repo")
	}
	if err := requireAutomationPathSegment("owner", owner); err != nil {
		return err
	}
	if err := requireAutomationPathSegment("repo", repo); err != nil {
		return err
	}
	return nil
}

func requireAutomationPathSegment(name, value string) error {
	if value == "." || value == ".." {
		return fmt.Errorf("%s cannot be a dot path segment", name)
	}
	return nil
}

func requireAutomationID(value string) error {
	if value == "" {
		return errors.New("missing required parameter: automation_id")
	}
	if err := requireAutomationPathSegment("automation_id", value); err != nil {
		return err
	}
	switch value {
	case "available-tools", "available-triggers", "configuration", "revisions":
		return errors.New("automation_id cannot match a reserved automation endpoint")
	}
	return nil
}

func sanitizeAutomationNames(response json.RawMessage) (json.RawMessage, error) {
	var payload any
	if err := json.Unmarshal(response, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	sanitizeAutomationNamesValue(payload)
	sanitizedResponse, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode sanitized response: %w", err)
	}
	return sanitizedResponse, nil
}

func sanitizeAutomationNamesValue(value any) {
	switch value := value.(type) {
	case map[string]any:
		for key, field := range value {
			if key == "name" {
				if name, ok := field.(string); ok {
					value[key] = sanitize.PlainText(name)
					continue
				}
			}
			sanitizeAutomationNamesValue(field)
		}
	case []any:
		for _, field := range value {
			sanitizeAutomationNamesValue(field)
		}
	}
}

func automationPath(owner, repo, suffix string) string {
	path := "agents/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/automations"
	if suffix != "" {
		path += "/" + suffix
	}
	return path
}

func ListAutomations(t translations.TranslationHelperFunc) inventory.ServerTool {
	return newAgentAutomationTool[ListAutomationsInput](
		t,
		"list_automations",
		"List repository automations",
		"List Copilot automations for a repository.",
		agentAutomationInputSchema(map[string]*jsonschema.Schema{
			"owner":   automationStringProperty(DescriptionRepositoryOwner),
			"repo":    automationStringProperty(DescriptionRepositoryName),
			"page":    automationIntegerProperty("Page number, starting at 1.", 1),
			"perPage": automationIntegerProperty("Results per page, from 1 to 100.", 1, 100),
			"type": {
				Type:        "string",
				Description: "Filter automations by type.",
				Enum:        []any{"personal", "shared"},
			},
		}, "owner", "repo"),
		agentAutomationObjectOutputSchema(),
		mcp.ToolAnnotations{ReadOnlyHint: true},
		func(args ListAutomationsInput) (agentAutomationRequest, error) {
			if err := requireAutomationRepository(args.Owner, args.Repo); err != nil {
				return agentAutomationRequest{}, err
			}
			if args.Page < 0 || args.PerPage < 0 || args.PerPage > 100 {
				return agentAutomationRequest{}, errors.New("page and perPage must be positive when provided; perPage cannot exceed 100")
			}
			query := make(url.Values)
			if args.Page > 0 {
				query.Set("page", strconv.Itoa(args.Page))
			}
			if args.PerPage > 0 {
				query.Set("per_page", strconv.Itoa(args.PerPage))
			}
			if args.Type != "" {
				query.Set("type", args.Type)
			}
			return agentAutomationRequest{
				method: http.MethodGet,
				path:   automationPath(args.Owner, args.Repo, ""),
				query:  query,
			}, nil
		},
	)
}

func GetAutomation(t translations.TranslationHelperFunc) inventory.ServerTool {
	return newAgentAutomationTool[GetAutomationInput](
		t,
		"get_automation",
		"Get a repository automation",
		"Get the configuration of a Copilot automation in a repository.",
		agentAutomationInputSchema(map[string]*jsonschema.Schema{
			"owner":         automationStringProperty(DescriptionRepositoryOwner),
			"repo":          automationStringProperty(DescriptionRepositoryName),
			"automation_id": automationStringProperty("The automation identifier."),
		}, "owner", "repo", "automation_id"),
		agentAutomationObjectOutputSchema(),
		mcp.ToolAnnotations{ReadOnlyHint: true},
		func(args GetAutomationInput) (agentAutomationRequest, error) {
			if err := requireAutomationRepository(args.Owner, args.Repo); err != nil {
				return agentAutomationRequest{}, err
			}
			if err := requireAutomationID(args.AutomationID); err != nil {
				return agentAutomationRequest{}, err
			}
			return agentAutomationRequest{
				method: http.MethodGet,
				path:   automationPath(args.Owner, args.Repo, url.PathEscape(args.AutomationID)),
			}, nil
		},
	)
}

func ListAutomationRevisions(t translations.TranslationHelperFunc) inventory.ServerTool {
	return newAgentAutomationTool[ListAutomationRevisionsInput](
		t,
		"list_automation_revisions",
		"List automation revisions",
		"List the saved revisions of a Copilot automation.",
		agentAutomationInputSchema(map[string]*jsonschema.Schema{
			"owner":         automationStringProperty(DescriptionRepositoryOwner),
			"repo":          automationStringProperty(DescriptionRepositoryName),
			"automation_id": automationStringProperty("The automation identifier."),
			"cursor":        automationStringProperty("Opaque pagination cursor."),
			"perPage":       automationIntegerProperty("Results per page, from 1 to 100.", 1, 100),
		}, "owner", "repo", "automation_id"),
		agentAutomationObjectOutputSchema(),
		mcp.ToolAnnotations{ReadOnlyHint: true},
		func(args ListAutomationRevisionsInput) (agentAutomationRequest, error) {
			if err := requireAutomationRepository(args.Owner, args.Repo); err != nil {
				return agentAutomationRequest{}, err
			}
			if err := requireAutomationID(args.AutomationID); err != nil {
				return agentAutomationRequest{}, err
			}
			if args.PerPage < 0 || args.PerPage > 100 {
				return agentAutomationRequest{}, errors.New("perPage must be between 1 and 100")
			}
			query := make(url.Values)
			if args.Cursor != "" {
				query.Set("cursor", args.Cursor)
			}
			if args.PerPage > 0 {
				query.Set("per_page", strconv.Itoa(args.PerPage))
			}
			return agentAutomationRequest{
				method: http.MethodGet,
				path:   automationPath(args.Owner, args.Repo, url.PathEscape(args.AutomationID)+"/revisions"),
				query:  query,
			}, nil
		},
	)
}

func GetAutomationRevision(t translations.TranslationHelperFunc) inventory.ServerTool {
	return newAgentAutomationTool[GetAutomationRevisionInput](
		t,
		"get_automation_revision",
		"Get an automation revision",
		"Get a specific saved revision of a Copilot automation.",
		agentAutomationInputSchema(map[string]*jsonschema.Schema{
			"owner":         automationStringProperty(DescriptionRepositoryOwner),
			"repo":          automationStringProperty(DescriptionRepositoryName),
			"automation_id": automationStringProperty("The automation identifier."),
			"revision_id":   automationStringProperty("The revision identifier."),
		}, "owner", "repo", "automation_id", "revision_id"),
		agentAutomationObjectOutputSchema(),
		mcp.ToolAnnotations{ReadOnlyHint: true},
		func(args GetAutomationRevisionInput) (agentAutomationRequest, error) {
			if err := requireAutomationRepository(args.Owner, args.Repo); err != nil {
				return agentAutomationRequest{}, err
			}
			if err := requireAutomationID(args.AutomationID); err != nil {
				return agentAutomationRequest{}, err
			}
			if args.RevisionID == "" {
				return agentAutomationRequest{}, errors.New("missing required parameter: revision_id")
			}
			if err := requireAutomationPathSegment("revision_id", args.RevisionID); err != nil {
				return agentAutomationRequest{}, err
			}
			return agentAutomationRequest{
				method: http.MethodGet,
				path:   automationPath(args.Owner, args.Repo, url.PathEscape(args.AutomationID)+"/revisions/"+url.PathEscape(args.RevisionID)),
			}, nil
		},
	)
}

func ListAvailableAutomationTools(t translations.TranslationHelperFunc) inventory.ServerTool {
	return newAgentAutomationTool[AutomationCatalogInput](
		t,
		"list_available_automation_tools",
		"List available automation tools",
		"List the tools that can be used by automations in a repository.",
		agentAutomationInputSchema(map[string]*jsonschema.Schema{
			"owner": automationStringProperty(DescriptionRepositoryOwner),
			"repo":  automationStringProperty(DescriptionRepositoryName),
		}, "owner", "repo"),
		agentAutomationObjectOutputSchema(),
		mcp.ToolAnnotations{ReadOnlyHint: true},
		func(args AutomationCatalogInput) (agentAutomationRequest, error) {
			if err := requireAutomationRepository(args.Owner, args.Repo); err != nil {
				return agentAutomationRequest{}, err
			}
			return agentAutomationRequest{
				method: http.MethodGet,
				path:   automationPath(args.Owner, args.Repo, "available-tools"),
			}, nil
		},
	)
}

func ListAvailableAutomationTriggers(t translations.TranslationHelperFunc) inventory.ServerTool {
	return newAgentAutomationTool[AutomationCatalogInput](
		t,
		"list_available_automation_triggers",
		"List available automation triggers",
		"List the trigger types that can be used by automations in a repository.",
		agentAutomationInputSchema(map[string]*jsonschema.Schema{
			"owner": automationStringProperty(DescriptionRepositoryOwner),
			"repo":  automationStringProperty(DescriptionRepositoryName),
		}, "owner", "repo"),
		agentAutomationArrayOutputSchema(),
		mcp.ToolAnnotations{ReadOnlyHint: true},
		func(args AutomationCatalogInput) (agentAutomationRequest, error) {
			if err := requireAutomationRepository(args.Owner, args.Repo); err != nil {
				return agentAutomationRequest{}, err
			}
			return agentAutomationRequest{
				method: http.MethodGet,
				path:   automationPath(args.Owner, args.Repo, "available-triggers"),
			}, nil
		},
	)
}

func GetAutomationsConfigurationForRepository(t translations.TranslationHelperFunc) inventory.ServerTool {
	return newAgentAutomationTool[AutomationCatalogInput](
		t,
		"get_automations_configuration_for_repository",
		"Get repository automation configuration",
		"Get whether automations are enabled, whether you can manage them, and the repository's automation trigger and write-scope policies.",
		agentAutomationInputSchema(map[string]*jsonschema.Schema{
			"owner": automationStringProperty(DescriptionRepositoryOwner),
			"repo":  automationStringProperty(DescriptionRepositoryName),
		}, "owner", "repo"),
		agentAutomationConfigurationOutputSchema(),
		mcp.ToolAnnotations{ReadOnlyHint: true},
		func(args AutomationCatalogInput) (agentAutomationRequest, error) {
			if err := requireAutomationRepository(args.Owner, args.Repo); err != nil {
				return agentAutomationRequest{}, err
			}
			return agentAutomationRequest{
				method: http.MethodGet,
				path:   automationPath(args.Owner, args.Repo, "configuration"),
			}, nil
		},
	)
}

func CreateAutomation(t translations.TranslationHelperFunc) inventory.ServerTool {
	properties := automationConfigurationProperties()
	properties["owner"] = automationStringProperty(DescriptionRepositoryOwner)
	properties["repo"] = automationStringProperty(DescriptionRepositoryName)
	return agentAutomationConfirmationRequired(newAgentAutomationTool[CreateAutomationInput](
		t,
		"create_automation",
		"Create a repository automation",
		"Create a Copilot automation in a repository after explicit user confirmation.",
		agentAutomationInputSchema(properties, "owner", "repo", "name", "prompt"),
		agentAutomationObjectOutputSchema(),
		mcp.ToolAnnotations{},
		func(args CreateAutomationInput) (agentAutomationRequest, error) {
			if err := requireAutomationRepository(args.Owner, args.Repo); err != nil {
				return agentAutomationRequest{}, err
			}
			body := map[string]any{"name": args.Name, "prompt": args.Prompt}
			if args.Name == "" {
				return agentAutomationRequest{}, errors.New("missing required parameter: name")
			}
			if args.Prompt == "" {
				return agentAutomationRequest{}, errors.New("missing required parameter: prompt")
			}
			if args.Triggers != nil {
				body["triggers"] = args.Triggers
			}
			if args.Model != nil {
				body["model"] = *args.Model
			}
			if args.ReasoningEffort != nil {
				body["reasoning_effort"] = *args.ReasoningEffort
			}
			if args.Tools != nil {
				body["tools"] = args.Tools
			}
			if args.Enabled != nil {
				body["enabled"] = *args.Enabled
			}
			if args.Type != nil {
				if *args.Type != "personal" && *args.Type != "shared" {
					return agentAutomationRequest{}, errors.New("type must be personal or shared")
				}
				body["type"] = *args.Type
			}
			if args.RequireActorWritePermission != nil {
				body["require_actor_write_permission"] = *args.RequireActorWritePermission
			}
			if args.ContinuousHistory != nil {
				body["continuous_history"] = *args.ContinuousHistory
			}
			target := agentAutomationTarget(args.Owner, args.Repo, "")
			return agentAutomationRequest{
				method:        http.MethodPost,
				path:          automationPath(args.Owner, args.Repo, ""),
				body:          body,
				successStatus: http.StatusCreated,
				confirmation:  agentAutomationMutationConfirmation("create", target, body),
			}, nil
		},
	))
}

func UpdateAutomation(t translations.TranslationHelperFunc) inventory.ServerTool {
	properties := automationConfigurationProperties()
	properties["owner"] = automationStringProperty(DescriptionRepositoryOwner)
	properties["repo"] = automationStringProperty(DescriptionRepositoryName)
	properties["automation_id"] = automationStringProperty("The automation identifier.")
	delete(properties, "type")
	return agentAutomationConfirmationRequired(newAgentAutomationTool[UpdateAutomationInput](
		t,
		"update_automation",
		"Update a repository automation",
		"Update a Copilot automation in a repository after explicit user confirmation. Only the provided fields are changed.",
		agentAutomationInputSchema(properties, "owner", "repo", "automation_id"),
		agentAutomationObjectOutputSchema(),
		mcp.ToolAnnotations{},
		func(args UpdateAutomationInput) (agentAutomationRequest, error) {
			if err := requireAutomationRepository(args.Owner, args.Repo); err != nil {
				return agentAutomationRequest{}, err
			}
			if args.AutomationID == "" {
				return agentAutomationRequest{}, errors.New("missing required parameter: automation_id")
			}
			body := make(map[string]any)
			if args.Name != nil {
				body["name"] = *args.Name
			}
			if args.Enabled != nil {
				body["enabled"] = *args.Enabled
			}
			if args.RequireActorWritePermission != nil {
				body["require_actor_write_permission"] = *args.RequireActorWritePermission
			}
			if args.ContinuousHistory != nil {
				body["continuous_history"] = *args.ContinuousHistory
			}
			if args.Model != nil {
				body["model"] = *args.Model
			}
			if args.ReasoningEffort != nil {
				body["reasoning_effort"] = *args.ReasoningEffort
			}
			if args.Triggers != nil {
				body["triggers"] = args.Triggers
			}
			if args.Prompt != nil {
				body["prompt"] = *args.Prompt
			}
			if args.Tools != nil {
				body["tools"] = args.Tools
			}
			if len(body) == 0 {
				return agentAutomationRequest{}, errors.New("at least one automation field must be provided")
			}
			target := agentAutomationTarget(args.Owner, args.Repo, args.AutomationID)
			return agentAutomationRequest{
				method:       http.MethodPatch,
				path:         automationPath(args.Owner, args.Repo, url.PathEscape(args.AutomationID)),
				body:         body,
				confirmation: agentAutomationMutationConfirmation("update", target, body),
			}, nil
		},
	))
}

func DeleteAutomation(t translations.TranslationHelperFunc) inventory.ServerTool {
	return agentAutomationConfirmationRequired(newAgentAutomationTool[DeleteAutomationInput](
		t,
		"delete_automation",
		"Delete a repository automation",
		"Delete a Copilot automation from a repository after explicit user confirmation.",
		agentAutomationInputSchema(map[string]*jsonschema.Schema{
			"owner":         automationStringProperty(DescriptionRepositoryOwner),
			"repo":          automationStringProperty(DescriptionRepositoryName),
			"automation_id": automationStringProperty("The automation identifier."),
		}, "owner", "repo", "automation_id"),
		agentAutomationObjectOutputSchema(),
		mcp.ToolAnnotations{DestructiveHint: new(true), IdempotentHint: true},
		func(args DeleteAutomationInput) (agentAutomationRequest, error) {
			if err := requireAutomationRepository(args.Owner, args.Repo); err != nil {
				return agentAutomationRequest{}, err
			}
			if args.AutomationID == "" {
				return agentAutomationRequest{}, errors.New("missing required parameter: automation_id")
			}
			target := agentAutomationTarget(args.Owner, args.Repo, args.AutomationID)
			return agentAutomationRequest{
				method:         http.MethodDelete,
				path:           automationPath(args.Owner, args.Repo, url.PathEscape(args.AutomationID)),
				successStatus:  http.StatusNoContent,
				noBody:         true,
				successMessage: "Automation deleted.",
				confirmation: &agentAutomationConfirmation{
					id:      agentAutomationConfirmationIDPrefix + "delete",
					value:   "DELETE " + target,
					message: fmt.Sprintf("Review the automation to delete: %s. Type %q to confirm.", target, "DELETE "+target),
				},
			}, nil
		},
	))
}
