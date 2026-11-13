package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
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
	gogithub "github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newPullRequestArtifactTool[In, Out any](t translations.TranslationHelperFunc, name, title, description string, readOnly bool, methods []string,
	handler func(context.Context, ToolDependencies, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error),
) inventory.ServerTool {
	key := "TOOL_" + strings.ToUpper(name)
	schema := issueDTOschema[In]()
	schema.Properties["method"] = inventory.EnumSchema(methods...)
	for _, field := range []string{"pull_number", "page", "perPage"} {
		if property := schema.Properties[field]; property != nil {
			property.Minimum = new(1.0)
		}
	}
	for _, field := range []string{"owner", "repo", "name"} {
		if property := schema.Properties[field]; property != nil {
			property.MinLength = new(1)
		}
	}
	for _, field := range []string{"body", "commit_oid"} {
		if property := schema.Properties[field]; property != nil {
			property.Type, property.Types = "string", nil
		}
	}
	if property := schema.Properties["perPage"]; property != nil {
		property.Maximum = new(100.0)
	}
	st := NewToolWithSchemaOptions(ToolsetMetadataPullRequests, mcp.Tool{
		Name: name, Description: t(key+"_DESCRIPTION", description),
		InputSchema: schema,
		Annotations: &mcp.ToolAnnotations{
			Title: t(key+"_USER_TITLE", title), ReadOnlyHint: readOnly,
			DestructiveHint: new(!readOnly),
		},
	}, scopes.RequireAll(scopes.Repo), inventory.TypedSchemaOptions{
		PreserveHandlerContent: true,
		Output: &jsonschema.ForOptions{TypeSchemas: map[reflect.Type]*jsonschema.Schema{
			reflect.TypeFor[json.RawMessage](): {Types: []string{"string", "integer"}},
		}},
	}, handler)
	st.FeatureRule = featureEnabledRule(FeatureFlagPullRequestArtifacts)
	return st
}

// PullRequestArtifactRead lists pull request artifact versions, including bodies.
func PullRequestArtifactRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	return newPullRequestArtifactTool(t, "pull_request_artifact_read", "Read pull request artifacts",
		"List GitHub pull request artifact versions including their bodies. commit_oid defaults to the current PR head; use all or an explicit commit OID to change the selection. Returns one page; request next_page when present. Repository access and the repository's pull_request_artifacts_api rollout are enforced by GitHub. Unavailable in lockdown mode.",
		true, []string{"list"},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input PullRequestArtifactReadInput) (*mcp.CallToolResult, *PullRequestArtifactReadOutput, error) {
			if input.Method != "list" {
				return utils.NewToolResultError("method must be list"), nil, nil
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
			if input.CommitOID != "" {
				query.Set("commit_oid", input.CommitOID)
			}
			var artifacts []PullRequestArtifact
			response, repository, failure := pullRequestArtifactRequest(ctx, deps, input.PullRequestArtifactCoordinates, "", http.MethodGet, query, nil, &artifacts, http.StatusOK)
			if failure != nil {
				return failure, nil, nil
			}
			if artifacts == nil || len(artifacts) > input.PerPage {
				return utils.NewToolResultError("invalid pull request artifact list response"), nil, nil
			}
			for i := range artifacts {
				if err := validatePullRequestArtifact(&artifacts[i]); err != nil {
					return utils.NewToolResultErrorFromErr("invalid pull request artifact list response", err), nil, nil
				}
				sanitizePullRequestArtifact(&artifacts[i])
			}
			output := &PullRequestArtifactReadOutput{List: &ListPullRequestArtifactsOutput{
				Artifacts: artifacts, Page: input.Page, PerPage: input.PerPage, NextPage: response.NextPage,
			}}
			return pullRequestArtifactResult(ctx, deps, repository, output), output, nil
		})
}

// PullRequestArtifactWrite upserts or deletes a pull request artifact by name.
func PullRequestArtifactWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return newPullRequestArtifactTool(t, "pull_request_artifact_write", "Write pull request artifacts",
		"Upsert or delete a GitHub pull request artifact by name, not its number or global ID. Upsert requires body and optionally accepts commit_oid; creates a new artifact or replaces the named artifact's content. Delete does not accept body or commit_oid. Writes are not automatically retried. Repository access and the repository's pull_request_artifacts_api rollout are enforced by GitHub. Unavailable in lockdown mode.",
		false, []string{"upsert", "delete"},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input PullRequestArtifactWriteInput) (*mcp.CallToolResult, *PullRequestArtifactWriteOutput, error) {
			if strings.TrimSpace(input.Name) == "" || input.Name == "." || input.Name == ".." || strings.ContainsAny(input.Name, "/\\") {
				return utils.NewToolResultError("name must be a nonempty artifact path segment"), nil, nil
			}
			switch input.Method {
			case "upsert":
				if input.Body == nil {
					return utils.NewToolResultError("body is required for upsert"), nil, nil
				}
				body := struct {
					Body      string  `json:"body"`
					CommitOID *string `json:"commit_oid,omitempty"`
				}{*input.Body, input.CommitOID}
				var artifact PullRequestArtifact
				_, repository, failure := pullRequestArtifactRequest(ctx, deps, input.PullRequestArtifactCoordinates, input.Name, http.MethodPut, nil, body, &artifact, http.StatusOK, http.StatusCreated)
				if failure != nil {
					return failure, nil, nil
				}
				if err := validatePullRequestArtifact(&artifact); err != nil {
					return utils.NewToolResultErrorFromErr("invalid pull request artifact response", err), nil, nil
				}
				if artifact.Name != input.Name {
					return utils.NewToolResultError("response artifact name does not match requested name"), nil, nil
				}
				sanitizePullRequestArtifact(&artifact)
				output := &PullRequestArtifactWriteOutput{Artifact: &artifact}
				return pullRequestArtifactResult(ctx, deps, repository, output), output, nil
			case "delete":
				if input.Body != nil || input.CommitOID != nil {
					return utils.NewToolResultError("body and commit_oid are not supported for delete"), nil, nil
				}
				if _, _, failure := pullRequestArtifactRequest(ctx, deps, input.PullRequestArtifactCoordinates, input.Name, http.MethodDelete, nil, nil, nil, http.StatusNoContent); failure != nil {
					return failure, nil, nil
				}
				output := &PullRequestArtifactWriteOutput{Deleted: &DeletePullRequestArtifactOutput{
					PullNumber: input.PullNumber, Name: sanitize.PlainText(input.Name), Message: "artifact deleted",
				}}
				return MarshalledTextResult(output), output, nil
			default:
				return utils.NewToolResultError("method must be upsert or delete"), nil, nil
			}
		})
}

