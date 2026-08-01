// Copyright 2013 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestRepositoriesService_ListCommits(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	// given
	mux.HandleFunc("/repos/o/r/commits", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r,
			values{
				"sha":    "s",
				"path":   "p",
				"author": "a",
				"since":  referenceTimeRaw,
				"until":  "2007-03-04T15:04:05Z",
			})
		fmt.Fprint(w, `[{"sha": "s"}]`)
	})

	opt := &CommitsListOptions{
		SHA:    "s",
		Path:   "p",
		Author: "a",
		Since:  referenceTime,
		Until:  time.Date(2007, time.March, 4, 15, 4, 5, 0, time.UTC),
	}
	ctx := t.Context()
	commits, _, err := client.Repositories.ListCommits(ctx, "o", "r", opt)
	if err != nil {
		t.Errorf("Repositories.ListCommits returned error: %v", err)
	}

	want := []*RepositoryCommit{{SHA: Ptr("s")}}
	if !cmp.Equal(commits, want) {
		t.Errorf("Repositories.ListCommits returned %+v, want %+v", commits, want)
	}

	const methodName = "ListCommits"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.ListCommits(ctx, "\n", "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListCommits(ctx, "o", "r", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_GetCommit(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/commits/s", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"per_page": "2", "page": "2"})
		fmt.Fprint(w, `{
		  "sha": "s",
		  "commit": { "message": "m" },
		  "author": { "login": "l" },
		  "committer": { "login": "l" },
		  "parents": [ { "sha": "s" } ],
		  "stats": { "additions": 104, "deletions": 4, "total": 108 },
		  "files": [
		    {
		      "filename": "f",
		      "additions": 10,
		      "deletions": 2,
		      "changes": 12,
		      "status": "s",
		      "patch": "p",
		      "blob_url": "b",
		      "raw_url": "r",
		      "contents_url": "c"
		    }
		  ]
		}`)
	})

	opts := &ListOptions{Page: 2, PerPage: 2}
	ctx := t.Context()
	commit, _, err := client.Repositories.GetCommit(ctx, "o", "r", "s", opts)
	if err != nil {
		t.Errorf("Repositories.GetCommit returned error: %v", err)
	}

	want := &RepositoryCommit{
		SHA: Ptr("s"),
		Commit: &Commit{
			Message: Ptr("m"),
		},
		Author: &User{
			Login: Ptr("l"),
		},
		Committer: &User{
			Login: Ptr("l"),
		},
		Parents: []*Commit{
			{
				SHA: Ptr("s"),
			},
		},
		Stats: &CommitStats{
			Additions: Ptr(104),
			Deletions: Ptr(4),
			Total:     Ptr(108),
		},
		Files: []*CommitFile{
			{
				Filename:    Ptr("f"),
				Additions:   Ptr(10),
				Deletions:   Ptr(2),
				Changes:     Ptr(12),
				Status:      Ptr("s"),
				Patch:       Ptr("p"),
				BlobURL:     Ptr("b"),
				RawURL:      Ptr("r"),
				ContentsURL: Ptr("c"),
			},
		},
	}
	if !cmp.Equal(commit, want) {
		t.Errorf("Repositories.GetCommit returned \n%+v, want \n%+v", commit, want)
	}

	const methodName = "GetCommit"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetCommit(ctx, "\n", "\n", "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetCommit(ctx, "o", "r", "s", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_GetCommitRaw_diff(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	const rawStr = "@@diff content"

	mux.HandleFunc("/repos/o/r/commits/s", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeV3Diff)
		fmt.Fprint(w, rawStr)
	})

	ctx := t.Context()
	got, _, err := client.Repositories.GetCommitRaw(ctx, "o", "r", "s", RawOptions{Type: Diff})
	if err != nil {
		t.Fatalf("Repositories.GetCommitRaw returned error: %v", err)
	}
	want := rawStr
	if got != want {
		t.Errorf("Repositories.GetCommitRaw returned %v want %v", got, want)
	}

	const methodName = "GetCommitRaw"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetCommitRaw(ctx, "\n", "\n", "\n", RawOptions{Type: Diff})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetCommitRaw(ctx, "o", "r", "s", RawOptions{Type: Diff})
		if got != "" {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want ''", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_GetCommitRaw_patch(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	const rawStr = "@@patch content"

	mux.HandleFunc("/repos/o/r/commits/s", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeV3Patch)
		fmt.Fprint(w, rawStr)
	})

	ctx := t.Context()
	got, _, err := client.Repositories.GetCommitRaw(ctx, "o", "r", "s", RawOptions{Type: Patch})
	if err != nil {
		t.Fatalf("Repositories.GetCommitRaw returned error: %v", err)
	}
	want := rawStr
	if got != want {
		t.Errorf("Repositories.GetCommitRaw returned %v want %v", got, want)
	}
}

