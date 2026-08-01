// Copyright 2014 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// Subscription identifies a repository or thread subscription.
type Subscription struct {
	Subscribed *bool      `json:"subscribed,omitempty"`
	Ignored    *bool      `json:"ignored,omitempty"`
	Reason     *string    `json:"reason,omitempty"`
	CreatedAt  *Timestamp `json:"created_at,omitempty"`
	URL        *string    `json:"url,omitempty"`

	// only populated for repository subscriptions
	RepositoryURL *string `json:"repository_url,omitempty"`

	// only populated for thread subscriptions
	ThreadURL *string `json:"thread_url,omitempty"`
}

// ListWatchers lists watchers of a particular repo.
//
// GitHub API docs: https://docs.github.com/rest/activity/watching?apiVersion=2022-11-28#list-watchers
//
//meta:operation GET /repos/{owner}/{repo}/subscribers
func (s *ActivityService) ListWatchers(ctx context.Context, owner, repo string, opts *ListOptions) ([]*User, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/subscribers", owner, repo)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var watchers []*User
	resp, err := s.client.Do(req, &watchers)
	if err != nil {
		return nil, resp, err
	}

	return watchers, resp, nil
}

// ListWatched lists the repositories the specified user is watching. Passing
// the empty string will fetch watched repos for the authenticated user.
//
// GitHub API docs: https://docs.github.com/rest/activity/watching?apiVersion=2022-11-28#list-repositories-watched-by-a-user
//
// GitHub API docs: https://docs.github.com/rest/activity/watching?apiVersion=2022-11-28#list-repositories-watched-by-the-authenticated-user
//
//meta:operation GET /user/subscriptions
//meta:operation GET /users/{username}/subscriptions
func (s *ActivityService) ListWatched(ctx context.Context, user string, opts *ListOptions) ([]*Repository, *Response, error) {
	var u string
	if user != "" {
		u = fmt.Sprintf("users/%v/subscriptions", user)
	} else {
		u = "user/subscriptions"
	}
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var watched []*Repository
	resp, err := s.client.Do(req, &watched)
	if err != nil {
		return nil, resp, err
	}

	return watched, resp, nil
}

// GetRepositorySubscription returns the subscription for the specified
// repository for the authenticated user. If the authenticated user is not
// watching the repository, a nil Subscription is returned.
//
// GitHub API docs: https://docs.github.com/rest/activity/watching?apiVersion=2022-11-28#get-a-repository-subscription
//
//meta:operation GET /repos/{owner}/{repo}/subscription
func (s *ActivityService) GetRepositorySubscription(ctx context.Context, owner, repo string) (*Subscription, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/subscription", owner, repo)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var sub *Subscription
	resp, err := s.client.Do(req, &sub)
	if err != nil {
		// if it's just a 404, don't return that as an error
		_, err = parseBoolResponse(err)
		return nil, resp, err
	}

	return sub, resp, nil
}

// SetRepositorySubscription sets the subscription for the specified repository
// for the authenticated user.
//
// To watch a repository, set subscription.Subscribed to true.
// To ignore notifications made within a repository, set subscription.Ignored to true.
// To stop watching a repository, use DeleteRepositorySubscription.
//
// GitHub API docs: https://docs.github.com/rest/activity/watching?apiVersion=2022-11-28#set-a-repository-subscription
//
//meta:operation PUT /repos/{owner}/{repo}/subscription
func (s *ActivityService) SetRepositorySubscription(ctx context.Context, owner, repo string, body *Subscription) (*Subscription, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/subscription", owner, repo)

	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, nil, err
	}

	var sub *Subscription
	resp, err := s.client.Do(req, &sub)
	if err != nil {
		return nil, resp, err
	}

	return sub, resp, nil
}

// DeleteRepositorySubscription deletes the subscription for the specified
// repository for the authenticated user.
//
// This is used to stop watching a repository. To control whether or not to
// receive notifications from a repository, use SetRepositorySubscription.
//
// GitHub API docs: https://docs.github.com/rest/activity/watching?apiVersion=2022-11-28#delete-a-repository-subscription
//
//meta:operation DELETE /repos/{owner}/{repo}/subscription
func (s *ActivityService) DeleteRepositorySubscription(ctx context.Context, owner, repo string) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/subscription", owner, repo)
	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
