// Copyright 2025 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestProjectsService_ListOrganizationProjects(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	// Combined handler: supports initial test case and dual before/after validation scenario.
	mux.HandleFunc("/orgs/o/projectsV2", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		q := r.URL.Query()
		if q.Get("before") == "b" && q.Get("after") == "a" {
			fmt.Fprint(w, `[]`)
			return
		}
		// default expectation for main part of test
		testFormValues(t, r, values{"q": "alpha", "after": "2", "before": "1"})
		fmt.Fprint(w, `[{"id":1,"title":"T1","created_at":"2011-01-02T15:04:05Z","updated_at":"2012-01-02T15:04:05Z"}]`)
	})

	opts := &ListProjectsOptions{Query: "alpha", ListProjectsPaginationOptions: ListProjectsPaginationOptions{After: "2", Before: "1"}}
	ctx := t.Context()
	projects, _, err := client.Projects.ListOrganizationProjects(ctx, "o", opts)
	if err != nil {
		t.Fatalf("Projects.ListOrganizationProjects returned error: %v", err)
	}
	if len(projects) != 1 || projects[0].GetID() != 1 || projects[0].GetTitle() != "T1" {
		t.Fatalf("Projects.ListOrganizationProjects returned %+v", projects)
	}

	const methodName = "ListOrganizationProjects"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Projects.ListOrganizationProjects(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.ListOrganizationProjects(ctx, "o", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})

	// still allow both set (no validation enforced) – ensure it does not error
	ctxBypass := context.WithValue(t.Context(), BypassRateLimitCheck, true)
	if _, _, err = client.Projects.ListOrganizationProjects(ctxBypass, "o", &ListProjectsOptions{ListProjectsPaginationOptions: ListProjectsPaginationOptions{Before: "b", After: "a"}}); err != nil {
		t.Fatalf("unexpected error when both before/after set: %v", err)
	}
}

func TestProjectsService_GetOrganizationProject(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/projectsV2/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":1,"title":"OrgProj","created_at":"2011-01-02T15:04:05Z","updated_at":"2012-01-02T15:04:05Z"}`)
	})

	ctx := t.Context()
	project, _, err := client.Projects.GetOrganizationProject(ctx, "o", 1)
	if err != nil {
		t.Fatalf("Projects.GetOrganizationProject returned error: %v", err)
	}
	if project.GetID() != 1 || project.GetTitle() != "OrgProj" {
		t.Fatalf("Projects.GetOrganizationProject returned %+v", project)
	}

	const methodName = "GetOrganizationProject"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.GetOrganizationProject(ctx, "o", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestProjectsService_ListUserProjects(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	// Combined handler: supports initial test case and dual before/after scenario.
	mux.HandleFunc("/users/u/projectsV2", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		q := r.URL.Query()
		if q.Get("before") == "b" && q.Get("after") == "a" {
			fmt.Fprint(w, `[]`)
			return
		}
		testFormValues(t, r, values{"q": "beta", "before": "1", "after": "2", "per_page": "2"})
		fmt.Fprint(w, `[{"id":2,"title":"UProj","created_at":"2011-01-02T15:04:05Z","updated_at":"2012-01-02T15:04:05Z"}]`)
	})

	opts := &ListProjectsOptions{Query: "beta", ListProjectsPaginationOptions: ListProjectsPaginationOptions{Before: "1", After: "2", PerPage: 2}}
	ctx := t.Context()
	var ctxBypass context.Context
	projects, _, err := client.Projects.ListUserProjects(ctx, "u", opts)
	if err != nil {
		t.Fatalf("Projects.ListUserProjects returned error: %v", err)
	}
	if len(projects) != 1 || projects[0].GetID() != 2 || projects[0].GetTitle() != "UProj" {
		t.Fatalf("Projects.ListUserProjects returned %+v", projects)
	}

	const methodName = "ListUserProjects"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Projects.ListUserProjects(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.ListUserProjects(ctx, "u", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})

	// still allow both set (no validation enforced) – ensure it does not error
	ctxBypass = context.WithValue(t.Context(), BypassRateLimitCheck, true)
	if _, _, err = client.Projects.ListUserProjects(ctxBypass, "u", &ListProjectsOptions{ListProjectsPaginationOptions: ListProjectsPaginationOptions{Before: "b", After: "a"}}); err != nil {
		t.Fatalf("unexpected error when both before/after set: %v", err)
	}
}

