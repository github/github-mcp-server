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

func TestIssuesService_ListLabels(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/labels", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"name": "a"},{"name": "b"}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	labels, _, err := client.Issues.ListLabels(ctx, "o", "r", opt)
	if err != nil {
		t.Errorf("Issues.ListLabels returned error: %v", err)
	}

	want := []*Label{{Name: "a"}, {Name: "b"}}
	if !cmp.Equal(labels, want) {
		t.Errorf("Issues.ListLabels returned %+v, want %+v", labels, want)
	}

	const methodName = "ListLabels"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.ListLabels(ctx, "\n", "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.ListLabels(ctx, "o", "r", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_ListLabels_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.ListLabels(ctx, "%", "%", nil)
	testURLParseError(t, err)
}

func TestIssuesService_GetLabel(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/labels/n", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"url":"u", "name": "n", "color": "c", "description": "d"}`)
	})

	ctx := t.Context()
	label, _, err := client.Issues.GetLabel(ctx, "o", "r", "n")
	if err != nil {
		t.Errorf("Issues.GetLabel returned error: %v", err)
	}

	want := &Label{URL: "u", Name: "n", Color: "c", Description: Ptr("d")}
	if !cmp.Equal(label, want) {
		t.Errorf("Issues.GetLabel returned %+v, want %+v", label, want)
	}

	const methodName = "GetLabel"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.GetLabel(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.GetLabel(ctx, "o", "r", "n")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_GetLabel_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.GetLabel(ctx, "%", "%", "%")
	testURLParseError(t, err)
}

