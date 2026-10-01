package github

import (
	"time"

	"github.com/google/go-github/v89/github"
)

// NotificationSubjectOutput identifies the issue, pull request, comment, or discussion behind a notification.
type NotificationSubjectOutput struct {
	Title            *string `json:"title,omitempty"`
	URL              *string `json:"url,omitempty"`
	LatestCommentURL *string `json:"latest_comment_url,omitempty"`
	Type             *string `json:"type,omitempty"`
}

// NotificationRepositoryOwnerOutput is the compact repository owner reference used by notifications.
type NotificationRepositoryOwnerOutput struct {
	Login     *string `json:"login,omitempty"`
	ID        *int64  `json:"id,omitempty"`
	NodeID    *string `json:"node_id,omitempty"`
	AvatarURL *string `json:"avatar_url,omitempty"`
	HTMLURL   *string `json:"html_url,omitempty"`
}

// NotificationRepositoryOutput identifies the repository associated with a notification.
type NotificationRepositoryOutput struct {
	ID       *int64                             `json:"id,omitempty"`
	NodeID   *string                            `json:"node_id,omitempty"`
	Name     *string                            `json:"name,omitempty"`
	FullName *string                            `json:"full_name,omitempty"`
	Private  *bool                              `json:"private,omitempty"`
	Owner    *NotificationRepositoryOwnerOutput `json:"owner,omitempty"`
	HTMLURL  *string                            `json:"html_url,omitempty"`
	URL      *string                            `json:"url,omitempty"`
}

// NotificationOutput is the compact typed output for notification read tools.
type NotificationOutput struct {
	ID         *string                       `json:"id,omitempty"`
	Repository *NotificationRepositoryOutput `json:"repository,omitempty"`
	Subject    *NotificationSubjectOutput    `json:"subject,omitempty"`
	Reason     *string                       `json:"reason,omitempty"`
	Unread     *bool                         `json:"unread,omitempty"`
	UpdatedAt  *string                       `json:"updated_at,omitempty"`
	LastReadAt *string                       `json:"last_read_at,omitempty"`
	URL        *string                       `json:"url,omitempty"`
}

func convertNotificationOutput(notification *github.Notification) *NotificationOutput {
	if notification == nil {
		return nil
	}
	return &NotificationOutput{
		ID:         notification.ID,
		Repository: convertNotificationRepositoryOutput(notification.Repository),
		Subject:    convertNotificationSubjectOutput(notification.Subject),
		Reason:     notification.Reason,
		Unread:     notification.Unread,
		UpdatedAt:  notificationTimestampOutput(notification.UpdatedAt),
		LastReadAt: notificationTimestampOutput(notification.LastReadAt),
		URL:        notification.URL,
	}
}

func convertNotificationSubjectOutput(subject *github.NotificationSubject) *NotificationSubjectOutput {
	if subject == nil {
		return nil
	}
	return &NotificationSubjectOutput{
		Title:            subject.Title,
		URL:              subject.URL,
		LatestCommentURL: subject.LatestCommentURL,
		Type:             subject.Type,
	}
}

func convertNotificationRepositoryOutput(repository *github.Repository) *NotificationRepositoryOutput {
	if repository == nil {
		return nil
	}
	return &NotificationRepositoryOutput{
		ID:       repository.ID,
		NodeID:   repository.NodeID,
		Name:     repository.Name,
		FullName: repository.FullName,
		Private:  repository.Private,
		Owner:    convertNotificationRepositoryOwnerOutput(repository.Owner),
		HTMLURL:  repository.HTMLURL,
		URL:      repository.URL,
	}
}

func convertNotificationRepositoryOwnerOutput(owner *github.User) *NotificationRepositoryOwnerOutput {
	if owner == nil {
		return nil
	}
	return &NotificationRepositoryOwnerOutput{
		Login:     owner.Login,
		ID:        owner.ID,
		NodeID:    owner.NodeID,
		AvatarURL: owner.AvatarURL,
		HTMLURL:   owner.HTMLURL,
	}
}

func notificationTimestampOutput(timestamp *github.Timestamp) *string {
	if timestamp == nil {
		return nil
	}
	value := timestamp.Time.Format(time.RFC3339Nano)
	return &value
}
