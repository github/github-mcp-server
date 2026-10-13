package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/google/jsonschema-go/jsonschema"
)

type GranularPullRequestCoordinate struct {
	Owner      string `json:"owner"`
	Repo       string `json:"repo"`
	PullNumber int    `json:"pullNumber"`
}

func (in GranularPullRequestCoordinate) pullRequestCoordinate() GranularPullRequestCoordinate {
	return in
}

type GranularPullRequestTitleInput struct {
	GranularPullRequestCoordinate
	Title string `json:"title"`
}

type GranularPullRequestBodyInput struct {
	GranularPullRequestCoordinate
	Body string `json:"body"`
}

type GranularPullRequestStateInput struct {
	GranularPullRequestCoordinate
	State GranularPullRequestState `json:"state"`
}

type GranularPullRequestDraftInput struct {
	GranularPullRequestCoordinate
	Draft bool `json:"draft"`
}

type GranularPullRequestReviewersInput struct {
	GranularPullRequestCoordinate
	Reviewers []string `json:"reviewers"`
}

type GranularCreatePullRequestReviewInput struct {
	GranularPullRequestCoordinate
	Body     string                         `json:"body,omitempty"`
	Event    GranularPullRequestReviewEvent `json:"event,omitempty"`
	CommitID string                         `json:"commitID,omitempty"`
}

type GranularSubmitPullRequestReviewInput struct {
	GranularPullRequestCoordinate
	Event GranularPullRequestReviewEvent `json:"event"`
	Body  string                         `json:"body,omitempty"`
}

type GranularPullRequestReviewCommentInput struct {
	GranularPullRequestCoordinate
	Path        string                            `json:"path"`
	Body        string                            `json:"body"`
	SubjectType GranularPullRequestCommentSubject `json:"subjectType"`
	Line        *int                              `json:"line,omitempty"`
	Side        *GranularPullRequestDiffSide      `json:"side,omitempty"`
	StartLine   *int                              `json:"startLine,omitempty"`
	StartSide   *GranularPullRequestDiffSide      `json:"startSide,omitempty"`
}

type GranularReviewThreadInput struct {
	ThreadID string `json:"threadID"`
}

type GranularResolveReviewThreadInput struct {
	GranularReviewThreadInput
	ResolutionReason *string `json:"resolutionReason,omitempty"`
}

type GranularAddPullRequestCommentReactionInput struct {
	Owner     string                         `json:"owner"`
	Repo      string                         `json:"repo"`
	CommentID int64                          `json:"comment_id"`
	Content   PullRequestCommentReactionType `json:"content"`
}

type GranularRemovePullRequestCommentReactionInput struct {
	Owner      string `json:"owner"`
	Repo       string `json:"repo"`
	CommentID  int64  `json:"comment_id"`
	ReactionID int64  `json:"reaction_id"`
}

type MinimalPullRequestCommentReaction = MinimalResponse

type PullRequestCommentReactionType string

func (PullRequestCommentReactionType) Values() []string {
	return []string{"+1", "-1", "laugh", "confused", "heart", "hooray", "rocket", "eyes"}
}

func (value PullRequestCommentReactionType) JSONSchema() *jsonschema.Schema {
	return granularPullRequestEnumSchema(value.Values())
}

type GranularPullRequestState string

func (GranularPullRequestState) Values() []string { return []string{"open", "closed"} }
func (value GranularPullRequestState) JSONSchema() *jsonschema.Schema {
	return granularPullRequestEnumSchema(value.Values())
}

type GranularPullRequestReviewEvent string

func (GranularPullRequestReviewEvent) Values() []string {
	return []string{"APPROVE", "REQUEST_CHANGES", "COMMENT"}
}
func (value GranularPullRequestReviewEvent) JSONSchema() *jsonschema.Schema {
	return granularPullRequestEnumSchema(value.Values())
}

type GranularPullRequestCommentSubject string

func (GranularPullRequestCommentSubject) Values() []string { return []string{"FILE", "LINE"} }
func (value GranularPullRequestCommentSubject) JSONSchema() *jsonschema.Schema {
	return granularPullRequestEnumSchema(value.Values())
}

type GranularPullRequestDiffSide string

func (GranularPullRequestDiffSide) Values() []string { return []string{"LEFT", "RIGHT"} }
func (value GranularPullRequestDiffSide) JSONSchema() *jsonschema.Schema {
	return granularPullRequestEnumSchema(value.Values())
}

func granularPullRequestEnumSchema(values []string) *jsonschema.Schema {
	enum := make([]any, len(values))
	for i, value := range values {
		enum[i] = value
	}
	return &jsonschema.Schema{Type: "string", Enum: enum}
}

