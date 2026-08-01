// Copyright 2021 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// ListHookDeliveries lists deliveries of an App webhook.
//
// GitHub API docs: https://docs.github.com/rest/apps/webhooks?apiVersion=2022-11-28#list-deliveries-for-an-app-webhook
//
//meta:operation GET /app/hook/deliveries
func (s *AppsService) ListHookDeliveries(ctx context.Context, opts *ListCursorOptions) ([]*HookDelivery, *Response, error) {
	u, err := addOptions("app/hook/deliveries", opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	deliveries := []*HookDelivery{}
	resp, err := s.client.Do(req, &deliveries)
	if err != nil {
		return nil, resp, err
	}

	return deliveries, resp, nil
}

// GetHookDelivery returns the App webhook delivery with the specified ID.
//
// GitHub API docs: https://docs.github.com/rest/apps/webhooks?apiVersion=2022-11-28#get-a-delivery-for-an-app-webhook
//
//meta:operation GET /app/hook/deliveries/{delivery_id}
func (s *AppsService) GetHookDelivery(ctx context.Context, deliveryID int64) (*HookDelivery, *Response, error) {
	u := fmt.Sprintf("app/hook/deliveries/%v", deliveryID)
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var h *HookDelivery
	resp, err := s.client.Do(req, &h)
	if err != nil {
		return nil, resp, err
	}

	return h, resp, nil
}

// RedeliverHookDelivery redelivers a delivery for an App webhook.
//
// GitHub API docs: https://docs.github.com/rest/apps/webhooks?apiVersion=2022-11-28#redeliver-a-delivery-for-an-app-webhook
//
//meta:operation POST /app/hook/deliveries/{delivery_id}/attempts
func (s *AppsService) RedeliverHookDelivery(ctx context.Context, deliveryID int64) (*HookDelivery, *Response, error) {
	u := fmt.Sprintf("app/hook/deliveries/%v/attempts", deliveryID)
	req, err := s.client.NewRequest(ctx, "POST", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var h *HookDelivery
	resp, err := s.client.Do(req, &h)
	if err != nil {
		return nil, resp, err
	}

	return h, resp, nil
}
