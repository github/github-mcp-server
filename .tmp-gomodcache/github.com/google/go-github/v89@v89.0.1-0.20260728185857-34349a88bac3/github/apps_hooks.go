// Copyright 2021 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
)

// GetHookConfig returns the webhook configuration for a GitHub App.
// The underlying transport must be authenticated as an app.
//
// GitHub API docs: https://docs.github.com/rest/apps/webhooks?apiVersion=2022-11-28#get-a-webhook-configuration-for-an-app
//
//meta:operation GET /app/hook/config
func (s *AppsService) GetHookConfig(ctx context.Context) (*HookConfig, *Response, error) {
	req, err := s.client.NewRequest(ctx, "GET", "app/hook/config", nil)
	if err != nil {
		return nil, nil, err
	}

	var config *HookConfig
	resp, err := s.client.Do(req, &config)
	if err != nil {
		return nil, resp, err
	}

	return config, resp, nil
}

// UpdateHookConfig updates the webhook configuration for a GitHub App.
// The underlying transport must be authenticated as an app.
//
// GitHub API docs: https://docs.github.com/rest/apps/webhooks?apiVersion=2022-11-28#update-a-webhook-configuration-for-an-app
//
//meta:operation PATCH /app/hook/config
func (s *AppsService) UpdateHookConfig(ctx context.Context, body HookConfig) (*HookConfig, *Response, error) {
	req, err := s.client.NewRequest(ctx, "PATCH", "app/hook/config", body)
	if err != nil {
		return nil, nil, err
	}

	var c *HookConfig
	resp, err := s.client.Do(req, &c)
	if err != nil {
		return nil, resp, err
	}

	return c, resp, nil
}