func TestProjectsService_GetUserProject(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/projectsV2/3", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":3,"title":"UserProj","created_at":"2011-01-02T15:04:05Z","updated_at":"2012-01-02T15:04:05Z"}`)
	})

	ctx := t.Context()
	project, _, err := client.Projects.GetUserProject(ctx, "u", 3)
	if err != nil {
		t.Fatalf("Projects.GetUserProject returned error: %v", err)
	}
	if project.GetID() != 3 || project.GetTitle() != "UserProj" {
		t.Fatalf("Projects.GetUserProject returned %+v", project)
	}

	const methodName = "GetUserProject"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.GetUserProject(ctx, "u", 3)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestProjectsService_ListOrganizationProjectFields(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/projectsV2/1/fields", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		q := r.URL.Query()
		if q.Get("before") == "b" && q.Get("after") == "a" { // bypass scenario
			fmt.Fprint(w, `[]`)
			return
		}
		testFormValues(t, r, values{"after": "2", "before": "1", "q": "text"})
		fmt.Fprint(w, `[
		{
			"id": 1,
			"node_id": "node_1",
			"name": "Status",
			"data_type": "single_select",
			"url": "https://api.github.com/projects/1/fields/field1",
			"options": [
				{"id": "1", "name": {"raw": "Todo", "html": "Todo"}, "color": "blue", "description": {"raw": "Tasks to be done", "html": "Tasks to be done"}},
				{"id": "2", "name": {"raw": "In Progress", "html": "In Progress"}, "color": "yellow"}
			],
			"created_at": "2011-01-02T15:04:05Z",
			"updated_at": "2012-01-02T15:04:05Z"
		},
		{
			"id": 2,
			"node_id": "node_2",
			"name": "Priority",
			"data_type": "text",
			"url": "https://api.github.com/projects/1/fields/field2",
			"created_at": "2011-01-02T15:04:05Z",
			"updated_at": "2012-01-02T15:04:05Z"
		}
		]`)
	})

	opts := &ListProjectsOptions{Query: "text", ListProjectsPaginationOptions: ListProjectsPaginationOptions{After: "2", Before: "1"}}
	ctx := t.Context()
	fields, _, err := client.Projects.ListOrganizationProjectFields(ctx, "o", 1, opts)
	if err != nil {
		t.Fatalf("Projects.ListOrganizationProjectFields returned error: %v", err)
	}
	if len(fields) != 2 {
		t.Fatalf("Projects.ListOrganizationProjectFields returned %d fields, want 2", len(fields))
	}
	if fields[0].ID == nil || *fields[0].ID != 1 || fields[1].ID == nil || *fields[1].ID != 2 {
		t.Fatalf("unexpected field IDs: %+v", fields)
	}

	const methodName = "ListOrganizationProjectFields"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Projects.ListOrganizationProjectFields(ctx, "\n", 1, opts)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.ListOrganizationProjectFields(ctx, "o", 1, opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
	ctxBypass := context.WithValue(ctx, BypassRateLimitCheck, true)
	if _, _, err = client.Projects.ListOrganizationProjectFields(ctxBypass, "o", 1, &ListProjectsOptions{ListProjectsPaginationOptions: ListProjectsPaginationOptions{Before: ("b"), After: ("a")}}); err != nil {
		t.Fatalf("unexpected error when both before/after set: %v", err)
	}
}

func TestProjectsService_ListUserProjectFields(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/projectsV2/1/fields", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		q := r.URL.Query()
		if q.Get("before") == "b" && q.Get("after") == "a" { // bypass scenario
			fmt.Fprint(w, `[]`)
			return
		}
		testFormValues(t, r, values{"after": "2", "before": "1", "q": "text"})
		fmt.Fprint(w, `[
		{
			"id": 1,
			"node_id": "node_1",
			"name": "Status",
			"data_type": "single_select",
			"url": "https://api.github.com/projects/1/fields/field1",
			"options": [
				{"id": "1", "name": {"raw": "Todo", "html": "Todo"}, "color": "blue", "description": {"raw": "Tasks to be done", "html": "Tasks to be done"}},
				{"id": "2", "name": {"raw": "In Progress", "html": "In Progress"}, "color": "yellow"}
			],
			"created_at": "2011-01-02T15:04:05Z",
			"updated_at": "2012-01-02T15:04:05Z"
		},
		{
			"id": 2,
			"node_id": "node_2",
			"name": "Priority",
			"data_type": "text",
			"url": "https://api.github.com/projects/1/fields/field2",
			"created_at": "2011-01-02T15:04:05Z",
			"updated_at": "2012-01-02T15:04:05Z"
		}
		]`)
	})

	opts := &ListProjectsOptions{Query: ("text"), ListProjectsPaginationOptions: ListProjectsPaginationOptions{After: ("2"), Before: ("1")}}
	ctx := t.Context()
	fields, _, err := client.Projects.ListUserProjectFields(ctx, "u", 1, opts)
	if err != nil {
		t.Fatalf("Projects.ListUserProjectFields returned error: %v", err)
	}
	if len(fields) != 2 {
		t.Fatalf("Projects.ListUserProjectFields returned %d fields, want 2", len(fields))
	}
	if fields[0].ID == nil || *fields[0].ID != 1 || fields[1].ID == nil || *fields[1].ID != 2 {
		t.Fatalf("unexpected field IDs: %+v", fields)
	}

	const methodName = "ListUserProjectFields"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Projects.ListUserProjectFields(ctx, "\n", 1, opts)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.ListUserProjectFields(ctx, "u", 1, opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
	ctxBypass := context.WithValue(ctx, BypassRateLimitCheck, true)
	if _, _, err = client.Projects.ListUserProjectFields(ctxBypass, "u", 1, &ListProjectsOptions{ListProjectsPaginationOptions: ListProjectsPaginationOptions{Before: ("b"), After: ("a")}}); err != nil {
		t.Fatalf("unexpected error when both before/after set: %v", err)
	}
}

func TestProjectsService_GetOrganizationProjectField(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/projectsV2/1/fields/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `
		{
			"id": 1,
			"node_id": "node_1",
			"name": "Status",
			"data_type": "single_select",
			"url": "https://api.github.com/projects/1/fields/field1",
			"options": [
				{"id": "1", "name": {"raw": "Todo", "html": "Todo"}, "color": "blue", "description": {"raw": "Tasks to be done", "html": "Tasks to be done"}},
				{"id": "2", "name": {"raw": "In Progress", "html": "In Progress"}, "color": "yellow"}
			],
			"created_at": "2011-01-02T15:04:05Z",
			"updated_at": "2012-01-02T15:04:05Z"
		}`)
	})

	ctx := t.Context()
	field, _, err := client.Projects.GetOrganizationProjectField(ctx, "o", 1, 1)
	if err != nil {
		t.Fatalf("Projects.GetOrganizationProjectField returned error: %v", err)
	}
	if field == nil || field.ID == nil || *field.ID != 1 {
		t.Fatalf("unexpected field: %+v", field)
	}

	const methodName = "GetOrganizationProjectField"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.GetOrganizationProjectField(ctx, "o", 1, 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestProjectsService_GetUserProjectField(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/projectsV2/1/fields/3", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `
		{
			"id": 3,
			"node_id": "node_3",
			"name": "Status",
			"data_type": "single_select",
			"url": "https://api.github.com/projects/1/fields/field3",
			"options": [
				{"id": "1", "name": {"raw": "Done", "html": "Done"}, "color": "red", "description": {"raw": "Done task", "html": "Done task"}},
				{"id": "2", "name": {"raw": "In Progress", "html": "In Progress"}, "color": "yellow"}
			],
			"created_at": "2011-01-02T15:04:05Z",
			"updated_at": "2012-01-02T15:04:05Z"
		}`)
	})

	ctx := t.Context()
	field, _, err := client.Projects.GetUserProjectField(ctx, "u", 1, 3)
	if err != nil {
		t.Fatalf("Projects.GetUserProjectField returned error: %v", err)
	}
	if field == nil || field.ID == nil || *field.ID != 3 {
		t.Fatalf("unexpected field: %+v", field)
	}

	const methodName = "GetUserProjectField"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.GetUserProjectField(ctx, "u", 1, 3)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestProjectsService_ListUserProjects_pagination(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	mux.HandleFunc("/users/u/projectsV2", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		after := q.Get("after")
		before := q.Get("before")
		if after == "" && before == "" {
			w.Header().Set("Link", "<http://example.org/users/u/projectsV2?after=ucursor2>; rel=\"next\"")
			fmt.Fprint(w, `[{"id":10,"title":"UP1","created_at":"2011-01-02T15:04:05Z","updated_at":"2012-01-02T15:04:05Z"}]`)
			return
		}
		if after == "ucursor2" {
			w.Header().Set("Link", "<http://example.org/users/u/projectsV2?before=ucursor2>; rel=\"prev\"")
			fmt.Fprint(w, `[{"id":11,"title":"UP2","created_at":"2011-01-02T15:04:05Z","updated_at":"2012-01-02T15:04:05Z"}]`)
			return
		}
		http.Error(w, "unexpected query", http.StatusBadRequest)
	})
	ctx := t.Context()
	first, resp, err := client.Projects.ListUserProjects(ctx, "u", nil)
	if err != nil {
		t.Fatalf("first page error: %v", err)
	}
	if len(first) != 1 || first[0].GetID() != 10 {
		t.Fatalf("unexpected first page %+v", first)
	}
	if resp.After != "ucursor2" {
		t.Fatalf("expected resp.After=ucursor2 got %q", resp.After)
	}

	opts := &ListProjectsOptions{ListProjectsPaginationOptions: ListProjectsPaginationOptions{After: (resp.After)}}
	second, resp2, err := client.Projects.ListUserProjects(ctx, "u", opts)
	if err != nil {
		t.Fatalf("second page error: %v", err)
	}
	if len(second) != 1 || second[0].GetID() != 11 {
		t.Fatalf("unexpected second page %+v", second)
	}
	if resp2.Before != "ucursor2" {
		t.Fatalf("expected resp2.Before=ucursor2 got %q", resp2.Before)
	}
}

func TestProjectsService_ListUserProjects_error(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	mux.HandleFunc("/users/u/projectsV2", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[]`)
	})
	ctx := t.Context()
	const methodName = "ListUserProjects"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.ListUserProjects(ctx, "u", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
	// bad options (bad username) should error
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Projects.ListUserProjects(ctx, "\n", nil)
		return err
	})
}

