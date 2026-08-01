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

func TestOrganizationsService_ListHookDeliveries(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/hooks/1/deliveries", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"cursor": "v1_12077215967"})
		fmt.Fprint(w, `[{"id":1}, {"id":2}]`)
	})

	opt := &ListCursorOptions{Cursor: "v1_12077215967"}

	ctx := t.Context()
	hooks, _, err := client.Organizations.ListHookDeliveries(ctx, "o", 1, opt)
	if err != nil {
		t.Errorf("Organizations.ListHookDeliveries returned error: %v", err)
	}

	want := []*HookDelivery{{ID: Ptr(int64(1))}, {ID: Ptr(int64(2))}}
	if d := cmp.Diff(hooks, want); d != "" {
		t.Errorf("Organizations.ListHooks want (-), got (+):\n%v", d)
	}

	const methodName = "ListHookDeliveries"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListHookDeliveries(ctx, "\n", -1, opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListHookDeliveries(ctx, "o", 1, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_ListHookDeliveries_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Organizations.ListHookDeliveries(ctx, "%", 1, nil)
	testURLParseError(t, err)
}

func TestOrganizationsService_GetHookDelivery(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/hooks/1/deliveries/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	hook, _, err := client.Organizations.GetHookDelivery(ctx, "o", 1, 1)
	if err != nil {
		t.Errorf("Organizations.GetHookDelivery returned error: %v", err)
	}

	want := &HookDelivery{ID: Ptr(int64(1))}
	if !cmp.Equal(hook, want) {
		t.Errorf("Organizations.GetHookDelivery returned %+v, want %+v", hook, want)
	}

	const methodName = "GetHookDelivery"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.GetHookDelivery(ctx, "\n", -1, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.GetHookDelivery(ctx, "o", 1, 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_GetHookDelivery_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Organizations.GetHookDelivery(ctx, "%", 1, 1)
	testURLParseError(t, err)
}

func TestOrganizationsService_RedeliverHookDelivery(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/hooks/1/deliveries/1/attempts", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	hook, _, err := client.Organizations.RedeliverHookDelivery(ctx, "o", 1, 1)
	if err != nil {
		t.Errorf("Organizations.RedeliverHookDelivery returned error: %v", err)
	}

	want := &HookDelivery{ID: Ptr(int64(1))}
	if !cmp.Equal(hook, want) {
		t.Errorf("Organizations.RedeliverHookDelivery returned %+v, want %+v", hook, want)
	}

	const methodName = "Rede;overHookDelivery"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.RedeliverHookDelivery(ctx, "\n", -1, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.RedeliverHookDelivery(ctx, "o", 1, 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_RedeliverHookDelivery_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Organizations.RedeliverHookDelivery(ctx, "%", 1, 1)
	testURLParseError(t, err)
}
