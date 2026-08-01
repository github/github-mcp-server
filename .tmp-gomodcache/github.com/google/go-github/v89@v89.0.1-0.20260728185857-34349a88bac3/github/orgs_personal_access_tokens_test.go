// Copyright 2023 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestOrganizationsService_ListFineGrainedPersonalAccessTokens(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/personal-access-tokens", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValuesList(t, r, url.Values{
			"per_page":  {"2"},
			"page":      {"2"},
			"sort":      {"created_at"},
			"direction": {"desc"},
			"owner[]":   {"octocat", "octodog", "otherbot"},
		})

		fmt.Fprint(w, `
		[
			{
				"id": 25381,
				"owner": {
					"login": "octocat",
					"id": 1,
					"node_id": "MDQ6VXNlcjE=",
					"avatar_url": "https://github.com/images/error/octocat_happy.gif",
					"gravatar_id": "",
					"url": "https://api.github.com/users/octocat",
					"html_url": "https://github.com/octocat",
					"followers_url": "https://api.github.com/users/octocat/followers",
					"following_url": "https://api.github.com/users/octocat/following{/other_user}",
					"gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
					"starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
					"subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
					"organizations_url": "https://api.github.com/users/octocat/orgs",
					"repos_url": "https://api.github.com/users/octocat/repos",
					"events_url": "https://api.github.com/users/octocat/events{/privacy}",
					"received_events_url": "https://api.github.com/users/octocat/received_events",
					"type": "User",
					"site_admin": false
				},
				"repository_selection": "all",
				"repositories_url": "https://api.github.com/organizations/652551/personal-access-tokens/25381/repositories",
				"permissions": {
					"organization": {
						"members": "read"
					},
					"repository": {
						"metadata": "read"
					}
				},
				"access_granted_at": `+refTimeStr(1136178000)+`,
				"token_expired": false,
				"token_expires_at": `+refTimeStr(1136178001)+`,
				"token_last_used_at": null
			}
		]`)
	})

	opts := &ListFineGrainedPATOptions{
		ListOptions: ListOptions{Page: 2, PerPage: 2},
		Sort:        "created_at",
		Direction:   "desc",
		Owner:       []string{"octocat", "octodog", "otherbot"},
	}
	ctx := t.Context()
	tokens, resp, err := client.Organizations.ListFineGrainedPersonalAccessTokens(ctx, "o", opts)
	if err != nil {
		t.Errorf("Organizations.ListFineGrainedPersonalAccessTokens returned error: %v", err)
	}

	want := []*PersonalAccessToken{
		{
			ID: Ptr(int64(25381)),
			Owner: &User{
				Login:             Ptr("octocat"),
				ID:                Ptr(int64(1)),
				NodeID:            Ptr("MDQ6VXNlcjE="),
				AvatarURL:         Ptr("https://github.com/images/error/octocat_happy.gif"),
				GravatarID:        Ptr(""),
				URL:               Ptr("https://api.github.com/users/octocat"),
				HTMLURL:           Ptr("https://github.com/octocat"),
				FollowersURL:      Ptr("https://api.github.com/users/octocat/followers"),
				FollowingURL:      Ptr("https://api.github.com/users/octocat/following{/other_user}"),
				GistsURL:          Ptr("https://api.github.com/users/octocat/gists{/gist_id}"),
				StarredURL:        Ptr("https://api.github.com/users/octocat/starred{/owner}{/repo}"),
				SubscriptionsURL:  Ptr("https://api.github.com/users/octocat/subscriptions"),
				OrganizationsURL:  Ptr("https://api.github.com/users/octocat/orgs"),
				ReposURL:          Ptr("https://api.github.com/users/octocat/repos"),
				EventsURL:         Ptr("https://api.github.com/users/octocat/events{/privacy}"),
				ReceivedEventsURL: Ptr("https://api.github.com/users/octocat/received_events"),
				Type:              Ptr("User"),
				SiteAdmin:         Ptr(false),
			},
			RepositorySelection: Ptr("all"),
			RepositoriesURL:     Ptr("https://api.github.com/organizations/652551/personal-access-tokens/25381/repositories"),
			Permissions: &PersonalAccessTokenPermissions{
				Org:  map[string]string{"members": "read"},
				Repo: map[string]string{"metadata": "read"},
			},
			AccessGrantedAt: refTimestamp(1136178000),
			TokenExpired:    Ptr(false),
			TokenExpiresAt:  refTimestamp(1136178001),
			TokenLastUsedAt: nil,
		},
	}
	if !cmp.Equal(tokens, want) {
		t.Errorf("Organizations.ListFineGrainedPersonalAccessTokens returned %+v, want %+v", tokens, want)
	}

	if resp == nil {
		t.Error("Organizations.ListFineGrainedPersonalAccessTokens returned nil response")
	}

	const methodName = "ListFineGrainedPersonalAccessTokens"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListFineGrainedPersonalAccessTokens(ctx, "o", nil)
		return err
	})
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListFineGrainedPersonalAccessTokens(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListFineGrainedPersonalAccessTokens(ctx, "o", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_ListFineGrainedPersonalAccessTokens_ownerOnly(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/personal-access-tokens", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"owner[]": "octocat"})

		fmt.Fprint(w, "[]")
	})

	opts := &ListFineGrainedPATOptions{Owner: []string{"octocat"}}
	ctx := t.Context()
	_, _, err := client.Organizations.ListFineGrainedPersonalAccessTokens(ctx, "o", opts)
	if err != nil {
		t.Errorf("Organizations.ListFineGrainedPersonalAccessTokens returned error: %v", err)
	}
}