func TestProjectsService_ListOrganizationProjectFields_pagination(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	// First page returns a Link header with rel="next" containing an after cursor
	mux.HandleFunc("/orgs/o/projectsV2/1/fields", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		after := q.Get("after")
		before := q.Get("before")
		if after == "" && before == "" {
			// first request
			w.Header().Set("Link", "<http://example.org/orgs/o/projectsV2/1/fields?after=cursor2>; rel=\"next\"")
			fmt.Fprint(w, `[{"id":1,"name":"Status","data_type":"single_select","created_at":"2011-01-02T15:04:05Z","updated_at":"2012-01-02T15:04:05Z"}]`)
			return
		}
		if after == "cursor2" {
			// second request simulates a previous link
			w.Header().Set("Link", "<http://example.org/orgs/o/projectsV2/1/fields?before=cursor2>; rel=\"prev\"")
			fmt.Fprint(w, `[{"id":2,"name":"Priority","data_type":"text","created_at":"2011-01-02T15:04:05Z","updated_at":"2012-01-02T15:04:05Z"}]`)
			return
		}
		// unexpected state
		http.Error(w, "unexpected query", http.StatusBadRequest)
	})

	ctx := t.Context()
	first, resp, err := client.Projects.ListOrganizationProjectFields(ctx, "o", 1, nil)
	if err != nil {
		t.Fatalf("first page error: %v", err)
	}
	if len(first) != 1 || first[0].ID == nil || *first[0].ID != 1 {
		t.Fatalf("unexpected first page %+v", first)
	}
	if resp.After != "cursor2" {
		t.Fatalf("expected resp.After=cursor2 got %q", resp.After)
	}

	opts := &ListProjectsOptions{ListProjectsPaginationOptions: ListProjectsPaginationOptions{After: (resp.After)}}
	second, resp2, err := client.Projects.ListOrganizationProjectFields(ctx, "o", 1, opts)
	if err != nil {
		t.Fatalf("second page error: %v", err)
	}
	if len(second) != 1 || second[0].ID == nil || *second[0].ID != 2 {
		t.Fatalf("unexpected second page %+v", second)
	}
	if resp2.Before != "cursor2" {
		t.Fatalf("expected resp2.Before=cursor2 got %q", resp2.Before)
	}
}

func TestProjectsService_ListOrganizationProjects_pagination(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/projectsV2", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		after := q.Get("after")
		before := q.Get("before")
		if after == "" && before == "" {
			w.Header().Set("Link", "<http://example.org/orgs/o/projectsV2?after=ocursor2>; rel=\"next\"")
			fmt.Fprint(w, `[{"id":20,"title":"OP1","created_at":"2011-01-02T15:04:05Z","updated_at":"2012-01-02T15:04:05Z"}]`)
			return
		}
		if after == "ocursor2" {
			w.Header().Set("Link", "<http://example.org/orgs/o/projectsV2?before=ocursor2>; rel=\"prev\"")
			fmt.Fprint(w, `[{"id":21,"title":"OP2","created_at":"2011-01-02T15:04:05Z","updated_at":"2012-01-02T15:04:05Z"}]`)
			return
		}
		http.Error(w, "unexpected query", http.StatusBadRequest)
	})

	ctx := t.Context()
	first, resp, err := client.Projects.ListOrganizationProjects(ctx, "o", nil)
	if err != nil {
		t.Fatalf("first page error: %v", err)
	}
	if len(first) != 1 || first[0].GetID() != 20 {
		t.Fatalf("unexpected first page %+v", first)
	}
	if resp.After != "ocursor2" {
		t.Fatalf("expected resp.After=ocursor2 got %q", resp.After)
	}

	opts := &ListProjectsOptions{ListProjectsPaginationOptions: ListProjectsPaginationOptions{After: (resp.After)}}
	second, resp2, err := client.Projects.ListOrganizationProjects(ctx, "o", opts)
	if err != nil {
		t.Fatalf("second page error: %v", err)
	}
	if len(second) != 1 || second[0].GetID() != 21 {
		t.Fatalf("unexpected second page %+v", second)
	}
	if resp2.Before != "ocursor2" {
		t.Fatalf("expected resp2.Before=ocursor2 got %q", resp2.Before)
	}
}

func TestProjectsService_ListOrganizationProjectItems(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/projectsV2/1/items", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		q := r.URL.Query()
		if q.Get("before") == "b" && q.Get("after") == "a" { // bypass scenario
			fmt.Fprint(w, `[]`)
			return
		}
		testFormValues(t, r, values{"after": "2", "before": "1", "per_page": "50", "fields": "10,11", "q": "status:open"})
		fmt.Fprint(w, `[{"id":17,"node_id":"PVTI_node"}]`)
	})

	opts := &ListProjectItemsOptions{ListProjectsOptions: ListProjectsOptions{ListProjectsPaginationOptions: ListProjectsPaginationOptions{After: ("2"), Before: ("1"), PerPage: (50)}, Query: ("status:open")}, Fields: []int64{10, 11}}
	ctx := t.Context()
	items, _, err := client.Projects.ListOrganizationProjectItems(ctx, "o", 1, opts)
	if err != nil {
		t.Fatalf("Projects.ListOrganizationProjectItems returned error: %v", err)
	}
	if len(items) != 1 || items[0].GetID() != 17 {
		t.Fatalf("Projects.ListOrganizationProjectItems returned %+v", items)
	}

	const methodName = "ListOrganizationProjectItems"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Projects.ListOrganizationProjectItems(ctx, "\n", 1, opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.ListOrganizationProjectItems(ctx, "o", 1, opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})

	ctxBypass := context.WithValue(ctx, BypassRateLimitCheck, true)
	if _, _, err = client.Projects.ListOrganizationProjectItems(ctxBypass, "o", 1, &ListProjectItemsOptions{ListProjectsOptions: ListProjectsOptions{ListProjectsPaginationOptions: ListProjectsPaginationOptions{Before: ("b"), After: ("a")}}}); err != nil {
		t.Fatalf("unexpected error when both before/after set: %v", err)
	}
}

