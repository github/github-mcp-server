// Copyright 2021 The go-github AUTHORS. All rights reserved.
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

func TestAppsService_ListHookDeliveries(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/app/hook/deliveries", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"cursor": "v1_12077215967"})
		fmt.Fprint(w, `[{"id":1}, {"id":2}]`)
	})

	opts := &ListCursorOptions{Cursor: "v1_12077215967"}

	ctx := t.Context()

	deliveries, _, err := client.Apps.ListHookDeliveries(ctx, opts)
	if err != nil {
		t.Errorf("Apps.ListHookDeliveries returned error: %v", err)
	}

	want := []*HookDelivery{{ID: Ptr(int64(1))}, {ID: Ptr(int64(2))}}
	if d := cmp.Diff(deliveries, want); d != "" {
		t.Errorf("Apps.ListHooks want (-), got (+):\n%v", d)
	}

	const methodName = "ListHookDeliveries"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Apps.ListHookDeliveries(ctx, opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestAppsService_GetHookDelivery(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/app/hook/deliveries/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	hook, _, err := client.Apps.GetHookDelivery(ctx, 1)
	if err != nil {
		t.Errorf("Apps.GetHookDelivery returned error: %v", err)
	}

	want := &HookDelivery{ID: Ptr(int64(1))}
	if !cmp.Equal(hook, want) {
		t.Errorf("Apps.GetHookDelivery returned %+v, want %+v", hook, want)
	}

	const methodName = "GetHookDelivery"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Apps.GetHookDelivery(ctx, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Apps.GetHookDelivery(ctx, 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestAppsService_RedeliverHookDelivery(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/app/hook/deliveries/1/attempts", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	hook, _, err := client.Apps.RedeliverHookDelivery(ctx, 1)
	if err != nil {
		t.Errorf("Apps.RedeliverHookDelivery returned error: %v", err)
	}

	want := &HookDelivery{ID: Ptr(int64(1))}
	if !cmp.Equal(hook, want) {
		t.Errorf("Apps.RedeliverHookDelivery returned %+v, want %+v", hook, want)
	}

	const methodName = "RedeliverHookDelivery"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Apps.RedeliverHookDelivery(ctx, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Apps.RedeliverHookDelivery(ctx, 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
