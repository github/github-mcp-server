// Copyright 2024 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// GetAllCustomProperties gets all custom properties that are defined for the specified enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/enterprise-admin/custom-properties?apiVersion=2022-11-28#get-custom-properties-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/properties/schema
func (s *EnterpriseService) GetAllCustomProperties(ctx context.Context, enterprise string) ([]*CustomProperty, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/properties/schema", enterprise)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var customProperties []*CustomProperty
	resp, err := s.client.Do(req, &customProperties)
	if err != nil {
		return nil, resp, err
	}

	return customProperties, resp, nil
}

// CreateOrUpdateCustomProperties creates new or updates existing custom properties that are defined for the specified enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/enterprise-admin/custom-properties?apiVersion=2022-11-28#create-or-update-custom-properties-for-an-enterprise
//
//meta:operation PATCH /enterprises/{enterprise}/properties/schema
func (s *EnterpriseService) CreateOrUpdateCustomProperties(ctx context.Context, enterprise string, properties []*CustomProperty) ([]*CustomProperty, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/properties/schema", enterprise)

	params := struct {
		Properties []*CustomProperty `json:"properties"`
	}{
		Properties: properties,
	}

	req, err := s.client.NewRequest(ctx, "PATCH", u, params)
	if err != nil {
		return nil, nil, err
	}

	var customProperties []*CustomProperty
	resp, err := s.client.Do(req, &customProperties)
	if err != nil {
		return nil, resp, err
	}

	return customProperties, resp, nil
}

// GetCustomProperty gets a custom property that is defined for the specified enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/enterprise-admin/custom-properties?apiVersion=2022-11-28#get-a-custom-property-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/properties/schema/{custom_property_name}
func (s *EnterpriseService) GetCustomProperty(ctx context.Context, enterprise, customPropertyName string) (*CustomProperty, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/properties/schema/%v", enterprise, customPropertyName)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var customProperty *CustomProperty
	resp, err := s.client.Do(req, &customProperty)
	if err != nil {
		return nil, resp, err
	}

	return customProperty, resp, nil
}

// CreateOrUpdateCustomProperty creates a new or updates an existing custom property that is defined for the specified enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/enterprise-admin/custom-properties?apiVersion=2022-11-28#create-or-update-a-custom-property-for-an-enterprise
//
//meta:operation PUT /enterprises/{enterprise}/properties/schema/{custom_property_name}
func (s *EnterpriseService) CreateOrUpdateCustomProperty(ctx context.Context, enterprise, customPropertyName string, body *CustomProperty) (*CustomProperty, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/properties/schema/%v", enterprise, customPropertyName)

	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, nil, err
	}

	var customProperty *CustomProperty
	resp, err := s.client.Do(req, &customProperty)
	if err != nil {
		return nil, resp, err
	}

	return customProperty, resp, nil
}

// RemoveCustomProperty removes a custom property that is defined for the specified enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/enterprise-admin/custom-properties?apiVersion=2022-11-28#remove-a-custom-property-for-an-enterprise
//
//meta:operation DELETE /enterprises/{enterprise}/properties/schema/{custom_property_name}
func (s *EnterpriseService) RemoveCustomProperty(ctx context.Context, enterprise, customPropertyName string) (*Response, error) {
	u := fmt.Sprintf("enterprises/%v/properties/schema/%v", enterprise, customPropertyName)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
