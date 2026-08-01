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

func TestRepositoriesService_GetHookConfiguration(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/hooks/1/config", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"content_type": "json", "insecure_ssl": "0", "secret": "********", "url": "https://example.com/webhook"}`)
	})

	ctx := t.Context()
	config, _, err := client.Repositories.GetHookConfiguration(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("Repositories.GetHookConfiguration returned error: %v", err)
	}

	want := &HookConfig{
		ContentType: Ptr("json"),
		InsecureSSL: Ptr("0"),
		Secret:      Ptr("********"),
		URL:         Ptr("https://example.com/webhook"),
	}
	if !cmp.Equal(config, want) {
		t.Errorf("Repositories.GetHookConfiguration returned %+v, want %+v", config, want)
	}

	const methodName = "GetHookConfiguration"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetHookConfiguration(ctx, "\n", "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetHookConfiguration(ctx, "o", "r", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_GetHookConfiguration_invalidOrg(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Repositories.GetHookConfiguration(ctx, "%", "%", 1)
	testURLParseError(t, err)
}

func TestRepositoriesService_UpdateHookConfiguration(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := HookConfig{}

	mux.HandleFunc("/repos/o/r/hooks/1/config", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"content_type": "json", "insecure_ssl": "0", "secret": "********", "url": "https://example.com/webhook"}`)
	})

	ctx := t.Context()
	config, _, err := client.Repositories.UpdateHookConfiguration(ctx, "o", "r", 1, input)
	if err != nil {
		t.Errorf("Repositories.UpdateHookConfiguration returned error: %v", err)
	}

	want := &HookConfig{
		ContentType: Ptr("json"),
		InsecureSSL: Ptr("0"),
		Secret:      Ptr("********"),
		URL:         Ptr("https://example.com/webhook"),
	}
	if !cmp.Equal(config, want) {
		t.Errorf("Repositories.UpdateHookConfiguration returned %+v, want %+v", config, want)
	}

	const methodName = "UpdateHookConfiguration"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.UpdateHookConfiguration(ctx, "\n", "\n", -1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.UpdateHookConfiguration(ctx, "o", "r", 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_UpdateHookConfiguration_invalidOrg(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Repositories.UpdateHookConfiguration(ctx, "%", "%", 1, HookConfig{})
	testURLParseError(t, err)
}
