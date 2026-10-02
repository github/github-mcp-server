package github

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/go-viper/mapstructure/v2"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type CopilotPullRequestOutput struct {
	Number int    `json:"number"`
	State  string `json:"state"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}

type AssignCopilotToIssueOutput struct {
	IssueNumber int                       `json:"issue_number"`
	IssueURL    string                    `json:"issue_url"`
	Message     string                    `json:"message"`
	Note        string                    `json:"note,omitempty"`
	Owner       string                    `json:"owner"`
	PullRequest *CopilotPullRequestOutput `json:"pull_request,omitempty"`
	Repo        string                    `json:"repo"`
}

type AssignCopilotToIssueWithIntentOutput struct {
	IsSuggestion bool                      `json:"is_suggestion"`
	IssueNumber  int                       `json:"issue_number"`
	IssueURL     string                    `json:"issue_url"`
	Message      string                    `json:"message"`
	Note         string                    `json:"note,omitempty"`
	Owner        string                    `json:"owner"`
	PullRequest  *CopilotPullRequestOutput `json:"pull_request,omitempty"`
	Repo         string                    `json:"repo"`
}

func assignCopilotToIssueOutputSchema() *jsonschema.Schema {
	return forbidOmittedNulls(repositoryOutputSchema[AssignCopilotToIssueOutput]())
}

func assignCopilotToIssueWithIntentOutputSchema() *jsonschema.Schema {
	return forbidOmittedNulls(repositoryOutputSchema[AssignCopilotToIssueWithIntentOutput]())
}

type CopilotReviewOutput struct{}

func (CopilotReviewOutput) MarshalJSON() ([]byte, error) {
	return []byte("null"), nil
}

func normalizeAssignCopilotToIssueArguments(raw json.RawMessage) (json.RawMessage, error) {
	args, err := rawArgumentMap(raw)
	if err != nil || args == nil {
		return raw, err
	}
	var input struct {
		Owner              string `mapstructure:"owner"`
		Repo               string `mapstructure:"repo"`
		IssueNumber        int32  `mapstructure:"issue_number"`
		BaseRef            string `mapstructure:"base_ref"`
		CustomInstructions string `mapstructure:"custom_instructions"`
	}
	if err := mapstructure.WeakDecode(args, &input); err != nil {
		return nil, &inventory.ToolInputError{Message: err.Error()}
	}
	return json.Marshal(map[string]any{
		"owner":               input.Owner,
		"repo":                input.Repo,
		"issue_number":        input.IssueNumber,
		"base_ref":            input.BaseRef,
		"custom_instructions": input.CustomInstructions,
	})
}

func normalizeAssignCopilotToIssueWithIntentArguments(raw json.RawMessage) (json.RawMessage, error) {
	args, err := rawArgumentMap(raw)
	if err != nil || args == nil {
		return raw, err
	}
	if _, ok := args["is_suggestion"]; !ok {
		return nil, &inventory.ToolInputError{Message: "is_suggestion is required"}
	}
	var input struct {
		Owner              string `mapstructure:"owner"`
		Repo               string `mapstructure:"repo"`
		IssueNumber        int32  `mapstructure:"issue_number"`
		BaseRef            string `mapstructure:"base_ref"`
		CustomInstructions string `mapstructure:"custom_instructions"`
		Rationale          string `mapstructure:"rationale"`
		Confidence         string `mapstructure:"confidence"`
		IsSuggestion       bool   `mapstructure:"is_suggestion"`
	}
	if err := mapstructure.WeakDecode(args, &input); err != nil {
		return nil, &inventory.ToolInputError{Message: err.Error()}
	}
	rationale := strings.TrimSpace(input.Rationale)
	if rationale == "" || len([]rune(rationale)) > 280 {
		rationale = "compatibility"
	}
	confidence := normalizeConfidence(input.Confidence)
	switch confidence {
	case "LOW", "MEDIUM", "HIGH":
	default:
		confidence = "LOW"
	}
	return json.Marshal(map[string]any{
		"owner":               input.Owner,
		"repo":                input.Repo,
		"issue_number":        input.IssueNumber,
		"base_ref":            input.BaseRef,
		"custom_instructions": input.CustomInstructions,
		"rationale":           rationale,
		"_compat_rationale":   input.Rationale,
		"confidence":          confidence,
		"_compat_confidence":  input.Confidence,
		"is_suggestion":       input.IsSuggestion,
	})
}

func normalizeRequestCopilotReviewArguments(raw json.RawMessage) (json.RawMessage, error) {
	args, err := rawArgumentMap(raw)
	if err != nil || args == nil {
		return raw, err
	}
	owner, err := RequiredParam[string](args, "owner")
	if err != nil {
		return nil, &inventory.ToolInputError{Message: err.Error()}
	}
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return nil, &inventory.ToolInputError{Message: err.Error()}
	}
	pullNumber, err := RequiredInt(args, "pullNumber")
	if err != nil {
		return nil, &inventory.ToolInputError{Message: err.Error()}
	}
	return json.Marshal(map[string]any{"owner": owner, "repo": repo, "pullNumber": pullNumber})
}

func normalizeUIGetArguments(raw json.RawMessage) (json.RawMessage, error) {
	args, err := rawArgumentMap(raw)
	if err != nil || args == nil {
		return raw, err
	}
	method, err := RequiredParam[string](args, "method")
	if err != nil {
		return nil, &inventory.ToolInputError{Message: err.Error()}
	}
	if _, err := RequiredParam[string](args, "owner"); err != nil {
		return nil, &inventory.ToolInputError{Message: err.Error()}
	}
	switch method {
	case "labels", "assignees", "milestones", "branches", "issue_fields", "reviewers":
		if _, err := RequiredParam[string](args, "repo"); err != nil {
			if _, exists := args["repo"]; exists {
				return nil, &inventory.ToolInputError{Message: err.Error()}
			}
		}
	case "issue_types":
		if _, exists := args["repo"]; exists {
			if _, err := RequiredParam[string](args, "repo"); err != nil {
				args["repo"] = ""
			}
		}
	default:
		args["method"] = "labels"
		args["_compat_method"] = method
		if _, exists := args["repo"]; exists {
			if _, err := RequiredParam[string](args, "repo"); err != nil {
				args["repo"] = ""
			}
		}
	}
	return json.Marshal(args)
}

func rawArgumentMap(raw json.RawMessage) (map[string]any, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	return args, nil
}

type UIGetLabelOutput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

type UIGetAssigneeOutput struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

type UIGetMilestoneOutput struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       string `json:"state"`
	OpenIssues  int    `json:"open_issues"`
	DueOn       string `json:"due_on"`
}

type UIGetBranchesOutput struct {
	Branches   []MinimalBranch `json:"branches"`
	TotalCount int             `json:"totalCount"`
	HasMore    bool            `json:"has_more"`
}

type UIGetIssueFieldOptionOutput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

type UIGetIssueFieldOutput struct {
	ID          string                         `json:"id"`
	Name        string                         `json:"name"`
	DataType    string                         `json:"data_type"`
	Description string                         `json:"description"`
	Options     *[]UIGetIssueFieldOptionOutput `json:"options,omitempty"`
}

type UIGetReviewersOutput struct {
	Users      []UIGetAssigneeOutput `json:"users"`
	Teams      []UIGetReviewerTeam   `json:"teams"`
	TotalCount int                   `json:"totalCount"`
	HasMore    bool                  `json:"has_more"`
}

type UIGetReviewerTeam struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
	Org  string `json:"org"`
}

type UIGetLabelsOutput struct {
	Labels     []UIGetLabelOutput `json:"labels"`
	TotalCount int                `json:"totalCount"`
	HasMore    bool               `json:"has_more"`
}

type UIGetAssigneesOutput struct {
	Assignees  []UIGetAssigneeOutput `json:"assignees"`
	TotalCount int                   `json:"totalCount"`
	HasMore    bool                  `json:"has_more"`
}

type UIGetMilestonesOutput struct {
	Milestones []UIGetMilestoneOutput `json:"milestones"`
	TotalCount int                    `json:"totalCount"`
	HasMore    bool                   `json:"has_more"`
}

type UIGetIssueFieldsOutput struct {
	Fields     []UIGetIssueFieldOutput `json:"fields"`
	TotalCount int                     `json:"totalCount"`
}

type UIGetOutput struct {
	Method      string
	Labels      *UIGetLabelsOutput
	Assignees   *UIGetAssigneesOutput
	Milestones  *UIGetMilestonesOutput
	IssueTypes  []*IssueTypeOutput
	Branches    *UIGetBranchesOutput
	IssueFields *UIGetIssueFieldsOutput
	Reviewers   *UIGetReviewersOutput
}

func (out UIGetOutput) MarshalJSON() ([]byte, error) {
	switch out.Method {
	case "labels":
		return json.Marshal(out.Labels)
	case "assignees":
		return json.Marshal(out.Assignees)
	case "milestones":
		return json.Marshal(out.Milestones)
	case "issue_types":
		return json.Marshal(out.IssueTypes)
	case "branches":
		return json.Marshal(out.Branches)
	case "issue_fields":
		return json.Marshal(out.IssueFields)
	case "reviewers":
		return json.Marshal(out.Reviewers)
	default:
		return []byte("null"), nil
	}
}

func decodeUIGetOutput(method string, content []byte) (*UIGetOutput, error) {
	output := &UIGetOutput{Method: method}
	switch method {
	case "labels":
		output.Labels = new(UIGetLabelsOutput)
		return output, json.Unmarshal(content, output.Labels)
	case "assignees":
		output.Assignees = new(UIGetAssigneesOutput)
		return output, json.Unmarshal(content, output.Assignees)
	case "milestones":
		output.Milestones = new(UIGetMilestonesOutput)
		return output, json.Unmarshal(content, output.Milestones)
	case "issue_types":
		return output, json.Unmarshal(content, &output.IssueTypes)
	case "branches":
		output.Branches = new(UIGetBranchesOutput)
		return output, json.Unmarshal(content, output.Branches)
	case "issue_fields":
		output.IssueFields = new(UIGetIssueFieldsOutput)
		return output, json.Unmarshal(content, output.IssueFields)
	case "reviewers":
		output.Reviewers = new(UIGetReviewersOutput)
		return output, json.Unmarshal(content, output.Reviewers)
	default:
		return nil, fmt.Errorf("unknown ui_get output method: %s", method)
	}
}

func uiGetOutputSchema() *jsonschema.Schema {
	return actionsUnionSchema(
		&jsonschema.Schema{Type: "null"},
		forbidOmittedNulls(repositoryOutputSchema[UIGetLabelsOutput]()),
		forbidOmittedNulls(repositoryOutputSchema[UIGetAssigneesOutput]()),
		forbidOmittedNulls(repositoryOutputSchema[UIGetMilestonesOutput]()),
		forbidOmittedNulls(repositoryOutputSchema[[]*IssueTypeOutput]()),
		forbidOmittedNulls(repositoryOutputSchema[UIGetBranchesOutput]()),
		forbidOmittedNulls(repositoryOutputSchema[UIGetIssueFieldsOutput]()),
		forbidOmittedNulls(repositoryOutputSchema[UIGetReviewersOutput]()),
	)
}

func uiGetTypedResult(method string, result *mcp.CallToolResult) (*mcp.CallToolResult, *UIGetOutput, error) {
	if result == nil || result.IsError {
		return result, nil, nil
	}
	if len(result.Content) != 1 {
		return nil, nil, fmt.Errorf("ui_get %s returned %d content blocks; expected one", method, len(result.Content))
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		return nil, nil, fmt.Errorf("ui_get %s returned non-text content", method)
	}
	output, err := decodeUIGetOutput(method, []byte(text.Text))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode ui_get %s output: %w", method, err)
	}
	return result, output, nil
}
