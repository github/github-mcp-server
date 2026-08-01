// Copyright 2023 The go-github AUTHORS. All rights reserved.
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

func TestActionsService_GetOrgOIDCSubjectClaimCustomTemplate(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/oidc/customization/sub", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"include_claim_keys":["repo","context"],"use_immutable_subject":true}`)
	})

	ctx := t.Context()
	template, _, err := client.Actions.GetOrgOIDCSubjectClaimCustomTemplate(ctx, "o")
	if err != nil {
		t.Errorf("Actions.GetOrgOIDCSubjectClaimCustomTemplate returned error: %v", err)
	}

	want := &OIDCSubjectClaimCustomTemplate{IncludeClaimKeys: []string{"repo", "context"}, UseImmutableSubject: Ptr(true)}
	if !cmp.Equal(template, want) {
		t.Errorf("Actions.GetOrgOIDCSubjectClaimCustomTemplate returned %+v, want %+v", template, want)
	}

	const methodName = "GetOrgOIDCSubjectClaimCustomTemplate"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetOrgOIDCSubjectClaimCustomTemplate(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetOrgOIDCSubjectClaimCustomTemplate(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_GetRepoOIDCSubjectClaimCustomTemplate(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/oidc/customization/sub", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"use_default":false,"include_claim_keys":["repo","context"],"use_immutable_subject":true,"sub_claim_prefix":"repo:o/r"}`)
	})

	ctx := t.Context()
	template, _, err := client.Actions.GetRepoOIDCSubjectClaimCustomTemplate(ctx, "o", "r")
	if err != nil {
		t.Errorf("Actions.GetRepoOIDCSubjectClaimCustomTemplate returned error: %v", err)
	}

	want := &OIDCSubjectClaimCustomTemplate{UseDefault: Ptr(false), IncludeClaimKeys: []string{"repo", "context"}, UseImmutableSubject: Ptr(true), SubClaimPrefix: Ptr("repo:o/r")}
	if !cmp.Equal(template, want) {
		t.Errorf("Actions.GetRepoOIDCSubjectClaimCustomTemplate returned %+v, want %+v", template, want)
	}

	const methodName = "GetRepoOIDCSubjectClaimCustomTemplate"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetRepoOIDCSubjectClaimCustomTemplate(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetRepoOIDCSubjectClaimCustomTemplate(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_SetOrgOIDCSubjectClaimCustomTemplate(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := OIDCSubjectClaimCustomTemplate{
		IncludeClaimKeys:    []string{"repo", "context"},
		UseImmutableSubject: Ptr(true),
	}

	mux.HandleFunc("/orgs/o/actions/oidc/customization/sub", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testHeader(t, r, "Content-Type", "application/json")
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusCreated)
	})

	ctx := t.Context()
	_, err := client.Actions.SetOrgOIDCSubjectClaimCustomTemplate(ctx, "o", input)
	if err != nil {
		t.Errorf("Actions.SetOrgOIDCSubjectClaimCustomTemplate returned error: %v", err)
	}

	const methodName = "SetOrgOIDCSubjectClaimCustomTemplate"

	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.SetOrgOIDCSubjectClaimCustomTemplate(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.SetOrgOIDCSubjectClaimCustomTemplate(ctx, "o", input)
	})
}

func TestActionsService_SetRepoOIDCSubjectClaimCustomTemplate(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := OIDCSubjectClaimCustomTemplate{
		UseDefault:       Ptr(false),
		IncludeClaimKeys: []string{"repo", "context"},
	}

	mux.HandleFunc("/repos/o/r/actions/oidc/customization/sub", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testHeader(t, r, "Content-Type", "application/json")
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusCreated)
	})

	ctx := t.Context()
	_, err := client.Actions.SetRepoOIDCSubjectClaimCustomTemplate(ctx, "o", "r", input)
	if err != nil {
		t.Errorf("Actions.SetRepoOIDCSubjectClaimCustomTemplate returned error: %v", err)
	}

	const methodName = "SetRepoOIDCSubjectClaimCustomTemplate"

	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.SetRepoOIDCSubjectClaimCustomTemplate(ctx, "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.SetRepoOIDCSubjectClaimCustomTemplate(ctx, "o", "r", input)
	})
}

func TestActionsService_SetRepoOIDCSubjectClaimCustomTemplateToDefault(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := OIDCSubjectClaimCustomTemplate{
		UseDefault: Ptr(true),
	}

	mux.HandleFunc("/repos/o/r/actions/oidc/customization/sub", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testHeader(t, r, "Content-Type", "application/json")
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusCreated)
	})

	ctx := t.Context()
	_, err := client.Actions.SetRepoOIDCSubjectClaimCustomTemplate(ctx, "o", "r", input)
	if err != nil {
		t.Errorf("Actions.SetRepoOIDCSubjectClaimCustomTemplate returned error: %v", err)
	}

	const methodName = "SetRepoOIDCSubjectClaimCustomTemplate"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.SetRepoOIDCSubjectClaimCustomTemplate(ctx, "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.SetRepoOIDCSubjectClaimCustomTemplate(ctx, "o", "r", input)
	})
}

