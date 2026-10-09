package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/google/go-github/v92/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// GranularIssueCoordinate is shared by the single-issue mutation DTOs.
type GranularIssueCoordinate struct {
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	IssueNumber int    `json:"issue_number"`
}

func (in GranularIssueCoordinate) issueCoordinate() GranularIssueCoordinate { return in }

type GranularCreateIssueInput struct {
	Owner             string `json:"owner"`
	Repo              string `json:"repo"`
	Title             string `json:"title"`
	Body              string `json:"body,omitempty"`
	ParentIssueNumber *int   `json:"parent_issue_number,omitempty"`
	ParentOwner       string `json:"parent_owner,omitempty"`
	ParentRepo        string `json:"parent_repo,omitempty"`
}

type GranularIssueTitleInput struct {
	GranularIssueCoordinate
	Title string `json:"title"`
}

type GranularIssueBodyInput struct {
	GranularIssueCoordinate
	Body string `json:"body"`
}

type GranularIssueMilestoneInput struct {
	GranularIssueCoordinate
	Milestone int `json:"milestone"`
}

// Entries retain the heterogeneous string/object input without a map-backed
// handler. The normalizer validates and canonicalizes intent before decoding.
type GranularIssueAssignee struct {
	Login        string `json:"login"`
	Rationale    string `json:"rationale,omitempty"`
	Confidence   string `json:"confidence,omitempty"`
	IsSuggestion bool   `json:"is_suggestion,omitempty"`
	StringForm   bool   `json:"-"`
}

func (in *GranularIssueAssignee) UnmarshalJSON(raw []byte) error {
	*in = GranularIssueAssignee{}
	if bytes.HasPrefix(bytes.TrimSpace(raw), []byte(`"`)) {
		in.StringForm = true
		return json.Unmarshal(raw, &in.Login)
	}
	type plain GranularIssueAssignee
	return json.Unmarshal(raw, (*plain)(in))
}

func (in GranularIssueAssignee) MarshalJSON() ([]byte, error) {
	if in.StringForm {
		return json.Marshal(in.Login)
	}
	type plain GranularIssueAssignee
	return json.Marshal(plain(in))
}

type GranularIssueLabel struct {
	Name         string `json:"name"`
	Rationale    string `json:"rationale,omitempty"`
	Confidence   string `json:"confidence,omitempty"`
	IsSuggestion bool   `json:"is_suggestion,omitempty"`
	StringForm   bool   `json:"-"`
}

func (in *GranularIssueLabel) UnmarshalJSON(raw []byte) error {
	*in = GranularIssueLabel{}
	if bytes.HasPrefix(bytes.TrimSpace(raw), []byte(`"`)) {
		in.StringForm = true
		return json.Unmarshal(raw, &in.Name)
	}
	type plain GranularIssueLabel
	return json.Unmarshal(raw, (*plain)(in))
}

func (in GranularIssueLabel) MarshalJSON() ([]byte, error) {
	if in.StringForm {
		return json.Marshal(in.Name)
	}
	type plain GranularIssueLabel
	return json.Marshal(plain(in))
}

type GranularIssueAssigneesInput struct {
	GranularIssueCoordinate
	Assignees []GranularIssueAssignee `json:"assignees"`
}

type GranularIssueLabelsInput struct {
	GranularIssueCoordinate
	Labels []GranularIssueLabel `json:"labels"`
}

type GranularIssueTypeInput struct {
	GranularIssueCoordinate
	IssueType    *string `json:"issue_type"`
	Rationale    string  `json:"rationale,omitempty"`
	Confidence   string  `json:"confidence,omitempty"`
	IsSuggestion bool    `json:"is_suggestion,omitempty"`
}

type GranularIssueStateInput struct {
	GranularIssueCoordinate
	State        string `json:"state"`
	StateReason  string `json:"state_reason,omitempty"`
	Rationale    string `json:"rationale,omitempty"`
	Confidence   string `json:"confidence,omitempty"`
	IsSuggestion bool   `json:"is_suggestion,omitempty"`
	DuplicateOf  int    `json:"duplicate_of,omitempty"`
}

type GranularAddSubIssueInput struct {
	GranularIssueCoordinate
	SubIssueID    int  `json:"sub_issue_id"`
	ReplaceParent bool `json:"replace_parent,omitempty"`
}

