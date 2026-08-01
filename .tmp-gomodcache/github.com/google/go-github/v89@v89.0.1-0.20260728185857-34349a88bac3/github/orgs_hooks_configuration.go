// Copyright 2023 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// GetHookConfiguration returns the configuration for the specified organization webhook.
//
// GitHub API docs: https://docs.github.com/rest/orgs/webhooks?apiVersion=2022-11-28#get-a-webhook-configuration-for-an-organization
//
//meta:operation GET /orgs/{org}/hooks/{hook_id}/config
func (s *OrganizationsService) GetHookConfiguration(ctx context.Context, org string, id int64) (*HookConfig, *Response, error) {
	u := fmt.Sprintf("orgs/%v/hooks/%v/config", org, id)
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
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

// UpdateHookConfiguration updates the configuration for the specified organization webhook.
//
// GitHub API docs: https://docs.github.com/rest/orgs/webhooks?apiVersion=2022-11-28#update-a-webhook-configuration-for-an-organization
//
//meta:operation PATCH /orgs/{org}/hooks/{hook_id}/config
func (s *OrganizationsService) UpdateHookConfiguration(ctx context.Context, org string, id int64, body HookConfig) (*HookConfig, *Response, error) {
	u := fmt.Sprintf("orgs/%v/hooks/%v/config", org, id)
	req, err := s.client.NewRequest(ctx, "PATCH", u, body)
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
