// Copyright 2025 The go-github AUTHORS. All rights reserved.
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

func TestOrganizationsService_ListIssueTypes(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/issue-types", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[
			{
				"id": 410,
				"node_id": "IT_kwDNAd3NAZo",
				"name": "Task",
				"description": "A specific piece of work",
				"created_at": `+refTimeStr(1136178000)+`,
				"updated_at": `+refTimeStr(1136178001)+`
			},
			{
				"id": 411,
				"node_id": "IT_kwDNAd3NAZs",
				"name": "Bug",
				"description": "An unexpected problem or behavior",
				"created_at": `+refTimeStr(1136178002)+`,
				"updated_at": `+refTimeStr(1136178003)+`
			}
		]`)
	})

	ctx := t.Context()
	issueTypes, _, err := client.Organizations.ListIssueTypes(ctx, "o")
	if err != nil {
		t.Errorf("Organizations.ListIssueTypes returned error: %v", err)
	}

	want := []*IssueType{
		{
			ID:          Ptr(int64(410)),
			NodeID:      Ptr("IT_kwDNAd3NAZo"),
			Name:        Ptr("Task"),
			Description: Ptr("A specific piece of work"),
			CreatedAt:   refTimestamp(1136178000),
			UpdatedAt:   refTimestamp(1136178001),
		},
		{
			ID:          Ptr(int64(411)),
			NodeID:      Ptr("IT_kwDNAd3NAZs"),
			Name:        Ptr("Bug"),
			Description: Ptr("An unexpected problem or behavior"),
			CreatedAt:   refTimestamp(1136178002),
			UpdatedAt:   refTimestamp(1136178003),
		},
	}
	if !cmp.Equal(issueTypes, want) {
		t.Errorf("Organizations.ListIssueTypes returned %+v, want %+v", issueTypes, want)
	}

	const methodName = "ListIssueTypes"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListIssueTypes(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListIssueTypes(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_CreateIssueType(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &CreateOrUpdateIssueTypesOptions{
		Name:        "Epic",
		Description: Ptr("An issue type for a multi-week tracking of work"),
		IsEnabled:   true,
		Color:       Ptr("green"),
		IsPrivate:   Ptr(true),
	}

	mux.HandleFunc("/orgs/o/issue-types", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{
				"id": 410,
				"node_id": "IT_kwDNAd3NAZo",
				"name": "Epic",
				"description": "An issue type for a multi-week tracking of work",
				"created_at": `+refTimeStr(1136178000)+`,
				"updated_at": `+refTimeStr(1136178001)+`
		}`)
	})

	ctx := t.Context()
	issueType, _, err := client.Organizations.CreateIssueType(ctx, "o", input)
	if err != nil {
		t.Errorf("Organizations.CreateIssueType returned error: %v", err)
	}
	want := &IssueType{
		ID:          Ptr(int64(410)),
		NodeID:      Ptr("IT_kwDNAd3NAZo"),
		Name:        Ptr("Epic"),
		Description: Ptr("An issue type for a multi-week tracking of work"),
		CreatedAt:   refTimestamp(1136178000),
		UpdatedAt:   refTimestamp(1136178001),
	}

	if !cmp.Equal(issueType, want) {
		t.Errorf("Organizations.CreateIssueType returned %+v, want %+v", issueType, want)
	}

	const methodName = "CreateIssueType"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.CreateIssueType(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.CreateIssueType(ctx, "o", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_UpdateIssueType(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &CreateOrUpdateIssueTypesOptions{
		Name:        "Epic",
		Description: Ptr("An issue type for a multi-week tracking of work"),
		IsEnabled:   true,
		Color:       Ptr("green"),
		IsPrivate:   Ptr(true),
	}

	mux.HandleFunc("/orgs/o/issue-types/410", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{
				"id": 410,
				"node_id": "IT_kwDNAd3NAZo",
				"name": "Epic",
				"description": "An issue type for a multi-week tracking of work",
				"created_at": `+refTimeStr(1136178000)+`,
				"updated_at": `+refTimeStr(1136178001)+`
		}`)
	})

	ctx := t.Context()
	issueType, _, err := client.Organizations.UpdateIssueType(ctx, "o", 410, input)
	if err != nil {
		t.Errorf("Organizations.UpdateIssueType returned error: %v", err)
	}
	want := &IssueType{
		ID:          Ptr(int64(410)),
		NodeID:      Ptr("IT_kwDNAd3NAZo"),
		Name:        Ptr("Epic"),
		Description: Ptr("An issue type for a multi-week tracking of work"),
		CreatedAt:   refTimestamp(1136178000),
		UpdatedAt:   refTimestamp(1136178001),
	}

	if !cmp.Equal(issueType, want) {
		t.Errorf("Organizations.UpdateIssueType returned %+v, want %+v", issueType, want)
	}

	const methodName = "UpdateIssueType"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.UpdateIssueType(ctx, "\n", -1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.UpdateIssueType(ctx, "o", 410, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_DeleteIssueType(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/issue-types/410", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Organizations.DeleteIssueType(ctx, "o", 410)
	if err != nil {
		t.Errorf("Organizations.DeleteIssueType returned error: %v", err)
	}

	const methodName = "DeleteIssueType"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.DeleteIssueType(ctx, "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.DeleteIssueType(ctx, "o", 410)
	})
}