type GranularRemoveSubIssueInput struct {
	GranularIssueCoordinate
	SubIssueID int `json:"sub_issue_id"`
}

type GranularReprioritizeSubIssueInput struct {
	GranularIssueCoordinate
	SubIssueID int `json:"sub_issue_id"`
	AfterID    int `json:"after_id,omitempty"`
	BeforeID   int `json:"before_id,omitempty"`
}

type GranularIssueField struct {
	FieldID              string   `json:"field_id"`
	TextValue            string   `json:"text_value,omitempty"`
	NumberValue          *float64 `json:"number_value,omitempty"`
	DateValue            string   `json:"date_value,omitempty"`
	SingleSelectOptionID string   `json:"single_select_option_id,omitempty"`
	Delete               bool     `json:"delete,omitempty"`
	Rationale            string   `json:"rationale,omitempty"`
	Confidence           string   `json:"confidence,omitempty"`
	IsSuggestion         bool     `json:"is_suggestion,omitempty"`
}

type GranularSetIssueFieldsInput struct {
	GranularIssueCoordinate
	Fields []GranularIssueField `json:"fields"`
}

type GranularAddIssueReactionInput struct {
	GranularIssueCoordinate
	Content string `json:"content"`
}

type GranularRemoveIssueReactionInput struct {
	GranularIssueCoordinate
	ReactionID int64 `json:"reaction_id"`
}

type GranularAddIssueCommentReactionInput struct {
	Owner     string `json:"owner"`
	Repo      string `json:"repo"`
	CommentID int64  `json:"comment_id"`
	Content   string `json:"content"`
}

type GranularRemoveIssueCommentReactionInput struct {
	Owner      string `json:"owner"`
	Repo       string `json:"repo"`
	CommentID  int64  `json:"comment_id"`
	ReactionID int64  `json:"reaction_id"`
}

type GranularIssueReactionMessage struct {
	Message string `json:"message"`
}

type GranularIssueUser struct {
	Login string `json:"login"`
	ID    int64  `json:"id,omitempty"`
}

type GranularIssueDependenciesSummary struct {
	BlockedBy      *int `json:"blocked_by,omitempty"`
	Blocking       *int `json:"blocking,omitempty"`
	TotalBlockedBy *int `json:"total_blocked_by,omitempty"`
	TotalBlocking  *int `json:"total_blocking,omitempty"`
}

func granularIssueMinimalResult(output *MinimalResponse) (*mcp.CallToolResult, *MinimalResponse, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
	}
	return utils.NewToolResultText(string(raw)), output, nil
}

func granularIssueAssigneesPayload(entries []GranularIssueAssignee) any {
	payload := make([]any, 0, len(entries))
	logins := make([]string, len(entries))
	objectForm := false
	for i, entry := range entries {
		logins[i] = entry.Login
		if entry.Rationale == "" && entry.Confidence == "" && !entry.IsSuggestion {
			payload = append(payload, entry.Login)
		} else {
			objectForm = true
			payload = append(payload, assigneeWithIntent{Login: entry.Login, Rationale: entry.Rationale, Confidence: entry.Confidence, Suggest: entry.IsSuggestion})
		}
	}
	if objectForm {
		return &assigneesUpdateRequest{Assignees: payload}
	}
	return &github.UpdateIssueRequest{Assignees: logins}
}

func granularIssueLabelsPayload(entries []GranularIssueLabel) any {
	payload := make([]any, 0, len(entries))
	names := make([]string, len(entries))
	objectForm := false
	for i, entry := range entries {
		names[i] = entry.Name
		if entry.Rationale == "" && entry.Confidence == "" && !entry.IsSuggestion {
			payload = append(payload, entry.Name)
		} else {
			objectForm = true
			payload = append(payload, labelWithIntent{Name: entry.Name, Rationale: entry.Rationale, Confidence: entry.Confidence, Suggest: entry.IsSuggestion})
		}
	}
	if objectForm {
		return &labelsUpdateRequest{Labels: payload}
	}
	return &github.UpdateIssueRequest{Labels: names}
}

