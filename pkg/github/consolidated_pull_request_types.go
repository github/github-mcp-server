package github

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/go-viper/mapstructure/v2"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type PullRequestReadInput struct {
	Method     string  `json:"method"`
	Owner      string  `json:"owner"`
	Repo       string  `json:"repo"`
	PullNumber int     `json:"pullNumber"`
	Page       *int    `json:"page,omitempty"`
	PerPage    *int    `json:"perPage,omitempty"`
	After      *string `json:"after,omitempty"`
}

type CreatePullRequestInput struct {
	Owner               string    `json:"owner"`
	Repo                string    `json:"repo"`
	Title               *string   `json:"title,omitempty"`
	Body                *string   `json:"body,omitempty"`
	Head                *string   `json:"head,omitempty"`
	Base                *string   `json:"base,omitempty"`
	Draft               *bool     `json:"draft,omitempty"`
	MaintainerCanModify *bool     `json:"maintainer_can_modify,omitempty"`
	Reviewers           *[]string `json:"reviewers,omitempty"`
	UISubmitted         *bool     `json:"_ui_submitted,omitempty"`
}

// UpdatePullRequestInput uses pointers because the handler distinguishes
// omitted fields from explicit zero values such as draft=false.
type UpdatePullRequestInput struct {
	Owner               string    `json:"owner"`
	Repo                string    `json:"repo"`
	PullNumber          int       `json:"pullNumber"`
	Title               *string   `json:"title,omitempty"`
	Body                *string   `json:"body,omitempty"`
	State               *string   `json:"state,omitempty"`
	Draft               *bool     `json:"draft,omitempty"`
	Base                *string   `json:"base,omitempty"`
	MaintainerCanModify *bool     `json:"maintainer_can_modify,omitempty"`
	Reviewers           *[]string `json:"reviewers,omitempty"`
	UISubmitted         *bool     `json:"_ui_submitted,omitempty"`
}

type MergePullRequestInput struct {
	Owner           string  `json:"owner"`
	Repo            string  `json:"repo"`
	PullNumber      int     `json:"pullNumber"`
	CommitTitle     *string `json:"commit_title,omitempty"`
	CommitMessage   *string `json:"commit_message,omitempty"`
	MergeMethod     *string `json:"merge_method,omitempty"`
	ExpectedHeadSHA *string `json:"expectedHeadSha,omitempty"`
}

type UpdatePullRequestBranchInput struct {
	Owner           string  `json:"owner"`
	Repo            string  `json:"repo"`
	PullNumber      int     `json:"pullNumber"`
	ExpectedHeadSHA *string `json:"expectedHeadSha,omitempty"`
}

// PullRequestReviewWriteInput covers both feature variants; resolutionReason
// is advertised and used only by the resolution-reason variant.
type PullRequestReviewWriteInput struct {
	Method           string  `json:"method"`
	Owner            string  `json:"owner"`
	Repo             string  `json:"repo"`
	PullNumber       int     `json:"pullNumber"`
	Body             string  `json:"body,omitempty"`
	Event            string  `json:"event,omitempty"`
	CommitID         *string `json:"commitID,omitempty"`
	ThreadID         string  `json:"threadId,omitempty"`
	ResolutionReason *string `json:"resolutionReason,omitempty"`
}

type AddCommentToPendingReviewInput struct {
	Owner       string  `json:"owner"`
	Repo        string  `json:"repo"`
	PullNumber  int     `json:"pullNumber"`
	Path        string  `json:"path"`
	Body        string  `json:"body"`
	SubjectType string  `json:"subjectType"`
	Line        *int    `json:"line,omitempty"`
	Side        *string `json:"side,omitempty"`
	StartLine   *int    `json:"startLine,omitempty"`
	StartSide   *string `json:"startSide,omitempty"`
}

