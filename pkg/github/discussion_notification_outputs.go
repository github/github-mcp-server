package github

import (
	"encoding/json"
	"time"

	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
)

type NotificationSubjectOutput struct {
	Title            *string `json:"title,omitempty"`
	URL              *string `json:"url,omitempty"`
	LatestCommentURL *string `json:"latest_comment_url,omitempty"`
	Type             *string `json:"type,omitempty"`
}

type NotificationRepositoryOutput struct {
	ID       *int64  `json:"id,omitempty"`
	Name     *string `json:"name,omitempty"`
	FullName *string `json:"full_name,omitempty"`
	Private  *bool   `json:"private,omitempty"`
	HTMLURL  *string `json:"html_url,omitempty"`
}

type NotificationOutput struct {
	ID         *string                       `json:"id,omitempty"`
	Repository *NotificationRepositoryOutput `json:"repository,omitempty"`
	Subject    *NotificationSubjectOutput    `json:"subject,omitempty"`
	Reason     *string                       `json:"reason,omitempty"`
	Unread     *bool                         `json:"unread,omitempty"`
	UpdatedAt  *time.Time                    `json:"updated_at,omitempty"`
	LastReadAt *time.Time                    `json:"last_read_at,omitempty"`
	URL        *string                       `json:"url,omitempty"`
}

func notificationOutput(n *github.Notification) *NotificationOutput {
	if n == nil {
		return nil
	}
	out := &NotificationOutput{
		ID: n.ID, Reason: n.Reason, Unread: n.Unread, URL: n.URL,
	}
	if n.UpdatedAt != nil {
		out.UpdatedAt = &n.UpdatedAt.Time
	}
	if n.LastReadAt != nil {
		out.LastReadAt = &n.LastReadAt.Time
	}
	if n.Subject != nil {
		out.Subject = &NotificationSubjectOutput{
			Title: n.Subject.Title, URL: n.Subject.URL,
			LatestCommentURL: n.Subject.LatestCommentURL, Type: n.Subject.Type,
		}
	}
	if n.Repository != nil {
		out.Repository = &NotificationRepositoryOutput{
			ID: n.Repository.ID, Name: n.Repository.Name, FullName: n.Repository.FullName,
			Private: n.Repository.Private, HTMLURL: n.Repository.HTMLURL,
		}
	}
	return out
}

type NotificationStatusOutput struct {
	Message string `json:"message"`
}

type NotificationSubscriptionOutput struct {
	Subscribed    *bool      `json:"subscribed,omitempty"`
	Ignored       *bool      `json:"ignored,omitempty"`
	Reason        *string    `json:"reason,omitempty"`
	CreatedAt     *time.Time `json:"created_at,omitempty"`
	URL           *string    `json:"url,omitempty"`
	ThreadURL     *string    `json:"thread_url,omitempty"`
	RepositoryURL *string    `json:"repository_url,omitempty"`
}

type NotificationSubscriptionResult struct {
	Subscription *NotificationSubscriptionOutput `json:"subscription,omitempty"`
	Message      string                          `json:"message,omitempty"`
}

func (out NotificationSubscriptionResult) MarshalJSON() ([]byte, error) {
	if out.Subscription != nil {
		return json.Marshal(struct {
			Subscription *NotificationSubscriptionOutput `json:"subscription"`
		}{out.Subscription})
	}
	return json.Marshal(NotificationStatusOutput{Message: out.Message})
}

func notificationSubscriptionOutput(s *github.Subscription) *NotificationSubscriptionResult {
	out := &NotificationSubscriptionOutput{}
	if s != nil {
		out.Subscribed = s.Subscribed
		out.Ignored = s.Ignored
		out.Reason = s.Reason
		out.URL = s.URL
		out.ThreadURL = s.ThreadURL
		out.RepositoryURL = s.RepositoryURL
		if s.CreatedAt != nil {
			out.CreatedAt = &s.CreatedAt.Time
		}
	}
	return &NotificationSubscriptionResult{Subscription: out}
}

func notificationSubscriptionResultSchema() *jsonschema.Schema {
	subscription, err := jsonschema.For[NotificationSubscriptionOutput](nil)
	if err != nil {
		panic(err)
	}
	return &jsonschema.Schema{
		Type: "object",
		OneOf: []*jsonschema.Schema{
			{
				Type:                 "object",
				Properties:           map[string]*jsonschema.Schema{"subscription": subscription},
				Required:             []string{"subscription"},
				AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
			},
			{
				Type:                 "object",
				Properties:           map[string]*jsonschema.Schema{"message": {Type: "string"}},
				Required:             []string{"message"},
				AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
			},
		},
	}
}

