// Copyright 2016 The go-github AUTHORS. All rights reserved.
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

func TestReactionsService_ListCommentReactions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/comments/1/reactions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)

		testFormValues(t, r, values{"content": "+1"})
		fmt.Fprint(w, `[{"id":1,"user":{"login":"l","id":2},"content":"+1"}]`)
	})

	opt := &ListReactionOptions{Content: "+1"}
	ctx := t.Context()
	reactions, _, err := client.Reactions.ListCommentReactions(ctx, "o", "r", 1, opt)
	if err != nil {
		t.Errorf("ListCommentReactions returned error: %v", err)
	}
	want := []*Reaction{{ID: Ptr(int64(1)), User: &User{Login: Ptr("l"), ID: Ptr(int64(2))}, Content: Ptr("+1")}}
	if !cmp.Equal(reactions, want) {
		t.Errorf("ListCommentReactions = %+v, want %+v", reactions, want)
	}

	const methodName = "ListCommentReactions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Reactions.ListCommentReactions(ctx, "\n", "\n", -1, opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Reactions.ListCommentReactions(ctx, "o", "r", 1, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestReactionsService_CreateCommentReaction(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/comments/1/reactions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)

		w.WriteHeader(http.StatusCreated)
		assertWrite(t, w, []byte(`{"id":1,"user":{"login":"l","id":2},"content":"+1"}`))
	})

	ctx := t.Context()
	got, _, err := client.Reactions.CreateCommentReaction(ctx, "o", "r", 1, "+1")
	if err != nil {
		t.Errorf("CreateCommentReaction returned error: %v", err)
	}
	want := &Reaction{ID: Ptr(int64(1)), User: &User{Login: Ptr("l"), ID: Ptr(int64(2))}, Content: Ptr("+1")}
	if !cmp.Equal(got, want) {
		t.Errorf("CreateCommentReaction = %+v, want %+v", got, want)
	}

	const methodName = "CreateCommentReaction"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Reactions.CreateCommentReaction(ctx, "\n", "\n", -1, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Reactions.CreateCommentReaction(ctx, "o", "r", 1, "+1")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestReactionsService_ListIssueReactions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/1/reactions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)
		testFormValues(t, r, values{"content": "+1"})

		w.WriteHeader(http.StatusOK)
		assertWrite(t, w, []byte(`[{"id":1,"user":{"login":"l","id":2},"content":"+1"}]`))
	})

	opt := &ListReactionOptions{Content: "+1"}
	ctx := t.Context()
	got, _, err := client.Reactions.ListIssueReactions(ctx, "o", "r", 1, opt)
	if err != nil {
		t.Errorf("ListIssueReactions returned error: %v", err)
	}
	want := []*Reaction{{ID: Ptr(int64(1)), User: &User{Login: Ptr("l"), ID: Ptr(int64(2))}, Content: Ptr("+1")}}
	if !cmp.Equal(got, want) {
		t.Errorf("ListIssueReactions = %+v, want %+v", got, want)
	}
}