type AddReplyToPullRequestCommentInput struct {
	Owner      string  `json:"owner"`
	Repo       string  `json:"repo"`
	PullNumber *int    `json:"pullNumber,omitempty"`
	CommentID  int64   `json:"commentId"`
	Body       *string `json:"body,omitempty"`
	Reaction   *string `json:"reaction,omitempty"`
}

// PullRequestReadOutput mirrors the method-specific top-level shapes that
// pull_request_read has always returned as text.
type PullRequestReadOutput struct {
	PullRequest   *MinimalPullRequest
	Diff          *PullRequestDiffOutput
	Status        *MinimalCombinedStatus
	Files         *[]MinimalPRFile
	Commits       *[]MinimalPullRequestCommit
	ReviewThreads *MinimalReviewThreadsResponse
	Reviews       *[]MinimalPullRequestReview
	Comments      *[]IssueReadCommentOutput
	CheckRuns     *MinimalCheckRunsResult
}

// PullRequestDiffOutput wraps the raw diff, which the text content returns verbatim.
type PullRequestDiffOutput struct {
	Diff string `json:"diff"`
}

func (out PullRequestReadOutput) MarshalJSON() ([]byte, error) {
	switch {
	case out.PullRequest != nil:
		return json.Marshal(out.PullRequest)
	case out.Diff != nil:
		return json.Marshal(out.Diff)
	case out.Status != nil:
		return json.Marshal(out.Status)
	case out.Files != nil:
		return json.Marshal(out.Files)
	case out.Commits != nil:
		return json.Marshal(out.Commits)
	case out.ReviewThreads != nil:
		return json.Marshal(out.ReviewThreads)
	case out.Reviews != nil:
		return json.Marshal(out.Reviews)
	case out.Comments != nil:
		return json.Marshal(out.Comments)
	case out.CheckRuns != nil:
		return json.Marshal(out.CheckRuns)
	default:
		// The SDK validates this zero value even for error results before the
		// protocol middleware removes it from the wire result.
		return []byte("null"), nil
	}
}

func pullRequestCommentsOutput(output *IssueReadOutput) *PullRequestReadOutput {
	if output == nil || output.Comments == nil {
		return nil
	}
	return &PullRequestReadOutput{Comments: output.Comments}
}

func pullRequestReadOutputSchema() *jsonschema.Schema {
	arrays := []*jsonschema.Schema{
		pullRequestOutputSchema[[]MinimalPRFile](),
		pullRequestOutputSchema[[]MinimalPullRequestCommit](),
		pullRequestOutputSchema[[]MinimalPullRequestReview](),
		pullRequestOutputSchema[[]IssueReadCommentOutput](),
	}
	for _, array := range arrays {
		array.Defs = nil
	}
	// An empty array is valid for every list method, so the list shapes form
	// one anyOf array variant instead of ambiguous oneOf branches.
	return repositoryUnionSchema(
		&jsonschema.Schema{Type: "null"},
		pullRequestOutputSchema[MinimalPullRequest](),
		pullRequestOutputSchema[PullRequestDiffOutput](),
		pullRequestOutputSchema[MinimalCombinedStatus](),
		pullRequestOutputSchema[MinimalReviewThreadsResponse](),
		pullRequestOutputSchema[MinimalCheckRunsResult](),
		&jsonschema.Schema{Type: "array", AnyOf: arrays},
	)
}

// PullRequestWriteOutput is either the created/updated pull request reference
// or the notice that an interactive form is awaiting user submission.
type PullRequestWriteOutput struct {
	PullRequest *PullRequestMutationReference
	Awaiting    *IssueWriteAwaitingOutput
}

func (out PullRequestWriteOutput) MarshalJSON() ([]byte, error) {
	if out.Awaiting != nil {
		return json.Marshal(out.Awaiting)
	}
	if out.PullRequest != nil {
		return json.Marshal(out.PullRequest)
	}
	return json.Marshal(PullRequestMutationReference{})
}

