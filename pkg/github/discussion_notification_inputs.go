package github

import (
	"encoding/json"
	"strings"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/go-viper/mapstructure/v2"
)

type ListNotificationsInput struct {
	Filter  *string `json:"filter,omitempty"`
	Since   *string `json:"since,omitempty"`
	Before  *string `json:"before,omitempty"`
	Owner   *string `json:"owner,omitempty"`
	Repo    *string `json:"repo,omitempty"`
	Page    *int    `json:"page,omitempty"`
	PerPage *int    `json:"perPage,omitempty"`
	After   *string `json:"after,omitempty"`
}

type DismissNotificationInput struct {
	ThreadID string `json:"threadID"`
	State    string `json:"state"`
}

type MarkAllNotificationsReadInput struct {
	LastReadAt *string `json:"lastReadAt,omitempty"`
	Owner      *string `json:"owner,omitempty"`
	Repo       *string `json:"repo,omitempty"`
}

type GetNotificationDetailsInput struct {
	NotificationID string `json:"notificationID"`
}

type ManageNotificationSubscriptionInput struct {
	NotificationID string `json:"notificationID"`
	Action         string `json:"action"`
}

type ManageRepositoryNotificationSubscriptionInput struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Action string `json:"action"`
}

type ListDiscussionsInput struct {
	Owner     string  `json:"owner"`
	Repo      *string `json:"repo,omitempty"`
	Category  *string `json:"category,omitempty"`
	OrderBy   *string `json:"orderBy,omitempty"`
	Direction *string `json:"direction,omitempty"`
	PerPage   *int    `json:"perPage,omitempty"`
	After     *string `json:"after,omitempty"`
}

type GetDiscussionInput struct {
	Owner            string `json:"owner"`
	Repo             string `json:"repo"`
	DiscussionNumber int32  `json:"discussionNumber"`
}

type GetDiscussionCommentsInput struct {
	Owner            string  `json:"owner"`
	Repo             string  `json:"repo"`
	DiscussionNumber int32   `json:"discussionNumber"`
	IncludeReplies   *bool   `json:"includeReplies,omitempty"`
	PerPage          *int    `json:"perPage,omitempty"`
	After            *string `json:"after,omitempty"`
}

type DiscussionCommentWriteInput struct {
	Method           string  `json:"method"`
	Owner            *string `json:"owner,omitempty"`
	Repo             *string `json:"repo,omitempty"`
	DiscussionNumber *int    `json:"discussionNumber,omitempty"`
	Body             *string `json:"body,omitempty"`
	CommentNodeID    *string `json:"commentNodeID,omitempty"`
}

type ListDiscussionCategoriesInput struct {
	Owner string  `json:"owner"`
	Repo  *string `json:"repo,omitempty"`
}

// Preserve presence and the existing parameter validators at the compatibility
// boundary. In particular, an explicit perPage: 0 differs from an omitted value
// for discussion comments, and mutation parameters are required per method.
func discussionNotificationArguments(input any) (map[string]any, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var args map[string]any
	err = json.Unmarshal(raw, &args)
	return args, err
}

func normalizeDiscussionReadArguments(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	// These two tools historically used WeakDecode: retain its case-insensitive
	// field matching, numeric-string conversion and fractional truncation.
	var input GetDiscussionInput
	if err := mapstructure.WeakDecode(args, &input); err != nil {
		return nil, err
	}
	for field, value := range map[string]any{
		"owner": input.Owner, "repo": input.Repo, "discussionNumber": input.DiscussionNumber,
	} {
		for source := range args {
			if strings.EqualFold(source, field) {
				args[field] = value
				break
			}
		}
	}
	return json.Marshal(args)
}

func normalizeDiscussionNotificationIntegers(fields ...string) inventory.InputNormalizer {
	return func(raw json.RawMessage) (json.RawMessage, error) {
		var args map[string]any
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}

		for _, field := range fields {
			if _, ok := args[field]; !ok {
				continue
			}
			value, err := OptionalIntParam(args, field)
			if err != nil {
				return nil, err
			}
			if value == 0 {
				switch field {
				case "perPage":
					value = 30
				case "page":
					value = 1
				}
			}
			args[field] = value
		}
		return json.Marshal(args)
	}
}

func normalizeDiscussionCommentWriteArguments(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	method, _ := args["method"].(string)
	used := map[string]bool{"method": true}
	switch method {
	case "add", "reply":
		used["owner"], used["repo"], used["discussionNumber"], used["body"] = true, true, true, true
		if method == "reply" {
			used["commentNodeID"] = true
		}
	case "update":
		used["commentNodeID"], used["body"] = true, true
	case "delete", "mark_answer", "unmark_answer":
		used["commentNodeID"] = true
	}
	// Unused method-specific parameters were never decoded by the legacy
	// handler, even when their values did not match the advertised schema.
	for _, field := range []string{"owner", "repo", "discussionNumber", "body", "commentNodeID"} {
		if !used[field] {
			delete(args, field)
		}
	}
	normalized, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	return normalizeDiscussionNotificationIntegers("discussionNumber")(normalized)
}