func TestReactionsService_ListIssueReactions_coverage(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()

	const methodName = "ListIssueReactions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Reactions.ListIssueReactions(ctx, "\n", "\n", -1, &ListReactionOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Reactions.ListIssueReactions(ctx, "o", "r", 1, nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestReactionsService_CreateIssueReaction(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/1/reactions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)

		w.WriteHeader(http.StatusCreated)
		assertWrite(t, w, []byte(`{"id":1,"user":{"login":"l","id":2},"content":"+1"}`))
	})

	ctx := t.Context()
	got, _, err := client.Reactions.CreateIssueReaction(ctx, "o", "r", 1, "+1")
	if err != nil {
		t.Errorf("CreateIssueReaction returned error: %v", err)
	}
	want := &Reaction{ID: Ptr(int64(1)), User: &User{Login: Ptr("l"), ID: Ptr(int64(2))}, Content: Ptr("+1")}
	if !cmp.Equal(got, want) {
		t.Errorf("CreateIssueReaction = %+v, want %+v", got, want)
	}

	const methodName = "CreateIssueReaction"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Reactions.CreateIssueReaction(ctx, "\n", "\n", -1, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Reactions.CreateIssueReaction(ctx, "o", "r", 1, "+1")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestReactionsService_ListIssueCommentReactions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/comments/1/reactions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)
		testFormValues(t, r, values{"content": "+1"})

		w.WriteHeader(http.StatusOK)
		assertWrite(t, w, []byte(`[{"id":1,"user":{"login":"l","id":2},"content":"+1"}]`))
	})

	opt := &ListReactionOptions{Content: "+1"}
	ctx := t.Context()
	got, _, err := client.Reactions.ListIssueCommentReactions(ctx, "o", "r", 1, opt)
	if err != nil {
		t.Errorf("ListIssueCommentReactions returned error: %v", err)
	}
	want := []*Reaction{{ID: Ptr(int64(1)), User: &User{Login: Ptr("l"), ID: Ptr(int64(2))}, Content: Ptr("+1")}}
	if !cmp.Equal(got, want) {
		t.Errorf("ListIssueCommentReactions = %+v, want %+v", got, want)
	}
}

func TestReactionsService_ListIssueCommentReactions_coverage(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()

	const methodName = "ListIssueCommentReactions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Reactions.ListIssueCommentReactions(ctx, "\n", "\n", -1, &ListReactionOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Reactions.ListIssueCommentReactions(ctx, "o", "r", 1, nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestReactionsService_CreateIssueCommentReaction(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/comments/1/reactions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)

		w.WriteHeader(http.StatusCreated)
		assertWrite(t, w, []byte(`{"id":1,"user":{"login":"l","id":2},"content":"+1"}`))
	})

	ctx := t.Context()
	got, _, err := client.Reactions.CreateIssueCommentReaction(ctx, "o", "r", 1, "+1")
	if err != nil {
		t.Errorf("CreateIssueCommentReaction returned error: %v", err)
	}
	want := &Reaction{ID: Ptr(int64(1)), User: &User{Login: Ptr("l"), ID: Ptr(int64(2))}, Content: Ptr("+1")}
	if !cmp.Equal(got, want) {
		t.Errorf("CreateIssueCommentReaction = %+v, want %+v", got, want)
	}

	const methodName = "CreateIssueCommentReaction"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Reactions.CreateIssueCommentReaction(ctx, "\n", "\n", -1, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Reactions.CreateIssueCommentReaction(ctx, "o", "r", 1, "+1")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestReactionsService_ListPullRequestCommentReactions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/pulls/comments/1/reactions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)
		testFormValues(t, r, values{"content": "+1"})

		w.WriteHeader(http.StatusOK)
		assertWrite(t, w, []byte(`[{"id":1,"user":{"login":"l","id":2},"content":"+1"}]`))
	})

	opt := &ListReactionOptions{Content: "+1"}
	ctx := t.Context()
	got, _, err := client.Reactions.ListPullRequestCommentReactions(ctx, "o", "r", 1, opt)
	if err != nil {
		t.Errorf("ListPullRequestCommentReactions returned error: %v", err)
	}
	want := []*Reaction{{ID: Ptr(int64(1)), User: &User{Login: Ptr("l"), ID: Ptr(int64(2))}, Content: Ptr("+1")}}
	if !cmp.Equal(got, want) {
		t.Errorf("ListPullRequestCommentReactions = %+v, want %+v", got, want)
	}
}

