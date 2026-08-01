// Copyright 2022 The go-github AUTHORS. All rights reserved.
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

func TestEnterpriseService_GetCodeSecurityAndAnalysis(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/code_security_and_analysis", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")

		fmt.Fprint(w, `
		{
		  "advanced_security_enabled_for_new_repositories": true,
		  "secret_scanning_enabled_for_new_repositories": true,
		  "secret_scanning_push_protection_enabled_for_new_repositories": true,
		  "secret_scanning_push_protection_custom_link": "https://github.com/test-org/test-repo/blob/main/README.md",
		  "secret_scanning_validity_checks_enabled": true
		}`)
	})

	ctx := t.Context()

	const methodName = "GetCodeSecurityAndAnalysis"

	settings, _, err := client.Enterprise.GetCodeSecurityAndAnalysis(ctx, "e")
	if err != nil {
		t.Errorf("Enterprise.%v returned error: %v", methodName, err)
	}
	want := &EnterpriseSecurityAnalysisSettings{
		AdvancedSecurityEnabledForNewRepositories:             Ptr(true),
		SecretScanningEnabledForNewRepositories:               Ptr(true),
		SecretScanningPushProtectionEnabledForNewRepositories: Ptr(true),
		SecretScanningPushProtectionCustomLink:                Ptr("https://github.com/test-org/test-repo/blob/main/README.md"),
		SecretScanningValidityChecksEnabled:                   Ptr(true),
	}

	if !cmp.Equal(settings, want) {
		t.Errorf("Enterprise.%v return \ngot: %+v,\nwant:%+v", methodName, settings, want)
	}

	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.GetCodeSecurityAndAnalysis(ctx, "o")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetCodeSecurityAndAnalysis(ctx, "e")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_UpdateCodeSecurityAndAnalysis(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &EnterpriseSecurityAnalysisSettings{
		AdvancedSecurityEnabledForNewRepositories:             Ptr(true),
		SecretScanningEnabledForNewRepositories:               Ptr(true),
		SecretScanningPushProtectionEnabledForNewRepositories: Ptr(true),
		SecretScanningPushProtectionCustomLink:                Ptr("https://github.com/test-org/test-repo/blob/main/README.md"),
		SecretScanningValidityChecksEnabled:                   Ptr(true),
	}

	mux.HandleFunc("/enterprises/e/code_security_and_analysis", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
	})

	ctx := t.Context()

	const methodName = "UpdateCodeSecurityAndAnalysis"

	_, err := client.Enterprise.UpdateCodeSecurityAndAnalysis(ctx, "e", input)
	if err != nil {
		t.Errorf("Enterprise.%v returned error: %v", methodName, err)
	}

	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Enterprise.UpdateCodeSecurityAndAnalysis(ctx, "o", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Enterprise.UpdateCodeSecurityAndAnalysis(ctx, "e", input)
	})
}

func TestEnterpriseService_EnableAdvancedSecurity(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/advanced_security/enable_all", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
	})

	ctx := t.Context()

	const methodName = "EnableDisableSecurityFeature"

	_, err := client.Enterprise.EnableDisableSecurityFeature(ctx, "e", "advanced_security", "enable_all")
	if err != nil {
		t.Errorf("Enterprise.%v returned error: %v", methodName, err)
	}

	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Enterprise.EnableDisableSecurityFeature(ctx, "o", "advanced_security", "enable_all")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Enterprise.EnableDisableSecurityFeature(ctx, "e", "advanced_security", "enable_all")
	})
}