func normalizeGranularIssueUpdateArguments(field string) inventory.InputNormalizer {
	return func(raw json.RawMessage) (json.RawMessage, error) {
		var args map[string]any
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
		for _, name := range []string{"owner", "repo"} {
			if _, err := RequiredParam[string](args, name); err != nil {
				return nil, err
			}
		}
		number, err := RequiredInt(args, "issue_number")
		if err != nil {
			return nil, err
		}
		args["issue_number"] = number
		if field == "milestone" {
			milestone, err := RequiredInt(args, field)
			if err != nil {
				return nil, err
			}
			args[field] = milestone
		} else if _, err := RequiredParam[string](args, field); err != nil {
			return nil, err
		}
		return json.Marshal(args)
	}
}

// Convert validated values without losing a present numeric zero or sending
// empty optional strings. Null and incorrectly typed values do not count.
func granularIssueFieldsFromMutation(fields []IssueFieldCreateOrUpdateInput) []GranularIssueField {
	result := make([]GranularIssueField, 0, len(fields))
	for _, field := range fields {
		entry := GranularIssueField{FieldID: field.FieldID.(string)}
		if field.TextValue != nil {
			entry.TextValue = string(*field.TextValue)
		}
		if field.NumberValue != nil {
			value := float64(*field.NumberValue)
			entry.NumberValue = &value
		}
		if field.DateValue != nil {
			entry.DateValue = string(*field.DateValue)
		}
		if field.SingleSelectOptionID != nil {
			entry.SingleSelectOptionID = (*field.SingleSelectOptionID).(string)
		}
		if field.Delete != nil {
			entry.Delete = bool(*field.Delete)
		}
		if field.Rationale != nil {
			entry.Rationale = string(*field.Rationale)
		}
		if field.Confidence != nil {
			entry.Confidence = *field.Confidence
		}
		if field.Suggest != nil {
			entry.IsSuggestion = bool(*field.Suggest)
		}
		result = append(result, entry)
	}
	return result
}

func granularIssueFieldMutation(fields []GranularIssueField) []IssueFieldCreateOrUpdateInput {
	result := make([]IssueFieldCreateOrUpdateInput, 0, len(fields))
	for _, field := range fields {
		entry := IssueFieldCreateOrUpdateInput{FieldID: githubv4.ID(field.FieldID)}
		if field.TextValue != "" {
			entry.TextValue = githubv4.NewString(githubv4.String(field.TextValue))
		}
		if field.NumberValue != nil {
			value := githubv4.Float(*field.NumberValue)
			entry.NumberValue = &value
		}
		if field.DateValue != "" {
			entry.DateValue = githubv4.NewString(githubv4.String(field.DateValue))
		}
		if field.SingleSelectOptionID != "" {
			value := githubv4.ID(field.SingleSelectOptionID)
			entry.SingleSelectOptionID = &value
		}
		if field.Delete {
			value := githubv4.Boolean(true)
			entry.Delete = &value
		}
		if field.Rationale != "" {
			entry.Rationale = githubv4.NewString(githubv4.String(field.Rationale))
		}
		if field.Confidence != "" {
			value := field.Confidence
			entry.Confidence = &value
		}
		if field.IsSuggestion {
			value := githubv4.Boolean(true)
			entry.Suggest = &value
		}
		result = append(result, entry)
	}
	return result
}

// Preserve supplied properties for the explicit schema to validate, rather than
// silently dropping (for example) duplicate_of:0 or an invalid optional value.
// Only legacy coercions and intent whitespace/case normalization are applied.
func marshalGranularIssueArguments[T any](args map[string]any, input T) (json.RawMessage, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var normalized map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber() // Preserve RequiredBigInt's full int64 result.
	if err := decoder.Decode(&normalized); err != nil {
		return nil, err
	}
	mergeGranularIssueArguments(args, normalized)
	return json.Marshal(args)
}

func mergeGranularIssueArguments(original, normalized map[string]any) {
	if rationale, ok := original["rationale"].(string); ok {
		original["rationale"] = strings.TrimSpace(rationale)
	}
	if confidence, ok := original["confidence"].(string); ok {
		confidence = normalizeConfidence(confidence)
		if confidence == "" {
			delete(original, "confidence")
		} else {
			original["confidence"] = confidence
		}
	}
	for key, value := range normalized {
		// Fields can contain optional values that the legacy parser ignores.
		// Keep those supplied values visible to SDK schema validation.
		if key == "fields" {
			before, beforeOK := original[key].([]any)
			after, afterOK := value.([]any)
			if beforeOK && afterOK && len(before) == len(after) {
				for i := range after {
					oldField, oldOK := before[i].(map[string]any)
					newField, newOK := after[i].(map[string]any)
					if oldOK && newOK {
						mergeGranularIssueArguments(oldField, newField)
						after[i] = oldField
					}
				}
			}
		}
		original[key] = value
	}
}

