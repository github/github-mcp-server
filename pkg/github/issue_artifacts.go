package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	ghErrors "github.com/github/github-mcp-server/v2/pkg/errors"
	"github.com/github/github-mcp-server/v2/pkg/ifc"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/sanitize"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var issueArtifactTypes = []string{"plan", "generic", "link", "instruction", "skill", "mcp_config", "reference"}

func newIssueArtifactTool[In, Out any](t translations.TranslationHelperFunc, name, title, description string, readOnly, destructive bool, enums []inventory.SchemaEnum,
	handler func(context.Context, ToolDependencies, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error),
) inventory.ServerTool {
	key := "TOOL_" + strings.ToUpper(name)
	schema := issueDTOschema[In]()
	for _, field := range []string{"issue_number", "artifact_number", "page", "perPage"} {
		if property := schema.Properties[field]; property != nil {
			property.Minimum = new(1.0)
		}
	}
	for _, field := range []string{"owner", "repo", "name"} {
		if property := schema.Properties[field]; property != nil {
			property.MinLength = new(1)
		}
	}
	if property := schema.Properties["perPage"]; property != nil {
		property.Maximum = new(100.0)
	}
	for _, enum := range enums {
		schema.Properties[enum.Path] = inventory.EnumSchema(enum.Values...)
	}
	if schema.Properties["name"] != nil && !slices.Contains(schema.Required, "name") {
		schema.Properties["name"].Type, schema.Properties["name"].Types = "string", nil
		schema.Properties["body"].Type, schema.Properties["body"].Types = "string", nil
	}
	st := NewToolWithSchemaOptions(ToolsetMetadataIssues, mcp.Tool{
		Name: name, Description: t(key+"_DESCRIPTION", description),
		InputSchema: schema,
		Annotations: &mcp.ToolAnnotations{
			Title: t(key+"_USER_TITLE", title), ReadOnlyHint: readOnly,
			DestructiveHint: new(destructive),
		},
	}, scopes.RequireAll(scopes.Repo), inventory.TypedSchemaOptions{
		PreserveHandlerContent: true,
	}, handler)
	st.FeatureRule = featureEnabledRule(FeatureFlagIssueArtifacts)
	return st
}

