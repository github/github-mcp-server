// Copyright 2015 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"errors"
	"fmt"
)

// ListHooks lists all Hooks for the specified organization.
//
// GitHub API docs: https://docs.github.com/rest/orgs/webhooks?apiVersion=2022-11-28#list-organization-webhooks
//
//meta:operation GET /orgs/{org}/hooks
func (s *OrganizationsService) ListHooks(ctx context.Context, org string, opts *ListOptions) ([]*Hook, *Response, error) {
	u := fmt.Sprintf("orgs/%v/hooks", org)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var hooks []*Hook
	resp, err := s.client.Do(req, &hooks)
	if err != nil {
		return nil, resp, err
	}

	return hooks, resp, nil
}

// GetHook returns a single specified Hook.
//
// GitHub API docs: https://docs.github.com/rest/orgs/webhooks?apiVersion=2022-11-28#get-an-organization-webhook
//
//meta:operation GET /orgs/{org}/hooks/{hook_id}
func (s *OrganizationsService) GetHook(ctx context.Context, org string, id int64) (*Hook, *Response, error) {
	u := fmt.Sprintf("orgs/%v/hooks/%v", org, id)
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var hook *Hook
	resp, err := s.client.Do(req, &hook)
	if err != nil {
		return nil, resp, err
	}

	return hook, resp, nil
}

// CreateHook creates a Hook for the specified org.
// Config is a required field.
//
// Note that only a subset of the hook fields are used and hook must
// not be nil.
//
// GitHub API docs: https://docs.github.com/rest/orgs/webhooks?apiVersion=2022-11-28#create-an-organization-webhook
//
//meta:operation POST /orgs/{org}/hooks
func (s *OrganizationsService) CreateHook(ctx context.Context, org string, hook *Hook) (*Hook, *Response, error) {
	if hook == nil {
		return nil, nil, errors.New("hook must be provided")
	}

	u := fmt.Sprintf("orgs/%v/hooks", org)

	hookReq := &createHookRequest{
		Name:   "web",
		Events: hook.Events,
		Active: hook.Active,
		Config: hook.Config,
	}

	req, err := s.client.NewRequest(ctx, "POST", u, hookReq)
	if err != nil {
		return nil, nil, err
	}

	var h *Hook
	resp, err := s.client.Do(req, &h)
	if err != nil {
		return nil, resp, err
	}

	return h, resp, nil
}

// EditHook updates a specified Hook.
//
// GitHub API docs: https://docs.github.com/rest/orgs/webhooks?apiVersion=2022-11-28#update-an-organization-webhook
//
//meta:operation PATCH /orgs/{org}/hooks/{hook_id}
func (s *OrganizationsService) EditHook(ctx context.Context, org string, id int64, body *Hook) (*Hook, *Response, error) {
	u := fmt.Sprintf("orgs/%v/hooks/%v", org, id)
	req, err := s.client.NewRequest(ctx, "PATCH", u, body)
	if err != nil {
		return nil, nil, err
	}

	var h *Hook
	resp, err := s.client.Do(req, &h)
	if err != nil {
		return nil, resp, err
	}

	return h, resp, nil
}

// PingHook triggers a 'ping' event to be sent to the Hook.
//
// GitHub API docs: https://docs.github.com/rest/orgs/webhooks?apiVersion=2022-11-28#ping-an-organization-webhook
//
//meta:operation POST /orgs/{org}/hooks/{hook_id}/pings
func (s *OrganizationsService) PingHook(ctx context.Context, org string, id int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/hooks/%v/pings", org, id)
	req, err := s.client.NewRequest(ctx, "POST", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DeleteHook deletes a specified Hook.
//
// GitHub API docs: https://docs.github.com/rest/orgs/webhooks?apiVersion=2022-11-28#delete-an-organization-webhook
//
//meta:operation DELETE /orgs/{org}/hooks/{hook_id}
func (s *OrganizationsService) DeleteHook(ctx context.Context, org string, id int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/hooks/%v", org, id)
	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
