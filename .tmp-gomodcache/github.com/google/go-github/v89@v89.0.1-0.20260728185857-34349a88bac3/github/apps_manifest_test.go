// Copyright 2019 The go-github AUTHORS. All rights reserved.
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

const (
	manifestJSON = `{
	"id": 1,
  "client_id": "a" ,
  "client_secret": "b",
  "webhook_secret": "c",
  "pem": "key"
}
`
)

func TestAppsService_CompleteAppManifest(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/app-manifests/code/conversions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, manifestJSON)
	})

	ctx := t.Context()
	cfg, _, err := client.Apps.CompleteAppManifest(ctx, "code")
	if err != nil {
		t.Errorf("Apps.CompleteAppManifest returned error: %v", err)
	}

	want := &AppConfig{
		ID:            Ptr(int64(1)),
		ClientID:      Ptr("a"),
		ClientSecret:  Ptr("b"),
		WebhookSecret: Ptr("c"),
		PEM:           Ptr("key"),
	}

	if !cmp.Equal(cfg, want) {
		t.Errorf("Apps.CompleteAppManifest returned %+v, want %+v", cfg, want)
	}

	const methodName = "CompleteAppManifest"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Apps.CompleteAppManifest(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Apps.CompleteAppManifest(ctx, "code")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