func TestProjectsService_AddOrganizationProjectItem(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &AddProjectItemOptions{Type: Ptr(ProjectV2ItemContentType("Issue")), ID: Ptr(int64(99))}

	mux.HandleFunc("/orgs/o/projectsV2/1/items", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":99,"node_id":"PVTI_new"}`)
	})

	ctx := t.Context()
	item, _, err := client.Projects.AddOrganizationProjectItem(ctx, "o", 1, input)
	if err != nil {
		t.Fatalf("Projects.AddOrganizationProjectItem returned error: %v", err)
	}
	if item.GetID() != 99 {
		t.Fatalf("unexpected item: %+v", item)
	}
}

func TestProjectsService_AddProjectItemForOrg_error(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	mux.HandleFunc("/orgs/o/projectsV2/1/items", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":1}`)
	})
	ctx := t.Context()
	const methodName = "AddOrganizationProjectItem"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.AddOrganizationProjectItem(ctx, "o", 1, &AddProjectItemOptions{Type: Ptr(ProjectV2ItemContentType("Issue")), ID: Ptr(int64(1))})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestProjectsService_GetOrganizationProjectItem(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	mux.HandleFunc("/orgs/o/projectsV2/1/items/17", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":17,"node_id":"PVTI_node"}`)
	})
	ctx := t.Context()
	opts := &GetProjectItemOptions{}
	item, _, err := client.Projects.GetOrganizationProjectItem(ctx, "o", 1, 17, opts)
	if err != nil {
		t.Fatalf("GetOrganizationProjectItem error: %v", err)
	}
	if item.GetID() != 17 {
		t.Fatalf("unexpected item: %+v", item)
	}
}

func TestProjectsService_GetOrganizationProjectItem_error(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	mux.HandleFunc("/orgs/o/projectsV2/1/items/17", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":17}`)
	})
	ctx := t.Context()
	const methodName = "GetOrganizationProjectItem"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.GetOrganizationProjectItem(ctx, "o", 1, 17, &GetProjectItemOptions{})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestProjectsService_GetOrganizationProjectItem_WithFieldsOption(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	mux.HandleFunc("/orgs/o/projectsV2/1/items/17", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		// Verify that fields option is properly added as comma-separated URL parameter
		testFormValues(t, r, values{"fields": "123,456,789"})
		fmt.Fprint(w, `{
			"id":17,
			"node_id":"PVTI_node_fields",
			"fields":[
				{"id":123,"name":"Status","data_type":"single_select"},
				{"id":456,"name":"Priority","data_type":"single_select"},
				{"id":789,"name":"Assignee","data_type":"text"}
			]
		}`)
	})
	ctx := t.Context()
	opts := &GetProjectItemOptions{
		Fields: []int64{123, 456, 789}, // Request specific field IDs
	}
	item, _, err := client.Projects.GetOrganizationProjectItem(ctx, "o", 1, 17, opts)
	if err != nil {
		t.Fatalf("GetOrganizationProjectItem error: %v", err)
	}
	if item.GetID() != 17 {
		t.Fatalf("unexpected item: %+v", item)
	}
	const methodName = "GetOrganizationProjectItemWithFields"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Projects.GetOrganizationProjectItem(ctx, "\n", 1, 17, opts)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.GetOrganizationProjectItem(ctx, "o", 1, 17, opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestProjectsService_UpdateOrganizationProjectItem(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	input := &UpdateProjectItemOptions{Archived: Ptr(true)}
	mux.HandleFunc("/orgs/o/projectsV2/1/items/17", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":17}`)
	})
	ctx := t.Context()
	item, _, err := client.Projects.UpdateOrganizationProjectItem(ctx, "o", 1, 17, input)
	if err != nil {
		t.Fatalf("UpdateOrganizationProjectItem error: %v", err)
	}
	if item.GetID() != 17 {
		t.Fatalf("unexpected item: %+v", item)
	}
}

func TestProjectsService_UpdateOrganizationProjectItem_error(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	input := &UpdateProjectItemOptions{Archived: Ptr(true)}
	mux.HandleFunc("/orgs/o/projectsV2/1/items/17", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":17}`)
	})
	ctx := t.Context()
	const methodName = "UpdateProjectItemForOrg"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.UpdateOrganizationProjectItem(ctx, "o", 1, 17, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestProjectsService_UpdateOrganizationProjectItem_WithFieldUpdates(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	input := &UpdateProjectItemOptions{
		Fields: []*UpdateProjectV2Field{
			{ID: 123, Value: "Updated text value"},
			{ID: 456, Value: "Done"},
		},
	}
	mux.HandleFunc("/orgs/o/projectsV2/1/items/17", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":17,"node_id":"PVTI_node_updated"}`)
	})

	ctx := t.Context()
	item, _, err := client.Projects.UpdateOrganizationProjectItem(ctx, "o", 1, 17, input)
	if err != nil {
		t.Fatalf("UpdateOrganizationProjectItem error: %v", err)
	}
	if item.GetID() != 17 {
		t.Fatalf("unexpected item: %+v", item)
	}

	const methodName = "UpdateOrganizationProjectItemWithFields"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Projects.UpdateOrganizationProjectItem(ctx, "\n", 1, 17, input)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.UpdateOrganizationProjectItem(ctx, "o", 1, 17, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestProjectsService_DeleteOrganizationProjectItem(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	mux.HandleFunc("/orgs/o/projectsV2/1/items/17", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})
	ctx := t.Context()
	if _, err := client.Projects.DeleteOrganizationProjectItem(ctx, "o", 1, 17); err != nil {
		t.Fatalf("DeleteOrganizationProjectItem error: %v", err)
	}
}

func TestProjectsService_DeleteOrganizationProjectItem_error(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	mux.HandleFunc("/orgs/o/projectsV2/1/items/17", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})
	ctx := t.Context()
	const methodName = "DeleteOrganizationProjectItem"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Projects.DeleteOrganizationProjectItem(ctx, "o", 1, 17)
	})
}

