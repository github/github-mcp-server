package github

import (
	"time"

	"github.com/github/github-mcp-server/pkg/sanitize"
	"github.com/google/go-github/v89/github"
	"github.com/shurcooL/githubv4"
)

// DiscussionAuthorOutput is the compact author reference returned by list_discussions.
type DiscussionAuthorOutput struct {
	Login *string `json:"login,omitempty"`
}

// DiscussionCategorySummaryOutput is the category shape nested in discussion results.
type DiscussionCategorySummaryOutput struct {
	Name *string `json:"name,omitempty"`
}

// DiscussionListItemOutput is the compact discussion shape returned by list_discussions.
type DiscussionListItemOutput struct {
	Number    *int                             `json:"number,omitempty"`
	Title     *string                          `json:"title,omitempty"`
	HTMLURL   *string                          `json:"html_url,omitempty"`
	CreatedAt *string                          `json:"created_at,omitempty"`
	UpdatedAt *string                          `json:"updated_at,omitempty"`
	User      *DiscussionAuthorOutput          `json:"user,omitempty"`
	Category  *DiscussionCategorySummaryOutput `json:"category,omitempty"`
}

// DiscussionPageInfoOutput describes the cursor window returned by discussion list tools.
type DiscussionPageInfoOutput struct {
	HasNextPage     bool   `json:"hasNextPage"`
	HasPreviousPage bool   `json:"hasPreviousPage"`
	StartCursor     string `json:"startCursor"`
	EndCursor       string `json:"endCursor"`
}

// ListDiscussionsOutput is the typed paginated output for list_discussions.
type ListDiscussionsOutput struct {
	Discussions []*DiscussionListItemOutput `json:"discussions"`
	PageInfo    DiscussionPageInfoOutput    `json:"pageInfo"`
	TotalCount  int                         `json:"totalCount"`
}

// DiscussionDetailCategoryOutput is the required category shape returned by get_discussion.
type DiscussionDetailCategoryOutput struct {
	Name string `json:"name"`
}

// DiscussionDetailOutput is the typed output for get_discussion.
type DiscussionDetailOutput struct {
	Number         int                            `json:"number"`
	Title          string                         `json:"title"`
	Body           string                         `json:"body"`
	URL            string                         `json:"url"`
	Closed         bool                           `json:"closed"`
	IsAnswered     bool                           `json:"isAnswered"`
	CreatedAt      string                         `json:"createdAt"`
	Category       DiscussionDetailCategoryOutput `json:"category"`
	AnswerChosenAt *string                        `json:"answerChosenAt,omitempty"`
}

// DiscussionReplyOutput is one nested reply returned when includeReplies is true.
type DiscussionReplyOutput struct {
	ID       string `json:"id"`
	Body     string `json:"body"`
	IsAnswer bool   `json:"isAnswer"`
}

// DiscussionCommentOutput is one top-level discussion comment.
type DiscussionCommentOutput struct {
	ID              string                   `json:"id"`
	Body            string                   `json:"body"`
	IsAnswer        bool                     `json:"isAnswer"`
	Replies         *[]DiscussionReplyOutput `json:"replies,omitempty"`
	ReplyTotalCount *int                     `json:"replyTotalCount,omitempty"`
}

// DiscussionCommentsOutput is the typed paginated output for get_discussion_comments.
type DiscussionCommentsOutput struct {
	Comments   []DiscussionCommentOutput `json:"comments"`
	PageInfo   DiscussionPageInfoOutput  `json:"pageInfo"`
	TotalCount int                       `json:"totalCount"`
}

// DiscussionCategoryOutput identifies one repository discussion category.
type DiscussionCategoryOutput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DiscussionCategoriesOutput is the typed output for list_discussion_categories.
type DiscussionCategoriesOutput struct {
	Categories []DiscussionCategoryOutput `json:"categories"`
	PageInfo   DiscussionPageInfoOutput   `json:"pageInfo"`
	TotalCount int                        `json:"totalCount"`
}

func convertDiscussionListItemOutput(discussion *github.Discussion) *DiscussionListItemOutput {
	if discussion == nil {
		return nil
	}
	return &DiscussionListItemOutput{
		Number:    discussion.Number,
		Title:     discussion.Title,
		HTMLURL:   discussion.HTMLURL,
		CreatedAt: discussionTimestampOutput(discussion.CreatedAt),
		UpdatedAt: discussionTimestampOutput(discussion.UpdatedAt),
		User:      convertDiscussionAuthorOutput(discussion.User),
		Category:  convertDiscussionCategorySummaryOutput(discussion.DiscussionCategory),
	}
}

func convertDiscussionAuthorOutput(author *github.User) *DiscussionAuthorOutput {
	if author == nil {
		return nil
	}
	return &DiscussionAuthorOutput{Login: author.Login}
}

func convertDiscussionCategorySummaryOutput(category *github.DiscussionCategory) *DiscussionCategorySummaryOutput {
	if category == nil {
		return nil
	}
	return &DiscussionCategorySummaryOutput{Name: category.Name}
}

func newDiscussionDetailOutput(
	number githubv4.Int,
	title githubv4.String,
	body githubv4.String,
	url githubv4.String,
	closed githubv4.Boolean,
	isAnswered githubv4.Boolean,
	createdAt githubv4.DateTime,
	categoryName githubv4.String,
	answerChosenAt *githubv4.DateTime,
) *DiscussionDetailOutput {
	output := &DiscussionDetailOutput{
		Number:     int(number),
		Title:      sanitizeDiscussionText(string(title)),
		Body:       sanitizeDiscussionText(string(body)),
		URL:        string(url),
		Closed:     bool(closed),
		IsAnswered: bool(isAnswered),
		CreatedAt:  createdAt.Time.Format(time.RFC3339Nano),
		Category: DiscussionDetailCategoryOutput{
			Name: string(categoryName),
		},
	}
	if answerChosenAt != nil {
		value := answerChosenAt.Time.Format(time.RFC3339Nano)
		output.AnswerChosenAt = &value
	}
	return output
}

func newDiscussionCommentsOutput(
	comments []MinimalDiscussionComment,
	includeReplies bool,
	pageInfo DiscussionPageInfoOutput,
	totalCount int,
) *DiscussionCommentsOutput {
	outputComments := make([]DiscussionCommentOutput, len(comments))
	for i, comment := range comments {
		outputComments[i] = DiscussionCommentOutput{
			ID:       comment.ID,
			Body:     comment.Body,
			IsAnswer: comment.IsAnswer,
		}
		if includeReplies {
			replies := make([]DiscussionReplyOutput, len(comment.Replies))
			for j, reply := range comment.Replies {
				replies[j] = DiscussionReplyOutput{
					ID:       reply.ID,
					Body:     reply.Body,
					IsAnswer: reply.IsAnswer,
				}
			}
			replyTotalCount := comment.ReplyTotalCount
			outputComments[i].Replies = &replies
			outputComments[i].ReplyTotalCount = &replyTotalCount
		}
	}
	return &DiscussionCommentsOutput{
		Comments:   outputComments,
		PageInfo:   pageInfo,
		TotalCount: totalCount,
	}
}

func discussionTimestampOutput(timestamp *github.Timestamp) *string {
	if timestamp == nil {
		return nil
	}
	value := timestamp.Time.Format(time.RFC3339Nano)
	return &value
}

func sanitizeDiscussionText(text string) string {
	return sanitize.Sanitize(text)
}