// Normalizers preserve legacy coercions and diagnostics before SDK validation.
func normalizeGranularCreateIssueArguments(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}

	owner, err := RequiredParam[string](args, "owner")
	if err != nil {
		return nil, err
	}
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return nil, err
	}
	title, err := RequiredParam[string](args, "title")
	if err != nil {
		return nil, err
	}
	body, _ := OptionalParam[string](args, "body")
	// The original create handler ignored OptionalParam errors for body.
	// Canonicalize the resulting default before the unchanged schema validates.
	if _, provided := args["body"]; provided {
		args["body"] = body
	}
	parentIssueNumber, err := OptionalIntParam(args, "parent_issue_number")
	if err != nil {
		return nil, err
	}
	parentValue, parentProvided := args["parent_issue_number"]
	parentProvided = parentProvided && parentValue != nil
	if parentProvided && parentIssueNumber < 1 {
		return nil, fmt.Errorf("parent_issue_number must be greater than 0")
	}
	parentOwner, err := OptionalParam[string](args, "parent_owner")
	if err != nil {
		return nil, err
	}
	parentRepo, err := OptionalParam[string](args, "parent_repo")
	if err != nil {
		return nil, err
	}
	if err := validateParentRepository(parentProvided, parentOwner, parentRepo); err != nil {
		return nil, err
	}

	var parentNumber *int
	if parentProvided {
		parentNumber = &parentIssueNumber
	}

	return marshalGranularIssueArguments(args, GranularCreateIssueInput{Owner: owner, Repo: repo, Title: title, Body: body, ParentIssueNumber: parentNumber, ParentOwner: parentOwner, ParentRepo: parentRepo})
}

func normalizeGranularUpdateIssueAssigneesArguments(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}

	owner, err := RequiredParam[string](args, "owner")
	if err != nil {
		return nil, err
	}
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return nil, err
	}
	issueNumber, err := RequiredInt(args, "issue_number")
	if err != nil {
		return nil, err
	}

	assigneesRaw, ok := args["assignees"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: assignees")
	}
	assigneesSlice, ok := assigneesRaw.([]any)
	if !ok {
		// Also accept []string for callers that pre-typed the array.
		if strs, ok := assigneesRaw.([]string); ok {
			assigneesSlice = make([]any, len(strs))
			for i, s := range strs {
				assigneesSlice[i] = s
			}
		} else {
			return nil, fmt.Errorf("parameter assignees must be an array")
		}
	}

	payload := make([]GranularIssueAssignee, 0, len(assigneesSlice))
	for _, item := range assigneesSlice {
		switch v := item.(type) {
		case string:
			payload = append(payload, GranularIssueAssignee{Login: v, StringForm: true})
		case map[string]any:
			login, err := RequiredParam[string](v, "login")
			if err != nil {
				return nil, fmt.Errorf("each assignee object must have a 'login' string")
			}
			rationale, err := OptionalParam[string](v, "rationale")
			if err != nil {
				return nil, err
			}
			rationale = strings.TrimSpace(rationale)
			if len([]rune(rationale)) > 280 {
				return nil, fmt.Errorf("assignee rationale must be 280 characters or less")
			}
			confidence, err := OptionalParam[string](v, "confidence")
			if err != nil {
				return nil, err
			}
			confidence = normalizeConfidence(confidence)
			if confidence != "" && confidence != "LOW" && confidence != "MEDIUM" && confidence != "HIGH" {
				return nil, fmt.Errorf("confidence must be one of: LOW, MEDIUM, HIGH")
			}
			isSuggestion, err := OptionalParam[bool](v, "is_suggestion")
			if err != nil {
				return nil, err
			}
			if rationale == "" && !isSuggestion && confidence == "" {
				payload = append(payload, GranularIssueAssignee{Login: login, StringForm: true})
			} else {

				payload = append(payload, GranularIssueAssignee{Login: login, Rationale: rationale, Confidence: confidence, IsSuggestion: isSuggestion})
			}
		default:
			return nil, fmt.Errorf("each assignee must be a string or an object with 'login' and optional 'rationale', 'confidence', and/or 'is_suggestion'")
		}
	}

	return marshalGranularIssueArguments(args, GranularIssueAssigneesInput{GranularIssueCoordinate: GranularIssueCoordinate{Owner: owner, Repo: repo, IssueNumber: issueNumber}, Assignees: payload})
}