func pullRequestWriteOutputSchema() *jsonschema.Schema {
	awaiting := pullRequestOutputSchema[IssueWriteAwaitingOutput]()
	awaiting.Properties["status"].Enum = []any{"awaiting_user_submission"}
	return repositoryUnionSchema(pullRequestOutputSchema[PullRequestMutationReference](), awaiting)
}

// PullRequestMutationReference includes a browser URL only when the API supplied one.
// Reactions have no browser URL; their API endpoint is not useful output.
type PullRequestMutationReference struct {
	ID      string `json:"id"`
	HTMLURL string `json:"html_url,omitempty"`
}

func pullRequestMutationReference(response MinimalResponse) *PullRequestMutationReference {
	return &PullRequestMutationReference{ID: response.ID, HTMLURL: response.URL}
}

func pullRequestFormArguments(req *mcp.CallToolRequest, args map[string]any) (map[string]any, error) {
	// Form deferral depends on which parameters the caller actually sent,
	// so inspect the raw request rather than the normalized typed input.
	if req == nil || req.Params == nil || len(req.Params.Arguments) == 0 {
		return args, nil
	}
	var formArgs map[string]any
	if err := json.Unmarshal(req.Params.Arguments, &formArgs); err != nil {
		return nil, err
	}
	return formArgs, nil
}

func pullRequestAwaitingFormResult(message string) (*mcp.CallToolResult, *PullRequestWriteOutput, error) {
	result := utils.NewToolResultAwaitingFormSubmission(message)
	output := &IssueWriteAwaitingOutput{
		Status: "awaiting_user_submission",
		Reason: "An interactive form is being shown to the user. The operation has not been performed.",
	}
	result.StructuredContent = output
	return result, &PullRequestWriteOutput{Awaiting: output}, nil
}

// PullRequestMergeOutput preserves a JSON null merge response rather than
// letting the SDK replace a nil pointer with an empty object.
type PullRequestMergeOutput struct {
	Result *PullRequestMergeResult
}

type PullRequestMergeResult struct {
	SHA     *string `json:"sha,omitempty"`
	Merged  *bool   `json:"merged,omitempty"`
	Message *string `json:"message,omitempty"`
}

func (out PullRequestMergeOutput) MarshalJSON() ([]byte, error) {
	return json.Marshal(out.Result)
}

func pullRequestMergeOutputSchema() *jsonschema.Schema {
	return nullableRepositoryOutputSchema[PullRequestMergeResult]()
}

// PullRequestBranchUpdateOutput carries either the API response or, for an
// accepted asynchronous update, the in-progress message.
type PullRequestBranchUpdateOutput struct {
	Result *PullRequestBranchUpdateResult
}

type PullRequestBranchUpdateResult struct {
	Message *string `json:"message,omitempty"`
}

func (out PullRequestBranchUpdateOutput) MarshalJSON() ([]byte, error) {
	return json.Marshal(out.Result)
}

func pullRequestBranchUpdateOutputSchema() *jsonschema.Schema {
	return nullableRepositoryOutputSchema[PullRequestBranchUpdateResult]()
}

func nullableRepositoryOutputSchema[T any]() *jsonschema.Schema {
	schema := pullRequestOutputSchema[T]()
	schema.Type = ""
	schema.Types = []string{"object", "null"}
	return schema
}

// PullRequestCommentReplyOutput is a single reply or reaction reference, or
// both when the caller requested a reply and a reaction together.
type PullRequestCommentReplyOutput struct {
	Response         *PullRequestMutationReference
	ReplyAndReaction *PullRequestReplyAndReactionOutput
}

// PullRequestReplyAndReactionOutput keeps the legacy map's sorted key order.
type PullRequestReplyAndReactionOutput struct {
	Comment  PullRequestMutationReference `json:"comment"`
	Reaction PullRequestMutationReference `json:"reaction"`
}

