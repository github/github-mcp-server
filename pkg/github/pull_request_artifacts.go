package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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

const (
	deletePullRequestArtifactConfirmationID    = "delete_pull_request_artifact_confirmation"
	deletePullRequestArtifactConfirmationField = "artifact_target"
	deletePullRequestArtifactConfirmationTTL   = 10 * time.Minute
)

type deletePullRequestArtifactState struct {
	PullRequestArtifactCoordinates
	Operation    string `json:"operation"`
	Name         string `json:"name"`
	RepositoryID int64  `json:"repository_id"`
	ExpiresAt    int64  `json:"expires_at"`
}

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
	for _, field := range []string{"owner", "repo", "name", "name_key"} {
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
		"List GitHub pull request artifact versions including their bodies. Names are sanitized for display; use the returned name_key, not name, for subsequent writes. commit_oid defaults to the current PR head; use all or an explicit commit OID to change the selection. Returns one page; request next_page when present. Repository access and the repository's pull_request_artifacts_api rollout are enforced by GitHub. Unavailable in lockdown mode.",
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
			response, repository, failure := pullRequestArtifactRequest(ctx, deps, input.PullRequestArtifactCoordinates, "", http.MethodGet, query, nil, &artifacts, 0, http.StatusOK)
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
		"Upsert or delete a GitHub pull request artifact using exactly one of name (the exact original name) or name_key (the lossless key returned by reads or writes), not its number or global ID. Never use a sanitized display name to address an existing artifact. Upsert requires body and optionally accepts commit_oid; creates a new artifact or replaces the named artifact's content. Delete requires explicit user confirmation through form elicitation with MCP protocol 2026-07-28 or later, and does not accept body or commit_oid. Writes are not automatically retried. Repository access and the repository's pull_request_artifacts_api rollout are enforced by GitHub. Unavailable in lockdown mode.",
		false, []string{"upsert", "delete"},
		func(ctx context.Context, deps ToolDependencies, req *mcp.CallToolRequest, input PullRequestArtifactWriteInput) (*mcp.CallToolResult, *PullRequestArtifactWriteOutput, error) {
			if input.NameKey != "" {
				if input.Name != "" {
					return utils.NewToolResultError("provide exactly one of name or name_key"), nil, nil
				}
				name, err := base64.RawURLEncoding.DecodeString(input.NameKey)
				if err != nil || !utf8.Valid(name) || base64.RawURLEncoding.EncodeToString(name) != input.NameKey {
					return utils.NewToolResultError("name_key must be a valid lossless key returned by artifact reads or writes"), nil, nil
				}
				input.Name = string(name)
			}
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
				_, repository, failure := pullRequestArtifactRequest(ctx, deps, input.PullRequestArtifactCoordinates, input.Name, http.MethodPut, nil, body, &artifact, 0, http.StatusOK, http.StatusCreated)
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
				repositoryID, confirmation := confirmPullRequestArtifactDeletion(ctx, deps, req, input)
				if confirmation != nil {
					return confirmation, nil, nil
				}
				if _, _, failure := pullRequestArtifactRequest(ctx, deps, input.PullRequestArtifactCoordinates, input.Name, http.MethodDelete, nil, nil, nil, repositoryID, http.StatusNoContent); failure != nil {
					return failure, nil, nil
				}
				output := &PullRequestArtifactWriteOutput{Deleted: &DeletePullRequestArtifactOutput{
					PullNumber: input.PullNumber, Name: sanitize.PlainText(input.Name),
					NameKey: base64.RawURLEncoding.EncodeToString([]byte(input.Name)), Message: "artifact deleted",
				}}
				return MarshalledTextResult(output), output, nil
			default:
				return utils.NewToolResultError("method must be upsert or delete"), nil, nil
			}
		})
}