func TestReactionsService_ListPullRequestCommentReactions_coverage(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()

	const methodName = "ListPullRequestCommentReactions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Reactions.ListPullRequestCommentReactions(ctx, "\n", "\n", -1, &ListReactionOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Reactions.ListPullRequestCommentReactions(ctx, "o", "r", 1, nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestReactionsService_CreatePullRequestCommentReaction(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/pulls/comments/1/reactions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)

		w.WriteHeader(http.StatusCreated)
		assertWrite(t, w, []byte(`{"id":1,"user":{"login":"l","id":2},"content":"+1"}`))
	})

	ctx := t.Context()
	got, _, err := client.Reactions.CreatePullRequestCommentReaction(ctx, "o", "r", 1, "+1")
	if err != nil {
		t.Errorf("CreatePullRequestCommentReaction returned error: %v", err)
	}
	want := &Reaction{ID: Ptr(int64(1)), User: &User{Login: Ptr("l"), ID: Ptr(int64(2))}, Content: Ptr("+1")}
	if !cmp.Equal(got, want) {
		t.Errorf("CreatePullRequestCommentReaction = %+v, want %+v", got, want)
	}

	const methodName = "CreatePullRequestCommentReaction"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Reactions.CreatePullRequestCommentReaction(ctx, "\n", "\n", -1, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Reactions.CreatePullRequestCommentReaction(ctx, "o", "r", 1, "+1")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestReactionsService_ListTeamDiscussionReactions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/teams/1/discussions/2/reactions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)
		testFormValues(t, r, values{"content": "+1"})

		w.WriteHeader(http.StatusOK)
		assertWrite(t, w, []byte(`[{"id":1,"user":{"login":"l","id":2},"content":"+1"}]`))
	})

	opt := &ListReactionOptions{Content: "+1"}
	ctx := t.Context()
	got, _, err := client.Reactions.ListTeamDiscussionReactions(ctx, 1, 2, opt)
	if err != nil {
		t.Errorf("ListTeamDiscussionReactions returned error: %v", err)
	}
	want := []*Reaction{{ID: Ptr(int64(1)), User: &User{Login: Ptr("l"), ID: Ptr(int64(2))}, Content: Ptr("+1")}}
	if !cmp.Equal(got, want) {
		t.Errorf("ListTeamDiscussionReactions = %+v, want %+v", got, want)
	}
}

func TestReactionsService_ListTeamDiscussionReactions_coverage(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()

	const methodName = "ListTeamDiscussionReactions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Reactions.ListTeamDiscussionReactions(ctx, -1, -2, &ListReactionOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Reactions.ListTeamDiscussionReactions(ctx, 1, 2, nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestReactionsService_CreateTeamDiscussionReaction(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/teams/1/discussions/2/reactions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)

		w.WriteHeader(http.StatusCreated)
		assertWrite(t, w, []byte(`{"id":1,"user":{"login":"l","id":2},"content":"+1"}`))
	})

	ctx := t.Context()
	got, _, err := client.Reactions.CreateTeamDiscussionReaction(ctx, 1, 2, "+1")
	if err != nil {
		t.Errorf("CreateTeamDiscussionReaction returned error: %v", err)
	}
	want := &Reaction{ID: Ptr(int64(1)), User: &User{Login: Ptr("l"), ID: Ptr(int64(2))}, Content: Ptr("+1")}
	if !cmp.Equal(got, want) {
		t.Errorf("CreateTeamDiscussionReaction = %+v, want %+v", got, want)
	}

	const methodName = "CreateTeamDiscussionReaction"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Reactions.CreateTeamDiscussionReaction(ctx, -1, -2, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Reactions.CreateTeamDiscussionReaction(ctx, 1, 2, "+1")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestReactionService_ListTeamDiscussionCommentReactions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/teams/1/discussions/2/comments/3/reactions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)
		testFormValues(t, r, values{"content": "+1"})

		w.WriteHeader(http.StatusOK)
		assertWrite(t, w, []byte(`[{"id":1,"user":{"login":"l","id":2},"content":"+1"}]`))
	})

	opt := &ListReactionOptions{Content: "+1"}
	ctx := t.Context()
	got, _, err := client.Reactions.ListTeamDiscussionCommentReactions(ctx, 1, 2, 3, opt)
	if err != nil {
		t.Errorf("ListTeamDiscussionCommentReactions returned error: %v", err)
	}
	want := []*Reaction{{ID: Ptr(int64(1)), User: &User{Login: Ptr("l"), ID: Ptr(int64(2))}, Content: Ptr("+1")}}
	if !cmp.Equal(got, want) {
		t.Errorf("ListTeamDiscussionCommentReactions = %+v, want %+v", got, want)
	}
}

