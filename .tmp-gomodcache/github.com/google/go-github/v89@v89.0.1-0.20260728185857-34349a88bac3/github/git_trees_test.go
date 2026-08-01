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

func TestGitService_GetTree(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/git/trees/s", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			  "sha": "s",
			  "tree": [ { "type": "blob" } ],
			  "truncated": true
			}`)
	})

	ctx := t.Context()
	tree, _, err := client.Git.GetTree(ctx, "o", "r", "s", true)
	if err != nil {
		t.Errorf("Git.GetTree returned error: %v", err)
	}

	want := Tree{
		SHA: Ptr("s"),
		Entries: []*TreeEntry{
			{
				Type: Ptr("blob"),
			},
		},
		Truncated: Ptr(true),
	}
	if !cmp.Equal(*tree, want) {
		t.Errorf("Git.GetTree returned %+v, want %+v", *tree, want)
	}

	const methodName = "GetTree"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Git.GetTree(ctx, "\n", "\n", "\n", true)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Git.GetTree(ctx, "o", "r", "s", true)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGitService_GetTree_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Git.GetTree(ctx, "%", "%", "%", false)
	testURLParseError(t, err)
}

func TestGitService_CreateTree(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := []*TreeEntry{
		{
			Path: Ptr("file.rb"),
			Mode: Ptr("100644"),
			Type: Ptr("blob"),
			SHA:  Ptr("7c258a9869f33c1e1e1f74fbb32f07c86cb5a75b"),
		},
	}

	mux.HandleFunc("/repos/o/r/git/trees", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, createTree{BaseTree: "b", Entries: []any{
			map[string]any{
				"path": "file.rb",
				"mode": "100644",
				"type": "blob",
				"sha":  "7c258a9869f33c1e1e1f74fbb32f07c86cb5a75b",
			},
		}})

		fmt.Fprint(w, `{
		  "sha": "cd8274d15fa3ae2ab983129fb037999f264ba9a7",
		  "tree": [
		    {
		      "path": "file.rb",
		      "mode": "100644",
		      "type": "blob",
		      "size": 132,
		      "sha": "7c258a9869f33c1e1e1f74fbb32f07c86cb5a75b"
		    }
		  ]
		}`)
	})

	ctx := t.Context()
	tree, _, err := client.Git.CreateTree(ctx, "o", "r", "b", input)
	if err != nil {
		t.Errorf("Git.CreateTree returned error: %v", err)
	}

	want := Tree{
		Ptr("cd8274d15fa3ae2ab983129fb037999f264ba9a7"),
		[]*TreeEntry{
			{
				Path: Ptr("file.rb"),
				Mode: Ptr("100644"),
				Type: Ptr("blob"),
				Size: Ptr(132),
				SHA:  Ptr("7c258a9869f33c1e1e1f74fbb32f07c86cb5a75b"),
			},
		},
		nil,
	}

	if !cmp.Equal(*tree, want) {
		t.Errorf("Git.CreateTree returned %+v, want %+v", *tree, want)
	}

	const methodName = "CreateTree"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Git.CreateTree(ctx, "\n", "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Git.CreateTree(ctx, "o", "r", "b", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGitService_CreateTree_Content(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := []*TreeEntry{
		{
			Path:    Ptr("content.md"),
			Mode:    Ptr("100644"),
			Content: Ptr("file content"),
		},
	}

	mux.HandleFunc("/repos/o/r/git/trees", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, createTree{BaseTree: "b", Entries: []any{
			map[string]any{
				"path":    "content.md",
				"mode":    "100644",
				"content": "file content",
			},
		}})

		fmt.Fprint(w, `{
		  "sha": "5c6780ad2c68743383b740fd1dab6f6a33202b11",
		  "url": "https://api.github.com/repos/o/r/git/trees/5c6780ad2c68743383b740fd1dab6f6a33202b11",
		  "tree": [
		    {
			  "mode": "100644",
			  "type": "blob",
			  "sha":  "aad8feacf6f8063150476a7b2bd9770f2794c08b",
			  "path": "content.md",
			  "size": 12,
			  "url": "https://api.github.com/repos/o/r/git/blobs/aad8feacf6f8063150476a7b2bd9770f2794c08b"
		    }
		  ]
		}`)
	})

	ctx := t.Context()
	tree, _, err := client.Git.CreateTree(ctx, "o", "r", "b", input)
	if err != nil {
		t.Errorf("Git.CreateTree returned error: %v", err)
	}

	want := Tree{
		Ptr("5c6780ad2c68743383b740fd1dab6f6a33202b11"),
		[]*TreeEntry{
			{
				Path: Ptr("content.md"),
				Mode: Ptr("100644"),
				Type: Ptr("blob"),
				Size: Ptr(12),
				SHA:  Ptr("aad8feacf6f8063150476a7b2bd9770f2794c08b"),
				URL:  Ptr("https://api.github.com/repos/o/r/git/blobs/aad8feacf6f8063150476a7b2bd9770f2794c08b"),
			},
		},
		nil,
	}

	if !cmp.Equal(*tree, want) {
		t.Errorf("Git.CreateTree returned %+v, want %+v", *tree, want)
	}

	const methodName = "CreateTree"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Git.CreateTree(ctx, "\n", "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Git.CreateTree(ctx, "o", "r", "b", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGitService_CreateTree_Delete(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := []*TreeEntry{
		{
			Path: Ptr("content.md"),
			Mode: Ptr("100644"),
		},
	}

	mux.HandleFunc("/repos/o/r/git/trees", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, createTree{BaseTree: "b", Entries: []any{
			map[string]any{
				"sha":  nil,
				"path": "content.md",
				"mode": "100644",
			},
		}})

		fmt.Fprint(w, `{
		  "sha": "5c6780ad2c68743383b740fd1dab6f6a33202b11",
		  "url": "https://api.github.com/repos/o/r/git/trees/5c6780ad2c68743383b740fd1dab6f6a33202b11",
		  "tree": [
		    {
			  "mode": "100644",
			  "type": "blob",
			  "sha":  null,
			  "path": "content.md",
			  "size": 12,
			  "url": "https://api.github.com/repos/o/r/git/blobs/aad8feacf6f8063150476a7b2bd9770f2794c08b"
		    }
		  ]
		}`)
	})

	ctx := t.Context()
	tree, _, err := client.Git.CreateTree(ctx, "o", "r", "b", input)
	if err != nil {
		t.Errorf("Git.CreateTree returned error: %v", err)
	}

	want := Tree{
		Ptr("5c6780ad2c68743383b740fd1dab6f6a33202b11"),
		[]*TreeEntry{
			{
				Path: Ptr("content.md"),
				Mode: Ptr("100644"),
				Type: Ptr("blob"),
				Size: Ptr(12),
				SHA:  nil,
				URL:  Ptr("https://api.github.com/repos/o/r/git/blobs/aad8feacf6f8063150476a7b2bd9770f2794c08b"),
			},
		},
		nil,
	}

	if !cmp.Equal(*tree, want) {
		t.Errorf("Git.CreateTree returned %+v, want %+v", *tree, want)
	}

	const methodName = "CreateTree"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Git.CreateTree(ctx, "\n", "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Git.CreateTree(ctx, "o", "r", "b", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGitService_CreateTree_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Git.CreateTree(ctx, "%", "%", "", nil)
	testURLParseError(t, err)
}

func TestTreeEntry_MarshalJSON(t *testing.T) {
	t.Parallel()
	testJSONMarshal(t, &TreeEntry{}, `{"sha": null}`)

	u := &TreeEntry{
		SHA:     Ptr("sha"),
		Path:    Ptr("path"),
		Mode:    Ptr("mode"),
		Type:    Ptr("type"),
		Size:    Ptr(1),
		Content: Ptr("content"),
		URL:     Ptr("url"),
	}

	want := `{
		"sha": "sha",
		"path": "path",
		"mode": "mode",
		"type": "type",
		"size": 1,
		"content": "content",
		"url": "url"
	}`

	testJSONMarshal(t, u, want)
}

func TestTreeEntry_MarshalJSON_withNilContentAndSHA(t *testing.T) {
	t.Parallel()
	te := TreeEntry{
		Path: Ptr("path"),
		Mode: Ptr("mode"),
		Type: Ptr("type"),
		Size: Ptr(1),
		URL:  Ptr("url"),
	}

	want := `{"sha":null,"path":"path","mode":"mode","type":"type"}`
	testJSONMarshalOnly(t, te, want)
	testJSONMarshalOnly(t, &te, want)
}

func TestTreeEntryWithFileDelete_MarshalJSON(t *testing.T) {
	t.Parallel()
	testJSONMarshal(t, &treeEntryWithFileDelete{}, `{"sha": null}`)

	u := &treeEntryWithFileDelete{
		SHA:     Ptr("sha"),
		Path:    Ptr("path"),
		Mode:    Ptr("mode"),
		Type:    Ptr("type"),
		Size:    Ptr(1),
		Content: Ptr("content"),
		URL:     Ptr("url"),
	}

	want := `{
		"sha": "sha",
		"path": "path",
		"mode": "mode",
		"type": "type",
		"size": 1,
		"content": "content",
		"url": "url"
	}`

	testJSONMarshal(t, u, want)
}