func TestActionsService_ListEnterpriseOIDCCustomPropertyClaims(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/actions/oidc/customization/properties/repo", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{"custom_property_name": "environment","inclusion_source": "enterprise"},{"custom_property_name": "lane","inclusion_source": "enterprise"}]`)
	})

	ctx := t.Context()
	claims, _, err := client.Actions.ListEnterpriseOIDCCustomPropertyClaims(ctx, "e")
	if err != nil {
		t.Errorf("Actions.ListEnterpriseOIDCCustomPropertyClaims returned error: %v", err)
	}

	want := []*OIDCCustomPropertyClaimResponse{
		{OIDCCustomPropertyClaim: OIDCCustomPropertyClaim{"environment"}, InclusionSource: InclusionSourceEnterprise},
		{OIDCCustomPropertyClaim: OIDCCustomPropertyClaim{"lane"}, InclusionSource: InclusionSourceEnterprise},
	}

	if !cmp.Equal(claims, want) {
		t.Errorf("Actions.ListEnterpriseOIDCCustomPropertyClaims returned %+v, want %+v", claims, want)
	}

	const methodName = "ListEnterpriseOIDCCustomPropertyClaims"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.ListEnterpriseOIDCCustomPropertyClaims(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.ListEnterpriseOIDCCustomPropertyClaims(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_ListOrgOIDCCustomPropertyClaims(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/oidc/customization/properties/repo", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{"custom_property_name": "environment","inclusion_source": "organization"},{"custom_property_name": "lane","inclusion_source": "organization"}]`)
	})

	ctx := t.Context()
	claims, _, err := client.Actions.ListOrgOIDCCustomPropertyClaims(ctx, "o")
	if err != nil {
		t.Errorf("Actions.ListOrgOIDCCustomPropertyClaims returned error: %v", err)
	}

	want := []*OIDCCustomPropertyClaimResponse{
		{OIDCCustomPropertyClaim: OIDCCustomPropertyClaim{"environment"}, InclusionSource: InclusionSourceOrganization},
		{OIDCCustomPropertyClaim: OIDCCustomPropertyClaim{"lane"}, InclusionSource: InclusionSourceOrganization},
	}

	if !cmp.Equal(claims, want) {
		t.Errorf("Actions.ListOrgOIDCCustomPropertyClaims returned %+v, want %+v", claims, want)
	}

	const methodName = "ListOrgOIDCCustomPropertyClaims"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.ListOrgOIDCCustomPropertyClaims(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.ListOrgOIDCCustomPropertyClaims(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_SetEnterpriseOIDCCustomPropertyClaim(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := OIDCCustomPropertyClaim{
		CustomPropertyName: "environment",
	}

	mux.HandleFunc("/enterprises/e/actions/oidc/customization/properties/repo", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "Content-Type", "application/json")
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"custom_property_name": "environment"}`)
	})

	ctx := t.Context()
	property, _, err := client.Actions.SetEnterpriseOIDCCustomPropertyClaim(ctx, "e", input)
	if err != nil {
		t.Errorf("Actions.SetEnterpriseOIDCCustomPropertyClaim returned error: %v", err)
	}

	want := &OIDCCustomPropertyClaim{CustomPropertyName: "environment"}
	if !cmp.Equal(property, want) {
		t.Errorf("Actions.SetEnterpriseOIDCCustomPropertyClaim returned %+v, want %+v", property, want)
	}

	const methodName = "SetEnterpriseOIDCCustomPropertyClaim"

	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.SetEnterpriseOIDCCustomPropertyClaim(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.SetEnterpriseOIDCCustomPropertyClaim(ctx, "e", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_SetOrgOIDCCustomPropertyClaim(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := OIDCCustomPropertyClaim{
		CustomPropertyName: "environment",
	}

	mux.HandleFunc("/orgs/o/actions/oidc/customization/properties/repo", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "Content-Type", "application/json")
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"custom_property_name": "environment"}`)
	})

	ctx := t.Context()
	property, _, err := client.Actions.SetOrgOIDCCustomPropertyClaim(ctx, "o", input)
	if err != nil {
		t.Errorf("Actions.SetOrgOIDCCustomPropertyClaim returned error: %v", err)
	}

	want := &OIDCCustomPropertyClaim{CustomPropertyName: "environment"}
	if !cmp.Equal(property, want) {
		t.Errorf("Actions.SetOrgOIDCCustomPropertyClaim returned %+v, want %+v", property, want)
	}

	const methodName = "SetOrgOIDCCustomPropertyClaim"

	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.SetOrgOIDCCustomPropertyClaim(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.SetOrgOIDCCustomPropertyClaim(ctx, "o", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_DeleteEnterpriseOIDCCustomPropertyClaim(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/actions/oidc/customization/properties/repo/environment", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Actions.DeleteEnterpriseOIDCCustomPropertyClaim(ctx, "e", "environment")
	if err != nil {
		t.Errorf("Actions.DeleteEnterpriseOIDCCustomPropertyClaim return error: %v", err)
	}

	const methodName = "DeleteEnterpriseOIDCCustomPropertyClaim"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.DeleteEnterpriseOIDCCustomPropertyClaim(ctx, "\n", "environment")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.DeleteEnterpriseOIDCCustomPropertyClaim(ctx, "e", "r")
	})
}

func TestActionsService_DeleteOrgOIDCCustomPropertyClaim(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/oidc/customization/properties/repo/environment", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Actions.DeleteOrgOIDCCustomPropertyClaim(ctx, "o", "environment")
	if err != nil {
		t.Errorf("Actions.DeleteOrgOIDCCustomPropertyClaim return error: %v", err)
	}

	const methodName = "DeleteOrgOIDCCustomPropertyClaim"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.DeleteOrgOIDCCustomPropertyClaim(ctx, "\n", "environment")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.DeleteOrgOIDCCustomPropertyClaim(ctx, "o", "r")
	})
}
