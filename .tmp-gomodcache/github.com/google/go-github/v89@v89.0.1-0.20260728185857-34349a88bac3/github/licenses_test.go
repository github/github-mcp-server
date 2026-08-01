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

func TestLicensesService_List(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/licenses", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"featured": "true", "page": "2", "per_page": "20"})
		fmt.Fprint(w, `[{"key":"mit","name":"MIT","spdx_id":"MIT","url":"https://api.github.com/licenses/mit","featured":true}]`)
	})

	opts := &ListLicensesOptions{Featured: Ptr(true), ListOptions: ListOptions{Page: 2, PerPage: 20}}
	ctx := t.Context()
	licenses, _, err := client.Licenses.List(ctx, opts)
	if err != nil {
		t.Errorf("Licenses.List returned error: %v", err)
	}

	want := []*License{{
		Key:      Ptr("mit"),
		Name:     Ptr("MIT"),
		SPDXID:   Ptr("MIT"),
		URL:      Ptr("https://api.github.com/licenses/mit"),
		Featured: Ptr(true),
	}}
	if !cmp.Equal(licenses, want) {
		t.Errorf("Licenses.List returned %+v, want %+v", licenses, want)
	}

	const methodName = "List"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Licenses.List(ctx, opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestLicensesService_Get(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/licenses/mit", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"key":"mit","name":"MIT"}`)
	})

	ctx := t.Context()
	license, _, err := client.Licenses.Get(ctx, "mit")
	if err != nil {
		t.Errorf("Licenses.Get returned error: %v", err)
	}

	want := &License{Key: Ptr("mit"), Name: Ptr("MIT")}
	if !cmp.Equal(license, want) {
		t.Errorf("Licenses.Get returned %+v, want %+v", license, want)
	}

	const methodName = "Get"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Licenses.Get(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Licenses.Get(ctx, "mit")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestLicensesService_Get_invalidTemplate(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Licenses.Get(ctx, "%")
	testURLParseError(t, err)
}