func TestIssuesService_CreateLabel(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := CreateIssueLabelRequest{Name: "n"}

	mux.HandleFunc("/repos/o/r/labels", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"url":"u"}`)
	})

	ctx := t.Context()
	label, _, err := client.Issues.CreateLabel(ctx, "o", "r", input)
	if err != nil {
		t.Errorf("Issues.CreateLabel returned error: %v", err)
	}

	want := &Label{URL: "u"}
	if !cmp.Equal(label, want) {
		t.Errorf("Issues.CreateLabel returned %+v, want %+v", label, want)
	}

	const methodName = "CreateLabel"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.CreateLabel(ctx, "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.CreateLabel(ctx, "o", "r", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_CreateLabel_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.CreateLabel(ctx, "%", "%", CreateIssueLabelRequest{})
	testURLParseError(t, err)
}

func TestIssuesService_UpdateLabel(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := UpdateIssueLabelRequest{NewName: Ptr("z")}

	mux.HandleFunc("/repos/o/r/labels/n", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testPlainBody(t, r, `{"new_name":"z"}`+"\n")
		fmt.Fprint(w, `{"url":"u"}`)
	})

	ctx := t.Context()
	label, _, err := client.Issues.UpdateLabel(ctx, "o", "r", "n", input)
	if err != nil {
		t.Errorf("Issues.UpdateLabel returned error: %v", err)
	}

	want := &Label{URL: "u"}
	if !cmp.Equal(label, want) {
		t.Errorf("Issues.UpdateLabel returned %+v, want %+v", label, want)
	}

	const methodName = "UpdateLabel"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.UpdateLabel(ctx, "\n", "\n", "\n", UpdateIssueLabelRequest{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.UpdateLabel(ctx, "o", "r", "n", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_UpdateLabel_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.UpdateLabel(ctx, "%", "%", "%", UpdateIssueLabelRequest{})
	testURLParseError(t, err)
}

func TestIssuesService_DeleteLabel(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/labels/n", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Issues.DeleteLabel(ctx, "o", "r", "n")
	if err != nil {
		t.Errorf("Issues.DeleteLabel returned error: %v", err)
	}

	const methodName = "DeleteLabel"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Issues.DeleteLabel(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Issues.DeleteLabel(ctx, "o", "r", "n")
	})
}

func TestIssuesService_DeleteLabel_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Issues.DeleteLabel(ctx, "%", "%", "%")
	testURLParseError(t, err)
}

func TestIssuesService_ListLabelsByIssue(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/1/labels", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"name":"a","id":1},{"name":"b","id":2}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	labels, _, err := client.Issues.ListLabelsByIssue(ctx, "o", "r", 1, opt)
	if err != nil {
		t.Errorf("Issues.ListLabelsByIssue returned error: %v", err)
	}

	want := []*Label{
		{Name: "a", ID: 1},
		{Name: "b", ID: 2},
	}
	if !cmp.Equal(labels, want) {
		t.Errorf("Issues.ListLabelsByIssue returned %+v, want %+v", labels, want)
	}

	const methodName = "ListLabelsByIssue"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.ListLabelsByIssue(ctx, "\n", "\n", -1, opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.ListLabelsByIssue(ctx, "o", "r", 1, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_ListLabelsByIssue_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.ListLabelsByIssue(ctx, "%", "%", 1, nil)
	testURLParseError(t, err)
}

func TestIssuesService_AddLabelsToIssue(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := []string{"a", "b"}

	mux.HandleFunc("/repos/o/r/issues/1/labels", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `[{"url":"u"}]`)
	})

	ctx := t.Context()
	labels, _, err := client.Issues.AddLabelsToIssue(ctx, "o", "r", 1, input)
	if err != nil {
		t.Errorf("Issues.AddLabelsToIssue returned error: %v", err)
	}

	want := []*Label{{URL: "u"}}
	if !cmp.Equal(labels, want) {
		t.Errorf("Issues.AddLabelsToIssue returned %+v, want %+v", labels, want)
	}

	const methodName = "AddLabelsToIssue"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.AddLabelsToIssue(ctx, "\n", "\n", -1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.AddLabelsToIssue(ctx, "o", "r", 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_AddLabelsToIssue_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.AddLabelsToIssue(ctx, "%", "%", 1, nil)
	testURLParseError(t, err)
}

func TestIssuesService_RemoveLabelForIssue(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/1/labels/l", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Issues.RemoveLabelForIssue(ctx, "o", "r", 1, "l")
	if err != nil {
		t.Errorf("Issues.RemoveLabelForIssue returned error: %v", err)
	}

	const methodName = "RemoveLabelForIssue"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Issues.RemoveLabelForIssue(ctx, "\n", "\n", -1, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Issues.RemoveLabelForIssue(ctx, "o", "r", 1, "l")
	})
}

func TestIssuesService_RemoveLabelForIssue_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Issues.RemoveLabelForIssue(ctx, "%", "%", 1, "%")
	testURLParseError(t, err)
}

func TestIssuesService_ReplaceLabelsForIssue(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := []string{"a", "b"}

	mux.HandleFunc("/repos/o/r/issues/1/labels", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `[{"url":"u"}]`)
	})

	ctx := t.Context()
	labels, _, err := client.Issues.ReplaceLabelsForIssue(ctx, "o", "r", 1, input)
	if err != nil {
		t.Errorf("Issues.ReplaceLabelsForIssue returned error: %v", err)
	}

	want := []*Label{{URL: "u"}}
	if !cmp.Equal(labels, want) {
		t.Errorf("Issues.ReplaceLabelsForIssue returned %+v, want %+v", labels, want)
	}

	const methodName = "ReplaceLabelsForIssue"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.ReplaceLabelsForIssue(ctx, "\n", "\n", -1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.ReplaceLabelsForIssue(ctx, "o", "r", 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_ReplaceLabelsForIssue_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.ReplaceLabelsForIssue(ctx, "%", "%", 1, nil)
	testURLParseError(t, err)
}

func TestIssuesService_RemoveLabelsForIssue(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/1/labels", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Issues.RemoveLabelsForIssue(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("Issues.RemoveLabelsForIssue returned error: %v", err)
	}

	const methodName = "RemoveLabelsForIssue"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Issues.RemoveLabelsForIssue(ctx, "\n", "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Issues.RemoveLabelsForIssue(ctx, "o", "r", 1)
	})
}

func TestIssuesService_RemoveLabelsForIssue_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Issues.RemoveLabelsForIssue(ctx, "%", "%", 1)
	testURLParseError(t, err)
}

func TestIssuesService_ListLabelsForMilestone(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/milestones/1/labels", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"name": "a"},{"name": "b"}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	labels, _, err := client.Issues.ListLabelsForMilestone(ctx, "o", "r", 1, opt)
	if err != nil {
		t.Errorf("Issues.ListLabelsForMilestone returned error: %v", err)
	}

	want := []*Label{{Name: "a"}, {Name: "b"}}
	if !cmp.Equal(labels, want) {
		t.Errorf("Issues.ListLabelsForMilestone returned %+v, want %+v", labels, want)
	}

	const methodName = "ListLabelsForMilestone"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.ListLabelsForMilestone(ctx, "\n", "\n", -1, opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.ListLabelsForMilestone(ctx, "o", "r", 1, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_ListLabelsForMilestone_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.ListLabelsForMilestone(ctx, "%", "%", 1, nil)
	testURLParseError(t, err)
}
