// Copyright 2014 The go-github AUTHORS. All rights reserved.
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

func TestActivityService_ListWatchers(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/subscribers", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"page": "2",
		})

		fmt.Fprint(w, `[{"id":1}]`)
	})

	ctx := t.Context()
	watchers, _, err := client.Activity.ListWatchers(ctx, "o", "r", &ListOptions{Page: 2})
	if err != nil {
		t.Errorf("Activity.ListWatchers returned error: %v", err)
	}

	want := []*User{{ID: Ptr(int64(1))}}
	if !cmp.Equal(watchers, want) {
		t.Errorf("Activity.ListWatchers returned %+v, want %+v", watchers, want)
	}

	const methodName = "ListWatchers"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Activity.ListWatchers(ctx, "\n", "\n", &ListOptions{Page: 2})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Activity.ListWatchers(ctx, "o", "r", &ListOptions{Page: 2})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActivityService_ListWatched_authenticatedUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"page": "2",
		})
		fmt.Fprint(w, `[{"id":1}]`)
	})

	ctx := t.Context()
	watched, _, err := client.Activity.ListWatched(ctx, "", &ListOptions{Page: 2})
	if err != nil {
		t.Errorf("Activity.ListWatched returned error: %v", err)
	}

	want := []*Repository{{ID: Ptr(int64(1))}}
	if !cmp.Equal(watched, want) {
		t.Errorf("Activity.ListWatched returned %+v, want %+v", watched, want)
	}

	const methodName = "ListWatched"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Activity.ListWatched(ctx, "\n", &ListOptions{Page: 2})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Activity.ListWatched(ctx, "", &ListOptions{Page: 2})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActivityService_ListWatched_specifiedUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"page": "2",
		})
		fmt.Fprint(w, `[{"id":1}]`)
	})

	ctx := t.Context()
	watched, _, err := client.Activity.ListWatched(ctx, "u", &ListOptions{Page: 2})
	if err != nil {
		t.Errorf("Activity.ListWatched returned error: %v", err)
	}

	want := []*Repository{{ID: Ptr(int64(1))}}
	if !cmp.Equal(watched, want) {
		t.Errorf("Activity.ListWatched returned %+v, want %+v", watched, want)
	}
}

func TestActivityService_GetRepositorySubscription_true(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/subscription", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"subscribed":true}`)
	})

	ctx := t.Context()
	sub, _, err := client.Activity.GetRepositorySubscription(ctx, "o", "r")
	if err != nil {
		t.Errorf("Activity.GetRepositorySubscription returned error: %v", err)
	}

	want := &Subscription{Subscribed: Ptr(true)}
	if !cmp.Equal(sub, want) {
		t.Errorf("Activity.GetRepositorySubscription returned %+v, want %+v", sub, want)
	}

	const methodName = "GetRepositorySubscription"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Activity.GetRepositorySubscription(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Activity.GetRepositorySubscription(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActivityService_GetRepositorySubscription_false(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/subscription", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNotFound)
	})

	ctx := t.Context()
	sub, _, err := client.Activity.GetRepositorySubscription(ctx, "o", "r")
	if err != nil {
		t.Errorf("Activity.GetRepositorySubscription returned error: %v", err)
	}

	var want *Subscription
	if !cmp.Equal(sub, want) {
		t.Errorf("Activity.GetRepositorySubscription returned %+v, want %+v", sub, want)
	}
}

func TestActivityService_GetRepositorySubscription_error(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/subscription", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusBadRequest)
	})

	ctx := t.Context()
	_, _, err := client.Activity.GetRepositorySubscription(ctx, "o", "r")
	if err == nil {
		t.Error("Expected HTTP 400 response")
	}
}

func TestActivityService_SetRepositorySubscription(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &Subscription{Subscribed: Ptr(true)}

	mux.HandleFunc("/repos/o/r/subscription", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"ignored":true}`)
	})

	ctx := t.Context()
	sub, _, err := client.Activity.SetRepositorySubscription(ctx, "o", "r", input)
	if err != nil {
		t.Errorf("Activity.SetRepositorySubscription returned error: %v", err)
	}

	want := &Subscription{Ignored: Ptr(true)}
	if !cmp.Equal(sub, want) {
		t.Errorf("Activity.SetRepositorySubscription returned %+v, want %+v", sub, want)
	}

	const methodName = "SetRepositorySubscription"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Activity.SetRepositorySubscription(ctx, "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Activity.SetRepositorySubscription(ctx, "o", "r", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActivityService_DeleteRepositorySubscription(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/subscription", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Activity.DeleteRepositorySubscription(ctx, "o", "r")
	if err != nil {
		t.Errorf("Activity.DeleteRepositorySubscription returned error: %v", err)
	}

	const methodName = "DeleteRepositorySubscription"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Activity.DeleteRepositorySubscription(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Activity.DeleteRepositorySubscription(ctx, "o", "r")
	})
}