func normalizeGranularUpdateIssueLabelsArguments(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}

	owner, err := RequiredParam[string](args, "owner")
	if err != nil {
		return nil, err
	}
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return nil, err
	}
	issueNumber, err := RequiredInt(args, "issue_number")
	if err != nil {
		return nil, err
	}

	labelsRaw, ok := args["labels"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: labels")
	}
	labelsSlice, ok := labelsRaw.([]any)
	if !ok {
		// Also accept []string for callers that pre-typed the array.
		if strs, ok := labelsRaw.([]string); ok {
			labelsSlice = make([]any, len(strs))
			for i, s := range strs {
				labelsSlice[i] = s
			}
		} else {
			return nil, fmt.Errorf("parameter labels must be an array")
		}
	}

	payload := make([]GranularIssueLabel, 0, len(labelsSlice))
	for _, item := range labelsSlice {
		switch v := item.(type) {
		case string:
			payload = append(payload, GranularIssueLabel{Name: v, StringForm: true})
		case map[string]any:
			name, err := RequiredParam[string](v, "name")
			if err != nil {
				return nil, fmt.Errorf("each label object must have a 'name' string")
			}
			rationale, err := OptionalParam[string](v, "rationale")
			if err != nil {
				return nil, err
			}
			rationale = strings.TrimSpace(rationale)
			if len([]rune(rationale)) > 280 {
				return nil, fmt.Errorf("label rationale must be 280 characters or less")
			}
			confidence, err := OptionalParam[string](v, "confidence")
			if err != nil {
				return nil, err
			}
			confidence = normalizeConfidence(confidence)
			if confidence != "" && confidence != "LOW" && confidence != "MEDIUM" && confidence != "HIGH" {
				return nil, fmt.Errorf("confidence must be one of: LOW, MEDIUM, HIGH")
			}
			isSuggestion, err := OptionalParam[bool](v, "is_suggestion")
			if err != nil {
				return nil, err
			}
			if rationale == "" && !isSuggestion && confidence == "" {
				payload = append(payload, GranularIssueLabel{Name: name, StringForm: true})
			} else {

				payload = append(payload, GranularIssueLabel{Name: name, Rationale: rationale, Confidence: confidence, IsSuggestion: isSuggestion})
			}
		default:
			return nil, fmt.Errorf("each label must be a string or an object with 'name' and optional 'rationale', 'confidence', and/or 'is_suggestion'")
		}
	}

	return marshalGranularIssueArguments(args, GranularIssueLabelsInput{GranularIssueCoordinate: GranularIssueCoordinate{Owner: owner, Repo: repo, IssueNumber: issueNumber}, Labels: payload})
}

func normalizeGranularUpdateIssueTypeArguments(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}

	owner, err := RequiredParam[string](args, "owner")
	if err != nil {
		return nil, err
	}
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return nil, err
	}
	issueNumber, err := RequiredInt(args, "issue_number")
	if err != nil {
		return nil, err
	}
	issueType, issueTypeProvided, err := OptionalNullableStringParam(args, "issue_type")
	if err != nil {
		return nil, err
	}
	if !issueTypeProvided {
		return nil, fmt.Errorf("missing required parameter: issue_type")
	}
	rationale, err := OptionalParam[string](args, "rationale")
	if err != nil {
		return nil, err
	}
	rationale = strings.TrimSpace(rationale)
	if len([]rune(rationale)) > 280 {
		return nil, fmt.Errorf("parameter rationale must be 280 characters or less")
	}
	confidence, err := OptionalParam[string](args, "confidence")
	if err != nil {
		return nil, err
	}
	confidence = normalizeConfidence(confidence)
	if confidence != "" && confidence != "LOW" && confidence != "MEDIUM" && confidence != "HIGH" {
		return nil, fmt.Errorf("confidence must be one of: LOW, MEDIUM, HIGH")
	}
	isSuggestion, err := OptionalParam[bool](args, "is_suggestion")
	if err != nil {
		return nil, err
	}
	if issueType == nil && (rationale != "" || confidence != "" || isSuggestion) {
		return nil, fmt.Errorf("suggestion metadata is not supported when removing an issue type; omit rationale, confidence, and is_suggestion")
	}
	return marshalGranularIssueArguments(args, GranularIssueTypeInput{GranularIssueCoordinate: GranularIssueCoordinate{Owner: owner, Repo: repo, IssueNumber: issueNumber}, IssueType: issueType, Rationale: rationale, Confidence: confidence, IsSuggestion: isSuggestion})
}