func (out PullRequestCommentReplyOutput) MarshalJSON() ([]byte, error) {
	if out.ReplyAndReaction != nil {
		return json.Marshal(out.ReplyAndReaction)
	}
	if out.Response != nil {
		return json.Marshal(out.Response)
	}
	return json.Marshal(PullRequestMutationReference{})
}

func pullRequestCommentReplyOutputSchema() *jsonschema.Schema {
	return repositoryUnionSchema(
		pullRequestOutputSchema[PullRequestMutationReference](),
		pullRequestOutputSchema[PullRequestReplyAndReactionOutput](),
	)
}

func pullRequestOutputSchema[T any]() *jsonschema.Schema {
	schema := repositoryOutputSchema[T]()
	describePullRequestOutputSchema(schema)
	return schema
}

// Only newly inferred schemas are customized, once during tool construction.
func describePullRequestOutputSchema(schema *jsonschema.Schema) {
	if schema == nil {
		return
	}
	for name, field := range schema.Properties {
		switch name {
		case "html_url":
			field.Description = "Browser URL."
		case "sha", "commit_id":
			field.Description = "Git commit SHA."
		case "created_at", "updated_at", "closed_at", "merged_at", "submitted_at", "started_at", "completed_at":
			field.Description = "RFC 3339 timestamp."
		case "line", "original_line", "start_line", "original_start_line":
			field.Description = "One-based diff line number."
		case "additions", "deletions", "changes":
			field.Description = "Number of lines."
		case "total_count", "totalCount":
			field.Description = "Total number of results."
		case "message":
			if schema.Properties["author"] == nil {
				field.Description = "Operation result message."
			}
		case "merged":
			field.Description = "Whether the pull request was merged."
		case "diff":
			field.Description = "Unified pull request diff."
		case "state":
			switch {
			case schema.Properties["draft"] != nil:
				field.Description = "Pull request state; empty for a sparse response."
				field.Enum = []any{"open", "closed", ""}
			case schema.Properties["statuses"] != nil || schema.Properties["context"] != nil:
				field.Description = "Commit status; empty for a sparse response."
				field.Enum = []any{"error", "failure", "pending", "success", ""}
			case schema.Properties["commit_id"] != nil:
				field.Description = "Review state; empty for a sparse response."
				field.Enum = []any{"APPROVED", "CHANGES_REQUESTED", "COMMENTED", "DISMISSED", "PENDING", ""}
			}
		case "status":
			switch {
			case schema.Properties["filename"] != nil:
				field.Description = "File change status."
				field.Enum = []any{"added", "removed", "modified", "renamed", "copied", "changed", "unchanged"}
			case schema.Properties["conclusion"] != nil:
				field.Description = "Check execution status; empty for a sparse response."
				field.Enum = []any{"queued", "in_progress", "completed", "waiting", "requested", "pending", ""}
			}
		case "conclusion":
			field.Description = "Completed check outcome."
			field.Enum = []any{"success", "failure", "neutral", "cancelled", "skipped", "timed_out", "action_required", "stale", "startup_failure"}
		}
		describePullRequestOutputSchema(field)
	}
	describePullRequestOutputSchema(schema.Items)
	for _, definition := range schema.Defs {
		describePullRequestOutputSchema(definition)
	}
	for _, variant := range append(slices.Clone(schema.OneOf), schema.AnyOf...) {
		describePullRequestOutputSchema(variant)
	}
}

// pullRequestMessageResult exposes the success message of review mutations
// whose exported helpers only return a text result.
func pullRequestMessageResult(result *mcp.CallToolResult, err error) (*mcp.CallToolResult, *RepositoryMessageOutput, error) {
	if err != nil || result == nil || result.IsError || len(result.Content) != 1 {
		return result, nil, err
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		return result, nil, nil
	}
	return result, &RepositoryMessageOutput{Message: text.Text}, nil
}

