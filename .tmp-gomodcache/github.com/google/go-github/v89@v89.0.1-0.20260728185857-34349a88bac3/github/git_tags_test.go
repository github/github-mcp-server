// Copyright 2013 The go-github AUTHORS. All rights reserved.
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

func TestGitService_GetTag(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/git/tags/s", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"tag": "t"}`)
	})

	ctx := t.Context()
	tag, _, err := client.Git.GetTag(ctx, "o", "r", "s")
	if err != nil {
		t.Errorf("Git.GetTag returned error: %v", err)
	}

	want := &Tag{Tag: Ptr("t")}
	if !cmp.Equal(tag, want) {
		t.Errorf("Git.GetTag returned %+v, want %+v", tag, want)
	}

	const methodName = "GetTag"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Git.GetTag(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Git.GetTag(ctx, "o", "r", "s")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGitService_CreateTag(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	inputTag := CreateTag{
		Tag:     "t",
		Object:  "s",
		Type:    "commit",
		Message: "test message",
	}

	mux.HandleFunc("/repos/o/r/git/tags", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, inputTag)
		fmt.Fprint(w, `{"tag": "t"}`)
	})

	ctx := t.Context()
	tag, _, err := client.Git.CreateTag(ctx, "o", "r", inputTag)
	if err != nil {
		t.Errorf("Git.CreateTag returned error: %v", err)
	}

	want := &Tag{Tag: Ptr("t")}
	if !cmp.Equal(tag, want) {
		t.Errorf("Git.CreateTag returned %+v, want %+v", tag, want)
	}

	const methodName = "CreateTag"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Git.CreateTag(ctx, "\n", "\n", inputTag)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Git.CreateTag(ctx, "o", "r", inputTag)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