func normalizeGranularUpdateIssueStateArguments(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}

	owner, err := RequiredParam[string](args, "owner")
	if err != nil {
		return nil, err
	}
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return nil, err
	}
	issueNumber, err := RequiredInt(args, "issue_number")
	if err != nil {
		return nil, err
	}
	state, err := RequiredParam[string](args, "state")
	if err != nil {
		return nil, err
	}
	stateReason, err := OptionalParam[string](args, "state_reason")
	if err != nil {
		return nil, err
	}
	rationale, err := OptionalParam[string](args, "rationale")
	if err != nil {
		return nil, err
	}
	rationale = strings.TrimSpace(rationale)
	if len([]rune(rationale)) > 280 {
		return nil, fmt.Errorf("parameter rationale must be 280 characters or less")
	}
	confidence, err := OptionalParam[string](args, "confidence")
	if err != nil {
		return nil, err
	}
	confidence = normalizeConfidence(confidence)
	if confidence != "" && confidence != "LOW" && confidence != "MEDIUM" && confidence != "HIGH" {
		return nil, fmt.Errorf("confidence must be one of: LOW, MEDIUM, HIGH")
	}
	isSuggestion, err := OptionalParam[bool](args, "is_suggestion")
	if err != nil {
		return nil, err
	}
	duplicateOf, err := OptionalIntParam(args, "duplicate_of")
	if err != nil {
		return nil, err
	}
	if stateReason != "" && state != "closed" {
		return nil, fmt.Errorf("state_reason can only be used when state is 'closed'")
	}
	if duplicateOf != 0 && stateReason != "duplicate" {
		return nil, fmt.Errorf("duplicate_of can only be used when state_reason is 'duplicate'")
	}
	if isSuggestion && stateReason == "duplicate" && duplicateOf == 0 {
		return nil, fmt.Errorf("duplicate_of is required when suggesting a close as duplicate")
	}
	if duplicateOf == 0 {
		delete(args, "duplicate_of")
	}

	return marshalGranularIssueArguments(args, GranularIssueStateInput{GranularIssueCoordinate: GranularIssueCoordinate{Owner: owner, Repo: repo, IssueNumber: issueNumber}, State: state, StateReason: stateReason, Rationale: rationale, Confidence: confidence, IsSuggestion: isSuggestion, DuplicateOf: duplicateOf})
}

func normalizeGranularAddSubIssueArguments(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}

	owner, err := RequiredParam[string](args, "owner")
	if err != nil {
		return nil, err
	}
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return nil, err
	}
	issueNumber, err := RequiredInt(args, "issue_number")
	if err != nil {
		return nil, err
	}
	subIssueID, err := RequiredInt(args, "sub_issue_id")
	if err != nil {
		return nil, err
	}
	replaceParent, _ := OptionalParam[bool](args, "replace_parent")
	// Like body on create, invalid replace_parent values historically defaulted.
	if _, provided := args["replace_parent"]; provided {
		args["replace_parent"] = replaceParent
	}

	return marshalGranularIssueArguments(args, GranularAddSubIssueInput{GranularIssueCoordinate: GranularIssueCoordinate{Owner: owner, Repo: repo, IssueNumber: issueNumber}, SubIssueID: subIssueID, ReplaceParent: replaceParent})
}

func normalizeGranularRemoveSubIssueArguments(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}

	owner, err := RequiredParam[string](args, "owner")
	if err != nil {
		return nil, err
	}
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return nil, err
	}
	issueNumber, err := RequiredInt(args, "issue_number")
	if err != nil {
		return nil, err
	}
	subIssueID, err := RequiredInt(args, "sub_issue_id")
	if err != nil {
		return nil, err
	}

	return marshalGranularIssueArguments(args, GranularRemoveSubIssueInput{GranularIssueCoordinate: GranularIssueCoordinate{Owner: owner, Repo: repo, IssueNumber: issueNumber}, SubIssueID: subIssueID})
}