func minimalPullRequestCommentReactionSchema() *jsonschema.Schema {
	schema := repositoryOutputSchema[MinimalPullRequestCommentReaction]()
	schema.Properties["id"].Description = "Reaction ID"
	schema.Properties["url"].Description = "Reaction URL"
	return schema
}

// normalizeGranularPullRequestArguments replays the untyped parameter checks
// in their original order, including ignored optional-string type errors.
// The advertised enum/minimum constraints remain unchanged; the compatibility
// validation schema leaves these legacy-unvalidated checks to GitHub.
func normalizeGranularPullRequestArguments(kind string) inventory.InputNormalizer {
	return func(raw json.RawMessage) (json.RawMessage, error) {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return nil, &inventory.ToolInputError{Message: "invalid arguments: arguments must be a JSON object"}
		}
		var args map[string]any
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, &inventory.ToolInputError{Message: "invalid arguments: " + err.Error()}
		}

		if err := normalizeGranularPullRequestFields(args, kind); err != nil {
			return nil, &inventory.ToolInputError{Message: err.Error()}
		}
		return json.Marshal(args)
	}
}

func granularPullRequestValidationSchema(advertised *jsonschema.Schema) *jsonschema.Schema {
	validation := *advertised
	validation.Properties = maps.Clone(advertised.Properties)
	for name, property := range advertised.Properties {
		if len(property.Enum) == 0 && property.Minimum == nil {
			continue
		}
		validationProperty := *property
		validationProperty.Enum = nil
		validationProperty.Minimum = nil
		validation.Properties[name] = &validationProperty
	}
	return &validation
}

func normalizeGranularResolveReviewThreadArguments(withResolutionReason bool) inventory.InputNormalizer {
	if withResolutionReason {
		return normalizeGranularPullRequestArguments("resolve_reason")
	}
	return normalizeGranularPullRequestArguments("resolve")
}

func normalizeGranularPullRequestFields(args map[string]any, kind string) error {
	requiredString := func(field string) error {
		_, err := RequiredParam[string](args, field)
		return err
	}
	ignoredString := func(field string) {
		if _, ok := args[field].(string); !ok {
			delete(args, field)
		}
	}
	switch kind {
	case "resolve", "resolve_reason", "unresolve":
		if err := requiredString("threadID"); err != nil {
			return err
		}
		if kind == "resolve_reason" {
			_, _, err := OptionalParamOK[string](args, "resolutionReason")
			return err
		}
		delete(args, "resolutionReason")
		return nil
	}
	for _, field := range []string{"owner", "repo"} {
		if err := requiredString(field); err != nil {
			return err
		}
	}
	if kind == "add_reaction" || kind == "remove_reaction" {
		commentID, err := RequiredBigInt(args, "comment_id")
		if err != nil {
			return err
		}
		args["comment_id"] = commentID
		if kind == "add_reaction" {
			return requiredString("content")
		}
		reactionID, err := RequiredBigInt(args, "reaction_id")
		if err != nil {
			return err
		}
		args["reaction_id"] = reactionID
		return nil
	}
	pullNumber, err := RequiredInt(args, "pullNumber")
	if err != nil {
		return err
	}
	args["pullNumber"] = pullNumber
	switch kind {
	case "title", "body", "state":
		return requiredString(kind)
	case "draft":
		if _, ok := args["draft"]; !ok {
			return fmt.Errorf("missing required parameter: draft")
		}
		_, err := OptionalParam[bool](args, "draft")
		return err
	case "reviewers":
		reviewers, err := OptionalStringArrayParam(args, "reviewers")
		if err != nil {
			return err
		}
		if len(reviewers) == 0 {
			return fmt.Errorf("missing required parameter: reviewers")
		}
		args["reviewers"] = reviewers
	case "create_review":
		for _, field := range []string{"body", "event", "commitID"} {
			ignoredString(field)
		}
	case "submit_review":
		if err := requiredString("event"); err != nil {
			return err
		}
		ignoredString("body")
	case "delete_review":
	case "comment":
		for _, field := range []string{"path", "body", "subjectType"} {
			if err := requiredString(field); err != nil {
				return err
			}
		}
		line, err := OptionalIntParam(args, "line")
		if err != nil {
			return err
		}
		args["line"] = line
		ignoredString("side")
		startLine, err := OptionalIntParam(args, "startLine")
		if err != nil {
			return err
		}
		args["startLine"] = startLine
		ignoredString("startSide")
	default:
		panic("unknown granular pull request argument kind: " + kind)
	}
	return nil
}
