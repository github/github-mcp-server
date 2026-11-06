package github

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
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
	schema := forbidOmittedNulls(repositoryOutputSchema[AssignCopilotToIssueOutput]())
	schema.Properties["issue_number"].Description = "The assigned issue's number."
	schema.Properties["issue_url"].Description = "Canonical browser URL of the assigned issue."
	schema.Properties["message"].Description = "Assignment outcome or tool error message."
	schema.Properties["note"].Description = "Additional status detail when Copilot has not created a pull request yet."
	schema.Properties["pull_request"].Properties["state"].Enum = []any{"OPEN", "CLOSED", "MERGED"}
	schema.Properties["pull_request"].Properties["url"].Description = "Canonical browser URL of the linked pull request."
	return schema
}

func assignCopilotToIssueWithIntentOutputSchema() *jsonschema.Schema {
	schema := forbidOmittedNulls(repositoryOutputSchema[AssignCopilotToIssueWithIntentOutput]())
	schema.Properties["issue_number"].Description = "The assigned issue's number."
	schema.Properties["issue_url"].Description = "Canonical browser URL of the assigned issue."
	schema.Properties["message"].Description = "Assignment outcome or tool error message."
	schema.Properties["note"].Description = "Additional status detail when Copilot has not created a pull request yet."
	schema.Properties["pull_request"].Properties["state"].Enum = []any{"OPEN", "CLOSED", "MERGED"}
	schema.Properties["pull_request"].Properties["url"].Description = "Canonical browser URL of the linked pull request."
	return schema
}

type CopilotReviewOutput struct {
	Status string `json:"status"`
}

func copilotReviewOutputSchema() *jsonschema.Schema {
	schema := forbidOmittedNulls(repositoryOutputSchema[CopilotReviewOutput]())
	schema.Properties["status"].Description = "Outcome of the Copilot review request; requested on success."
	return schema
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
	delete(args, "_compat_method")
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
		return nil, &inventory.ToolInputError{Message: fmt.Sprintf("unknown method: %s", method)}
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
	Method      string                  `json:"method"`
	Labels      *UIGetLabelsOutput      `json:"labels,omitempty"`
	Assignees   *UIGetAssigneesOutput   `json:"assignees,omitempty"`
	Milestones  *UIGetMilestonesOutput  `json:"milestones,omitempty"`
	IssueTypes  *[]*IssueTypeOutput     `json:"issue_types,omitempty"`
	Branches    *UIGetBranchesOutput    `json:"branches,omitempty"`
	IssueFields *UIGetIssueFieldsOutput `json:"issue_fields,omitempty"`
	Reviewers   *UIGetReviewersOutput   `json:"reviewers,omitempty"`
}

func decodeUIGetOutput(method string, content []byte) (*UIGetOutput, error) {
	output := &UIGetOutput{Method: method}
	switch method {
	case "labels":
		output.Labels = new(UIGetLabelsOutput)
		err := json.Unmarshal(content, output.Labels)
		return output, err
	case "assignees":
		output.Assignees = new(UIGetAssigneesOutput)
		err := json.Unmarshal(content, output.Assignees)
		return output, err
	case "milestones":
		output.Milestones = new(UIGetMilestonesOutput)
		err := json.Unmarshal(content, output.Milestones)
		return output, err
	case "issue_types":
		var issueTypes []*IssueTypeOutput
		err := json.Unmarshal(content, &issueTypes)
		output.IssueTypes = &issueTypes
		return output, err
	case "branches":
		output.Branches = new(UIGetBranchesOutput)
		err := json.Unmarshal(content, output.Branches)
		return output, err
	case "issue_fields":
		output.IssueFields = new(UIGetIssueFieldsOutput)
		err := json.Unmarshal(content, output.IssueFields)
		return output, err
	case "reviewers":
		output.Reviewers = new(UIGetReviewersOutput)
		err := json.Unmarshal(content, output.Reviewers)
		return output, err
	default:
		return nil, fmt.Errorf("unknown ui_get output method: %s", method)
	}
}

func uiGetOutputSchema() *jsonschema.Schema {
	schema := forbidOmittedNulls(repositoryOutputSchema[UIGetOutput]())
	schema.Properties["method"].Enum = []any{"labels", "assignees", "milestones", "issue_types", "branches", "issue_fields", "reviewers"}
	schema.Properties["method"].Description = "The method that produced the corresponding typed output."
	schema.Required = []string{"method"}
	schema.Properties["issue_types"].Type = ""
	schema.Properties["issue_types"].Types = []string{"array", "null"}
	schema.Properties["issue_types"].Items = repositoryOutputSchema[[]*IssueTypeOutput]().Items
	schema.Properties["labels"].Properties["totalCount"].Description = "Total number of matching labels."
	schema.Properties["labels"].Properties["has_more"].Description = "True when results were truncated at the pagination limit."
	schema.Properties["assignees"].Properties["totalCount"].Description = "Total number of matching assignees."
	schema.Properties["assignees"].Properties["has_more"].Description = "True when results were truncated at the pagination limit."
	schema.Properties["milestones"].Properties["totalCount"].Description = "Total number of matching milestones."
	schema.Properties["milestones"].Properties["has_more"].Description = "True when results were truncated at the pagination limit."
	schema.Properties["reviewers"].Properties["totalCount"].Description = "Total number of matching users and teams."
	typedArrays := map[string][]string{
		"labels":       {"labels"},
		"assignees":    {"assignees"},
		"milestones":   {"milestones"},
		"branches":     {"branches"},
		"issue_fields": {"fields"},
		"reviewers":    {"users", "teams"},
	}
	for property, arrays := range typedArrays {
		nested := schema.Properties[property]
		for _, name := range arrays {
			array := nested.Properties[name]
			array.Type, array.Types = "array", nil
		}
	}
	schema.Properties["milestones"].Properties["milestones"].Items.Properties["number"].Description = "Milestone number."
	schema.Properties["milestones"].Properties["milestones"].Items.Properties["open_issues"].Description = "Number of open issues assigned to the milestone."
	schema.Properties["milestones"].Properties["milestones"].Items.Properties["due_on"].Description = "Due date in YYYY-MM-DD form, or empty when no due date is set."
	schema.Properties["milestones"].Properties["milestones"].Items.Properties["state"].Enum = []any{"open", "closed"}
	schema.Properties["issue_fields"].Properties["fields"].Items.Properties["data_type"].Enum = []any{"text", "number", "date", "single_select"}
	schema.Properties["issue_fields"].Properties["totalCount"].Description = "Number of supported issue fields returned."
	schema.Properties["branches"].Properties["totalCount"].Description = "Number of branches returned."
	schema.Properties["branches"].Properties["has_more"].Description = "True when results were truncated at the pagination limit."
	constrainMethodOutput(schema, map[string]string{
		"labels": "labels", "assignees": "assignees", "milestones": "milestones",
		"issue_types": "issue_types", "branches": "branches", "issue_fields": "issue_fields", "reviewers": "reviewers",
	})
	return schema
}

func uiGetTypedResult(method string, result *mcp.CallToolResult) (*mcp.CallToolResult, *UIGetOutput, error) {
	if result == nil {
		return nil, nil, fmt.Errorf("ui_get %s returned no result", method)
	}
	if result.IsError {
		return result, &UIGetOutput{Method: uiGetSchemaMethod(method)}, nil
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

func uiGetSchemaMethod(method string) string {
	switch method {
	case "labels", "assignees", "milestones", "issue_types", "branches", "issue_fields", "reviewers":
		return method
	default:
		return "labels"
	}
}