type DiscussionPageInfoOutput struct {
	HasNextPage     bool   `json:"hasNextPage"`
	HasPreviousPage bool   `json:"hasPreviousPage"`
	StartCursor     string `json:"startCursor"`
	EndCursor       string `json:"endCursor"`
}

type DiscussionListItemOutput struct {
	Number    *int       `json:"number,omitempty"`
	Title     *string    `json:"title,omitempty"`
	HTMLURL   *string    `json:"html_url,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	Author    string     `json:"author"`
	Category  string     `json:"category"`
}

type ListDiscussionsOutput struct {
	Discussions []*DiscussionListItemOutput `json:"discussions"`
	PageInfo    DiscussionPageInfoOutput    `json:"pageInfo"`
	TotalCount  int                         `json:"totalCount"`
}

type DiscussionCategoryOutput struct {
	Name string `json:"name"`
}

type DiscussionOutput struct {
	Number         int                      `json:"number"`
	Title          string                   `json:"title"`
	Body           string                   `json:"body"`
	URL            string                   `json:"url"`
	Closed         bool                     `json:"closed"`
	IsAnswered     bool                     `json:"isAnswered"`
	CreatedAt      time.Time                `json:"createdAt"`
	Category       DiscussionCategoryOutput `json:"category"`
	AnswerChosenAt *time.Time               `json:"answerChosenAt,omitempty"`
}

type DiscussionReplyOutput struct {
	ID       string `json:"id"`
	Body     string `json:"body"`
	IsAnswer bool   `json:"isAnswer,omitempty"`
}

type DiscussionCommentOutput struct {
	ID              string                  `json:"id"`
	Body            string                  `json:"body"`
	IsAnswer        bool                    `json:"isAnswer,omitempty"`
	Replies         []DiscussionReplyOutput `json:"replies,omitempty"`
	ReplyTotalCount int                     `json:"replyTotalCount,omitempty"`
}

type DiscussionCommentsOutput struct {
	Comments   []DiscussionCommentOutput `json:"comments"`
	PageInfo   DiscussionPageInfoOutput  `json:"pageInfo"`
	TotalCount int                       `json:"totalCount"`
}

func discussionCommentOutputs(comments []MinimalDiscussionComment) []DiscussionCommentOutput {
	out := make([]DiscussionCommentOutput, 0, len(comments))
	for _, comment := range comments {
		item := DiscussionCommentOutput{
			ID: comment.ID, Body: comment.Body, IsAnswer: comment.IsAnswer,
			ReplyTotalCount: comment.ReplyTotalCount,
		}
		for _, reply := range comment.Replies {
			item.Replies = append(item.Replies, DiscussionReplyOutput{
				ID: reply.ID, Body: reply.Body, IsAnswer: reply.IsAnswer,
			})
		}
		out = append(out, item)
	}
	return out
}

type DiscussionCategoryItemOutput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type DiscussionCategoriesOutput struct {
	Categories []DiscussionCategoryItemOutput `json:"categories"`
	PageInfo   DiscussionPageInfoOutput       `json:"pageInfo"`
	TotalCount int                            `json:"totalCount"`
}

type DiscussionAnswerOutput struct {
	DiscussionID  string `json:"discussionID"`
	DiscussionURL string `json:"discussionURL"`
}

// The comment mutations return id/url; answer mutations return a discussion
// reference instead. Marshal the selected concrete variant without an envelope.
type DiscussionCommentWriteOutput struct {
	Comment *MinimalResponse
	Answer  *DiscussionAnswerOutput
}

func (out DiscussionCommentWriteOutput) MarshalJSON() ([]byte, error) {
	if out.Answer != nil {
		return json.Marshal(out.Answer)
	}
	if out.Comment != nil {
		return json.Marshal(out.Comment)
	}
	// The SDK validates an unpointered zero value even for IsError results.
	// The inventory middleware removes this zero output before it is sent.
	return json.Marshal(MinimalResponse{})
}

func discussionCommentWriteOutputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "object",
		OneOf: []*jsonschema.Schema{
			{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"id": {Type: "string"}, "url": {Type: "string"},
				},
				Required:             []string{"id", "url"},
				AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
			},
			{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"discussionID": {Type: "string"}, "discussionURL": {Type: "string"},
				},
				Required:             []string{"discussionID", "discussionURL"},
				AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
			},
		},
	}
}