func TestReactionService_ListTeamDiscussionCommentReactions_coverage(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()

	const methodName = "ListTeamDiscussionCommentReactions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Reactions.ListTeamDiscussionCommentReactions(ctx, -1, -2, -3, &ListReactionOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Reactions.ListTeamDiscussionCommentReactions(ctx, 1, 2, 3, nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestReactionService_CreateTeamDiscussionCommentReaction(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/teams/1/discussions/2/comments/3/reactions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)

		w.WriteHeader(http.StatusCreated)
		assertWrite(t, w, []byte(`{"id":1,"user":{"login":"l","id":2},"content":"+1"}`))
	})

	ctx := t.Context()
	got, _, err := client.Reactions.CreateTeamDiscussionCommentReaction(ctx, 1, 2, 3, "+1")
	if err != nil {
		t.Errorf("CreateTeamDiscussionCommentReaction returned error: %v", err)
	}
	want := &Reaction{ID: Ptr(int64(1)), User: &User{Login: Ptr("l"), ID: Ptr(int64(2))}, Content: Ptr("+1")}
	if !cmp.Equal(got, want) {
		t.Errorf("CreateTeamDiscussionCommentReaction = %+v, want %+v", got, want)
	}

	const methodName = "CreateTeamDiscussionCommentReaction"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Reactions.CreateTeamDiscussionCommentReaction(ctx, -1, -2, -3, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Reactions.CreateTeamDiscussionCommentReaction(ctx, 1, 2, 3, "+1")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestReactionsService_DeleteCommentReaction(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/comments/1/reactions/2", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	if _, err := client.Reactions.DeleteCommentReaction(ctx, "o", "r", 1, 2); err != nil {
		t.Errorf("DeleteCommentReaction returned error: %v", err)
	}

	const methodName = "DeleteCommentReaction"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Reactions.DeleteCommentReaction(ctx, "\n", "\n", -1, -2)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Reactions.DeleteCommentReaction(ctx, "o", "r", 1, 2)
	})
}

func TestReactionsService_DeleteCommentReactionByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repositories/1/comments/2/reactions/3", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	if _, err := client.Reactions.DeleteCommentReactionByID(ctx, 1, 2, 3); err != nil {
		t.Errorf("Reactions.DeleteCommentReactionByID returned error: %v", err)
	}

	const methodName = "DeleteCommentReactionByID"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Reactions.DeleteCommentReactionByID(ctx, -1, -2, -3)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Reactions.DeleteCommentReactionByID(ctx, 1, 2, 3)
	})
}

func TestReactionsService_DeleteIssueReaction(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/1/reactions/2", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	if _, err := client.Reactions.DeleteIssueReaction(ctx, "o", "r", 1, 2); err != nil {
		t.Errorf("DeleteIssueReaction returned error: %v", err)
	}

	const methodName = "DeleteIssueReaction"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Reactions.DeleteIssueReaction(ctx, "\n", "\n", -1, -2)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Reactions.DeleteIssueReaction(ctx, "o", "r", 1, 2)
	})
}

func TestReactionsService_DeleteIssueReactionByRepoID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repositories/1/issues/2/reactions/3", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	if _, err := client.Reactions.DeleteIssueReactionByID(ctx, 1, 2, 3); err != nil {
		t.Errorf("Reactions.DeleteIssueReactionByID returned error: %v", err)
	}

	const methodName = "DeleteIssueReactionByID"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Reactions.DeleteIssueReactionByID(ctx, -1, -2, -3)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Reactions.DeleteIssueReactionByID(ctx, 1, 2, 3)
	})
}

func TestReactionsService_DeleteIssueCommentReaction(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/comments/1/reactions/2", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	if _, err := client.Reactions.DeleteIssueCommentReaction(ctx, "o", "r", 1, 2); err != nil {
		t.Errorf("DeleteIssueCommentReaction returned error: %v", err)
	}

	const methodName = "DeleteIssueCommentReaction"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Reactions.DeleteIssueCommentReaction(ctx, "\n", "\n", -1, -2)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Reactions.DeleteIssueCommentReaction(ctx, "o", "r", 1, 2)
	})
}