func TestProjectsService_ListUserProjectItems(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	mux.HandleFunc("/users/u/projectsV2/2/items", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"per_page": "20", "q": "type:issue"})
		fmt.Fprint(w, `[{"id":7,"node_id":"PVTI_user"}]`)
	})
	ctx := t.Context()
	items, _, err := client.Projects.ListUserProjectItems(ctx, "u", 2, &ListProjectItemsOptions{ListProjectsOptions: ListProjectsOptions{ListProjectsPaginationOptions: ListProjectsPaginationOptions{PerPage: (20)}, Query: ("type:issue")}})
	if err != nil {
		t.Fatalf("ListUserProjectItems error: %v", err)
	}
	if len(items) != 1 || items[0].GetID() != 7 {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestProjectsService_ListUserProjectItems_error(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	mux.HandleFunc("/users/u/projectsV2/2/items", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[]`)
	})
	ctx := t.Context()
	const methodName = "ListUserProjectItems"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.ListUserProjectItems(ctx, "u", 2, nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Projects.ListUserProjectItems(ctx, "\n", 2, nil)
		return err
	})
}

func TestProjectsService_AddUserProjectItem(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	input := &AddProjectItemOptions{Type: Ptr(ProjectV2ItemContentType("PullRequest")), ID: Ptr(int64(123))}
	mux.HandleFunc("/users/u/projectsV2/2/items", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":123,"node_id":"PVTI_new_user"}`)
	})
	ctx := t.Context()
	item, _, err := client.Projects.AddUserProjectItem(ctx, "u", 2, input)
	if err != nil {
		t.Fatalf("AddUserProjectItem error: %v", err)
	}
	if item.GetID() != 123 {
		t.Fatalf("unexpected item: %+v", item)
	}
}

func TestProjectsService_AddUserProjectItem_error(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	mux.HandleFunc("/users/u/projectsV2/2/items", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{"id":5}`)
	})
	ctx := t.Context()
	const methodName = "AddUserProjectItem"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.AddUserProjectItem(ctx, "u", 2, &AddProjectItemOptions{Type: Ptr(ProjectV2ItemContentType("Issue")), ID: Ptr(int64(5))})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestProjectsService_GetUserProjectItem(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	mux.HandleFunc("/users/u/projectsV2/2/items/55", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":55,"node_id":"PVTI_user_item"}`)
	})
	ctx := t.Context()
	opts := &GetProjectItemOptions{}
	item, _, err := client.Projects.GetUserProjectItem(ctx, "u", 2, 55, opts)
	if err != nil {
		t.Fatalf("GetUserProjectItem error: %v", err)
	}
	if item.GetID() != 55 {
		t.Fatalf("unexpected item: %+v", item)
	}
}

func TestProjectsService_GetUserProjectItem_error(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	mux.HandleFunc("/users/u/projectsV2/2/items/55", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":55}`)
	})
	ctx := t.Context()
	const methodName = "GetUserProjectItem"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.GetUserProjectItem(ctx, "u", 2, 55, &GetProjectItemOptions{})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestProjectsService_GetUserProjectItem_WithFieldsOption(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	mux.HandleFunc("/users/u/projectsV2/2/items/55", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		// Verify that fields option is properly added as comma-separated URL parameter
		testFormValues(t, r, values{"fields": "100,200"})
		fmt.Fprint(w, `{
			"id":55,
			"node_id":"PVTI_user_item_fields",
			"fields":[
				{"id":100,"name":"Status","data_type":"single_select"},
				{"id":200,"name":"Milestone","data_type":"text"}
			]
		}`)
	})
	ctx := t.Context()
	opts := &GetProjectItemOptions{
		Fields: []int64{100, 200}, // Request specific field IDs
	}
	item, _, err := client.Projects.GetUserProjectItem(ctx, "u", 2, 55, opts)
	if err != nil {
		t.Fatalf("GetUserProjectItem error: %v", err)
	}
	if item.GetID() != 55 {
		t.Fatalf("unexpected item: %+v", item)
	}

	const methodName = "GetUserProjectItemWithFields"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Projects.GetUserProjectItem(ctx, "\n", 2, 55, opts)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.GetUserProjectItem(ctx, "u", 2, 55, opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestProjectsService_UpdateUserProjectItem(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	input := &UpdateProjectItemOptions{Archived: Ptr(false)}
	mux.HandleFunc("/users/u/projectsV2/2/items/55", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":55}`)
	})
	ctx := t.Context()
	item, _, err := client.Projects.UpdateUserProjectItem(ctx, "u", 2, 55, input)
	if err != nil {
		t.Fatalf("UpdateUserProjectItem error: %v", err)
	}
	if item.GetID() != 55 {
		t.Fatalf("unexpected item: %+v", item)
	}
}

func TestProjectsService_UpdateUserProjectItem_error(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	mux.HandleFunc("/users/u/projectsV2/2/items/55", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		fmt.Fprint(w, `{"id":55}`)
	})
	ctx := t.Context()
	const methodName = "UpdateUserProjectItem"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.UpdateUserProjectItem(ctx, "u", 2, 55, &UpdateProjectItemOptions{Archived: Ptr(false)})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestProjectsService_UpdateUserProjectItem_WithFieldUpdates(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	opts := &UpdateProjectItemOptions{
		Fields: []*UpdateProjectV2Field{
			{ID: 100, Value: "In Progress"},
			{ID: 200, Value: float64(5)}, // number field
		},
	}
	mux.HandleFunc("/users/u/projectsV2/2/items/55", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, opts)
		fmt.Fprint(w, `{"id":55,"node_id":"PVTI_user_updated"}`)
	})

	ctx := t.Context()
	item, _, err := client.Projects.UpdateUserProjectItem(ctx, "u", 2, 55, opts)
	if err != nil {
		t.Fatalf("UpdateUserProjectItem error: %v", err)
	}
	if item.GetID() != 55 {
		t.Fatalf("unexpected item: %+v", item)
	}

	const methodName = "UpdateUserProjectItemWithFields"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Projects.UpdateUserProjectItem(ctx, "\n", 2, 55, opts)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.UpdateUserProjectItem(ctx, "u", 2, 55, opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestProjectsService_DeleteUserProjectItem(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	mux.HandleFunc("/users/u/projectsV2/2/items/55", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})
	ctx := t.Context()
	if _, err := client.Projects.DeleteUserProjectItem(ctx, "u", 2, 55); err != nil {
		t.Fatalf("DeleteUserProjectItem error: %v", err)
	}
}

func TestProjectsService_DeleteUserProjectItem_error(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	mux.HandleFunc("/users/u/projectsV2/2/items/55", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})
	ctx := t.Context()
	const methodName = "DeleteUserProjectItem"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Projects.DeleteUserProjectItem(ctx, "u", 2, 55)
	})
}

func TestProjectV2Item_UnmarshalJSON_Issue(t *testing.T) {
	t.Parallel()

	item := ProjectV2Item{
		ID:          Ptr(int64(123)),
		NodeID:      Ptr("PVTI_test"),
		ContentType: Ptr(ProjectV2ItemContentTypeIssue),
		Content: &ProjectV2ItemContent{
			Issue: &Issue{
				ID:     Ptr(int64(456)),
				Number: Ptr(10),
				Title:  Ptr("Test Issue"),
				State:  Ptr("open"),
				Body:   Ptr("Issue body"),
				Repository: &Repository{
					ID:   Ptr(int64(789)),
					Name: Ptr("test-repo"),
				},
			},
		},
		CreatedAt: &referenceTimestamp,
	}

	want := `{
		"id": 123,
		"node_id": "PVTI_test",
		"content_type": "Issue",
		"content": {
			"id": 456,
			"number": 10,
			"title": "Test Issue",
			"state": "open",
			"body": "Issue body",
			"repository": {
				"id": 789,
				"name": "test-repo"
			}
		},
		"created_at": ` + referenceTimeStr + `
	}`

	testJSONUnmarshalOnly(t, item, want)
}

