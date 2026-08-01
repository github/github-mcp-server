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

func TestOrganizationsService_ListSecurityManagerTeams(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/security-managers", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{"id":1}]`)
	})

	ctx := t.Context()
	teams, _, err := client.Organizations.ListSecurityManagerTeams(ctx, "o")
	if err != nil {
		t.Errorf("Organizations.ListSecurityManagerTeams returned error: %v", err)
	}

	want := []*Team{{ID: Ptr(int64(1))}}
	if !cmp.Equal(teams, want) {
		t.Errorf("Organizations.ListSecurityManagerTeams returned %+v, want %+v", teams, want)
	}

	const methodName = "ListSecurityManagerTeams"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListSecurityManagerTeams(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListSecurityManagerTeams(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_ListSecurityManagerTeams_invalidOrg(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Organizations.ListSecurityManagerTeams(ctx, "%")
	testURLParseError(t, err)
}

func TestOrganizationsService_AddSecurityManagerTeam(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/security-managers/teams/t", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
	})

	ctx := t.Context()
	_, err := client.Organizations.AddSecurityManagerTeam(ctx, "o", "t")
	if err != nil {
		t.Errorf("Organizations.AddSecurityManagerTeam returned error: %v", err)
	}

	const methodName = "AddSecurityManagerTeam"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.AddSecurityManagerTeam(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.AddSecurityManagerTeam(ctx, "o", "t")
	})
}

func TestOrganizationsService_AddSecurityManagerTeam_invalidOrg(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Organizations.AddSecurityManagerTeam(ctx, "%", "t")
	testURLParseError(t, err)
}

func TestOrganizationsService_AddSecurityManagerTeam_invalidTeam(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Organizations.AddSecurityManagerTeam(ctx, "%", "t")
	testURLParseError(t, err)
}

func TestOrganizationsService_RemoveSecurityManagerTeam(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/security-managers/teams/t", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Organizations.RemoveSecurityManagerTeam(ctx, "o", "t")
	if err != nil {
		t.Errorf("Organizations.RemoveSecurityManagerTeam returned error: %v", err)
	}

	const methodName = "RemoveSecurityManagerTeam"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.RemoveSecurityManagerTeam(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.RemoveSecurityManagerTeam(ctx, "o", "t")
	})
}

func TestOrganizationsService_RemoveSecurityManagerTeam_invalidOrg(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Organizations.RemoveSecurityManagerTeam(ctx, "%", "t")
	testURLParseError(t, err)
}

func TestOrganizationsService_RemoveSecurityManagerTeam_invalidTeam(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Organizations.RemoveSecurityManagerTeam(ctx, "%", "t")
	testURLParseError(t, err)
}
