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

func TestUsersService_ListFollowers_authenticatedUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/followers", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"id":1}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	users, _, err := client.Users.ListFollowers(ctx, "", opt)
	if err != nil {
		t.Errorf("Users.ListFollowers returned error: %v", err)
	}

	want := []*User{{ID: Ptr(int64(1))}}
	if !cmp.Equal(users, want) {
		t.Errorf("Users.ListFollowers returned %+v, want %+v", users, want)
	}

	const methodName = "ListFollowers"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Users.ListFollowers(ctx, "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.ListFollowers(ctx, "", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_ListFollowers_specifiedUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/followers", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{"id":1}]`)
	})

	ctx := t.Context()
	users, _, err := client.Users.ListFollowers(ctx, "u", nil)
	if err != nil {
		t.Errorf("Users.ListFollowers returned error: %v", err)
	}

	want := []*User{{ID: Ptr(int64(1))}}
	if !cmp.Equal(users, want) {
		t.Errorf("Users.ListFollowers returned %+v, want %+v", users, want)
	}

	const methodName = "ListFollowers"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Users.ListFollowers(ctx, "\n", nil)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.ListFollowers(ctx, "u", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_ListFollowers_invalidUser(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Users.ListFollowers(ctx, "%", nil)
	testURLParseError(t, err)
}

func TestUsersService_ListFollowing_authenticatedUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/following", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"id":1}]`)
	})

	opts := &ListOptions{Page: 2}
	ctx := t.Context()
	users, _, err := client.Users.ListFollowing(ctx, "", opts)
	if err != nil {
		t.Errorf("Users.ListFollowing returned error: %v", err)
	}

	want := []*User{{ID: Ptr(int64(1))}}
	if !cmp.Equal(users, want) {
		t.Errorf("Users.ListFollowing returned %+v, want %+v", users, want)
	}

	const methodName = "ListFollowing"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Users.ListFollowing(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.ListFollowing(ctx, "", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_ListFollowing_specifiedUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/following", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{"id":1}]`)
	})

	ctx := t.Context()
	users, _, err := client.Users.ListFollowing(ctx, "u", nil)
	if err != nil {
		t.Errorf("Users.ListFollowing returned error: %v", err)
	}

	want := []*User{{ID: Ptr(int64(1))}}
	if !cmp.Equal(users, want) {
		t.Errorf("Users.ListFollowing returned %+v, want %+v", users, want)
	}

	const methodName = "ListFollowing"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Users.ListFollowing(ctx, "\n", nil)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.ListFollowing(ctx, "u", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_ListFollowing_invalidUser(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Users.ListFollowing(ctx, "%", nil)
	testURLParseError(t, err)
}

func TestUsersService_IsFollowing_authenticatedUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/following/t", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	following, _, err := client.Users.IsFollowing(ctx, "", "t")
	if err != nil {
		t.Errorf("Users.IsFollowing returned error: %v", err)
	}
	if want := true; following != want {
		t.Errorf("Users.IsFollowing returned %+v, want %+v", following, want)
	}

	const methodName = "IsFollowing"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Users.IsFollowing(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.IsFollowing(ctx, "", "t")
		if got {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want false", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_IsFollowing_specifiedUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/following/t", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	following, _, err := client.Users.IsFollowing(ctx, "u", "t")
	if err != nil {
		t.Errorf("Users.IsFollowing returned error: %v", err)
	}
	if want := true; following != want {
		t.Errorf("Users.IsFollowing returned %+v, want %+v", following, want)
	}

	const methodName = "IsFollowing"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Users.IsFollowing(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.IsFollowing(ctx, "u", "t")
		if got {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want false", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_IsFollowing_false(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/following/t", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNotFound)
	})

	ctx := t.Context()
	following, _, err := client.Users.IsFollowing(ctx, "u", "t")
	if err != nil {
		t.Errorf("Users.IsFollowing returned error: %v", err)
	}
	if want := false; following != want {
		t.Errorf("Users.IsFollowing returned %+v, want %+v", following, want)
	}

	const methodName = "IsFollowing"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Users.IsFollowing(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.IsFollowing(ctx, "u", "t")
		if got {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want false", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_IsFollowing_error(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/following/t", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		http.Error(w, "BadRequest", http.StatusBadRequest)
	})

	ctx := t.Context()
	following, _, err := client.Users.IsFollowing(ctx, "u", "t")
	if err == nil {
		t.Error("Expected HTTP 400 response")
	}
	if want := false; following != want {
		t.Errorf("Users.IsFollowing returned %+v, want %+v", following, want)
	}

	const methodName = "IsFollowing"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Users.IsFollowing(ctx, "u", "t")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.IsFollowing(ctx, "u", "t")
		if got {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want false", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_IsFollowing_invalidUser(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Users.IsFollowing(ctx, "%", "%")
	testURLParseError(t, err)
}

func TestUsersService_Follow(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/following/u", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
	})

	ctx := t.Context()
	_, err := client.Users.Follow(ctx, "u")
	if err != nil {
		t.Errorf("Users.Follow returned error: %v", err)
	}

	const methodName = "Follow"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Users.Follow(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Users.Follow(ctx, "u")
	})
}

func TestUsersService_Follow_invalidUser(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Users.Follow(ctx, "%")
	testURLParseError(t, err)
}

func TestUsersService_Unfollow(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/following/u", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Users.Unfollow(ctx, "u")
	if err != nil {
		t.Errorf("Users.Unfollow returned error: %v", err)
	}

	const methodName = "Unfollow"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Users.Unfollow(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Users.Unfollow(ctx, "u")
	})
}

func TestUsersService_Unfollow_invalidUser(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Users.Unfollow(ctx, "%")
	testURLParseError(t, err)
}