func TestProjectV2Item_UnmarshalJSON_PullRequest(t *testing.T) {
	t.Parallel()

	item := ProjectV2Item{
		ID:          Ptr(int64(124)),
		NodeID:      Ptr("PVTI_pr"),
		ContentType: Ptr(ProjectV2ItemContentTypePullRequest),
		Content: &ProjectV2ItemContent{
			PullRequest: &PullRequest{
				ID:             Ptr(int64(457)),
				Number:         Ptr(20),
				Title:          Ptr("Test PR"),
				State:          Ptr("closed"),
				Merged:         Ptr(true),
				MergeCommitSHA: Ptr("abc123"),
				Head: &PullRequestBranch{
					Ref: Ptr("feature-branch"),
					SHA: Ptr("def456"),
				},
				Base: &PullRequestBranch{
					Ref: Ptr("main"),
					SHA: Ptr("ghi789"),
				},
			},
		},
		CreatedAt: &referenceTimestamp,
	}

	want := `{
		"id": 124,
		"node_id": "PVTI_pr",
		"content_type": "PullRequest",
		"content": {
			"id": 457,
			"number": 20,
			"title": "Test PR",
			"state": "closed",
			"merged": true,
			"merge_commit_sha": "abc123",
			"head": {
				"ref": "feature-branch",
				"sha": "def456"
			},
			"base": {
				"ref": "main",
				"sha": "ghi789"
			}
		},
		"created_at": ` + referenceTimeStr + `
	}`

	testJSONUnmarshalOnly(t, item, want)
}

func TestProjectV2Item_UnmarshalJSON_DraftIssue(t *testing.T) {
	t.Parallel()

	item := ProjectV2Item{
		ID:          Ptr(int64(125)),
		NodeID:      Ptr("PVTI_draft"),
		ContentType: Ptr(ProjectV2ItemContentTypeDraftIssue),
		Content: &ProjectV2ItemContent{
			DraftIssue: &ProjectV2DraftIssue{
				ID:     Ptr(int64(458)),
				NodeID: Ptr("DI_test"),
				Title:  Ptr("Draft Issue Title"),
				Body:   Ptr("Draft issue body content"),
			},
		},
		CreatedAt: &referenceTimestamp,
	}

	want := `{
		"id": 125,
		"node_id": "PVTI_draft",
		"content_type": "DraftIssue",
		"content": {
			"id": 458,
			"node_id": "DI_test",
			"title": "Draft Issue Title",
			"body": "Draft issue body content"
		},
		"created_at": ` + referenceTimeStr + `
	}`

	testJSONUnmarshalOnly(t, item, want)
}

func TestProjectV2Item_UnmarshalJSON_NullContent(t *testing.T) {
	t.Parallel()

	item := ProjectV2Item{
		ID:          Ptr(int64(126)),
		NodeID:      Ptr("PVTI_null"),
		ContentType: Ptr(ProjectV2ItemContentTypeIssue),
		Content:     nil, // Content is null
	}

	want := `{
		"id": 126,
		"node_id": "PVTI_null",
		"content_type": "Issue",
		"content": null
	}`

	testJSONUnmarshalOnly(t, item, want)
}

func TestProjectV2Item_UnmarshalJSON_MissingContentType(t *testing.T) {
	t.Parallel()

	want := `{
		"id": 127,
		"node_id": "PVTI_no_type",
		"content": {
			"id": 459,
			"title": "Some content"
		}
	}`

	item := ProjectV2Item{
		ID:      Ptr(int64(127)),
		NodeID:  Ptr("PVTI_no_type"),
		Content: nil,
	}

	testJSONUnmarshalOnly(t, item, want)
}

func TestProjectV2Item_UnmarshalJSON_EmptyJSON(t *testing.T) {
	t.Parallel()

	item := ProjectV2Item{}
	want := "null"

	testJSONUnmarshalOnly(t, item, want)
}

