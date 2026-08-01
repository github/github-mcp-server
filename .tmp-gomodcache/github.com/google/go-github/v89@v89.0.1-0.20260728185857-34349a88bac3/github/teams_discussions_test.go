// Copyright 2018 The go-github AUTHORS. All rights reserved.
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

func TestTeamsService_ListDiscussionsByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/organizations/1/team/2/discussions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"direction": "desc",
			"page":      "2",
		})
		fmt.Fprint(w,
			`[
				{
					"author": {
						"login": "author",
						"id": 0,
						"avatar_url": "https://avatars1.githubusercontent.com/u/0?v=4",
						"gravatar_id": "",
						"url": "https://api.github.com/users/author",
						"html_url": "https://github.com/author",
						"followers_url": "https://api.github.com/users/author/followers",
						"following_url": "https://api.github.com/users/author/following{/other_user}",
						"gists_url": "https://api.github.com/users/author/gists{/gist_id}",
						"starred_url": "https://api.github.com/users/author/starred{/owner}{/repo}",
						"subscriptions_url": "https://api.github.com/users/author/subscriptions",
						"organizations_url": "https://api.github.com/users/author/orgs",
						"repos_url": "https://api.github.com/users/author/repos",
						"events_url": "https://api.github.com/users/author/events{/privacy}",
						"received_events_url": "https://api.github.com/users/author/received_events",
						"type": "User",
						"site_admin": false
					},
					"body": "test",
					"body_html": "<p>test</p>",
					"body_version": "version",
					"comments_count": 1,
					"comments_url": "https://api.github.com/teams/2/discussions/3/comments",
					"created_at": `+refTimeStr(1136178000)+`,
					"last_edited_at": null,
					"html_url": "https://github.com/orgs/1/teams/2/discussions/3",
					"node_id": "node",
					"number": 3,
					"pinned": false,
					"private": false,
					"team_url": "https://api.github.com/teams/2",
					"title": "test",
					"updated_at": `+refTimeStr(1136178001)+`,
					"url": "https://api.github.com/teams/2/discussions/3"
				}
			]`)
	})
	ctx := t.Context()
	discussions, _, err := client.Teams.ListDiscussionsByID(ctx, 1, 2, &DiscussionListOptions{"desc", ListOptions{Page: 2}})
	if err != nil {
		t.Errorf("Teams.ListDiscussionsByID returned error: %v", err)
	}

	want := []*TeamDiscussion{
		{
			Author: &User{
				Login:             Ptr("author"),
				ID:                Ptr(int64(0)),
				AvatarURL:         Ptr("https://avatars1.githubusercontent.com/u/0?v=4"),
				GravatarID:        Ptr(""),
				URL:               Ptr("https://api.github.com/users/author"),
				HTMLURL:           Ptr("https://github.com/author"),
				FollowersURL:      Ptr("https://api.github.com/users/author/followers"),
				FollowingURL:      Ptr("https://api.github.com/users/author/following{/other_user}"),
				GistsURL:          Ptr("https://api.github.com/users/author/gists{/gist_id}"),
				StarredURL:        Ptr("https://api.github.com/users/author/starred{/owner}{/repo}"),
				SubscriptionsURL:  Ptr("https://api.github.com/users/author/subscriptions"),
				OrganizationsURL:  Ptr("https://api.github.com/users/author/orgs"),
				ReposURL:          Ptr("https://api.github.com/users/author/repos"),
				EventsURL:         Ptr("https://api.github.com/users/author/events{/privacy}"),
				ReceivedEventsURL: Ptr("https://api.github.com/users/author/received_events"),
				Type:              Ptr("User"),
				SiteAdmin:         Ptr(false),
			},
			Body:          Ptr("test"),
			BodyHTML:      Ptr("<p>test</p>"),
			BodyVersion:   Ptr("version"),
			CommentsCount: Ptr(1),
			CommentsURL:   Ptr("https://api.github.com/teams/2/discussions/3/comments"),
			CreatedAt:     refTimestamp(1136178000),
			LastEditedAt:  nil,
			HTMLURL:       Ptr("https://github.com/orgs/1/teams/2/discussions/3"),
			NodeID:        Ptr("node"),
			Number:        Ptr(3),
			Pinned:        Ptr(false),
			Private:       Ptr(false),
			TeamURL:       Ptr("https://api.github.com/teams/2"),
			Title:         Ptr("test"),
			UpdatedAt:     refTimestamp(1136178001),
			URL:           Ptr("https://api.github.com/teams/2/discussions/3"),
		},
	}
	if !cmp.Equal(discussions, want) {
		t.Errorf("Teams.ListDiscussionsByID returned %+v, want %+v", discussions, want)
	}

	const methodName = "ListDiscussionsByID"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.ListDiscussionsByID(ctx, -1, -2, nil)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.ListDiscussionsByID(ctx, 1, 2, &DiscussionListOptions{"desc", ListOptions{Page: 2}})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_ListDiscussionsBySlug(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/teams/s/discussions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"direction": "desc",
			"page":      "2",
		})
		fmt.Fprint(w,
			`[
				{
					"author": {
						"login": "author",
						"id": 0,
						"avatar_url": "https://avatars1.githubusercontent.com/u/0?v=4",
						"gravatar_id": "",
						"url": "https://api.github.com/users/author",
						"html_url": "https://github.com/author",
						"followers_url": "https://api.github.com/users/author/followers",
						"following_url": "https://api.github.com/users/author/following{/other_user}",
						"gists_url": "https://api.github.com/users/author/gists{/gist_id}",
						"starred_url": "https://api.github.com/users/author/starred{/owner}{/repo}",
						"subscriptions_url": "https://api.github.com/users/author/subscriptions",
						"organizations_url": "https://api.github.com/users/author/orgs",
						"repos_url": "https://api.github.com/users/author/repos",
						"events_url": "https://api.github.com/users/author/events{/privacy}",
						"received_events_url": "https://api.github.com/users/author/received_events",
						"type": "User",
						"site_admin": false
					},
					"body": "test",
					"body_html": "<p>test</p>",
					"body_version": "version",
					"comments_count": 1,
					"comments_url": "https://api.github.com/teams/2/discussions/3/comments",
					"created_at": `+refTimeStr(1136178000)+`,
					"last_edited_at": null,
					"html_url": "https://github.com/orgs/1/teams/2/discussions/3",
					"node_id": "node",
					"number": 3,
					"pinned": false,
					"private": false,
					"team_url": "https://api.github.com/teams/2",
					"title": "test",
					"updated_at": `+refTimeStr(1136178001)+`,
					"url": "https://api.github.com/teams/2/discussions/3"
				}
			]`)
	})
	ctx := t.Context()
	discussions, _, err := client.Teams.ListDiscussionsBySlug(ctx, "o", "s", &DiscussionListOptions{"desc", ListOptions{Page: 2}})
	if err != nil {
		t.Errorf("Teams.ListDiscussionsBySlug returned error: %v", err)
	}

	want := []*TeamDiscussion{
		{
			Author: &User{
				Login:             Ptr("author"),
				ID:                Ptr(int64(0)),
				AvatarURL:         Ptr("https://avatars1.githubusercontent.com/u/0?v=4"),
				GravatarID:        Ptr(""),
				URL:               Ptr("https://api.github.com/users/author"),
				HTMLURL:           Ptr("https://github.com/author"),
				FollowersURL:      Ptr("https://api.github.com/users/author/followers"),
				FollowingURL:      Ptr("https://api.github.com/users/author/following{/other_user}"),
				GistsURL:          Ptr("https://api.github.com/users/author/gists{/gist_id}"),
				StarredURL:        Ptr("https://api.github.com/users/author/starred{/owner}{/repo}"),
				SubscriptionsURL:  Ptr("https://api.github.com/users/author/subscriptions"),
				OrganizationsURL:  Ptr("https://api.github.com/users/author/orgs"),
				ReposURL:          Ptr("https://api.github.com/users/author/repos"),
				EventsURL:         Ptr("https://api.github.com/users/author/events{/privacy}"),
				ReceivedEventsURL: Ptr("https://api.github.com/users/author/received_events"),
				Type:              Ptr("User"),
				SiteAdmin:         Ptr(false),
			},
			Body:          Ptr("test"),
			BodyHTML:      Ptr("<p>test</p>"),
			BodyVersion:   Ptr("version"),
			CommentsCount: Ptr(1),
			CommentsURL:   Ptr("https://api.github.com/teams/2/discussions/3/comments"),
			CreatedAt:     refTimestamp(1136178000),
			LastEditedAt:  nil,
			HTMLURL:       Ptr("https://github.com/orgs/1/teams/2/discussions/3"),
			NodeID:        Ptr("node"),
			Number:        Ptr(3),
			Pinned:        Ptr(false),
			Private:       Ptr(false),
			TeamURL:       Ptr("https://api.github.com/teams/2"),
			Title:         Ptr("test"),
			UpdatedAt:     refTimestamp(1136178001),
			URL:           Ptr("https://api.github.com/teams/2/discussions/3"),
		},
	}
	if !cmp.Equal(discussions, want) {
		t.Errorf("Teams.ListDiscussionsBySlug returned %+v, want %+v", discussions, want)
	}

	const methodName = "ListDiscussionsBySlug"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.ListDiscussionsBySlug(ctx, "o\no", "s\ns", nil)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.ListDiscussionsBySlug(ctx, "o", "s", &DiscussionListOptions{"desc", ListOptions{Page: 2}})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_GetDiscussionByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/organizations/1/team/2/discussions/3", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"number":3}`)
	})

	ctx := t.Context()
	discussion, _, err := client.Teams.GetDiscussionByID(ctx, 1, 2, 3)
	if err != nil {
		t.Errorf("Teams.GetDiscussionByID returned error: %v", err)
	}

	want := &TeamDiscussion{Number: Ptr(3)}
	if !cmp.Equal(discussion, want) {
		t.Errorf("Teams.GetDiscussionByID returned %+v, want %+v", discussion, want)
	}

	const methodName = "GetDiscussionByID"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.GetDiscussionByID(ctx, -1, -2, -3)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.GetDiscussionByID(ctx, 1, 2, 3)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_GetDiscussionBySlug(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/teams/s/discussions/3", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"number":3}`)
	})

	ctx := t.Context()
	discussion, _, err := client.Teams.GetDiscussionBySlug(ctx, "o", "s", 3)
	if err != nil {
		t.Errorf("Teams.GetDiscussionBySlug returned error: %v", err)
	}

	want := &TeamDiscussion{Number: Ptr(3)}
	if !cmp.Equal(discussion, want) {
		t.Errorf("Teams.GetDiscussionBySlug returned %+v, want %+v", discussion, want)
	}

	const methodName = "GetDiscussionBySlug"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.GetDiscussionBySlug(ctx, "o\no", "s\ns", -3)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.GetDiscussionBySlug(ctx, "o", "s", 3)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_CreateDiscussionByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := TeamDiscussion{Title: Ptr("c_t"), Body: Ptr("c_b")}

	mux.HandleFunc("/organizations/1/team/2/discussions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"number":3}`)
	})

	ctx := t.Context()
	comment, _, err := client.Teams.CreateDiscussionByID(ctx, 1, 2, input)
	if err != nil {
		t.Errorf("Teams.CreateDiscussionByID returned error: %v", err)
	}

	want := &TeamDiscussion{Number: Ptr(3)}
	if !cmp.Equal(comment, want) {
		t.Errorf("Teams.CreateDiscussionByID returned %+v, want %+v", comment, want)
	}

	const methodName = "CreateDiscussionByID"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.CreateDiscussionByID(ctx, -1, -2, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.CreateDiscussionByID(ctx, 1, 2, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_CreateDiscussionBySlug(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := TeamDiscussion{Title: Ptr("c_t"), Body: Ptr("c_b")}

	mux.HandleFunc("/orgs/o/teams/s/discussions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"number":3}`)
	})

	ctx := t.Context()
	comment, _, err := client.Teams.CreateDiscussionBySlug(ctx, "o", "s", input)
	if err != nil {
		t.Errorf("Teams.CreateDiscussionBySlug returned error: %v", err)
	}

	want := &TeamDiscussion{Number: Ptr(3)}
	if !cmp.Equal(comment, want) {
		t.Errorf("Teams.CreateDiscussionBySlug returned %+v, want %+v", comment, want)
	}

	const methodName = "CreateDiscussionBySlug"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.CreateDiscussionBySlug(ctx, "o\no", "s\ns", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.CreateDiscussionBySlug(ctx, "o", "s", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_EditDiscussionByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := TeamDiscussion{Title: Ptr("e_t"), Body: Ptr("e_b")}

	mux.HandleFunc("/organizations/1/team/2/discussions/3", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"number":3}`)
	})

	ctx := t.Context()
	comment, _, err := client.Teams.EditDiscussionByID(ctx, 1, 2, 3, input)
	if err != nil {
		t.Errorf("Teams.EditDiscussionByID returned error: %v", err)
	}

	want := &TeamDiscussion{Number: Ptr(3)}
	if !cmp.Equal(comment, want) {
		t.Errorf("Teams.EditDiscussionByID returned %+v, want %+v", comment, want)
	}

	const methodName = "EditDiscussionByID"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.EditDiscussionByID(ctx, -1, -2, -3, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.EditDiscussionByID(ctx, 1, 2, 3, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_EditDiscussionBySlug(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := TeamDiscussion{Title: Ptr("e_t"), Body: Ptr("e_b")}

	mux.HandleFunc("/orgs/o/teams/s/discussions/3", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"number":3}`)
	})

	ctx := t.Context()
	comment, _, err := client.Teams.EditDiscussionBySlug(ctx, "o", "s", 3, input)
	if err != nil {
		t.Errorf("Teams.EditDiscussionBySlug returned error: %v", err)
	}

	want := &TeamDiscussion{Number: Ptr(3)}
	if !cmp.Equal(comment, want) {
		t.Errorf("Teams.EditDiscussionBySlug returned %+v, want %+v", comment, want)
	}

	const methodName = "EditDiscussionBySlug"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.EditDiscussionBySlug(ctx, "o\no", "s\ns", -3, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.EditDiscussionBySlug(ctx, "o", "s", 3, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_DeleteDiscussionByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/organizations/1/team/2/discussions/3", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Teams.DeleteDiscussionByID(ctx, 1, 2, 3)
	if err != nil {
		t.Errorf("Teams.DeleteDiscussionByID returned error: %v", err)
	}

	const methodName = "DeleteDiscussionByID"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Teams.DeleteDiscussionByID(ctx, -1, -2, -3)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Teams.DeleteDiscussionByID(ctx, 1, 2, 3)
	})
}

func TestTeamsService_DeleteDiscussionBySlug(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/teams/s/discussions/3", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Teams.DeleteDiscussionBySlug(ctx, "o", "s", 3)
	if err != nil {
		t.Errorf("Teams.DeleteDiscussionBySlug returned error: %v", err)
	}

	const methodName = "DeleteDiscussionBySlug"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Teams.DeleteDiscussionBySlug(ctx, "o\no", "s\ns", -3)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Teams.DeleteDiscussionBySlug(ctx, "o", "s", 3)
	})
}