func TestRepositoriesService_GetCommitRaw_invalid(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Repositories.GetCommitRaw(ctx, "o", "r", "s", RawOptions{100})
	if err == nil {
		t.Fatal("Repositories.GetCommitRaw should return error")
	}
	if !strings.Contains(err.Error(), "unsupported raw type") {
		t.Error("Repositories.GetCommitRaw should return unsupported raw type error")
	}
}

func TestRepositoriesService_GetCommitSHA1(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	const sha1 = "01234abcde"

	mux.HandleFunc("/repos/o/r/commits/master", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeV3SHA)

		fmt.Fprint(w, sha1)
	})

	ctx := t.Context()
	got, _, err := client.Repositories.GetCommitSHA1(ctx, "o", "r", "master", "")
	if err != nil {
		t.Errorf("Repositories.GetCommitSHA1 returned error: %v", err)
	}

	want := sha1
	if got != want {
		t.Errorf("Repositories.GetCommitSHA1 = %v, want %v", got, want)
	}

	mux.HandleFunc("/repos/o/r/commits/tag", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeV3SHA)
		testHeader(t, r, "If-None-Match", `"`+sha1+`"`)

		w.WriteHeader(http.StatusNotModified)
	})

	got, _, err = client.Repositories.GetCommitSHA1(ctx, "o", "r", "tag", sha1)
	if err == nil {
		t.Error("Expected HTTP 304 response")
	}

	want = ""
	if got != want {
		t.Errorf("Repositories.GetCommitSHA1 = %v, want %v", got, want)
	}

	const methodName = "GetCommitSHA1"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetCommitSHA1(ctx, "\n", "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetCommitSHA1(ctx, "o", "r", "master", "")
		if got != "" {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want ''", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_NonAlphabetCharacter_GetCommitSHA1(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	const sha1 = "01234abcde"

	mux.HandleFunc("/repos/o/r/commits/master%2520hash", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeV3SHA)

		fmt.Fprint(w, sha1)
	})

	ctx := t.Context()
	got, _, err := client.Repositories.GetCommitSHA1(ctx, "o", "r", "master%20hash", "")
	if err != nil {
		t.Errorf("Repositories.GetCommitSHA1 returned error: %v", err)
	}

	if want := sha1; got != want {
		t.Errorf("Repositories.GetCommitSHA1 = %v, want %v", got, want)
	}

	mux.HandleFunc("/repos/o/r/commits/tag", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeV3SHA)
		testHeader(t, r, "If-None-Match", `"`+sha1+`"`)

		w.WriteHeader(http.StatusNotModified)
	})

	got, _, err = client.Repositories.GetCommitSHA1(ctx, "o", "r", "tag", sha1)
	if err == nil {
		t.Error("Expected HTTP 304 response")
	}

	if want := ""; got != want {
		t.Errorf("Repositories.GetCommitSHA1 = %v, want %v", got, want)
	}
}