func pullRequestArtifactRequest(ctx context.Context, deps ToolDependencies, coordinates PullRequestArtifactCoordinates, name, method string, query url.Values, body, output any, statuses ...int) (*gogithub.Response, *gogithub.Repository, *mcp.CallToolResult) {
	for _, segment := range []string{coordinates.Owner, coordinates.Repo} {
		if strings.TrimSpace(segment) == "" || segment == "." || segment == ".." || strings.ContainsAny(segment, "/\\") {
			return nil, nil, utils.NewToolResultError("owner and repo must be nonempty repository path segments")
		}
	}
	if coordinates.PullNumber <= 0 {
		return nil, nil, utils.NewToolResultError("pull_number must be positive")
	}
	if deps.GetFlags(ctx).LockdownMode {
		return nil, nil, utils.NewToolResultError("pull request artifacts are unavailable in lockdown mode")
	}
	client, err := deps.GetClient(ctx)
	if err != nil {
		return nil, nil, utils.NewToolResultErrorFromErr("failed to get GitHub client", err)
	}
	repository, repoResponse, err := client.Repositories.Get(ctx, url.PathEscape(coordinates.Owner), url.PathEscape(coordinates.Repo))
	if repoResponse != nil {
		defer func() { _ = repoResponse.Body.Close() }()
	}
	if err != nil {
		return nil, nil, ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to resolve pull request artifact repository", repoResponse, err)
	}
	if repository == nil || repository.GetID() <= 0 {
		return nil, nil, utils.NewToolResultError("repository response omitted a positive repository ID")
	}
	path := fmt.Sprintf("repositories/%d/pulls/%d/artifacts", repository.GetID(), coordinates.PullNumber)
	if name != "" {
		path += "/" + url.PathEscape(name)
	}
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	request, err := client.NewRequest(ctx, method, path, body)
	if err != nil {
		return nil, nil, utils.NewToolResultErrorFromErr("failed to create pull request artifact request", err)
	}
	response, err := client.Do(request, output)
	if response != nil {
		defer func() { _ = response.Body.Close() }()
	}
	if err != nil {
		message := "failed to " + strings.ToLower(method) + " pull request artifact"
		if response != nil && response.StatusCode == http.StatusTooManyRequests {
			if seconds, parseErr := strconv.Atoi(response.Header.Get("Retry-After")); parseErr == nil && seconds > 0 {
				message += fmt.Sprintf("; Retry after %d seconds", seconds)
			}
		}
		return nil, nil, ghErrors.NewGitHubAPIErrorResponse(ctx, message, response, err)
	}
	if !slices.Contains(statuses, response.StatusCode) {
		return nil, nil, ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "unexpected pull request artifact response", response, nil)
	}
	return response, repository, nil
}

func validatePullRequestArtifact(artifact *PullRequestArtifact) error {
	if artifact.Number <= 0 || strings.TrimSpace(artifact.Name) == "" {
		return fmt.Errorf("response omitted required artifact metadata")
	}
	// Storage IDs are opaque; preserve their wire representation rather than
	// interpreting them as issue artifact IDs or accepting arbitrary JSON.
	var id string
	if err := json.Unmarshal(artifact.ID, &id); err == nil && id != "" {
		return nil
	}
	var number uint64
	if err := json.Unmarshal(artifact.ID, &number); err == nil && number > 0 {
		return nil
	}
	return fmt.Errorf("response omitted a valid artifact ID")
}

func sanitizePullRequestArtifact(artifact *PullRequestArtifact) {
	artifact.Name = sanitize.PlainText(artifact.Name)
	artifact.Body = sanitize.Content(artifact.Body)
	if artifact.Creator != nil {
		artifact.Creator.Login = sanitize.PlainText(artifact.Creator.Login)
	}
}

func pullRequestArtifactResult(ctx context.Context, deps ToolDependencies, repository *gogithub.Repository, output any) *mcp.CallToolResult {
	result := MarshalledTextResult(output)
	if shouldAttachIFCLabel(ctx, deps, result) && repository.Private != nil {
		setIFCLabel(result, ifc.LabelRepoUserContent(*repository.Private))
	}
	return result
}