// pullRequestArgumentSpec lists parameters in the order the legacy handler
// read them, so normalization reports the same first error the handler did.
type pullRequestArgumentSpec struct {
	required       []string
	requiredInts   []string
	ints           []string
	strings        []string
	bools          []string
	stringArrays   []string
	ignoredStrings []string
	// deferred required strings are filled with "" so the handler can still
	// show its interactive form before reporting them as missing.
	deferred []string
	methods  []string
}

var pullRequestArgumentSpecs = map[string]pullRequestArgumentSpec{
	"read": {
		required: []string{"method", "owner", "repo"}, requiredInts: []string{"pullNumber"},
		ints:    []string{"page", "perPage"},
		methods: []string{"get", "get_diff", "get_status", "get_files", "get_commits", "get_review_comments", "get_reviews", "get_comments", "get_check_runs"},
	},
	"create": {
		required: []string{"owner", "repo"}, deferred: []string{"title", "head", "base"},
		strings: []string{"title", "head", "base", "body"}, bools: []string{"draft", "maintainer_can_modify"},
		stringArrays: []string{"reviewers"},
	},
	"update": {
		required: []string{"owner", "repo"}, requiredInts: []string{"pullNumber"},
		strings: []string{"title", "body", "state", "base"}, bools: []string{"draft", "maintainer_can_modify"},
		stringArrays: []string{"reviewers"},
	},
	"merge": {
		required: []string{"owner", "repo"}, requiredInts: []string{"pullNumber"},
		strings: []string{"commit_title", "commit_message", "merge_method", "expectedHeadSha"},
	},
	"branch": {
		required: []string{"owner", "repo"}, requiredInts: []string{"pullNumber"}, strings: []string{"expectedHeadSha"},
	},
	"pending_comment": {
		required: []string{"owner", "repo"}, requiredInts: []string{"pullNumber"},
		ints: []string{"line", "startLine"}, ignoredStrings: []string{"side", "startSide"},
	},
	"reply": {
		required: []string{"owner", "repo"}, strings: []string{"body", "reaction"},
	},
}