func normalizeGranularReprioritizeSubIssueArguments(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}

	owner, err := RequiredParam[string](args, "owner")
	if err != nil {
		return nil, err
	}
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return nil, err
	}
	issueNumber, err := RequiredInt(args, "issue_number")
	if err != nil {
		return nil, err
	}
	subIssueID, err := RequiredInt(args, "sub_issue_id")
	if err != nil {
		return nil, err
	}
	afterID, err := OptionalIntParam(args, "after_id")
	if err != nil {
		return nil, err
	}
	beforeID, err := OptionalIntParam(args, "before_id")
	if err != nil {
		return nil, err
	}
	if afterID == 0 {
		delete(args, "after_id")
	}
	if beforeID == 0 {
		delete(args, "before_id")
	}

	return marshalGranularIssueArguments(args, GranularReprioritizeSubIssueInput{GranularIssueCoordinate: GranularIssueCoordinate{Owner: owner, Repo: repo, IssueNumber: issueNumber}, SubIssueID: subIssueID, AfterID: afterID, BeforeID: beforeID})
}

func granularIssueInputNormalizer(normalize inventory.InputNormalizer) inventory.InputNormalizer {
	return func(raw json.RawMessage) (json.RawMessage, error) {
		normalized, err := normalize(raw)
		if err != nil {
			return nil, &inventory.ToolInputError{Message: err.Error()}
		}
		return normalized, nil
	}
}

func normalizeGranularSetIssueFieldsArguments(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}

	owner, err := RequiredParam[string](args, "owner")
	if err != nil {
		return nil, err
	}
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return nil, err
	}
	issueNumber, err := RequiredInt(args, "issue_number")
	if err != nil {
		return nil, err
	}

	fieldsRaw, ok := args["fields"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: fields")
	}

	// Accept both []any and []map[string]any input forms
	var fieldMaps []map[string]any
	switch v := fieldsRaw.(type) {
	case []any:
		for _, f := range v {
			fieldMap, ok := f.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("each field must be an object with 'field_id' and a value")
			}
			fieldMaps = append(fieldMaps, fieldMap)
		}
	case []map[string]any:
		fieldMaps = v
	default:
		return nil, fmt.Errorf("invalid parameter: fields must be an array")
	}
	if len(fieldMaps) == 0 {
		return nil, fmt.Errorf("fields array must not be empty")
	}

	issueFields := make([]IssueFieldCreateOrUpdateInput, 0, len(fieldMaps))
	for _, fieldMap := range fieldMaps {
		fieldID, err := RequiredParam[string](fieldMap, "field_id")
		if err != nil {
			return nil, fmt.Errorf("field_id is required and must be a string")
		}

		input := IssueFieldCreateOrUpdateInput{
			FieldID: githubv4.ID(fieldID),
		}

		// Count how many value keys are present; exactly one is required.
		valueCount := 0

		if v, err := OptionalParam[string](fieldMap, "text_value"); err == nil && v != "" {
			input.TextValue = githubv4.NewString(githubv4.String(v))
			valueCount++
		}
		if v, err := OptionalParam[float64](fieldMap, "number_value"); err == nil {
			if _, exists := fieldMap["number_value"]; exists {
				gqlFloat := githubv4.Float(v)
				input.NumberValue = &gqlFloat
				valueCount++
			}
		}
		if v, err := OptionalParam[string](fieldMap, "date_value"); err == nil && v != "" {
			input.DateValue = githubv4.NewString(githubv4.String(v))
			valueCount++
		}
		if v, err := OptionalParam[string](fieldMap, "single_select_option_id"); err == nil && v != "" {
			optionID := githubv4.ID(v)
			input.SingleSelectOptionID = &optionID
			valueCount++
		}
		if _, exists := fieldMap["delete"]; exists {
			del, err := OptionalParam[bool](fieldMap, "delete")
			if err == nil && del {
				deleteVal := githubv4.Boolean(true)
				input.Delete = &deleteVal
				valueCount++
			}
		}

		if valueCount == 0 {
			return nil, fmt.Errorf("each field must have a value (text_value, number_value, date_value, single_select_option_id) or delete: true")
		}
		if valueCount > 1 {
			return nil, fmt.Errorf("each field must have exactly one value (text_value, number_value, date_value, single_select_option_id) or delete: true, but multiple were provided")
		}

		if _, exists := fieldMap["rationale"]; exists {
			rationale, err := OptionalParam[string](fieldMap, "rationale")
			if err != nil {
				return nil, err
			}
			rationale = strings.TrimSpace(rationale)
			if len([]rune(rationale)) > 280 {
				return nil, fmt.Errorf("field rationale must be 280 characters or less")
			}
			if rationale != "" {
				input.Rationale = githubv4.NewString(githubv4.String(rationale))
			}
		}

		confidence, err := OptionalParam[string](fieldMap, "confidence")
		if err != nil {
			return nil, err
		}
		confidence = normalizeConfidence(confidence)
		if confidence != "" && confidence != "LOW" && confidence != "MEDIUM" && confidence != "HIGH" {
			return nil, fmt.Errorf("confidence must be one of: LOW, MEDIUM, HIGH")
		}
		if confidence != "" {
			input.Confidence = &confidence
		}

		isSuggestion, err := OptionalParam[bool](fieldMap, "is_suggestion")
		if err != nil {
			return nil, err
		}
		if isSuggestion {
			suggestVal := githubv4.Boolean(true)
			input.Suggest = &suggestVal
		}

		issueFields = append(issueFields, input)
	}

	return marshalGranularIssueArguments(args, GranularSetIssueFieldsInput{GranularIssueCoordinate: GranularIssueCoordinate{Owner: owner, Repo: repo, IssueNumber: issueNumber}, Fields: granularIssueFieldsFromMutation(issueFields)})
}

