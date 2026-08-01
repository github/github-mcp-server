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

func TestAdminService_UpdateUserLDAPMapping(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &UserLDAPMapping{
		LDAPDN: Ptr("uid=asdf,ou=users,dc=github,dc=com"),
	}

	mux.HandleFunc("/admin/ldap/users/u/mapping", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":1,"ldap_dn":"uid=asdf,ou=users,dc=github,dc=com"}`)
	})

	ctx := t.Context()
	mapping, _, err := client.Admin.UpdateUserLDAPMapping(ctx, "u", input)
	if err != nil {
		t.Errorf("Admin.UpdateUserLDAPMapping returned error: %v", err)
	}

	want := &UserLDAPMapping{
		ID:     Ptr(int64(1)),
		LDAPDN: Ptr("uid=asdf,ou=users,dc=github,dc=com"),
	}
	if !cmp.Equal(mapping, want) {
		t.Errorf("Admin.UpdateUserLDAPMapping returned %+v, want %+v", mapping, want)
	}

	const methodName = "UpdateUserLDAPMapping"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Admin.UpdateUserLDAPMapping(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Admin.UpdateUserLDAPMapping(ctx, "u", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestAdminService_UpdateTeamLDAPMapping(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &TeamLDAPMapping{
		LDAPDN: Ptr("cn=Enterprise Ops,ou=teams,dc=github,dc=com"),
	}

	mux.HandleFunc("/admin/ldap/teams/1/mapping", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":1,"ldap_dn":"cn=Enterprise Ops,ou=teams,dc=github,dc=com"}`)
	})

	ctx := t.Context()
	mapping, _, err := client.Admin.UpdateTeamLDAPMapping(ctx, 1, input)
	if err != nil {
		t.Errorf("Admin.UpdateTeamLDAPMapping returned error: %v", err)
	}

	want := &TeamLDAPMapping{
		ID:     Ptr(int64(1)),
		LDAPDN: Ptr("cn=Enterprise Ops,ou=teams,dc=github,dc=com"),
	}
	if !cmp.Equal(mapping, want) {
		t.Errorf("Admin.UpdateTeamLDAPMapping returned %+v, want %+v", mapping, want)
	}

	const methodName = "UpdateTeamLDAPMapping"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Admin.UpdateTeamLDAPMapping(ctx, -1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Admin.UpdateTeamLDAPMapping(ctx, 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestAdminService_TeamLDAPMapping_String(t *testing.T) {
	t.Parallel()
	v := &TeamLDAPMapping{
		ID:              Ptr(int64(1)),
		LDAPDN:          Ptr("a"),
		URL:             Ptr("b"),
		Name:            Ptr("c"),
		Slug:            Ptr("d"),
		Description:     Ptr("e"),
		Privacy:         Ptr("f"),
		Permission:      Ptr("g"),
		MembersURL:      Ptr("h"),
		RepositoriesURL: Ptr("i"),
	}

	want := `github.TeamLDAPMapping{ID:1, LDAPDN:"a", URL:"b", Name:"c", Slug:"d", Description:"e", Privacy:"f", Permission:"g", MembersURL:"h", RepositoriesURL:"i"}`
	if got := v.String(); got != want {
		t.Errorf("TeamLDAPMapping.String = `%v`, want `%v`", got, want)
	}
}

func TestAdminService_UserLDAPMapping_String(t *testing.T) {
	t.Parallel()
	v := &UserLDAPMapping{
		ID:                Ptr(int64(1)),
		LDAPDN:            Ptr("a"),
		Login:             Ptr("b"),
		AvatarURL:         Ptr("c"),
		GravatarID:        Ptr("d"),
		Type:              Ptr("e"),
		SiteAdmin:         Ptr(true),
		URL:               Ptr("f"),
		EventsURL:         Ptr("g"),
		FollowingURL:      Ptr("h"),
		FollowersURL:      Ptr("i"),
		GistsURL:          Ptr("j"),
		OrganizationsURL:  Ptr("k"),
		ReceivedEventsURL: Ptr("l"),
		ReposURL:          Ptr("m"),
		StarredURL:        Ptr("n"),
		SubscriptionsURL:  Ptr("o"),
	}

	want := `github.UserLDAPMapping{ID:1, LDAPDN:"a", Login:"b", AvatarURL:"c", GravatarID:"d", Type:"e", SiteAdmin:true, URL:"f", EventsURL:"g", FollowingURL:"h", FollowersURL:"i", GistsURL:"j", OrganizationsURL:"k", ReceivedEventsURL:"l", ReposURL:"m", StarredURL:"n", SubscriptionsURL:"o"}`
	if got := v.String(); got != want {
		t.Errorf("UserLDAPMapping.String = `%v`, want `%v`", got, want)
	}
}
