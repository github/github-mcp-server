package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

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

type agentAutomationRequest struct {
	method string
	path   string
	query  url.Values
	body   any
}

type agentAutomationRequestBuilder[Input any] func(Input) (agentAutomationRequest, error)

func newAgentAutomationTool[Input any](
	t translations.TranslationHelperFunc,
	name, title, description string,
	inputSchema *jsonschema.Schema,
	outputSchema json.RawMessage,
	build agentAutomationRequestBuilder[Input],
) inventory.ServerTool {
	tool := NewTool[Input, json.RawMessage](
		ToolsetMetadataCopilot,
		mcp.Tool{
			Name:         name,
			Description:  t("TOOL_"+name+"_DESCRIPTION", description),
			InputSchema:  inputSchema,
			OutputSchema: outputSchema,
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_"+name+"_USER_TITLE", title),
				ReadOnlyHint: true,
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args Input) (*mcp.CallToolResult, json.RawMessage, error) {
			request, err := build(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
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
			httpResponse, err := client.Do(httpRequest, &response)
			if httpResponse != nil {
				defer func() { _ = httpResponse.Body.Close() }()
			}
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to access GitHub agent automations", httpResponse, err), nil, nil
			}
			if httpResponse.StatusCode != http.StatusOK {
				return utils.NewToolResultError(fmt.Sprintf("GitHub agent automations returned unexpected status %d", httpResponse.StatusCode)), nil, nil
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
