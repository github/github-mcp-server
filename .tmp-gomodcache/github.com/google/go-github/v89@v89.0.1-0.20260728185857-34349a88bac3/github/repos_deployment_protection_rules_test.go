// Copyright 2024 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRepositoriesService_GetAllDeploymentProtectionRules(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/environments/e/deployment_protection_rules", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"total_count":2, "custom_deployment_protection_rules":[{ "id": 3, "node_id": "IEH37kRlcGxveW1lbnRTdGF0ddiv", "enabled": true, "app": { "id": 1, "node_id": "GHT58kRlcGxveW1lbnRTdTY!bbcy", "slug": "a-custom-app", "integration_url": "https://api.github.com/apps/a-custom-app"}}, { "id": 4, "node_id": "MDE2OkRlcGxveW1lbnRTdHJ41128", "enabled": true, "app": { "id": 1, "node_id": "UHVE67RlcGxveW1lbnRTdTY!jfeuy", "slug": "another-custom-app", "integration_url": "https://api.github.com/apps/another-custom-app"}}]}`)
	})

	ctx := t.Context()
	got, _, err := client.Repositories.GetAllDeploymentProtectionRules(ctx, "o", "r", "e")
	if err != nil {
		t.Errorf("Repositories.GetAllDeploymentProtectionRules returned error: %v", err)
	}

	want := &ListDeploymentProtectionRuleResponse{
		ProtectionRules: []*CustomDeploymentProtectionRule{
			{ID: Ptr(int64(3)), NodeID: Ptr("IEH37kRlcGxveW1lbnRTdGF0ddiv"), Enabled: Ptr(true), App: &CustomDeploymentProtectionRuleApp{ID: Ptr(int64(1)), NodeID: Ptr("GHT58kRlcGxveW1lbnRTdTY!bbcy"), Slug: Ptr("a-custom-app"), IntegrationURL: Ptr("https://api.github.com/apps/a-custom-app")}},
			{ID: Ptr(int64(4)), NodeID: Ptr("MDE2OkRlcGxveW1lbnRTdHJ41128"), Enabled: Ptr(true), App: &CustomDeploymentProtectionRuleApp{ID: Ptr(int64(1)), NodeID: Ptr("UHVE67RlcGxveW1lbnRTdTY!jfeuy"), Slug: Ptr("another-custom-app"), IntegrationURL: Ptr("https://api.github.com/apps/another-custom-app")}},
		},
		TotalCount: Ptr(2),
	}
	if !cmp.Equal(got, want) {
		t.Errorf("Repositories.GetAllDeploymentProtectionRules = %+v, want %+v", got, want)
	}

	const methodName = "GetAllDeploymentProtectionRules"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetAllDeploymentProtectionRules(ctx, "o", "r", "e")
		if got != nil {
			t.Errorf("got non-nil Repositories.GetAllDeploymentProtectionRules response: %+v", got)
		}
		return resp, err
	})
}