func TestOrganizationsService_ListFineGrainedPersonalAccessTokenRequests(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/personal-access-token-requests", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValuesList(t, r, url.Values{
			"per_page":   {"2"},
			"page":       {"2"},
			"sort":       {"created_at"},
			"direction":  {"desc"},
			"owner[]":    {"octocat", "octodog"},
			"token_id[]": {"11579703", "11579704"},
		})

		fmt.Fprint(w, `[
			{
				"id": 1848980,
				"reason": null,
				"owner": {
					"login": "octocat",
					"id": 1,
					"node_id": "MDQ6VXNlcjE=",
					"avatar_url": "https://github.com/images/error/octocat_happy.gif",
					"gravatar_id": "",
					"url": "https://api.github.com/users/octocat",
					"html_url": "https://github.com/octocat",
					"followers_url": "https://api.github.com/users/octocat/followers",
					"following_url": "https://api.github.com/users/octocat/following{/other_user}",
					"gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
					"starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
					"subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
					"organizations_url": "https://api.github.com/users/octocat/orgs",
					"repos_url": "https://api.github.com/users/octocat/repos",
					"events_url": "https://api.github.com/users/octocat/events{/privacy}",
					"received_events_url": "https://api.github.com/users/octocat/received_events",
					"type": "User",
					"site_admin": false
				},
				"repository_selection": "all",
				"repositories_url": "https://api.github.com/organizations/135028681/personal-access-token-requests/1848980/repositories",
				"permissions": {
					"repository": {
						"metadata": "read"
					}
				},
				"created_at": `+refTimeStr(1136178000)+`,
				"token_id": 11579703,
				"token_name": "testFineGrained",
				"token_expired": false,
				"token_expires_at": `+refTimeStr(1136178001)+`,
				"token_last_used_at": null
			}
		]`)
	})

	opts := &ListFineGrainedPATOptions{
		ListOptions: ListOptions{Page: 2, PerPage: 2},
		Sort:        "created_at",
		Direction:   "desc",
		Owner:       []string{"octocat", "octodog"},
		TokenID:     []int64{11579703, 11579704},
	}
	ctx := t.Context()
	requests, resp, err := client.Organizations.ListFineGrainedPersonalAccessTokenRequests(ctx, "o", opts)
	if err != nil {
		t.Errorf("Organizations.ListFineGrainedPersonalAccessTokenRequests returned error: %v", err)
	}

	want := []*FineGrainedPersonalAccessTokenRequest{
		{
			ID:     1848980,
			Reason: "",
			Owner: User{
				Login:             Ptr("octocat"),
				ID:                Ptr(int64(1)),
				NodeID:            Ptr("MDQ6VXNlcjE="),
				AvatarURL:         Ptr("https://github.com/images/error/octocat_happy.gif"),
				GravatarID:        Ptr(""),
				URL:               Ptr("https://api.github.com/users/octocat"),
				HTMLURL:           Ptr("https://github.com/octocat"),
				FollowersURL:      Ptr("https://api.github.com/users/octocat/followers"),
				FollowingURL:      Ptr("https://api.github.com/users/octocat/following{/other_user}"),
				GistsURL:          Ptr("https://api.github.com/users/octocat/gists{/gist_id}"),
				StarredURL:        Ptr("https://api.github.com/users/octocat/starred{/owner}{/repo}"),
				SubscriptionsURL:  Ptr("https://api.github.com/users/octocat/subscriptions"),
				OrganizationsURL:  Ptr("https://api.github.com/users/octocat/orgs"),
				ReposURL:          Ptr("https://api.github.com/users/octocat/repos"),
				EventsURL:         Ptr("https://api.github.com/users/octocat/events{/privacy}"),
				ReceivedEventsURL: Ptr("https://api.github.com/users/octocat/received_events"),
				Type:              Ptr("User"),
				SiteAdmin:         Ptr(false),
			},
			RepositorySelection: "all",
			RepositoriesURL:     "https://api.github.com/organizations/135028681/personal-access-token-requests/1848980/repositories",
			Permissions: PersonalAccessTokenPermissions{
				Repo: map[string]string{"metadata": "read"},
			},
			CreatedAt:       refTimestamp(1136178000),
			TokenID:         11579703,
			TokenName:       "testFineGrained",
			TokenExpired:    false,
			TokenExpiresAt:  refTimestamp(1136178001),
			TokenLastUsedAt: nil,
		},
	}
	if !cmp.Equal(requests, want) {
		t.Errorf("Organizations.ListFineGrainedPersonalAccessTokenRequests returned %+v, want %+v", requests, want)
	}

	if resp == nil {
		t.Error("Organizations.ListFineGrainedPersonalAccessTokenRequests returned nil response")
	}

	const methodName = "ListFineGrainedPersonalAccessTokenRequests"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListFineGrainedPersonalAccessTokenRequests(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListFineGrainedPersonalAccessTokenRequests(ctx, "o", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_ListFineGrainedPersonalAccessTokenRequests_ownerOnly(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/personal-access-token-requests", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"owner[]": "octocat"})
		fmt.Fprint(w, "[]")
	})

	opts := &ListFineGrainedPATOptions{
		Owner: []string{"octocat"},
	}
	ctx := t.Context()
	_, _, err := client.Organizations.ListFineGrainedPersonalAccessTokenRequests(ctx, "o", opts)
	if err != nil {
		t.Errorf("Organizations.ListFineGrainedPersonalAccessTokenRequests returned error: %v", err)
	}
}

