// Copyright 2022 The go-github AUTHORS. All rights reserved.
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

func TestDependabotService_ListRepoAlerts(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/dependabot/alerts", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"state": "open"})
		fmt.Fprint(w, `[{"number":1,"state":"open"},{"number":42,"state":"fixed"}]`)
	})

	opts := &ListAlertsOptions{State: Ptr("open")}
	ctx := t.Context()
	alerts, _, err := client.Dependabot.ListRepoAlerts(ctx, "o", "r", opts)
	if err != nil {
		t.Errorf("Dependabot.ListRepoAlerts returned error: %v", err)
	}

	want := []*DependabotAlert{
		{Number: Ptr(1), State: Ptr("open")},
		{Number: Ptr(42), State: Ptr("fixed")},
	}
	if !cmp.Equal(alerts, want) {
		t.Errorf("Dependabot.ListRepoAlerts returned %+v, want %+v", alerts, want)
	}

	const methodName = "ListRepoAlerts"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Dependabot.ListRepoAlerts(ctx, "\n", "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Dependabot.ListRepoAlerts(ctx, "o", "r", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestDependabotService_GetRepoAlert(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/dependabot/alerts/42", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"number":42,"state":"fixed"}`)
	})

	ctx := t.Context()
	alert, _, err := client.Dependabot.GetRepoAlert(ctx, "o", "r", 42)
	if err != nil {
		t.Errorf("Dependabot.GetRepoAlert returned error: %v", err)
	}

	want := &DependabotAlert{
		Number: Ptr(42),
		State:  Ptr("fixed"),
	}
	if !cmp.Equal(alert, want) {
		t.Errorf("Dependabot.GetRepoAlert returned %+v, want %+v", alert, want)
	}

	const methodName = "GetRepoAlert"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Dependabot.GetRepoAlert(ctx, "\n", "\n", 0)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Dependabot.GetRepoAlert(ctx, "o", "r", 42)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestDependabotService_ListOrgAlerts(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/dependabot/alerts", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"state": "open"})
		fmt.Fprint(w, `[{"number":1,"state":"open"},{"number":42,"state":"fixed"}]`)
	})

	opts := &ListAlertsOptions{State: Ptr("open")}
	ctx := t.Context()
	alerts, _, err := client.Dependabot.ListOrgAlerts(ctx, "o", opts)
	if err != nil {
		t.Errorf("Dependabot.ListOrgAlerts returned error: %v", err)
	}

	want := []*DependabotAlert{
		{Number: Ptr(1), State: Ptr("open")},
		{Number: Ptr(42), State: Ptr("fixed")},
	}
	if !cmp.Equal(alerts, want) {
		t.Errorf("Dependabot.ListOrgAlerts returned %+v, want %+v", alerts, want)
	}

	const methodName = "ListOrgAlerts"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Dependabot.ListOrgAlerts(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Dependabot.ListOrgAlerts(ctx, "o", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestDependabotService_UpdateAlert(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	state := Ptr("dismissed")
	dismissedReason := Ptr("no_bandwidth")
	dismissedComment := Ptr("no time to fix this")

	alertState := &DependabotAlertState{State: *state, DismissedReason: dismissedReason, DismissedComment: dismissedComment}

	mux.HandleFunc("/repos/o/r/dependabot/alerts/42", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		fmt.Fprint(w, `{"number":42,"state":"dismissed","dismissed_reason":"no_bandwidth","dismissed_comment":"no time to fix this"}`)
	})

	ctx := t.Context()
	alert, _, err := client.Dependabot.UpdateAlert(ctx, "o", "r", 42, alertState)
	if err != nil {
		t.Errorf("Dependabot.UpdateAlert returned error: %v", err)
	}

	want := &DependabotAlert{
		Number:           Ptr(42),
		State:            Ptr("dismissed"),
		DismissedReason:  Ptr("no_bandwidth"),
		DismissedComment: Ptr("no time to fix this"),
	}
	if !cmp.Equal(alert, want) {
		t.Errorf("Dependabot.UpdateAlert returned %+v, want %+v", alert, want)
	}

	const methodName = "UpdateAlert"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Dependabot.UpdateAlert(ctx, "\n", "\n", 0, alertState)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Dependabot.UpdateAlert(ctx, "o", "r", 42, alertState)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
