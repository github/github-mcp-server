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

func TestGistsService_List_specifiedUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/gists", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"since": referenceTimeRaw,
		})
		fmt.Fprint(w, `[{"id": "1"}]`)
	})

	opt := &GistListOptions{Since: referenceTime}
	ctx := t.Context()
	gists, _, err := client.Gists.List(ctx, "u", opt)
	if err != nil {
		t.Errorf("Gists.List returned error: %v", err)
	}

	want := []*Gist{{ID: Ptr("1")}}
	if !cmp.Equal(gists, want) {
		t.Errorf("Gists.List returned %+v, want %+v", gists, want)
	}

	const methodName = "List"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Gists.List(ctx, "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Gists.List(ctx, "u", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGistsService_List_authenticatedUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/gists", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{"id": "1"}]`)
	})

	ctx := t.Context()
	gists, _, err := client.Gists.List(ctx, "", nil)
	if err != nil {
		t.Errorf("Gists.List returned error: %v", err)
	}

	want := []*Gist{{ID: Ptr("1")}}
	if !cmp.Equal(gists, want) {
		t.Errorf("Gists.List returned %+v, want %+v", gists, want)
	}

	const methodName = "List"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Gists.List(ctx, "\n", nil)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Gists.List(ctx, "", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGistsService_List_invalidUser(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Gists.List(ctx, "%", nil)
	testURLParseError(t, err)
}

func TestGistsService_ListAll(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/gists/public", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"since": referenceTimeRaw,
		})
		fmt.Fprint(w, `[{"id": "1"}]`)
	})

	opt := &GistListOptions{Since: referenceTime}
	ctx := t.Context()
	gists, _, err := client.Gists.ListAll(ctx, opt)
	if err != nil {
		t.Errorf("Gists.ListAll returned error: %v", err)
	}

	want := []*Gist{{ID: Ptr("1")}}
	if !cmp.Equal(gists, want) {
		t.Errorf("Gists.ListAll returned %+v, want %+v", gists, want)
	}

	const methodName = "ListAll"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Gists.ListAll(ctx, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGistsService_ListStarred(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/gists/starred", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"since": referenceTimeRaw,
		})
		fmt.Fprint(w, `[{"id": "1"}]`)
	})

	opt := &GistListOptions{Since: referenceTime}
	ctx := t.Context()
	gists, _, err := client.Gists.ListStarred(ctx, opt)
	if err != nil {
		t.Errorf("Gists.ListStarred returned error: %v", err)
	}

	want := []*Gist{{ID: Ptr("1")}}
	if !cmp.Equal(gists, want) {
		t.Errorf("Gists.ListStarred returned %+v, want %+v", gists, want)
	}

	const methodName = "ListStarred"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Gists.ListStarred(ctx, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGistsService_Get(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/gists/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id": "1"}`)
	})

	ctx := t.Context()
	gist, _, err := client.Gists.Get(ctx, "1")
	if err != nil {
		t.Errorf("Gists.Get returned error: %v", err)
	}

	want := &Gist{ID: Ptr("1")}
	if !cmp.Equal(gist, want) {
		t.Errorf("Gists.Get returned %+v, want %+v", gist, want)
	}

	const methodName = "Get"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Gists.Get(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Gists.Get(ctx, "1")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGistsService_Get_invalidID(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Gists.Get(ctx, "%")
	testURLParseError(t, err)
}

func TestGistsService_GetRevision(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/gists/1/s", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id": "1"}`)
	})

	ctx := t.Context()
	gist, _, err := client.Gists.GetRevision(ctx, "1", "s")
	if err != nil {
		t.Errorf("Gists.GetRevision returned error: %v", err)
	}

	want := &Gist{ID: Ptr("1")}
	if !cmp.Equal(gist, want) {
		t.Errorf("Gists.GetRevision returned %+v, want %+v", gist, want)
	}

	const methodName = "GetRevision"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Gists.GetRevision(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Gists.GetRevision(ctx, "1", "s")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGistsService_GetRevision_invalidID(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Gists.GetRevision(ctx, "%", "%")
	testURLParseError(t, err)
}

func TestGistsService_Create(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := CreateGistRequest{
		Description: Ptr("Gist description"),
		Public:      Ptr(false),
		Files: map[GistFilename]*CreateGistFile{
			"test.txt": {Content: "Gist file content"},
		},
	}

	mux.HandleFunc("/gists", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)

		fmt.Fprint(w,
			`
			{
				"id": "1",
				"description": "Gist description",
				"public": false,
				"files": {
					"test.txt": {
						"filename": "test.txt"
					}
				}
			}`)
	})

	ctx := t.Context()
	gist, _, err := client.Gists.Create(ctx, input)
	if err != nil {
		t.Errorf("Gists.Create returned error: %v", err)
	}

	want := &Gist{
		ID:          Ptr("1"),
		Description: Ptr("Gist description"),
		Public:      Ptr(false),
		Files: map[GistFilename]GistFile{
			"test.txt": {Filename: Ptr("test.txt")},
		},
	}
	if !cmp.Equal(gist, want) {
		t.Errorf("Gists.Create returned %+v, want %+v", gist, want)
	}

	const methodName = "Create"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Gists.Create(ctx, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGistsService_Update(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := UpdateGistRequest{
		Description: Ptr("New description"),
		Files: map[GistFilename]*UpdateGistFile{
			"new.txt": {Content: Ptr("new file content")},
			// A nil value deletes the file from the gist.
			"old.txt": nil,
		},
	}

	mux.HandleFunc("/gists/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)

		fmt.Fprint(w,
			`
			{
				"id": "1",
				"description": "new description",
				"public": false,
				"files": {
					"test.txt": {
						"filename": "test.txt"
					},
					"new.txt": {
						"filename": "new.txt"
					}
				}
			}`)
	})

	ctx := t.Context()
	gist, _, err := client.Gists.Update(ctx, "1", input)
	if err != nil {
		t.Errorf("Gists.Update returned error: %v", err)
	}

	want := &Gist{
		ID:          Ptr("1"),
		Description: Ptr("new description"),
		Public:      Ptr(false),
		Files: map[GistFilename]GistFile{
			"test.txt": {Filename: Ptr("test.txt")},
			"new.txt":  {Filename: Ptr("new.txt")},
		},
	}
	if !cmp.Equal(gist, want) {
		t.Errorf("Gists.Update returned %+v, want %+v", gist, want)
	}

	const methodName = "Update"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Gists.Update(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Gists.Update(ctx, "1", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGistsService_Update_invalidID(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Gists.Update(ctx, "%", UpdateGistRequest{})
	testURLParseError(t, err)
}

func TestGistsService_ListCommits(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/gists/1/commits", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, nil)
		fmt.Fprint(w, `
		  [
		    {
		      "url": "https://api.github.com/gists/1/1",
		      "version": "1",
		      "user": {
		        "id": 1
		      },
		      "change_status": {
		        "deletions": 0,
		        "additions": 180,
		        "total": 180
		      },
		      "committed_at": `+referenceTimeStr+`
		    }
		  ]
		`)
	})

	ctx := t.Context()
	gistCommits, _, err := client.Gists.ListCommits(ctx, "1", nil)
	if err != nil {
		t.Errorf("Gists.ListCommits returned error: %v", err)
	}

	want := []*GistCommit{{
		URL:         Ptr("https://api.github.com/gists/1/1"),
		Version:     Ptr("1"),
		User:        &User{ID: Ptr(int64(1))},
		CommittedAt: &referenceTimestamp,
		ChangeStatus: &CommitStats{
			Additions: Ptr(180),
			Deletions: Ptr(0),
			Total:     Ptr(180),
		},
	}}

	if !cmp.Equal(gistCommits, want) {
		t.Errorf("Gists.ListCommits returned %+v, want %+v", gistCommits, want)
	}

	const methodName = "ListCommits"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Gists.ListCommits(ctx, "\n", nil)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Gists.ListCommits(ctx, "1", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGistsService_ListCommits_withOptions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/gists/1/commits", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"page": "2",
		})
		fmt.Fprint(w, `[]`)
	})

	ctx := t.Context()
	_, _, err := client.Gists.ListCommits(ctx, "1", &ListOptions{Page: 2})
	if err != nil {
		t.Errorf("Gists.ListCommits returned error: %v", err)
	}

	const methodName = "ListCommits"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Gists.ListCommits(ctx, "\n", &ListOptions{Page: 2})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Gists.ListCommits(ctx, "1", &ListOptions{Page: 2})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGistsService_ListCommits_invalidID(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Gists.ListCommits(ctx, "%", nil)
	testURLParseError(t, err)
}

func TestGistsService_Delete(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/gists/1", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Gists.Delete(ctx, "1")
	if err != nil {
		t.Errorf("Gists.Delete returned error: %v", err)
	}

	const methodName = "Delete"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Gists.Delete(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Gists.Delete(ctx, "1")
	})
}

func TestGistsService_Delete_invalidID(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Gists.Delete(ctx, "%")
	testURLParseError(t, err)
}

func TestGistsService_Star(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/gists/1/star", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
	})

	ctx := t.Context()
	_, err := client.Gists.Star(ctx, "1")
	if err != nil {
		t.Errorf("Gists.Star returned error: %v", err)
	}

	const methodName = "Star"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Gists.Star(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Gists.Star(ctx, "1")
	})
}

func TestGistsService_Star_invalidID(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Gists.Star(ctx, "%")
	testURLParseError(t, err)
}

func TestGistsService_Unstar(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/gists/1/star", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Gists.Unstar(ctx, "1")
	if err != nil {
		t.Errorf("Gists.Unstar returned error: %v", err)
	}

	const methodName = "Unstar"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Gists.Unstar(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Gists.Unstar(ctx, "1")
	})
}

func TestGistsService_Unstar_invalidID(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Gists.Unstar(ctx, "%")
	testURLParseError(t, err)
}

func TestGistsService_IsStarred_hasStar(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/gists/1/star", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	star, _, err := client.Gists.IsStarred(ctx, "1")
	if err != nil {
		t.Errorf("Gists.IsStarred returned error: %v", err)
	}
	if want := true; star != want {
		t.Errorf("Gists.IsStarred returned %+v, want %+v", star, want)
	}

	const methodName = "IsStarred"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Gists.IsStarred(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Gists.IsStarred(ctx, "1")
		if got {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want false", methodName, got)
		}
		return resp, err
	})
}

func TestGistsService_IsStarred_noStar(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/gists/1/star", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNotFound)
	})

	ctx := t.Context()
	star, _, err := client.Gists.IsStarred(ctx, "1")
	if err != nil {
		t.Errorf("Gists.IsStarred returned error: %v", err)
	}
	if want := false; star != want {
		t.Errorf("Gists.IsStarred returned %+v, want %+v", star, want)
	}

	const methodName = "IsStarred"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Gists.IsStarred(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Gists.IsStarred(ctx, "1")
		if got {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want false", methodName, got)
		}
		return resp, err
	})
}

func TestGistsService_IsStarred_invalidID(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Gists.IsStarred(ctx, "%")
	testURLParseError(t, err)
}

func TestGistsService_Fork(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/gists/1/forks", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{"id": "2"}`)
	})

	ctx := t.Context()
	gist, _, err := client.Gists.Fork(ctx, "1")
	if err != nil {
		t.Errorf("Gists.Fork returned error: %v", err)
	}

	want := &Gist{ID: Ptr("2")}
	if !cmp.Equal(gist, want) {
		t.Errorf("Gists.Fork returned %+v, want %+v", gist, want)
	}

	const methodName = "Fork"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Gists.Fork(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Gists.Fork(ctx, "1")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGistsService_ListForks(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/gists/1/forks", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, nil)
		fmt.Fprint(w, `
		  [
		    {"url": "https://api.github.com/gists/1",
		     "user": {"id": 1},
		     "id": "1",
		     "created_at": `+refTimeStr(1136178000)+`,
		     "updated_at": `+refTimeStr(1136178001)+`
		    }
		  ]
		`)
	})

	ctx := t.Context()
	gistForks, _, err := client.Gists.ListForks(ctx, "1", nil)
	if err != nil {
		t.Errorf("Gists.ListForks returned error: %v", err)
	}

	want := []*GistFork{{
		URL:       Ptr("https://api.github.com/gists/1"),
		ID:        Ptr("1"),
		User:      &User{ID: Ptr(int64(1))},
		CreatedAt: refTimestamp(1136178000),
		UpdatedAt: refTimestamp(1136178001),
	}}

	if !cmp.Equal(gistForks, want) {
		t.Errorf("Gists.ListForks returned %+v, want %+v", gistForks, want)
	}

	const methodName = "ListForks"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Gists.ListForks(ctx, "\n", nil)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Gists.ListForks(ctx, "1", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGistsService_ListForks_withOptions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/gists/1/forks", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"page": "2",
		})
		fmt.Fprint(w, `[]`)
	})

	ctx := t.Context()
	gistForks, _, err := client.Gists.ListForks(ctx, "1", &ListOptions{Page: 2})
	if err != nil {
		t.Errorf("Gists.ListForks returned error: %v", err)
	}

	want := []*GistFork{}
	if !cmp.Equal(gistForks, want) {
		t.Errorf("Gists.ListForks returned %+v, want %+v", gistForks, want)
	}

	const methodName = "ListForks"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Gists.ListForks(ctx, "\n", &ListOptions{Page: 2})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Gists.ListForks(ctx, "1", &ListOptions{Page: 2})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