func confirmPullRequestArtifactDeletion(ctx context.Context, deps ToolDependencies, req *mcp.CallToolRequest, input PullRequestArtifactWriteInput) (int64, *mcp.CallToolResult) {
	if failure := validatePullRequestArtifactCoordinates(ctx, deps, input.PullRequestArtifactCoordinates); failure != nil {
		return 0, failure
	}
	if req == nil || req.Params == nil || req.ProtocolVersion() < inventory.ProtocolVersionMultiRoundTrip {
		return 0, utils.NewToolResultError("Artifact deletion requires MCP protocol version 2026-07-28 or later and form elicitation.")
	}
	capabilities := req.ClientCapabilities()
	if capabilities == nil || capabilities.Elicitation == nil || (capabilities.Elicitation.Form == nil && capabilities.Elicitation.URL != nil) {
		return 0, utils.NewToolResultError("Artifact deletion requires client support for form elicitation.")
	}
	sealer := requestStateSealerFromDeps(deps)
	if sealer == nil {
		return 0, utils.NewToolResultError("Artifact deletion is unavailable because request-state protection is not configured.")
	}
	target := fmt.Sprintf("%s/%s#%d artifact %q", input.Owner, input.Repo, input.PullNumber, input.Name)
	response, ok := req.Params.InputResponses[deletePullRequestArtifactConfirmationID]
	if !ok {
		_, repository, failure := resolvePullRequestArtifactRepository(ctx, deps, input.PullRequestArtifactCoordinates)
		if failure != nil {
			return 0, failure
		}
		state, err := json.Marshal(deletePullRequestArtifactState{
			PullRequestArtifactCoordinates: input.PullRequestArtifactCoordinates,
			Operation:                      "pull_request_artifact_write:delete",
			Name:                           input.Name, RepositoryID: repository.GetID(),
			ExpiresAt: time.Now().Add(deletePullRequestArtifactConfirmationTTL).Unix(),
		})
		if err != nil {
			return 0, utils.NewToolResultErrorFromErr("failed to marshal artifact deletion state", err)
		}
		requestState, err := sealer.Seal(ctx, state)
		if err != nil {
			return 0, utils.NewToolResultErrorFromErr("failed to seal artifact deletion state", err)
		}
		return 0, &mcp.CallToolResult{
			InputRequests: mcp.InputRequestMap{
				deletePullRequestArtifactConfirmationID: &mcp.ElicitParams{
					Mode:    "form",
					Message: fmt.Sprintf("Type %q to confirm deletion of this pull request artifact.", target),
					RequestedSchema: &jsonschema.Schema{
						Type: "object",
						Properties: map[string]*jsonschema.Schema{
							deletePullRequestArtifactConfirmationField: {
								Type: "string", Title: "Artifact target",
								Description: fmt.Sprintf("Enter %s exactly to confirm deletion", target),
							},
						},
						Required: []string{deletePullRequestArtifactConfirmationField},
					},
				},
			},
			RequestState: requestState,
		}
	}
	stateJSON, err := sealer.Open(req.Params.RequestState)
	if err != nil {
		return 0, utils.NewToolResultError("Artifact deletion confirmation state was missing or invalid. The artifact was not deleted.")
	}
	var state deletePullRequestArtifactState
	if err := json.Unmarshal(stateJSON, &state); err != nil {
		return 0, utils.NewToolResultError("Artifact deletion confirmation state was invalid. The artifact was not deleted.")
	}
	if state.Operation != "pull_request_artifact_write:delete" || state.PullRequestArtifactCoordinates != input.PullRequestArtifactCoordinates || state.Name != input.Name || state.RepositoryID <= 0 {
		return 0, utils.NewToolResultError("Artifact deletion target changed or confirmation state was invalid. The artifact was not deleted.")
	}
	if state.ExpiresAt <= time.Now().Unix() {
		return 0, utils.NewToolResultError("Artifact deletion confirmation expired. The artifact was not deleted.")
	}
	confirmation, ok := response.(*mcp.ElicitResult)
	if !ok || confirmation == nil || confirmation.Action != "accept" {
		return 0, utils.NewToolResultError("Artifact deletion was not confirmed. The artifact was not deleted.")
	}
	if confirmedTarget, ok := confirmation.Content[deletePullRequestArtifactConfirmationField].(string); !ok || confirmedTarget != target {
		return 0, utils.NewToolResultError("Artifact target confirmation did not match. The artifact was not deleted.")
	}
	return state.RepositoryID, nil
}

func validatePullRequestArtifactCoordinates(ctx context.Context, deps ToolDependencies, coordinates PullRequestArtifactCoordinates) *mcp.CallToolResult {
	for _, segment := range []string{coordinates.Owner, coordinates.Repo} {
		if strings.TrimSpace(segment) == "" || segment == "." || segment == ".." || strings.ContainsAny(segment, "/\\") {
			return utils.NewToolResultError("owner and repo must be nonempty repository path segments")
		}
	}
	if coordinates.PullNumber <= 0 {
		return utils.NewToolResultError("pull_number must be positive")
	}
	if deps.GetFlags(ctx).LockdownMode {
		return utils.NewToolResultError("pull request artifacts are unavailable in lockdown mode")
	}
	return nil
}

func resolvePullRequestArtifactRepository(ctx context.Context, deps ToolDependencies, coordinates PullRequestArtifactCoordinates) (*gogithub.Client, *gogithub.Repository, *mcp.CallToolResult) {
	if failure := validatePullRequestArtifactCoordinates(ctx, deps, coordinates); failure != nil {
		return nil, nil, failure
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
	return client, repository, nil
}

func pullRequestArtifactRequest(ctx context.Context, deps ToolDependencies, coordinates PullRequestArtifactCoordinates, name, method string, query url.Values, body, output any, expectedRepositoryID int64, statuses ...int) (*gogithub.Response, *gogithub.Repository, *mcp.CallToolResult) {
	client, repository, failure := resolvePullRequestArtifactRepository(ctx, deps, coordinates)
	if failure != nil {
		return nil, nil, failure
	}
	if expectedRepositoryID != 0 && repository.GetID() != expectedRepositoryID {
		return nil, nil, utils.NewToolResultError("Repository identity changed after artifact deletion confirmation was requested. The artifact was not deleted.")
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
	artifact.NameKey = base64.RawURLEncoding.EncodeToString([]byte(artifact.Name))
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