func TestOrganizationsService_ListFineGrainedPersonalAccessTokenRequests_tokenIDOnly(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/personal-access-token-requests", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"token_id[]": "11579703"})
		fmt.Fprint(w, "[]")
	})

	opts := &ListFineGrainedPATOptions{
		TokenID: []int64{11579703},
	}
	ctx := t.Context()
	_, _, err := client.Organizations.ListFineGrainedPersonalAccessTokenRequests(ctx, "o", opts)
	if err != nil {
		t.Errorf("Organizations.ListFineGrainedPersonalAccessTokenRequests returned error: %v", err)
	}
}

func TestOrganizationsService_ReviewPersonalAccessTokenRequest(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := ReviewPersonalAccessTokenRequestOptions{
		Action: "a",
		Reason: Ptr("r"),
	}

	mux.HandleFunc("/orgs/o/personal-access-token-requests/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	res, err := client.Organizations.ReviewPersonalAccessTokenRequest(ctx, "o", 1, input)
	if err != nil {
		t.Errorf("Organizations.ReviewPersonalAccessTokenRequest returned error: %v", err)
	}

	if res.StatusCode != http.StatusNoContent {
		t.Errorf("Organizations.ReviewPersonalAccessTokenRequest returned %v, want %v", res.StatusCode, http.StatusNoContent)
	}

	const methodName = "ReviewPersonalAccessTokenRequest"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.ReviewPersonalAccessTokenRequest(ctx, "\n", 0, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.ReviewPersonalAccessTokenRequest(ctx, "o", 1, input)
	})
}