func TestReactionsService_DeleteIssueCommentReactionByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repositories/1/issues/comments/2/reactions/3", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	if _, err := client.Reactions.DeleteIssueCommentReactionByID(ctx, 1, 2, 3); err != nil {
		t.Errorf("Reactions.DeleteIssueCommentReactionByID returned error: %v", err)
	}

	const methodName = "DeleteIssueCommentReactionByID"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Reactions.DeleteIssueCommentReactionByID(ctx, -1, -2, -3)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Reactions.DeleteIssueCommentReactionByID(ctx, 1, 2, 3)
	})
}

func TestReactionsService_DeletePullRequestCommentReaction(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/pulls/comments/1/reactions/2", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	if _, err := client.Reactions.DeletePullRequestCommentReaction(ctx, "o", "r", 1, 2); err != nil {
		t.Errorf("DeletePullRequestCommentReaction returned error: %v", err)
	}

	const methodName = "DeletePullRequestCommentReaction"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Reactions.DeletePullRequestCommentReaction(ctx, "\n", "\n", -1, -2)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Reactions.DeletePullRequestCommentReaction(ctx, "o", "r", 1, 2)
	})
}

func TestReactionsService_DeletePullRequestCommentReactionByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repositories/1/pulls/comments/2/reactions/3", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	if _, err := client.Reactions.DeletePullRequestCommentReactionByID(ctx, 1, 2, 3); err != nil {
		t.Errorf("Reactions.DeletePullRequestCommentReactionByID returned error: %v", err)
	}

	const methodName = "DeletePullRequestCommentReactionByID"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Reactions.DeletePullRequestCommentReactionByID(ctx, -1, -2, -3)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Reactions.DeletePullRequestCommentReactionByID(ctx, 1, 2, 3)
	})
}

func TestReactionsService_DeleteTeamDiscussionReaction(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/teams/s/discussions/1/reactions/2", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	if _, err := client.Reactions.DeleteTeamDiscussionReaction(ctx, "o", "s", 1, 2); err != nil {
		t.Errorf("Reactions.DeleteTeamDiscussionReaction returned error: %v", err)
	}

	const methodName = "DeleteTeamDiscussionReaction"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Reactions.DeleteTeamDiscussionReaction(ctx, "\n", "\n", -1, -2)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Reactions.DeleteTeamDiscussionReaction(ctx, "o", "s", 1, 2)
	})
}

func TestReactionsService_DeleteTeamDiscussionReactionByTeamIDAndOrgID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/organizations/1/team/2/discussions/3/reactions/4", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	if _, err := client.Reactions.DeleteTeamDiscussionReactionByOrgIDAndTeamID(ctx, 1, 2, 3, 4); err != nil {
		t.Errorf("Reactions.DeleteTeamDiscussionReactionByOrgIDAndTeamID returned error: %v", err)
	}

	const methodName = "DeleteTeamDiscussionReactionByOrgIDAndTeamID"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Reactions.DeleteTeamDiscussionReactionByOrgIDAndTeamID(ctx, -1, -2, -3, -4)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Reactions.DeleteTeamDiscussionReactionByOrgIDAndTeamID(ctx, 1, 2, 3, 4)
	})
}

func TestReactionsService_DeleteTeamDiscussionCommentReaction(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/teams/s/discussions/1/comments/2/reactions/3", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	if _, err := client.Reactions.DeleteTeamDiscussionCommentReaction(ctx, "o", "s", 1, 2, 3); err != nil {
		t.Errorf("Reactions.DeleteTeamDiscussionCommentReaction returned error: %v", err)
	}

	const methodName = "DeleteTeamDiscussionCommentReaction"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Reactions.DeleteTeamDiscussionCommentReaction(ctx, "\n", "\n", -1, -2, -3)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Reactions.DeleteTeamDiscussionCommentReaction(ctx, "o", "s", 1, 2, 3)
	})
}

