package github

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/github/github-mcp-server/pkg/inventory"
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
	State string `json:"state"`
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
	Body     string `json:"body,omitempty"`
	Event    string `json:"event,omitempty"`
	CommitID string `json:"commitID,omitempty"`
}

type GranularSubmitPullRequestReviewInput struct {
	GranularPullRequestCoordinate
	Event string `json:"event"`
	Body  string `json:"body,omitempty"`
}

type GranularReviewThreadInput struct {
	ThreadID string `json:"threadID"`
}

type GranularResolveReviewThreadInput struct {
	GranularReviewThreadInput
	ResolutionReason *string `json:"resolutionReason,omitempty"`
}

type GranularAddPullRequestCommentReactionInput struct {
	Owner     string `json:"owner"`
	Repo      string `json:"repo"`
	CommentID int64  `json:"comment_id"`
	Content   string `json:"content"`
}

type GranularRemovePullRequestCommentReactionInput struct {
	Owner      string `json:"owner"`
	Repo       string `json:"repo"`
	CommentID  int64  `json:"comment_id"`
	ReactionID int64  `json:"reaction_id"`
}

// normalizeGranularPullRequestArguments replays the untyped parameter checks
// in their original order, including ignored optional-string type errors.
// Schema enums/minima are intentionally descriptive rather than restrictive:
// the legacy handlers left those checks to GitHub, including negative IDs.
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