func TestRepositoriesService_TrailingPercent_GetCommitSHA1(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	const sha1 = "01234abcde"

	mux.HandleFunc("/repos/o/r/commits/comm%", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeV3SHA)

		fmt.Fprint(w, sha1)
	})

	ctx := t.Context()
	got, _, err := client.Repositories.GetCommitSHA1(ctx, "o", "r", "comm%", "")
	if err != nil {
		t.Errorf("Repositories.GetCommitSHA1 returned error: %v", err)
	}

	if want := sha1; got != want {
		t.Errorf("Repositories.GetCommitSHA1 = %v, want %v", got, want)
	}

	mux.HandleFunc("/repos/o/r/commits/tag", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeV3SHA)
		testHeader(t, r, "If-None-Match", `"`+sha1+`"`)

		w.WriteHeader(http.StatusNotModified)
	})

	got, _, err = client.Repositories.GetCommitSHA1(ctx, "o", "r", "tag", sha1)
	if err == nil {
		t.Error("Expected HTTP 304 response")
	}

	if want := ""; got != want {
		t.Errorf("Repositories.GetCommitSHA1 = %v, want %v", got, want)
	}
}

func TestRepositoriesService_CompareCommits(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		base string
		head string
	}{
		{base: "b", head: "h"},
		{base: "123base", head: "head123"},
		{base: "`~!@#$%^&*()_+-=[]\\{}|;':\",./<>?/*-+123base", head: "head123`~!@#$%^&*()_+-=[]\\{}|;':\",./<>?/*-+"},
	}

	for i, sample := range testCases {
		t.Run(fmt.Sprintf("case #%v", i+1), func(t *testing.T) {
			t.Parallel()
			client, mux, _ := setup(t)

			base := sample.base
			head := sample.head

			encodedBase := url.PathEscape(base)
			encodedHead := url.PathEscape(head)

			escapedBase := url.QueryEscape(base)
			escapedHead := url.QueryEscape(head)

			pattern := fmt.Sprintf("/repos/o/r/compare/%v...%v", encodedBase, encodedHead)

			mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
				testMethod(t, r, "GET")
				testFormValues(t, r, values{"per_page": "2", "page": "2"})
				fmt.Fprintf(w, `{
  "base_commit": {
    "sha": "s",
    "commit": {
      "author": { "name": "n" },
      "committer": { "name": "n" },
      "message": "m",
      "tree": { "sha": "t" }
    },
    "author": { "login": "l" },
    "committer": { "login": "l" },
    "parents": [ { "sha": "s" } ]
  },
  "status": "s",
  "ahead_by": 1,
  "behind_by": 2,
  "total_commits": 1,
  "commits": [
    {
      "sha": "s",
      "commit": { "author": { "name": "n" } },
      "author": { "login": "l" },
      "committer": { "login": "l" },
      "parents": [ { "sha": "s" } ]
    }
  ],
  "files": [ { "filename": "f" } ],
  "html_url":      "https://github.com/o/r/compare/%[1]v...%[2]v",
  "permalink_url": "https://github.com/o/r/compare/o:bbcd538c8e72b8c175046e27cc8f907076331401...o:0328041d1152db8ae77652d1618a02e57f745f17",
  "diff_url":      "https://github.com/o/r/compare/%[1]v...%[2]v.diff",
  "patch_url":     "https://github.com/o/r/compare/%[1]v...%[2]v.patch",
  "url":           "https://api.github.com/repos/o/r/compare/%[1]v...%[2]v"
}`, escapedBase, escapedHead)
			})

			opts := &ListOptions{Page: 2, PerPage: 2}
			ctx := t.Context()
			got, _, err := client.Repositories.CompareCommits(ctx, "o", "r", base, head, opts)
			if err != nil {
				t.Errorf("Repositories.CompareCommits returned error: %v", err)
			}

			want := &CommitsComparison{
				BaseCommit: &RepositoryCommit{
					SHA: Ptr("s"),
					Commit: &Commit{
						Author:    &CommitAuthor{Name: Ptr("n")},
						Committer: &CommitAuthor{Name: Ptr("n")},
						Message:   Ptr("m"),
						Tree:      &Tree{SHA: Ptr("t")},
					},
					Author:    &User{Login: Ptr("l")},
					Committer: &User{Login: Ptr("l")},
					Parents: []*Commit{
						{
							SHA: Ptr("s"),
						},
					},
				},
				Status:       Ptr("s"),
				AheadBy:      Ptr(1),
				BehindBy:     Ptr(2),
				TotalCommits: Ptr(1),
				Commits: []*RepositoryCommit{
					{
						SHA: Ptr("s"),
						Commit: &Commit{
							Author: &CommitAuthor{Name: Ptr("n")},
						},
						Author:    &User{Login: Ptr("l")},
						Committer: &User{Login: Ptr("l")},
						Parents: []*Commit{
							{
								SHA: Ptr("s"),
							},
						},
					},
				},
				Files: []*CommitFile{
					{
						Filename: Ptr("f"),
					},
				},
				HTMLURL:      Ptr(fmt.Sprintf("https://github.com/o/r/compare/%v...%v", escapedBase, escapedHead)),
				PermalinkURL: Ptr("https://github.com/o/r/compare/o:bbcd538c8e72b8c175046e27cc8f907076331401...o:0328041d1152db8ae77652d1618a02e57f745f17"),
				DiffURL:      Ptr(fmt.Sprintf("https://github.com/o/r/compare/%v...%v.diff", escapedBase, escapedHead)),
				PatchURL:     Ptr(fmt.Sprintf("https://github.com/o/r/compare/%v...%v.patch", escapedBase, escapedHead)),
				URL:          Ptr(fmt.Sprintf("https://api.github.com/repos/o/r/compare/%v...%v", escapedBase, escapedHead)),
			}

			if !cmp.Equal(got, want) {
				t.Errorf("Repositories.CompareCommits returned \n%+v, want \n%+v", got, want)
			}

			const methodName = "CompareCommits"
			testBadOptions(t, methodName, func() (err error) {
				_, _, err = client.Repositories.CompareCommits(ctx, "\n", "\n", "\n", "\n", opts)
				return err
			})

			testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
				got, resp, err := client.Repositories.CompareCommits(ctx, "o", "r", base, head, opts)
				if got != nil {
					t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
				}
				return resp, err
			})
		})
	}
}