func TestReactionsService_DeleteTeamDiscussionCommentReactionByOrgIDAndTeamID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/organizations/1/team/2/discussions/3/comments/4/reactions/5", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	if _, err := client.Reactions.DeleteTeamDiscussionCommentReactionByOrgIDAndTeamID(ctx, 1, 2, 3, 4, 5); err != nil {
		t.Errorf("Reactions.DeleteTeamDiscussionCommentReactionByOrgIDAndTeamID returned error: %v", err)
	}

	const methodName = "DeleteTeamDiscussionCommentReactionByOrgIDAndTeamID"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Reactions.DeleteTeamDiscussionCommentReactionByOrgIDAndTeamID(ctx, -1, -2, -3, -4, -5)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Reactions.DeleteTeamDiscussionCommentReactionByOrgIDAndTeamID(ctx, 1, 2, 3, 4, 5)
	})
}

func TestReactionService_CreateReleaseReaction(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/releases/1/reactions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)

		w.WriteHeader(http.StatusCreated)
		assertWrite(t, w, []byte(`{"id":1,"user":{"login":"l","id":2},"content":"rocket"}`))
	})

	const methodName = "CreateReleaseReaction"
	ctx := t.Context()
	got, _, err := client.Reactions.CreateReleaseReaction(ctx, "o", "r", 1, "rocket")
	if err != nil {
		t.Errorf("%v returned error: %v", methodName, err)
	}

	want := &Reaction{ID: Ptr(int64(1)), User: &User{Login: Ptr("l"), ID: Ptr(int64(2))}, Content: Ptr("rocket")}
	if !cmp.Equal(got, want) {
		t.Errorf("%v = %+v, want %+v", methodName, got, want)
	}

	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Reactions.CreateReleaseReaction(ctx, "\n", "\n", -1, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Reactions.CreateReleaseReaction(ctx, "o", "r", 1, "rocket")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestReactionsService_ListReleaseReactions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/releases/1/reactions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)
		testFormValues(t, r, values{"content": "+1"})

		w.WriteHeader(http.StatusOK)
		assertWrite(t, w, []byte(`[{"id":1,"user":{"login":"l","id":2},"content":"+1"}]`))
	})

	opt := &ListReactionOptions{Content: "+1"}
	ctx := t.Context()
	got, _, err := client.Reactions.ListReleaseReactions(ctx, "o", "r", 1, opt)
	if err != nil {
		t.Errorf("ListReleaseReactions returned error: %v", err)
	}
	want := []*Reaction{{ID: Ptr(int64(1)), User: &User{Login: Ptr("l"), ID: Ptr(int64(2))}, Content: Ptr("+1")}}
	if !cmp.Equal(got, want) {
		t.Errorf("ListReleaseReactions = %+v, want %+v", got, want)
	}
}

func TestReactionsService_ListReleaseReactions_coverage(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()

	const methodName = "ListReleaseReactions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Reactions.ListReleaseReactions(ctx, "\n", "\n", -1, &ListReactionOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Reactions.ListReleaseReactions(ctx, "o", "r", 1, nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestReactionsService_DeleteReleaseReaction(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/releases/1/reactions/2", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	if _, err := client.Reactions.DeleteReleaseReaction(ctx, "o", "r", 1, 2); err != nil {
		t.Errorf("DeleteReleaseReaction returned error: %v", err)
	}

	const methodName = "DeleteReleaseReaction"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Reactions.DeleteReleaseReaction(ctx, "\n", "\n", -1, -2)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Reactions.DeleteReleaseReaction(ctx, "o", "r", 1, 2)
	})
}

func TestReactionsService_DeleteReleaseReactionByRepoID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repositories/1/releases/2/reactions/3", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	if _, err := client.Reactions.DeleteReleaseReactionByID(ctx, 1, 2, 3); err != nil {
		t.Errorf("Reactions.DeleteReleaseReactionByID returned error: %v", err)
	}

	const methodName = "DeleteReleaseReactionByID"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Reactions.DeleteIssueReactionByID(ctx, -1, -2, -3)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Reactions.DeleteIssueReactionByID(ctx, 1, 2, 3)
	})
}