func TestRepositoriesService_CreateCustomDeploymentProtectionRule(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &CustomDeploymentProtectionRuleRequest{
		IntegrationID: Ptr(int64(5)),
	}

	mux.HandleFunc("/repos/o/r/environments/e/deployment_protection_rules", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":3, "node_id": "IEH37kRlcGxveW1lbnRTdGF0ddiv", "enabled": true, "app": {"id": 1, "node_id": "GHT58kRlcGxveW1lbnRTdTY!bbcy", "slug": "a-custom-app", "integration_url": "https://api.github.com/apps/a-custom-app"}}`)
	})

	ctx := t.Context()
	got, _, err := client.Repositories.CreateCustomDeploymentProtectionRule(ctx, "o", "r", "e", input)
	if err != nil {
		t.Errorf("Repositories.CreateCustomDeploymentProtectionRule returned error: %v", err)
	}

	want := &CustomDeploymentProtectionRule{
		ID:      Ptr(int64(3)),
		NodeID:  Ptr("IEH37kRlcGxveW1lbnRTdGF0ddiv"),
		Enabled: Ptr(true),
		App: &CustomDeploymentProtectionRuleApp{
			ID:             Ptr(int64(1)),
			NodeID:         Ptr("GHT58kRlcGxveW1lbnRTdTY!bbcy"),
			Slug:           Ptr("a-custom-app"),
			IntegrationURL: Ptr("https://api.github.com/apps/a-custom-app"),
		},
	}
	if !cmp.Equal(got, want) {
		t.Errorf("Repositories.CreateCustomDeploymentProtectionRule = %+v, want %+v", got, want)
	}

	const methodName = "CreateCustomDeploymentProtectionRule"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.CreateCustomDeploymentProtectionRule(ctx, "\n", "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.CreateCustomDeploymentProtectionRule(ctx, "o", "r", "e", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_ListCustomDeploymentRuleIntegrations(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/environments/e/deployment_protection_rules/apps", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"page":     "1",
			"per_page": "30",
		})
		fmt.Fprint(w, `{"total_count": 2, "available_custom_deployment_protection_rule_integrations": [{"id": 1, "node_id": "GHT58kRlcGxveW1lbnRTdTY!bbcy", "slug": "a-custom-app", "integration_url": "https://api.github.com/apps/a-custom-app"}, {"id": 2, "node_id": "UHVE67RlcGxveW1lbnRTdTY!jfeuy", "slug": "another-custom-app", "integration_url": "https://api.github.com/apps/another-custom-app"}]}`)
	})

	ctx := t.Context()
	opts := &ListOptions{Page: 1, PerPage: 30}
	got, _, err := client.Repositories.ListCustomDeploymentRuleIntegrations(ctx, "o", "r", "e", opts)
	if err != nil {
		t.Errorf("Repositories.ListCustomDeploymentRuleIntegrations returned error: %v", err)
	}

	want := &ListCustomDeploymentRuleIntegrationsResponse{
		TotalCount: Ptr(2),
		AvailableIntegrations: []*CustomDeploymentProtectionRuleApp{
			{ID: Ptr(int64(1)), NodeID: Ptr("GHT58kRlcGxveW1lbnRTdTY!bbcy"), Slug: Ptr("a-custom-app"), IntegrationURL: Ptr("https://api.github.com/apps/a-custom-app")},
			{ID: Ptr(int64(2)), NodeID: Ptr("UHVE67RlcGxveW1lbnRTdTY!jfeuy"), Slug: Ptr("another-custom-app"), IntegrationURL: Ptr("https://api.github.com/apps/another-custom-app")},
		},
	}
	if !cmp.Equal(got, want) {
		t.Errorf("Repositories.ListCustomDeploymentRuleIntegrations = %+v, want %+v", got, want)
	}

	const methodName = "ListCustomDeploymentRuleIntegrations"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.ListCustomDeploymentRuleIntegrations(ctx, "\n", "\n", "\n", opts)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListCustomDeploymentRuleIntegrations(ctx, "o", "r", "e", opts)
		if got != nil {
			t.Errorf("got non-nil Repositories.ListCustomDeploymentRuleIntegrations response: %+v", got)
		}
		return resp, err
	})
}

func TestRepositoriesService_GetCustomDeploymentProtectionRule(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/environments/e/deployment_protection_rules/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":1, "node_id": "IEH37kRlcGxveW1lbnRTdGF0ddiv", "enabled": true, "app": {"id": 1, "node_id": "GHT58kRlcGxveW1lbnRTdTY!bbcy", "slug": "a-custom-app", "integration_url": "https://api.github.com/apps/a-custom-app"}}`)
	})

	ctx := t.Context()
	got, _, err := client.Repositories.GetCustomDeploymentProtectionRule(ctx, "o", "r", "e", 1)
	if err != nil {
		t.Errorf("Repositories.GetCustomDeploymentProtectionRule returned error: %v", err)
	}

	want := &CustomDeploymentProtectionRule{
		ID:      Ptr(int64(1)),
		NodeID:  Ptr("IEH37kRlcGxveW1lbnRTdGF0ddiv"),
		Enabled: Ptr(true),
		App: &CustomDeploymentProtectionRuleApp{
			ID:             Ptr(int64(1)),
			NodeID:         Ptr("GHT58kRlcGxveW1lbnRTdTY!bbcy"),
			Slug:           Ptr("a-custom-app"),
			IntegrationURL: Ptr("https://api.github.com/apps/a-custom-app"),
		},
	}

	if !cmp.Equal(got, want) {
		t.Errorf("Repositories.GetCustomDeploymentProtectionRule = %+v, want %+v", got, want)
	}

	const methodName = "GetCustomDeploymentProtectionRule"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetCustomDeploymentProtectionRule(ctx, "o", "r", "e", 1)
		if got != nil {
			t.Errorf("got non-nil Repositories.GetCustomDeploymentProtectionRule response: %+v", got)
		}
		return resp, err
	})
}

func TestRepositoriesService_DisableCustomDeploymentProtectionRule(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/environments/e/deployment_protection_rules/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Repositories.DisableCustomDeploymentProtectionRule(ctx, "o", "r", "e", 1)
	if err != nil {
		t.Errorf("Repositories.DisableCustomDeploymentProtectionRule returned error: %v", err)
	}

	if !cmp.Equal(resp.StatusCode, 204) {
		t.Errorf("Repositories.DisableCustomDeploymentProtectionRule returned  status code %+v, want %+v", resp.StatusCode, "204")
	}

	const methodName = "DisableCustomDeploymentProtectionRule"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Repositories.DisableCustomDeploymentProtectionRule(ctx, "\n", "\n", "\n", 1)
		return err
	})
}