func normalizeGranularAddIssueReactionArguments(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}

	owner, err := RequiredParam[string](args, "owner")
	if err != nil {
		return nil, err
	}
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return nil, err
	}
	issueNumber, err := RequiredInt(args, "issue_number")
	if err != nil {
		return nil, err
	}
	content, err := RequiredParam[string](args, "content")
	if err != nil {
		return nil, err
	}

	return marshalGranularIssueArguments(args, GranularAddIssueReactionInput{GranularIssueCoordinate: GranularIssueCoordinate{Owner: owner, Repo: repo, IssueNumber: issueNumber}, Content: content})
}

func normalizeGranularRemoveIssueReactionArguments(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}

	owner, err := RequiredParam[string](args, "owner")
	if err != nil {
		return nil, err
	}
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return nil, err
	}
	issueNumber, err := RequiredInt(args, "issue_number")
	if err != nil {
		return nil, err
	}
	reactionID, err := RequiredBigInt(args, "reaction_id")
	if err != nil {
		return nil, err
	}

	return marshalGranularIssueArguments(args, GranularRemoveIssueReactionInput{GranularIssueCoordinate: GranularIssueCoordinate{Owner: owner, Repo: repo, IssueNumber: issueNumber}, ReactionID: reactionID})
}

func normalizeGranularAddIssueCommentReactionArguments(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}

	owner, err := RequiredParam[string](args, "owner")
	if err != nil {
		return nil, err
	}
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return nil, err
	}
	commentID, err := RequiredBigInt(args, "comment_id")
	if err != nil {
		return nil, err
	}
	content, err := RequiredParam[string](args, "content")
	if err != nil {
		return nil, err
	}

	return marshalGranularIssueArguments(args, GranularAddIssueCommentReactionInput{Owner: owner, Repo: repo, CommentID: commentID, Content: content})
}

func normalizeGranularRemoveIssueCommentReactionArguments(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}

	owner, err := RequiredParam[string](args, "owner")
	if err != nil {
		return nil, err
	}
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return nil, err
	}
	commentID, err := RequiredBigInt(args, "comment_id")
	if err != nil {
		return nil, err
	}
	reactionID, err := RequiredBigInt(args, "reaction_id")
	if err != nil {
		return nil, err
	}

	return marshalGranularIssueArguments(args, GranularRemoveIssueCommentReactionInput{Owner: owner, Repo: repo, CommentID: commentID, ReactionID: reactionID})
}