func TestRepositoriesService_CompareCommitsRaw_diff(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		base string
		head string
	}{
		{base: "b", head: "h"},
		{base: "123base", head: "head123"},
		{base: "`~!@#$%^&*()_+-=[]\\{}|;':\",./<>?/*-+123base", head: "head123`~!@#$%^&*()_+-=[]\\{}|;':\",./<>?/*-+"},
	}

	for i, sample := range testCases {
		t.Run(fmt.Sprintf("case #%v", i+1), func(t *testing.T) {
			t.Parallel()
			client, mux, _ := setup(t)

			base := sample.base
			head := sample.head

			encodedBase := url.PathEscape(base)
			encodedHead := url.PathEscape(head)

			pattern := fmt.Sprintf("/repos/o/r/compare/%v...%v", encodedBase, encodedHead)
			const rawStr = "@@diff content"

			mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
				testMethod(t, r, "GET")
				testHeader(t, r, "Accept", mediaTypeV3Diff)
				fmt.Fprint(w, rawStr)
			})

			ctx := t.Context()
			got, _, err := client.Repositories.CompareCommitsRaw(ctx, "o", "r", base, head, RawOptions{Type: Diff})
			if err != nil {
				t.Fatalf("Repositories.CompareCommitsRaw returned error: %v", err)
			}
			want := rawStr
			if got != want {
				t.Errorf("Repositories.CompareCommitsRaw returned %v want %v", got, want)
			}

			const methodName = "CompareCommitsRaw"
			testBadOptions(t, methodName, func() (err error) {
				_, _, err = client.Repositories.CompareCommitsRaw(ctx, "\n", "\n", "\n", "\n", RawOptions{Type: Diff})
				return err
			})

			testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
				got, resp, err := client.Repositories.CompareCommitsRaw(ctx, "o", "r", base, head, RawOptions{Type: Diff})
				if got != "" {
					t.Errorf("testNewRequestAndDoFailure %v = %#v, want ''", methodName, got)
				}
				return resp, err
			})
		})
	}
}

