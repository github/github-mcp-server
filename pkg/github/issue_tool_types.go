package github

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
)

type IssueMetadataInput struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo,omitempty"`
}

type IssueTypeOutput struct {
	ID          *int64     `json:"id,omitempty"`
	NodeID      *string    `json:"node_id,omitempty"`
	Name        *string    `json:"name,omitempty"`
	Description *string    `json:"description,omitempty"`
	Color       *string    `json:"color,omitempty"`
	IsEnabled   *bool      `json:"is_enabled,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

func issueTypeOutputs(types []*github.IssueType) []*IssueTypeOutput {
	if types == nil {
		return nil
	}
	output := make([]*IssueTypeOutput, len(types))
	for i, item := range types {
		if item != nil {
			output[i] = &IssueTypeOutput{
				ID: item.ID, NodeID: item.NodeID, Name: item.Name,
				Description: item.Description, Color: item.Color, IsEnabled: item.IsEnabled,
				CreatedAt: githubTimestampTime(item.CreatedAt), UpdatedAt: githubTimestampTime(item.UpdatedAt),
			}
		}
	}
	return output
}

type AddIssueCommentInput struct {
	Owner       string  `json:"owner"`
	Repo        string  `json:"repo"`
	IssueNumber int     `json:"issue_number"`
	CommentID   *int64  `json:"comment_id,omitempty"`
	Body        *string `json:"body,omitempty"`
	Reaction    *string `json:"reaction,omitempty"`
}

type UpdateIssueCommentInput struct {
	Owner     string  `json:"owner"`
	Repo      string  `json:"repo"`
	CommentID int64   `json:"comment_id"`
	Body      *string `json:"body"`
}

// A single mutation returns id/url; a combined comment and reaction returns
// both references. The explicit union schema excludes partial combinations.
type AddIssueCommentOutput struct {
	Single   *MinimalResponse
	Combined *IssueCommentAndReactionOutput
}

type IssueCommentAndReactionOutput struct {
	Comment  MinimalResponse `json:"comment"`
	Reaction MinimalResponse `json:"reaction"`
}

func (output AddIssueCommentOutput) MarshalJSON() ([]byte, error) {
	if output.Combined != nil {
		return json.Marshal(output.Combined)
	}
	// The SDK serializes a nil typed pointer as its element's zero value even
	// on errors. Represent an absent union as null, never as a partial object.
	return json.Marshal(output.Single)
}

func addIssueCommentOutputSchema() *jsonschema.Schema {
	ref := func() *jsonschema.Schema {
		return &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"id": {Type: "string"}, "url": {Type: "string"},
			},
			Required:             []string{"id", "url"},
			AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
		}
	}
	return &jsonschema.Schema{
		Types: []string{"object", "null"},
		OneOf: []*jsonschema.Schema{
			{Type: "null"},
			ref(),
			{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"comment": ref(), "reaction": ref(),
				},
				Required:             []string{"comment", "reaction"},
				AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
			},
		},
	}
}

type IssueDependencyReadInput struct {
	Method      string `json:"method"`
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	IssueNumber int    `json:"issue_number"`
	Page        int    `json:"page,omitempty"`
	PerPage     int    `json:"perPage,omitempty"`
}

type IssueDependencyPageInfo struct {
	HasNextPage bool `json:"hasNextPage"`
	NextPage    int  `json:"nextPage"`
}

type IssueDependencyReadOutput struct {
	Issues   []MinimalIssueRef       `json:"issues"`
	PageInfo IssueDependencyPageInfo `json:"pageInfo"`
}

type IssueDependencyWriteInput struct {
	Method             string `json:"method"`
	Type               string `json:"type"`
	Owner              string `json:"owner"`
	Repo               string `json:"repo"`
	IssueNumber        int    `json:"issue_number"`
	RelatedIssueNumber int    `json:"related_issue_number"`
	RelatedOwner       string `json:"related_owner,omitempty"`
	RelatedRepo        string `json:"related_repo,omitempty"`
}

type IssueDependencyWriteOutput struct {
	BlockedIssue  MinimalIssueRef `json:"blocked_issue"`
	BlockingIssue MinimalIssueRef `json:"blocking_issue"`
	Message       string          `json:"message"`
}

type FindDuplicateInput struct {
	Owner               string   `json:"owner"`
	Repo                string   `json:"repo"`
	IssueNumber         int      `json:"issue_number"`
	ConfidenceThreshold *float64 `json:"confidence_threshold,omitempty"`
	Page                *int     `json:"page,omitempty"`
	PerPage             *int     `json:"perPage,omitempty"`
}

func validateIssueCoordinate(owner, repo string, number int) error {
	if owner == "" {
		return fmt.Errorf("missing required parameter: owner")
	}
	if repo == "" {
		return fmt.Errorf("missing required parameter: repo")
	}
	if number == 0 {
		return fmt.Errorf("missing required parameter: issue_number")
	}
	return nil
}

// Only the dependency writer historically accepted case-insensitive methods
// and directions. Do not broaden the dependency reader's method handling.
func normalizeIssueDependencyWriteArguments(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]json.RawMessage
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	for _, field := range []string{"method", "type"} {
		var value string
		if err := json.Unmarshal(args[field], &value); err == nil {
			args[field], _ = json.Marshal(strings.ToLower(value))
		}
	}
	return json.Marshal(args)
}

func normalizeIssueCommentID(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if value, exists := args["comment_id"]; exists {
		id, err := toInt64(value)
		if err != nil {
			return nil, fmt.Errorf("parameter comment_id is not a valid number: %w", err)
		}
		args["comment_id"] = id
	}
	return json.Marshal(args)
}

func normalizeIssueStrings(required, optional []string) inventory.InputNormalizer {
	return func(raw json.RawMessage) (json.RawMessage, error) {
		var args map[string]any
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
		for _, field := range required {
			if _, err := RequiredParam[string](args, field); err != nil {
				return nil, err
			}
		}

		for _, field := range optional {
			if _, err := OptionalParam[string](args, field); err != nil {
				return nil, err
			}
		}
		return raw, nil
	}
}

func normalizeIssueIntegers(required, optional []string) inventory.InputNormalizer {
	return func(raw json.RawMessage) (json.RawMessage, error) {
		var args map[string]any
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
		for _, field := range required {
			value, err := RequiredInt(args, field)
			if err != nil {
				return nil, err
			}
			args[field] = value
		}
		for _, field := range optional {
			if _, exists := args[field]; !exists {
				continue
			}
			value, err := OptionalIntParam(args, field)
			if err != nil {
				return nil, err
			}
			args[field] = value
		}
		return json.Marshal(args)
	}
}

func normalizeDuplicateThreshold(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if _, err := OptionalParam[float64](args, "confidence_threshold"); err != nil {
		return nil, err
	}
	return raw, nil
}
