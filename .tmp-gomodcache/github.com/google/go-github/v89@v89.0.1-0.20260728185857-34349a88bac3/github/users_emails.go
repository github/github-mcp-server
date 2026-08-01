// Copyright 2013 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import "context"

// UserEmail represents user's email address.
type UserEmail struct {
	Email      *string `json:"email,omitempty"`
	Primary    *bool   `json:"primary,omitempty"`
	Verified   *bool   `json:"verified,omitempty"`
	Visibility *string `json:"visibility,omitempty"`
}

// ListEmails lists all email addresses for the authenticated user.
//
// GitHub API docs: https://docs.github.com/rest/users/emails?apiVersion=2022-11-28#list-email-addresses-for-the-authenticated-user
//
//meta:operation GET /user/emails
func (s *UsersService) ListEmails(ctx context.Context, opts *ListOptions) ([]*UserEmail, *Response, error) {
	u := "user/emails"
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var emails []*UserEmail
	resp, err := s.client.Do(req, &emails)
	if err != nil {
		return nil, resp, err
	}

	return emails, resp, nil
}

// AddEmails adds email addresses of the authenticated user.
//
// GitHub API docs: https://docs.github.com/rest/users/emails?apiVersion=2022-11-28#add-an-email-address-for-the-authenticated-user
//
//meta:operation POST /user/emails
func (s *UsersService) AddEmails(ctx context.Context, body []string) ([]*UserEmail, *Response, error) {
	u := "user/emails"
	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var e []*UserEmail
	resp, err := s.client.Do(req, &e)
	if err != nil {
		return nil, resp, err
	}

	return e, resp, nil
}

// DeleteEmails deletes email addresses from authenticated user.
//
// GitHub API docs: https://docs.github.com/rest/users/emails?apiVersion=2022-11-28#delete-an-email-address-for-the-authenticated-user
//
//meta:operation DELETE /user/emails
func (s *UsersService) DeleteEmails(ctx context.Context, emails []string) (*Response, error) {
	u := "user/emails"
	req, err := s.client.NewRequest(ctx, "DELETE", u, emails)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// SetEmailVisibility sets the visibility for the primary email address of the authenticated user.
// `visibility` can be "private" or "public".
//
// GitHub API docs: https://docs.github.com/rest/users/emails?apiVersion=2022-11-28#set-primary-email-visibility-for-the-authenticated-user
//
//meta:operation PATCH /user/email/visibility
func (s *UsersService) SetEmailVisibility(ctx context.Context, visibility string) ([]*UserEmail, *Response, error) {
	u := "user/email/visibility"

	updateVisibilityReq := &UserEmail{
		Visibility: &visibility,
	}

	req, err := s.client.NewRequest(ctx, "PATCH", u, updateVisibilityReq)
	if err != nil {
		return nil, nil, err
	}

	var e []*UserEmail
	resp, err := s.client.Do(req, &e)
	if err != nil {
		return nil, resp, err
	}

	return e, resp, nil
}
