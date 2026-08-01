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

func TestRepositoriesService_ListTrafficReferrers(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/traffic/popular/referrers", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{
			"referrer": "Google",
			"count": 4,
			"uniques": 3
 		}]`)
	})
	ctx := t.Context()
	got, _, err := client.Repositories.ListTrafficReferrers(ctx, "o", "r")
	if err != nil {
		t.Errorf("Repositories.ListTrafficReferrers returned error: %+v", err)
	}

	want := []*TrafficReferrer{{
		Referrer: Ptr("Google"),
		Count:    Ptr(4),
		Uniques:  Ptr(3),
	}}
	if !cmp.Equal(got, want) {
		t.Errorf("Repositories.ListTrafficReferrers returned %+v, want %+v", got, want)
	}

	const methodName = "ListTrafficReferrers"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.ListTrafficReferrers(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListTrafficReferrers(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_ListTrafficPaths(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/traffic/popular/paths", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{
			"path": "/github/hubot",
			"title": "github/hubot: A customizable life embetterment robot.",
			"count": 3542,
			"uniques": 2225
 		}]`)
	})
	ctx := t.Context()
	got, _, err := client.Repositories.ListTrafficPaths(ctx, "o", "r")
	if err != nil {
		t.Errorf("Repositories.ListTrafficPaths returned error: %+v", err)
	}

	want := []*TrafficPath{{
		Path:    Ptr("/github/hubot"),
		Title:   Ptr("github/hubot: A customizable life embetterment robot."),
		Count:   Ptr(3542),
		Uniques: Ptr(2225),
	}}
	if !cmp.Equal(got, want) {
		t.Errorf("Repositories.ListTrafficPaths returned %+v, want %+v", got, want)
	}

	const methodName = "ListTrafficPaths"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.ListTrafficPaths(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListTrafficPaths(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_ListTrafficViews(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/traffic/views", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"count": 7,
			"uniques": 6,
			"views": [{
				"timestamp": `+referenceTimeStr+`,
				"count": 7,
				"uniques": 6
		}]}`)
	})

	ctx := t.Context()
	got, _, err := client.Repositories.ListTrafficViews(ctx, "o", "r", nil)
	if err != nil {
		t.Errorf("Repositories.ListTrafficViews returned error: %+v", err)
	}

	want := &TrafficViews{
		Views: []*TrafficData{{
			Timestamp: &referenceTimestamp,
			Count:     Ptr(7),
			Uniques:   Ptr(6),
		}},
		Count:   Ptr(7),
		Uniques: Ptr(6),
	}

	if !cmp.Equal(got, want) {
		t.Errorf("Repositories.ListTrafficViews returned %+v, want %+v", got, want)
	}

	const methodName = "ListTrafficViews"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.ListTrafficViews(ctx, "\n", "\n", &TrafficBreakdownOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListTrafficViews(ctx, "o", "r", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_ListTrafficClones(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/traffic/clones", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"count": 7,
			"uniques": 6,
			"clones": [{
				"timestamp": `+referenceTimeStr+`,
				"count": 7,
				"uniques": 6
		}]}`)
	})

	ctx := t.Context()
	got, _, err := client.Repositories.ListTrafficClones(ctx, "o", "r", nil)
	if err != nil {
		t.Errorf("Repositories.ListTrafficClones returned error: %+v", err)
	}

	want := &TrafficClones{
		Clones: []*TrafficData{{
			Timestamp: &referenceTimestamp,
			Count:     Ptr(7),
			Uniques:   Ptr(6),
		}},
		Count:   Ptr(7),
		Uniques: Ptr(6),
	}

	if !cmp.Equal(got, want) {
		t.Errorf("Repositories.ListTrafficClones returned %+v, want %+v", got, want)
	}

	const methodName = "ListTrafficClones"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.ListTrafficClones(ctx, "\n", "\n", &TrafficBreakdownOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListTrafficClones(ctx, "o", "r", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
