// Copyright 2021 The go-github AUTHORS. All rights reserved.
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

func TestAppsService_GetHookConfig(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/app/hook/config", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"content_type": "json",
			"insecure_ssl": "0",
			"secret": "********",
			"url": "https://example.com/webhook"
		}`)
	})

	ctx := t.Context()
	config, _, err := client.Apps.GetHookConfig(ctx)
	if err != nil {
		t.Errorf("Apps.GetHookConfig returned error: %v", err)
	}

	want := &HookConfig{
		ContentType: Ptr("json"),
		InsecureSSL: Ptr("0"),
		Secret:      Ptr("********"),
		URL:         Ptr("https://example.com/webhook"),
	}
	if !cmp.Equal(config, want) {
		t.Errorf("Apps.GetHookConfig returned %+v, want %+v", config, want)
	}

	const methodName = "GetHookConfig"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Apps.GetHookConfig(ctx)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestAppsService_UpdateHookConfig(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := HookConfig{
		ContentType: Ptr("json"),
		InsecureSSL: Ptr("1"),
		Secret:      Ptr("s"),
		URL:         Ptr("u"),
	}

	mux.HandleFunc("/app/hook/config", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{
			"content_type": "json",
			"insecure_ssl": "1",
			"secret": "********",
			"url": "u"
		}`)
	})

	ctx := t.Context()
	config, _, err := client.Apps.UpdateHookConfig(ctx, input)
	if err != nil {
		t.Errorf("Apps.UpdateHookConfig returned error: %v", err)
	}

	want := &HookConfig{
		ContentType: Ptr("json"),
		InsecureSSL: Ptr("1"),
		Secret:      Ptr("********"),
		URL:         Ptr("u"),
	}
	if !cmp.Equal(config, want) {
		t.Errorf("Apps.UpdateHookConfig returned %+v, want %+v", config, want)
	}

	const methodName = "UpdateHookConfig"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Apps.UpdateHookConfig(ctx, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
