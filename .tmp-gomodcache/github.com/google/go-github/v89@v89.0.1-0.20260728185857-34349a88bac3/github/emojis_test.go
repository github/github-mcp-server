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

func TestEmojisService_List(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/emojis", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"+1": "+1.png"}`)
	})

	ctx := t.Context()
	emoji, _, err := client.ListEmojis(ctx)
	if err != nil {
		t.Errorf("List returned error: %v", err)
	}

	want := map[string]string{"+1": "+1.png"}
	if !cmp.Equal(want, emoji) {
		t.Errorf("List returned %+v, want %+v", emoji, want)
	}

	const methodName = "List"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Emojis.List(ctx)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