func TestRepositoriesService_CompareCommitsRaw_patch(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		base string
		head string
	}{
		{base: "b", head: "h"},
		{base: "123base", head: "head123"},
		{base: "`~!@#$%^&*()_+-=[]\\{}|;':\",./<>?/*-+123base", head: "head123`~!@#$%^&*()_+-=[]\\{}|;':\",./<>?/*-+"},
	}

	for i, sample := range testCases {
		t.Run(fmt.Sprintf("case #%v", i+1), func(t *testing.T) {
			t.Parallel()
			client, mux, _ := setup(t)

			base := sample.base
			head := sample.head

			encodedBase := url.PathEscape(base)
			encodedHead := url.PathEscape(head)

			pattern := fmt.Sprintf("/repos/o/r/compare/%v...%v", encodedBase, encodedHead)
			const rawStr = "@@patch content"

			mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
				testMethod(t, r, "GET")
				testHeader(t, r, "Accept", mediaTypeV3Patch)
				fmt.Fprint(w, rawStr)
			})

			ctx := t.Context()
			got, _, err := client.Repositories.CompareCommitsRaw(ctx, "o", "r", base, head, RawOptions{Type: Patch})
			if err != nil {
				t.Fatalf("Repositories.CompareCommitsRaw returned error: %v", err)
			}
			want := rawStr
			if got != want {
				t.Errorf("Repositories.CompareCommitsRaw returned %v want %v", got, want)
			}
		})
	}
}

func TestRepositoriesService_CompareCommitsRaw_invalid(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	testCases := []struct {
		base string
		head string
	}{
		{base: "b", head: "h"},
		{base: "123base", head: "head123"},
		{base: "`~!@#$%^&*()_+-=[]\\{}|;':\",./<>?/*-+123base", head: "head123`~!@#$%^&*()_+-=[]\\{}|;':\",./<>?/*-+"},
	}

	for i, sample := range testCases {
		t.Run(fmt.Sprintf("case #%v", i+1), func(t *testing.T) {
			t.Parallel()
			client, _, _ := setup(t)
			_, _, err := client.Repositories.CompareCommitsRaw(ctx, "o", "r", sample.base, sample.head, RawOptions{100})
			if err == nil {
				t.Fatal("Repositories.GetCommitRaw should return error")
			}
			if !strings.Contains(err.Error(), "unsupported raw type") {
				t.Error("Repositories.GetCommitRaw should return unsupported raw type error")
			}
		})
	}
}

func TestRepositoriesService_ListBranchesHeadCommit(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/commits/s/branches-where-head", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{"name": "b","commit":{"sha":"2e90302801c870f17b6152327d9b9a03c8eca0e2","url":"https://api.github.com/repos/google/go-github/commits/2e90302801c870f17b6152327d9b9a03c8eca0e2"},"protected":true}]`)
	})

	ctx := t.Context()
	branches, _, err := client.Repositories.ListBranchesHeadCommit(ctx, "o", "r", "s")
	if err != nil {
		t.Errorf("Repositories.ListBranchesHeadCommit returned error: %v", err)
	}

	want := []*BranchCommit{
		{
			Name: Ptr("b"),
			Commit: &Commit{
				SHA: Ptr("2e90302801c870f17b6152327d9b9a03c8eca0e2"),
				URL: Ptr("https://api.github.com/repos/google/go-github/commits/2e90302801c870f17b6152327d9b9a03c8eca0e2"),
			},
			Protected: Ptr(true),
		},
	}
	if !cmp.Equal(branches, want) {
		t.Errorf("Repositories.ListBranchesHeadCommit returned %+v, want %+v", branches, want)
	}

	const methodName = "ListBranchesHeadCommit"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.ListBranchesHeadCommit(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListBranchesHeadCommit(ctx, "o", "r", "s")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