// IssueArtifactWrite creates, updates, or deletes an issue artifact.
func IssueArtifactWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return newIssueArtifactTool(t, "issue_artifact_write", "Write issue artifacts",
		"Create, update, or delete a GitHub issue artifact. Create requires type, name, and body (which may be empty). Update requires artifact_number and name, body, or both; an empty body clears it and type is immutable. Delete requires artifact_number. Artifact numbers are issue-scoped, not global IDs. Experimental: endpoint availability and access requirements are unverified.",
		false, true, []inventory.SchemaEnum{{Path: "method", Values: []string{"create", "update", "delete"}}, {Path: "type", Values: issueArtifactTypes}},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input IssueArtifactWriteInput) (*mcp.CallToolResult, *IssueArtifactWriteOutput, error) {
			var artifact IssueArtifact
			switch input.Method {
			case "create":
				if input.ArtifactNumber != 0 {
					return utils.NewToolResultError("artifact_number is not supported for create"), nil, nil
				}
				if !slices.Contains(issueArtifactTypes, input.Type) {
					return utils.NewToolResultError("unsupported artifact type"), nil, nil
				}
				if input.Name == nil || strings.TrimSpace(*input.Name) == "" {
					return utils.NewToolResultError("name must not be empty"), nil, nil
				}
				if input.Body == nil {
					return utils.NewToolResultError("body is required for create; use an empty string for an empty body"), nil, nil
				}
				body := struct {
					Type string `json:"type"`
					Name string `json:"name"`
					Body string `json:"body"`
				}{input.Type, *input.Name, *input.Body}
				if result := issueArtifactRequest(ctx, deps, input.IssueArtifactCoordinates, 0, http.MethodPost, nil, body, &artifact, http.StatusCreated); result != nil {
					return result, nil, nil
				}
			case "update":
				if input.ArtifactNumber <= 0 {
					return utils.NewToolResultError("artifact_number must be positive"), nil, nil
				}
				if input.Type != "" {
					return utils.NewToolResultError("type is immutable; only supported for create"), nil, nil
				}
				if input.Name == nil && input.Body == nil {
					return utils.NewToolResultError("supply name, body, or both"), nil, nil
				}
				if input.Name != nil && strings.TrimSpace(*input.Name) == "" {
					return utils.NewToolResultError("name must not be empty"), nil, nil
				}
				body := struct {
					Name *string `json:"name,omitempty"`
					Body *string `json:"body,omitempty"`
				}{input.Name, input.Body}
				if result := issueArtifactRequest(ctx, deps, input.IssueArtifactCoordinates, input.ArtifactNumber, http.MethodPatch, nil, body, &artifact, http.StatusOK); result != nil {
					return result, nil, nil
				}
			case "delete":
				if input.ArtifactNumber <= 0 {
					return utils.NewToolResultError("artifact_number must be positive"), nil, nil
				}
				if input.Type != "" || input.Name != nil || input.Body != nil {
					return utils.NewToolResultError("type, name, and body are not supported for delete"), nil, nil
				}
				if result := issueArtifactRequest(ctx, deps, input.IssueArtifactCoordinates, input.ArtifactNumber, http.MethodDelete, nil, nil, nil, http.StatusNoContent); result != nil {
					return result, nil, nil
				}
				output := &IssueArtifactWriteOutput{Deleted: &DeleteIssueArtifactOutput{IssueNumber: input.IssueNumber, ArtifactNumber: input.ArtifactNumber, Message: "artifact deleted"}}
				return MarshalledTextResult(output), output, nil
			default:
				return utils.NewToolResultError("method must be create, update, or delete"), nil, nil
			}
			if err := validateIssueArtifact(&artifact, input.ArtifactNumber); err != nil {
				return utils.NewToolResultErrorFromErr("invalid artifact response", err), nil, nil
			}
			sanitizeIssueArtifact(&artifact)
			output := &IssueArtifactWriteOutput{Artifact: &artifact}
			result := attachRepoVisibilityIFCLabelLazy(ctx, deps, input.Owner, input.Repo, MarshalledTextResult(output), ifc.LabelRepoUserContent)
			return result, output, nil
		})
}

// IssueArtifactRead lists issue artifacts or gets one artifact, including bodies.
func IssueArtifactRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	return newIssueArtifactTool(t, "issue_artifact_read", "Read issue artifacts",
		"List or get GitHub issue artifacts including their bodies. Get requires artifact_number (issue-scoped, not its global ID). List accepts an optional type filter and page/perPage; request successive pages until a page is shorter than perPage because no Link headers are provided.",
		true, false, []inventory.SchemaEnum{{Path: "method", Values: []string{"list", "get"}}, {Path: "type", Values: issueArtifactTypes}},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input IssueArtifactReadInput) (*mcp.CallToolResult, *IssueArtifactReadOutput, error) {
			switch input.Method {
			case "get":
				if input.ArtifactNumber <= 0 {
					return utils.NewToolResultError("artifact_number must be positive"), nil, nil
				}
				if input.Type != "" || input.Page != 0 || input.PerPage != 0 {
					return utils.NewToolResultError("type, page, and perPage are only supported for list"), nil, nil
				}
				var response struct {
					Artifact *IssueArtifact `json:"artifact"`
				}
				if result := issueArtifactRequest(ctx, deps, input.IssueArtifactCoordinates, input.ArtifactNumber, http.MethodGet, nil, nil, &response, http.StatusOK); result != nil {
					return result, nil, nil
				}
				if err := validateIssueArtifact(response.Artifact, input.ArtifactNumber); err != nil {
					return utils.NewToolResultErrorFromErr("invalid artifact response", err), nil, nil
				}
				sanitizeIssueArtifact(response.Artifact)
				output := &IssueArtifactReadOutput{Artifact: response.Artifact}
				result := attachRepoVisibilityIFCLabelLazy(ctx, deps, input.Owner, input.Repo, MarshalledTextResult(output), ifc.LabelRepoUserContent)
				return result, output, nil
			case "list":
				if input.ArtifactNumber != 0 {
					return utils.NewToolResultError("artifact_number is only supported for get"), nil, nil
				}
				if input.Type != "" && !slices.Contains(issueArtifactTypes, input.Type) {
					return utils.NewToolResultError("unsupported artifact type"), nil, nil
				}
				if input.Page < 0 || input.PerPage < 0 || input.PerPage > 100 {
					return utils.NewToolResultError("page must be positive and perPage must be between 1 and 100"), nil, nil
				}
				if input.Page == 0 {
					input.Page = 1
				}
				if input.PerPage == 0 {
					input.PerPage = 30
				}
				query := url.Values{"page": {strconv.Itoa(input.Page)}, "per_page": {strconv.Itoa(input.PerPage)}}
				if input.Type != "" {
					query.Set("type", input.Type)
				}
				var artifacts []IssueArtifact
				if result := issueArtifactRequest(ctx, deps, input.IssueArtifactCoordinates, 0, http.MethodGet, query, nil, &artifacts, http.StatusOK); result != nil {
					return result, nil, nil
				}
				if artifacts == nil || len(artifacts) > input.PerPage {
					return utils.NewToolResultError("invalid artifact list response"), nil, nil
				}
				for i := range artifacts {
					if err := validateIssueArtifact(&artifacts[i], 0); err != nil {
						return utils.NewToolResultErrorFromErr("invalid artifact list response", err), nil, nil
					}
					sanitizeIssueArtifact(&artifacts[i])
				}
				output := &IssueArtifactReadOutput{List: &ListIssueArtifactsOutput{Artifacts: artifacts, Page: input.Page, PerPage: input.PerPage}}
				result := attachRepoVisibilityIFCLabelLazy(ctx, deps, input.Owner, input.Repo, MarshalledTextResult(output), ifc.LabelRepoUserContent)
				return result, output, nil
			default:
				return utils.NewToolResultError("method must be list or get"), nil, nil
			}
		})
}