func TestProjectV2Item_UnmarshalJSON_InvalidJSON(t *testing.T) {
	t.Parallel()

	var item ProjectV2Item
	if err := json.Unmarshal([]byte("~~~"), &item); err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestProjectV2Item_Marshal_Issue(t *testing.T) {
	t.Parallel()
	testJSONMarshal(t, &ProjectV2Item{}, "{}")

	item := &ProjectV2Item{
		ContentType: Ptr(ProjectV2ItemContentTypeIssue),
		Content: &ProjectV2ItemContent{
			Issue: &Issue{
				Number: Ptr(42),
				Title:  Ptr("Bug report"),
				State:  Ptr("open"),
			},
		},
		ID: Ptr(int64(123)),
	}

	want := `{
		"content_type":"Issue",
		"content":{
			"number":42,
			"state":"open",
			"title":"Bug report"
		},
		"id":123
	}`

	testJSONMarshal(t, item, want)
}

func TestProjectV2Item_Marshal_PullRequest(t *testing.T) {
	t.Parallel()
	testJSONMarshal(t, ProjectV2Item{}, "{}")

	item := ProjectV2Item{
		ContentType: Ptr(ProjectV2ItemContentTypePullRequest),
		Content: &ProjectV2ItemContent{
			PullRequest: &PullRequest{
				Number: Ptr(99),
				Title:  Ptr("Feature addition"),
				State:  Ptr("closed"),
			},
		},
		ID: Ptr(int64(456)),
	}

	want := `{
		"content_type":"PullRequest",
		"content":{
			"number":99,
			"state":"closed",
			"title":"Feature addition"
		},
		"id":456
	}`

	testJSONMarshal(t, item, want)
}

func TestProjectV2Item_Marshal_DraftIssue(t *testing.T) {
	t.Parallel()
	testJSONMarshal(t, &ProjectV2Item{}, "{}")

	item := &ProjectV2Item{
		ContentType: Ptr(ProjectV2ItemContentTypeDraftIssue),
		Content: &ProjectV2ItemContent{
			DraftIssue: &ProjectV2DraftIssue{
				Title: Ptr("Draft task"),
				Body:  Ptr("Work in progress"),
			},
		},
		ID: Ptr(int64(789)),
	}

	want := `{
		"content_type":"DraftIssue",
		"content":{
			"body":"Work in progress",
			"title":"Draft task"
		},
		"id":789
	}`

	testJSONMarshal(t, item, want)
}

func TestProjectV2Item_Marshal_MissingContent(t *testing.T) {
	t.Parallel()

	item := &ProjectV2Item{
		ContentType: Ptr(ProjectV2ItemContentTypeIssue),
		Content:     &ProjectV2ItemContent{},
		ID:          Ptr(int64(789)),
	}

	want := `{
		"content_type":"Issue",
		"content":null,
		"id":789
	}`

	testJSONMarshalOnly(t, item, want)
}

func TestProjectV2ItemContent_Marshal(t *testing.T) {
	t.Parallel()

	t.Run("issue", func(t *testing.T) {
		t.Parallel()

		content := ProjectV2ItemContent{
			Issue: &Issue{
				Number: Ptr(42),
				Title:  Ptr("Bug report"),
				State:  Ptr("open"),
			},
		}

		want := `{
			"number":42,
			"state":"open",
			"title":"Bug report"
		}`

		testJSONMarshalOnly(t, content, want)
		testJSONMarshalOnly(t, &content, want)
	})

	t.Run("pull request", func(t *testing.T) {
		t.Parallel()

		content := ProjectV2ItemContent{
			PullRequest: &PullRequest{
				Number: Ptr(99),
				Title:  Ptr("Feature addition"),
				State:  Ptr("closed"),
			},
		}

		want := `{
			"number":99,
			"state":"closed",
			"title":"Feature addition"
		}`

		testJSONMarshalOnly(t, content, want)
		testJSONMarshalOnly(t, &content, want)
	})

	t.Run("draft issue", func(t *testing.T) {
		t.Parallel()

		content := ProjectV2ItemContent{
			DraftIssue: &ProjectV2DraftIssue{
				Title: Ptr("Draft task"),
				Body:  Ptr("Work in progress"),
			},
		}

		want := `{
			"body":"Work in progress",
			"title":"Draft task"
		}`

		testJSONMarshalOnly(t, content, want)
		testJSONMarshalOnly(t, &content, want)
	})
}

func TestProjectsService_CreateOrganizationProjectDraftItem(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := CreateProjectV2DraftItemRequest{Title: "My draft", Body: Ptr("Draft body")}

	mux.HandleFunc("/orgs/o/projectsV2/1/drafts", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":42,"node_id":"PVTI_draft","content_type":"DraftIssue"}`)
	})

	ctx := t.Context()
	item, _, err := client.Projects.CreateOrganizationProjectDraftItem(ctx, "o", 1, input)
	if err != nil {
		t.Fatalf("Projects.CreateOrganizationProjectDraftItem returned error: %v", err)
	}
	want := &ProjectV2Item{ID: Ptr(int64(42)), NodeID: Ptr("PVTI_draft"), ContentType: Ptr(ProjectV2ItemContentTypeDraftIssue)}
	if diff := cmp.Diff(want, item); diff != "" {
		t.Errorf("Projects.CreateOrganizationProjectDraftItem mismatch (-want +got):\n%v", diff)
	}

	const methodName = "CreateOrganizationProjectDraftItem"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Projects.CreateOrganizationProjectDraftItem(ctx, "\n", 1, input)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.CreateOrganizationProjectDraftItem(ctx, "o", 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestProjectsService_CreateUserProjectDraftItem(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := CreateProjectV2DraftItemRequest{Title: "My draft"}

	mux.HandleFunc("/user/12345/projectsV2/1/drafts", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":43,"node_id":"PVTI_draft_u","content_type":"DraftIssue"}`)
	})

	ctx := t.Context()
	item, _, err := client.Projects.CreateUserProjectDraftItem(ctx, 12345, 1, input)
	if err != nil {
		t.Fatalf("Projects.CreateUserProjectDraftItem returned error: %v", err)
	}
	want := &ProjectV2Item{ID: Ptr(int64(43)), NodeID: Ptr("PVTI_draft_u"), ContentType: Ptr(ProjectV2ItemContentTypeDraftIssue)}
	if diff := cmp.Diff(want, item); diff != "" {
		t.Errorf("Projects.CreateUserProjectDraftItem mismatch (-want +got):\n%v", diff)
	}

	const methodName = "CreateUserProjectDraftItem"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.CreateUserProjectDraftItem(ctx, 12345, 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestProjectsService_AddOrganizationProjectField(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := AddProjectV2FieldRequest{
		Name:     Ptr("Priority"),
		DataType: Ptr("single_select"),
		SingleSelectOptions: []*ProjectV2FieldSingleSelectOption{
			{Name: "High", Color: Ptr("RED"), Description: Ptr("Urgent")},
			{Name: "Low", Color: Ptr("GRAY")},
		},
	}

	mux.HandleFunc("/orgs/o/projectsV2/1/fields", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":7,"name":"Priority","data_type":"single_select"}`)
	})

	ctx := t.Context()
	field, _, err := client.Projects.AddOrganizationProjectField(ctx, "o", 1, input)
	if err != nil {
		t.Fatalf("Projects.AddOrganizationProjectField returned error: %v", err)
	}
	want := &ProjectV2Field{ID: Ptr(int64(7)), Name: Ptr("Priority"), DataType: Ptr("single_select")}
	if diff := cmp.Diff(want, field); diff != "" {
		t.Errorf("Projects.AddOrganizationProjectField mismatch (-want +got):\n%v", diff)
	}

	const methodName = "AddOrganizationProjectField"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Projects.AddOrganizationProjectField(ctx, "\n", 1, input)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.AddOrganizationProjectField(ctx, "o", 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestProjectsService_AddUserProjectField(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := AddProjectV2FieldRequest{
		Name:     Ptr("Sprint"),
		DataType: Ptr("iteration"),
		IterationConfiguration: &ProjectV2FieldIterationConfiguration{
			StartDate: Ptr("2026-01-01"),
			Duration:  Ptr(14),
			Iterations: []*ProjectV2FieldIterationConfigurationIteration{
				{Title: Ptr("Sprint 1"), StartDate: Ptr("2026-01-01"), Duration: Ptr(14)},
			},
		},
	}

	mux.HandleFunc("/users/u/projectsV2/1/fields", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":8,"name":"Sprint","data_type":"iteration"}`)
	})

	ctx := t.Context()
	field, _, err := client.Projects.AddUserProjectField(ctx, "u", 1, input)
	if err != nil {
		t.Fatalf("Projects.AddUserProjectField returned error: %v", err)
	}
	want := &ProjectV2Field{ID: Ptr(int64(8)), Name: Ptr("Sprint"), DataType: Ptr("iteration")}
	if diff := cmp.Diff(want, field); diff != "" {
		t.Errorf("Projects.AddUserProjectField mismatch (-want +got):\n%v", diff)
	}

	const methodName = "AddUserProjectField"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Projects.AddUserProjectField(ctx, "\n", 1, input)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.AddUserProjectField(ctx, "u", 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestProjectsService_CreateOrganizationProjectView(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := CreateProjectV2ViewRequest{
		Name:          "My board",
		Layout:        "board",
		Filter:        Ptr("is:open"),
		VisibleFields: []int64{1, 2, 3},
	}

	mux.HandleFunc("/orgs/o/projectsV2/1/views", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		// The first field_id is larger than 2^53 to ensure it is decoded as an
		// int64 without the precision loss that a float64 would introduce.
		fmt.Fprint(w, `{"id":5,"number":2,"name":"My board","layout":"board","sort_by":[[9007199254740993,"asc"],[456,"desc"]]}`)
	})

	ctx := t.Context()
	view, _, err := client.Projects.CreateOrganizationProjectView(ctx, "o", 1, input)
	if err != nil {
		t.Fatalf("Projects.CreateOrganizationProjectView returned error: %v", err)
	}
	want := &ProjectV2View{
		ID:     5,
		Number: 2,
		Name:   "My board",
		Layout: "board",
		SortBy: []*ProjectV2ViewSortBy{
			{FieldID: Ptr(int64(9007199254740993)), Direction: Ptr("asc")},
			{FieldID: Ptr(int64(456)), Direction: Ptr("desc")},
		},
	}
	if diff := cmp.Diff(want, view); diff != "" {
		t.Errorf("Projects.CreateOrganizationProjectView mismatch (-want +got):\n%v", diff)
	}

	const methodName = "CreateOrganizationProjectView"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Projects.CreateOrganizationProjectView(ctx, "\n", 1, input)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.CreateOrganizationProjectView(ctx, "o", 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestProjectsService_CreateUserProjectView(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := CreateProjectV2ViewRequest{Name: "My table", Layout: "table"}

	mux.HandleFunc("/users/12345/projectsV2/1/views", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":6,"number":3,"name":"My table","layout":"table"}`)
	})

	ctx := t.Context()
	view, _, err := client.Projects.CreateUserProjectView(ctx, 12345, 1, input)
	if err != nil {
		t.Fatalf("Projects.CreateUserProjectView returned error: %v", err)
	}
	want := &ProjectV2View{ID: 6, Number: 3, Name: "My table", Layout: "table"}
	if diff := cmp.Diff(want, view); diff != "" {
		t.Errorf("Projects.CreateUserProjectView mismatch (-want +got):\n%v", diff)
	}

	const methodName = "CreateUserProjectView"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.CreateUserProjectView(ctx, 12345, 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestProjectsService_ListOrganizationProjectViewItems(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/projectsV2/1/views/2/items", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"after": "2", "before": "1", "per_page": "50", "fields": "10,11", "q": "status:open"})
		fmt.Fprint(w, `[{"id":21,"node_id":"PVTI_view_item"}]`)
	})

	opts := &ListProjectItemsOptions{ListProjectsOptions: ListProjectsOptions{ListProjectsPaginationOptions: ListProjectsPaginationOptions{After: "2", Before: "1", PerPage: 50}, Query: "status:open"}, Fields: []int64{10, 11}}
	ctx := t.Context()
	items, _, err := client.Projects.ListOrganizationProjectViewItems(ctx, "o", 1, 2, opts)
	if err != nil {
		t.Fatalf("Projects.ListOrganizationProjectViewItems returned error: %v", err)
	}
	want := []*ProjectV2Item{{ID: Ptr(int64(21)), NodeID: Ptr("PVTI_view_item")}}
	if diff := cmp.Diff(want, items); diff != "" {
		t.Errorf("Projects.ListOrganizationProjectViewItems mismatch (-want +got):\n%v", diff)
	}

	const methodName = "ListOrganizationProjectViewItems"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Projects.ListOrganizationProjectViewItems(ctx, "\n", 1, 2, opts)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.ListOrganizationProjectViewItems(ctx, "o", 1, 2, opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestProjectsService_ListUserProjectViewItems(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/projectsV2/1/views/2/items", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"per_page": "30"})
		fmt.Fprint(w, `[{"id":22,"node_id":"PVTI_view_item_u"}]`)
	})

	opts := &ListProjectItemsOptions{ListProjectsOptions: ListProjectsOptions{ListProjectsPaginationOptions: ListProjectsPaginationOptions{PerPage: 30}}}
	ctx := t.Context()
	items, _, err := client.Projects.ListUserProjectViewItems(ctx, "u", 1, 2, opts)
	if err != nil {
		t.Fatalf("Projects.ListUserProjectViewItems returned error: %v", err)
	}
	want := []*ProjectV2Item{{ID: Ptr(int64(22)), NodeID: Ptr("PVTI_view_item_u")}}
	if diff := cmp.Diff(want, items); diff != "" {
		t.Errorf("Projects.ListUserProjectViewItems mismatch (-want +got):\n%v", diff)
	}

	const methodName = "ListUserProjectViewItems"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Projects.ListUserProjectViewItems(ctx, "\n", 1, 2, opts)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Projects.ListUserProjectViewItems(ctx, "u", 1, 2, opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestProjectV2ViewSortBy_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	t.Run("numeric and string field_id", func(t *testing.T) {
		t.Parallel()
		// The second element uses a string field_id, which the OpenAPI schema
		// permits. The first exceeds 2^53 to confirm int64 (not float64) decoding.
		want := []*ProjectV2ViewSortBy{
			{FieldID: Ptr(int64(9007199254740993)), Direction: Ptr("asc")},
			{FieldID: Ptr(int64(456)), Direction: Ptr("desc")},
		}
		testJSONUnmarshalOnly(t, want, `[[9007199254740993,"asc"],["456","desc"]]`)
	})

	t.Run("wrong tuple length is an error", func(t *testing.T) {
		t.Parallel()
		var got ProjectV2ViewSortBy
		if err := json.Unmarshal([]byte(`[1]`), &got); err == nil {
			t.Error("Unmarshal of a 1-element tuple = nil error, want error")
		}
	})

	t.Run("non-array is an error", func(t *testing.T) {
		t.Parallel()
		var got ProjectV2ViewSortBy
		if err := json.Unmarshal([]byte(`{"a":1}`), &got); err == nil {
			t.Error("Unmarshal of a non-array = nil error, want error")
		}
	})

	t.Run("field_id of an unsupported type is an error", func(t *testing.T) {
		t.Parallel()
		var got ProjectV2ViewSortBy
		if err := json.Unmarshal([]byte(`[true,"asc"]`), &got); err == nil {
			t.Error("Unmarshal of a boolean field_id = nil error, want error")
		}
	})

	t.Run("non-numeric string field_id is an error", func(t *testing.T) {
		t.Parallel()
		var got ProjectV2ViewSortBy
		if err := json.Unmarshal([]byte(`["abc","asc"]`), &got); err == nil {
			t.Error(`Unmarshal of field_id "abc" = nil error, want error`)
		}
	})

	t.Run("non-string direction is an error", func(t *testing.T) {
		t.Parallel()
		var got ProjectV2ViewSortBy
		if err := json.Unmarshal([]byte(`[123,456]`), &got); err == nil {
			t.Error("Unmarshal of a numeric direction = nil error, want error")
		}
	})

	t.Run("round trips through MarshalJSON", func(t *testing.T) {
		t.Parallel()
		in := &ProjectV2ViewSortBy{FieldID: Ptr(int64(9007199254740993)), Direction: Ptr("asc")}
		testJSONMarshal(t, in, `[9007199254740993,"asc"]`)
	})
}