// normalizePullRequestArguments replays the legacy handler's parameter
// checks before strict schema validation. Legacy handlers bypassed schema
// validation, so they accepted numeric strings and whole floats and reported
// their own error messages; those messages are preserved here.
func normalizePullRequestArguments(kind string) func(json.RawMessage) (json.RawMessage, error) {
	spec, ok := pullRequestArgumentSpecs[kind]
	if !ok {
		panic("unknown pull request argument kind: " + kind)
	}
	return func(raw json.RawMessage) (json.RawMessage, error) {
		var args map[string]any
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
		for _, field := range spec.required {
			if _, err := RequiredParam[string](args, field); err != nil {
				return nil, err
			}
		}
		for _, field := range spec.requiredInts {
			value, err := RequiredInt(args, field)
			if err != nil {
				return nil, err
			}
			args[field] = value
		}
		if kind == "pending_comment" {
			for _, field := range []string{"path", "body", "subjectType"} {
				if _, err := RequiredParam[string](args, field); err != nil {
					return nil, err
				}
			}
		}
		if kind == "reply" {
			commentID, err := RequiredBigInt(args, "commentId")
			if err != nil {
				return nil, err
			}
			if commentID < 1 {
				return nil, fmt.Errorf("commentId must be greater than 0")
			}
			args["commentId"] = commentID
		}
		for _, field := range spec.deferred {
			if _, exists := args[field]; !exists {
				args[field] = ""
			}
		}

		stringFields := spec.strings
		boolFields := spec.bools
		stringArrayFields := spec.stringArrays
		if kind == "reply" {
			stringFields = []string{"body", "reaction"}
		}
		if kind == "create" || kind == "update" {
			// This flag is inspected from the raw call only to control form
			// deferral; legacy handlers never type-validated it.
			if _, err := OptionalParam[bool](args, "_ui_submitted"); err != nil {
				delete(args, "_ui_submitted")
			}
		}
		for _, field := range stringFields {
			if _, err := OptionalParam[string](args, field); err != nil {
				return nil, err
			}
		}
		for _, field := range boolFields {
			if _, err := OptionalParam[bool](args, field); err != nil {
				return nil, err
			}
		}
		for _, field := range stringArrayFields {
			if _, exists := args[field]; !exists {
				continue
			}
			values, err := OptionalStringArrayParam(args, field)
			if err != nil {
				return nil, err
			}
			args[field] = values
		}
		for _, field := range spec.ints {
			if _, exists := args[field]; !exists {
				continue
			}
			value, err := OptionalIntParam(args, field)
			if err != nil {
				return nil, err
			}
			args[field] = value
		}
		if spec.methods != nil && !slices.Contains(spec.methods, args["method"].(string)) {
			return nil, fmt.Errorf("unknown method: %s", args["method"])
		}
		if kind == "read" {
			if args["method"] != "get_review_comments" {
				delete(args, "after")
			} else if _, err := OptionalParam[string](args, "after"); err != nil {
				return nil, err
			}
		}
		if kind == "reply" {
			_, hasBody := args["body"]
			_, hasReaction := args["reaction"]
			if !hasBody && !hasReaction {
				return nil, fmt.Errorf("at least one of body or reaction is required")
			}
			// pullNumber is only read, and validated, when a reply body is sent.
			if value, exists := args["pullNumber"]; exists {
				pullNumber, err := toInt(value)
				switch {
				case err == nil && hasBody:
					args["pullNumber"] = pullNumber
				case err != nil && hasBody:
					return nil, fmt.Errorf("parameter pullNumber is not a valid number: %w", err)
				default:
					delete(args, "pullNumber")
				}
			}
			if hasBody && args["body"] == "" {
				return nil, fmt.Errorf("body cannot be empty when provided")
			}
			if hasReaction && args["reaction"] == "" {
				return nil, fmt.Errorf("reaction cannot be empty when provided")
			}
		}
		for _, field := range spec.ignoredStrings {
			if _, isString := args[field].(string); !isString {
				delete(args, field)
			}
		}
		return json.Marshal(args)
	}
}

// normalizePullRequestReviewWriteArguments retains the legacy WeakDecode
// semantics: case-insensitive keys, numeric-string conversion and fractional
// truncation of pullNumber.
func normalizePullRequestReviewWriteArguments(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	var params PullRequestReviewWriteParams
	if err := mapstructure.WeakDecode(args, &params); err != nil {
		return nil, err
	}
	if !slices.Contains([]string{"create", "submit_pending", "delete_pending", "resolve_thread", "unresolve_thread"}, params.Method) {
		return nil, fmt.Errorf("unknown method: %s", params.Method)
	}
	canonical := map[string]any{
		"method": params.Method, "owner": params.Owner, "repo": params.Repo, "pullNumber": params.PullNumber,
	}
	// Empty optional strings are equivalent to omitted ones for the handler,
	// but would fail the schema's enum for event.
	for key, value := range map[string]string{"body": params.Body, "event": params.Event, "threadId": params.ThreadID} {
		if value != "" {
			canonical[key] = value
		}
	}
	if params.CommitID != nil {
		canonical["commitID"] = *params.CommitID
	}
	if params.ResolutionReason != nil {
		canonical["resolutionReason"] = *params.ResolutionReason
	}
	for key := range args {
		for name := range pullRequestReviewWriteFields {
			if strings.EqualFold(key, name) {
				delete(args, key)
			}
		}
	}
	maps.Copy(args, canonical)
	return json.Marshal(args)
}

var pullRequestReviewWriteFields = map[string]struct{}{
	"method": {}, "owner": {}, "repo": {}, "pullNumber": {}, "body": {}, "event": {},
	"commitID": {}, "threadId": {}, "resolutionReason": {},
}