func issueArtifactRequest(ctx context.Context, deps ToolDependencies, coordinates IssueArtifactCoordinates, number int, method string, query url.Values, body, output any, status int) *mcp.CallToolResult {
	for _, segment := range []string{coordinates.Owner, coordinates.Repo} {
		if strings.TrimSpace(segment) == "" || segment == "." || segment == ".." || strings.ContainsAny(segment, "/\\") {
			return utils.NewToolResultError("owner and repo must be nonempty repository path segments")
		}
	}
	if coordinates.IssueNumber <= 0 {
		return utils.NewToolResultError("issue_number must be positive")
	}
	// Artifact responses do not identify authors, so lockdown cannot establish
	// whether their content is safe to return.
	if deps.GetFlags(ctx).LockdownMode {
		return utils.NewToolResultError("issue artifacts are unavailable in lockdown mode because artifact authors cannot be verified")
	}
	client, err := deps.GetClient(ctx)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to get GitHub client", err)
	}
	path := fmt.Sprintf("repos/%s/%s/issues/%d/artifacts", url.PathEscape(coordinates.Owner), url.PathEscape(coordinates.Repo), coordinates.IssueNumber)
	if number > 0 {
		path += "/" + strconv.Itoa(number)
	}
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	request, err := client.NewRequest(ctx, method, path, body)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to create issue artifact request", err)
	}
	response, err := client.Do(request, output)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to "+strings.ToLower(method)+" issue artifact", response, err)
	}
	if response.StatusCode != status {
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "unexpected issue artifact response", response, nil)
	}
	return nil
}

func validateIssueArtifact(artifact *IssueArtifact, number int) error {
	if artifact == nil || artifact.ID <= 0 || artifact.Number <= 0 || artifact.Type == "" || strings.TrimSpace(artifact.Name) == "" {
		return fmt.Errorf("response omitted required artifact metadata")
	}
	if number > 0 && artifact.Number != number {
		return fmt.Errorf("response artifact number %d does not match requested number %d", artifact.Number, number)
	}
	return nil
}

func sanitizeIssueArtifact(artifact *IssueArtifact) {
	artifact.Name = sanitize.PlainText(artifact.Name)
	artifact.Body = sanitize.Content(artifact.Body)
}
