// Copyright 2013 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGitService_GetRef_singleRef(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/git/ref/heads/b", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `
		  {
		    "ref": "refs/heads/b",
		    "url": "https://api.github.com/repos/o/r/git/refs/heads/b",
		    "object": {
		      "type": "commit",
		      "sha": "aa218f56b14c9653891f9e74264a383fa43fefbd",
		      "url": "https://api.github.com/repos/o/r/git/commits/aa218f56b14c9653891f9e74264a383fa43fefbd"
		    }
		  }`)
	})

	ctx := t.Context()
	ref, _, err := client.Git.GetRef(ctx, "o", "r", "refs/heads/b")
	if err != nil {
		t.Fatalf("Git.GetRef returned error: %v", err)
	}

	want := &Reference{
		Ref: Ptr("refs/heads/b"),
		URL: Ptr("https://api.github.com/repos/o/r/git/refs/heads/b"),
		Object: &GitObject{
			Type: Ptr("commit"),
			SHA:  Ptr("aa218f56b14c9653891f9e74264a383fa43fefbd"),
			URL:  Ptr("https://api.github.com/repos/o/r/git/commits/aa218f56b14c9653891f9e74264a383fa43fefbd"),
		},
	}
	if !cmp.Equal(ref, want) {
		t.Errorf("Git.GetRef returned %+v, want %+v", ref, want)
	}

	// without 'refs/' prefix
	if _, _, err := client.Git.GetRef(ctx, "o", "r", "heads/b"); err != nil {
		t.Errorf("Git.GetRef returned error: %v", err)
	}

	const methodName = "GetRef"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Git.GetRef(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Git.GetRef(ctx, "o", "r", "refs/heads/b")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGitService_GetRef_noRefs(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/git/refs/heads/b", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNotFound)
	})

	ctx := t.Context()
	ref, resp, err := client.Git.GetRef(ctx, "o", "r", "refs/heads/b")
	if err == nil {
		t.Error("Expected HTTP 404 response")
	}
	if got, want := resp.Response.StatusCode, http.StatusNotFound; got != want {
		t.Errorf("Git.GetRef returned status %v, want %v", got, want)
	}
	if ref != nil {
		t.Errorf("Git.GetRef return %+v, want nil", ref)
	}

	const methodName = "GetRef"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Git.GetRef(ctx, "o", "r", "refs/heads/b")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Git.GetRef(ctx, "o", "r", "refs/heads/b")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGitService_ListMatchingRefs_singleRef(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/git/matching-refs/heads/b", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `
		  [
		    {
		      "ref": "refs/heads/b",
		      "url": "https://api.github.com/repos/o/r/git/refs/heads/b",
		      "object": {
		        "type": "commit",
		        "sha": "aa218f56b14c9653891f9e74264a383fa43fefbd",
		        "url": "https://api.github.com/repos/o/r/git/commits/aa218f56b14c9653891f9e74264a383fa43fefbd"
		      }
		    }
		  ]`)
	})

	ctx := t.Context()
	refs, _, err := client.Git.ListMatchingRefs(ctx, "o", "r", "heads/b")
	if err != nil {
		t.Fatalf("Git.ListMatchingRefs returned error: %v", err)
	}

	ref := refs[0]
	want := &Reference{
		Ref: Ptr("refs/heads/b"),
		URL: Ptr("https://api.github.com/repos/o/r/git/refs/heads/b"),
		Object: &GitObject{
			Type: Ptr("commit"),
			SHA:  Ptr("aa218f56b14c9653891f9e74264a383fa43fefbd"),
			URL:  Ptr("https://api.github.com/repos/o/r/git/commits/aa218f56b14c9653891f9e74264a383fa43fefbd"),
		},
	}
	if !cmp.Equal(ref, want) {
		t.Errorf("Git.ListMatchingRefs returned %+v, want %+v", ref, want)
	}

	const methodName = "ListMatchingRefs"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Git.ListMatchingRefs(ctx, "\n", "\n", "")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Git.ListMatchingRefs(ctx, "o", "r", "")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGitService_ListMatchingRefs_multipleRefs(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/git/matching-refs/heads/b", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `
		  [
		    {
			    "ref": "refs/heads/booger",
			    "url": "https://api.github.com/repos/o/r/git/refs/heads/booger",
			    "object": {
			      "type": "commit",
			      "sha": "aa218f56b14c9653891f9e74264a383fa43fefbd",
			      "url": "https://api.github.com/repos/o/r/git/commits/aa218f56b14c9653891f9e74264a383fa43fefbd"
			    }
		  	},
		    {
		      "ref": "refs/heads/bandsaw",
		      "url": "https://api.github.com/repos/o/r/git/refs/heads/bandsaw",
		      "object": {
		        "type": "commit",
		        "sha": "612077ae6dffb4d2fbd8ce0cccaa58893b07b5ac",
		        "url": "https://api.github.com/repos/o/r/git/commits/612077ae6dffb4d2fbd8ce0cccaa58893b07b5ac"
		      }
		    }
		  ]
		`)
	})

	ctx := t.Context()
	refs, _, err := client.Git.ListMatchingRefs(ctx, "o", "r", "heads/b")
	if err != nil {
		t.Errorf("Git.ListMatchingRefs returned error: %v", err)
	}

	want := &Reference{
		Ref: Ptr("refs/heads/booger"),
		URL: Ptr("https://api.github.com/repos/o/r/git/refs/heads/booger"),
		Object: &GitObject{
			Type: Ptr("commit"),
			SHA:  Ptr("aa218f56b14c9653891f9e74264a383fa43fefbd"),
			URL:  Ptr("https://api.github.com/repos/o/r/git/commits/aa218f56b14c9653891f9e74264a383fa43fefbd"),
		},
	}
	if !cmp.Equal(refs[0], want) {
		t.Errorf("Git.ListMatchingRefs returned %+v, want %+v", refs[0], want)
	}

	const methodName = "ListMatchingRefs"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Git.ListMatchingRefs(ctx, "\n", "\n", "")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Git.ListMatchingRefs(ctx, "o", "r", "")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGitService_ListMatchingRefs_noRefs(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/git/matching-refs/heads/b", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, "[]")
	})

	ctx := t.Context()
	refs, _, err := client.Git.ListMatchingRefs(ctx, "o", "r", "heads/b")
	if err != nil {
		t.Errorf("Git.ListMatchingRefs returned error: %v", err)
	}

	if len(refs) != 0 {
		t.Errorf("Git.ListMatchingRefs returned %+v, want an empty slice", refs)
	}

	const methodName = "ListMatchingRefs"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Git.ListMatchingRefs(ctx, "\n", "\n", "")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Git.ListMatchingRefs(ctx, "o", "r", "")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGitService_ListMatchingRefs_allRefs(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/git/matching-refs/", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `
		  [
		    {
		      "ref": "refs/heads/branchA",
		      "url": "https://api.github.com/repos/o/r/git/refs/heads/branchA",
		      "object": {
		        "type": "commit",
		        "sha": "aa218f56b14c9653891f9e74264a383fa43fefbd",
		        "url": "https://api.github.com/repos/o/r/git/commits/aa218f56b14c9653891f9e74264a383fa43fefbd"
		      }
		    },
		    {
		      "ref": "refs/heads/branchB",
		      "url": "https://api.github.com/repos/o/r/git/refs/heads/branchB",
		      "object": {
		        "type": "commit",
		        "sha": "aa218f56b14c9653891f9e74264a383fa43fefbd",
		        "url": "https://api.github.com/repos/o/r/git/commits/aa218f56b14c9653891f9e74264a383fa43fefbd"
		      }
		    }
		  ]`)
	})

	ctx := t.Context()
	refs, _, err := client.Git.ListMatchingRefs(ctx, "o", "r", "")
	if err != nil {
		t.Errorf("Git.ListMatchingRefs returned error: %v", err)
	}

	want := []*Reference{
		{
			Ref: Ptr("refs/heads/branchA"),
			URL: Ptr("https://api.github.com/repos/o/r/git/refs/heads/branchA"),
			Object: &GitObject{
				Type: Ptr("commit"),
				SHA:  Ptr("aa218f56b14c9653891f9e74264a383fa43fefbd"),
				URL:  Ptr("https://api.github.com/repos/o/r/git/commits/aa218f56b14c9653891f9e74264a383fa43fefbd"),
			},
		},
		{
			Ref: Ptr("refs/heads/branchB"),
			URL: Ptr("https://api.github.com/repos/o/r/git/refs/heads/branchB"),
			Object: &GitObject{
				Type: Ptr("commit"),
				SHA:  Ptr("aa218f56b14c9653891f9e74264a383fa43fefbd"),
				URL:  Ptr("https://api.github.com/repos/o/r/git/commits/aa218f56b14c9653891f9e74264a383fa43fefbd"),
			},
		},
	}
	if !cmp.Equal(refs, want) {
		t.Errorf("Git.ListMatchingRefs returned %+v, want %+v", refs, want)
	}

	const methodName = "ListMatchingRefs"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Git.ListMatchingRefs(ctx, "\n", "\n", "")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Git.ListMatchingRefs(ctx, "o", "r", "")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGitService_CreateRef(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	args := CreateRef{
		Ref: "refs/heads/b",
		SHA: "aa218f56b14c9653891f9e74264a383fa43fefbd",
	}

	mux.HandleFunc("/repos/o/r/git/refs", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, args)
		fmt.Fprint(w, `
		  {
		    "ref": "refs/heads/b",
		    "url": "https://api.github.com/repos/o/r/git/refs/heads/b",
		    "object": {
		      "type": "commit",
		      "sha": "aa218f56b14c9653891f9e74264a383fa43fefbd",
		      "url": "https://api.github.com/repos/o/r/git/commits/aa218f56b14c9653891f9e74264a383fa43fefbd"
		    }
		  }`)
	})

	ctx := t.Context()
	ref, _, err := client.Git.CreateRef(ctx, "o", "r", CreateRef{
		Ref: "refs/heads/b",
		SHA: "aa218f56b14c9653891f9e74264a383fa43fefbd",
	})
	if err != nil {
		t.Errorf("Git.CreateRef returned error: %v", err)
	}

	want := &Reference{
		Ref: Ptr("refs/heads/b"),
		URL: Ptr("https://api.github.com/repos/o/r/git/refs/heads/b"),
		Object: &GitObject{
			Type: Ptr("commit"),
			SHA:  Ptr("aa218f56b14c9653891f9e74264a383fa43fefbd"),
			URL:  Ptr("https://api.github.com/repos/o/r/git/commits/aa218f56b14c9653891f9e74264a383fa43fefbd"),
		},
	}
	if !cmp.Equal(ref, want) {
		t.Errorf("Git.CreateRef returned %+v, want %+v", ref, want)
	}

	// without 'refs/' prefix
	_, _, err = client.Git.CreateRef(ctx, "o", "r", CreateRef{
		Ref: "heads/b",
		SHA: "aa218f56b14c9653891f9e74264a383fa43fefbd",
	})
	if err != nil {
		t.Errorf("Git.CreateRef returned error: %v", err)
	}

	const methodName = "CreateRef"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Git.CreateRef(ctx, "o", "r", CreateRef{Ref: ""})
		return err
	})
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Git.CreateRef(ctx, "\n", "\n", CreateRef{
			Ref: "refs/heads/b",
			SHA: "aa218f56b14c9653891f9e74264a383fa43fefbd",
		})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Git.CreateRef(ctx, "o", "r", CreateRef{
			Ref: "refs/heads/b",
			SHA: "aa218f56b14c9653891f9e74264a383fa43fefbd",
		})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGitService_UpdateRef(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	args := UpdateRef{
		SHA:   "aa218f56b14c9653891f9e74264a383fa43fefbd",
		Force: Ptr(true),
	}

	mux.HandleFunc("/repos/o/r/git/refs/heads/b", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, args)
		fmt.Fprint(w, `
		  {
		    "ref": "refs/heads/b",
		    "url": "https://api.github.com/repos/o/r/git/refs/heads/b",
		    "object": {
		      "type": "commit",
		      "sha": "aa218f56b14c9653891f9e74264a383fa43fefbd",
		      "url": "https://api.github.com/repos/o/r/git/commits/aa218f56b14c9653891f9e74264a383fa43fefbd"
		    }
		  }`)
	})

	ctx := t.Context()
	ref, _, err := client.Git.UpdateRef(ctx, "o", "r", "refs/heads/b", UpdateRef{
		SHA:   "aa218f56b14c9653891f9e74264a383fa43fefbd",
		Force: Ptr(true),
	})
	if err != nil {
		t.Errorf("Git.UpdateRef returned error: %v", err)
	}

	want := &Reference{
		Ref: Ptr("refs/heads/b"),
		URL: Ptr("https://api.github.com/repos/o/r/git/refs/heads/b"),
		Object: &GitObject{
			Type: Ptr("commit"),
			SHA:  Ptr("aa218f56b14c9653891f9e74264a383fa43fefbd"),
			URL:  Ptr("https://api.github.com/repos/o/r/git/commits/aa218f56b14c9653891f9e74264a383fa43fefbd"),
		},
	}
	if !cmp.Equal(ref, want) {
		t.Errorf("Git.UpdateRef returned %+v, want %+v", ref, want)
	}

	// without 'refs/' prefix
	_, _, err = client.Git.UpdateRef(ctx, "o", "r", "heads/b", UpdateRef{
		SHA:   "aa218f56b14c9653891f9e74264a383fa43fefbd",
		Force: Ptr(true),
	})
	if err != nil {
		t.Errorf("Git.UpdateRef returned error: %v", err)
	}

	const methodName = "UpdateRef"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Git.UpdateRef(ctx, "o", "r", "", UpdateRef{SHA: "aa218f56b14c9653891f9e74264a383fa43fefbd"})
		return err
	})
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Git.UpdateRef(ctx, "o", "r", "refs/heads/b", UpdateRef{SHA: ""})
		return err
	})
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Git.UpdateRef(ctx, "\n", "\n", "refs/heads/b", UpdateRef{
			SHA:   "aa218f56b14c9653891f9e74264a383fa43fefbd",
			Force: Ptr(true),
		})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Git.UpdateRef(ctx, "o", "r", "refs/heads/b", UpdateRef{
			SHA:   "aa218f56b14c9653891f9e74264a383fa43fefbd",
			Force: Ptr(true),
		})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGitService_DeleteRef(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/git/refs/heads/b", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Git.DeleteRef(ctx, "o", "r", "refs/heads/b")
	if err != nil {
		t.Errorf("Git.DeleteRef returned error: %v", err)
	}

	// without 'refs/' prefix
	if _, err := client.Git.DeleteRef(ctx, "o", "r", "heads/b"); err != nil {
		t.Errorf("Git.DeleteRef returned error: %v", err)
	}

	const methodName = "DeleteRef"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Git.DeleteRef(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Git.DeleteRef(ctx, "o", "r", "refs/heads/b")
	})
}

func TestGitService_GetRef_pathEscape(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/git/ref/heads/b", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		if strings.Contains(r.URL.RawPath, "%2F") {
			t.Errorf("RawPath still contains escaped / as %%2F: %v", r.URL.RawPath)
		}
		fmt.Fprint(w, `
		  {
		    "ref": "refs/heads/b",
		    "url": "https://api.github.com/repos/o/r/git/refs/heads/b",
		    "object": {
		      "type": "commit",
		      "sha": "aa218f56b14c9653891f9e74264a383fa43fefbd",
		      "url": "https://api.github.com/repos/o/r/git/commits/aa218f56b14c9653891f9e74264a383fa43fefbd"
		    }
		  }`)
	})

	ctx := t.Context()
	_, _, err := client.Git.GetRef(ctx, "o", "r", "refs/heads/b")
	if err != nil {
		t.Fatalf("Git.GetRef returned error: %v", err)
	}

	const methodName = "GetRef"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Git.GetRef(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Git.GetRef(ctx, "o", "r", "refs/heads/b")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGitService_UpdateRef_pathEscape(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	args := UpdateRef{
		SHA:   "aa218f56b14c9653891f9e74264a383fa43fefbd",
		Force: Ptr(true),
	}

	mux.HandleFunc("/repos/o/r/git/refs/heads/b#1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, args)
		fmt.Fprint(w, `
		  {
		    "ref": "refs/heads/b#1",
		    "url": "https://api.github.com/repos/o/r/git/refs/heads/b%231",
		    "object": {
		      "type": "commit",
		      "sha": "aa218f56b14c9653891f9e74264a383fa43fefbd",
		      "url": "https://api.github.com/repos/o/r/git/commits/aa218f56b14c9653891f9e74264a383fa43fefbd"
		    }
		  }`)
	})

	ctx := t.Context()
	ref, _, err := client.Git.UpdateRef(ctx, "o", "r", "refs/heads/b#1", UpdateRef{
		SHA:   "aa218f56b14c9653891f9e74264a383fa43fefbd",
		Force: Ptr(true),
	})
	if err != nil {
		t.Errorf("Git.UpdateRef returned error: %v", err)
	}

	want := &Reference{
		Ref: Ptr("refs/heads/b#1"),
		URL: Ptr("https://api.github.com/repos/o/r/git/refs/heads/b%231"),
		Object: &GitObject{
			Type: Ptr("commit"),
			SHA:  Ptr("aa218f56b14c9653891f9e74264a383fa43fefbd"),
			URL:  Ptr("https://api.github.com/repos/o/r/git/commits/aa218f56b14c9653891f9e74264a383fa43fefbd"),
		},
	}
	if !cmp.Equal(ref, want) {
		t.Errorf("Git.UpdateRef returned %+v, want %+v", ref, want)
	}
}
