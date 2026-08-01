// Copyright 2014 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestActivityService_ListNotification(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/notifications", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"all":           "true",
			"participating": "true",
			"since":         referenceTimeRaw,
			"before":        "2007-03-04T15:04:05Z",
		})

		fmt.Fprint(w, `[{"id":"1", "subject":{"title":"t"}}]`)
	})

	opt := &NotificationListOptions{
		All:           true,
		Participating: true,
		Since:         referenceTime,
		Before:        time.Date(2007, time.March, 4, 15, 4, 5, 0, time.UTC),
	}
	ctx := t.Context()
	notifications, _, err := client.Activity.ListNotifications(ctx, opt)
	if err != nil {
		t.Errorf("Activity.ListNotifications returned error: %v", err)
	}

	want := []*Notification{{ID: Ptr("1"), Subject: &NotificationSubject{Title: Ptr("t")}}}
	if !cmp.Equal(notifications, want) {
		t.Errorf("Activity.ListNotifications returned %+v, want %+v", notifications, want)
	}

	const methodName = "ListNotifications"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Activity.ListNotifications(ctx, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActivityService_ListRepositoryNotifications(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/notifications", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{"id":"1"}]`)
	})

	ctx := t.Context()
	notifications, _, err := client.Activity.ListRepositoryNotifications(ctx, "o", "r", nil)
	if err != nil {
		t.Errorf("Activity.ListRepositoryNotifications returned error: %v", err)
	}

	want := []*Notification{{ID: Ptr("1")}}
	if !cmp.Equal(notifications, want) {
		t.Errorf("Activity.ListRepositoryNotifications returned %+v, want %+v", notifications, want)
	}

	const methodName = "ListRepositoryNotifications"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Activity.ListRepositoryNotifications(ctx, "\n", "\n", &NotificationListOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Activity.ListRepositoryNotifications(ctx, "o", "r", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActivityService_MarkNotificationsRead(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/notifications", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testHeader(t, r, "Content-Type", "application/json")
		want := markReadOptions{
			LastReadAt: referenceTimestamp,
		}
		testJSONBody(t, r, want)

		w.WriteHeader(http.StatusResetContent)
	})

	ctx := t.Context()
	_, err := client.Activity.MarkNotificationsRead(ctx, referenceTimestamp)
	if err != nil {
		t.Errorf("Activity.MarkNotificationsRead returned error: %v", err)
	}

	const methodName = "MarkNotificationsRead"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Activity.MarkNotificationsRead(ctx, referenceTimestamp)
	})
}

func TestActivityService_MarkNotificationsRead_EmptyLastReadAt(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/notifications", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testHeader(t, r, "Content-Type", "application/json")
		want := markReadOptions{
			LastReadAt: referenceTimestamp,
		}
		testJSONBody(t, r, want)

		w.WriteHeader(http.StatusResetContent)
	})

	ctx := t.Context()
	_, err := client.Activity.MarkNotificationsRead(ctx, referenceTimestamp)
	if err != nil {
		t.Errorf("Activity.MarkNotificationsRead returned error: %v", err)
	}
}

func TestActivityService_MarkRepositoryNotificationsRead(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/notifications", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testHeader(t, r, "Content-Type", "application/json")
		want := markReadOptions{
			LastReadAt: referenceTimestamp,
		}
		testJSONBody(t, r, want)
		w.WriteHeader(http.StatusResetContent)
	})

	ctx := t.Context()
	_, err := client.Activity.MarkRepositoryNotificationsRead(ctx, "o", "r", referenceTimestamp)
	if err != nil {
		t.Errorf("Activity.MarkRepositoryNotificationsRead returned error: %v", err)
	}

	const methodName = "MarkRepositoryNotificationsRead"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Activity.MarkRepositoryNotificationsRead(ctx, "\n", "\n", referenceTimestamp)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Activity.MarkRepositoryNotificationsRead(ctx, "o", "r", referenceTimestamp)
	})
}

func TestActivityService_MarkRepositoryNotificationsRead_EmptyLastReadAt(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/notifications", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testHeader(t, r, "Content-Type", "application/json")
		want := markReadOptions{
			LastReadAt: referenceTimestamp,
		}
		testJSONBody(t, r, want)

		w.WriteHeader(http.StatusResetContent)
	})

	ctx := t.Context()
	_, err := client.Activity.MarkRepositoryNotificationsRead(ctx, "o", "r", referenceTimestamp)
	if err != nil {
		t.Errorf("Activity.MarkRepositoryNotificationsRead returned error: %v", err)
	}
}

func TestActivityService_GetThread(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/notifications/threads/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":"1"}`)
	})

	ctx := t.Context()
	notification, _, err := client.Activity.GetThread(ctx, "1")
	if err != nil {
		t.Errorf("Activity.GetThread returned error: %v", err)
	}

	want := &Notification{ID: Ptr("1")}
	if !cmp.Equal(notification, want) {
		t.Errorf("Activity.GetThread returned %+v, want %+v", notification, want)
	}

	const methodName = "GetThread"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Activity.GetThread(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Activity.GetThread(ctx, "1")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActivityService_MarkThreadRead(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/notifications/threads/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		w.WriteHeader(http.StatusResetContent)
	})

	ctx := t.Context()
	_, err := client.Activity.MarkThreadRead(ctx, "1")
	if err != nil {
		t.Errorf("Activity.MarkThreadRead returned error: %v", err)
	}

	const methodName = "MarkThreadRead"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Activity.MarkThreadRead(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Activity.MarkThreadRead(ctx, "1")
	})
}

func TestActivityService_MarkThreadDone(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/notifications/threads/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusResetContent)
	})

	ctx := t.Context()
	_, err := client.Activity.MarkThreadDone(ctx, "1")
	if err != nil {
		t.Errorf("Activity.MarkThreadDone returned error: %v", err)
	}

	const methodName = "MarkThreadDone"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Activity.MarkThreadDone(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Activity.MarkThreadDone(ctx, "1")
	})
}

func TestActivityService_GetThreadSubscription(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/notifications/threads/1/subscription", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"subscribed":true}`)
	})

	ctx := t.Context()
	sub, _, err := client.Activity.GetThreadSubscription(ctx, "1")
	if err != nil {
		t.Errorf("Activity.GetThreadSubscription returned error: %v", err)
	}

	want := &Subscription{Subscribed: Ptr(true)}
	if !cmp.Equal(sub, want) {
		t.Errorf("Activity.GetThreadSubscription returned %+v, want %+v", sub, want)
	}

	const methodName = "GetThreadSubscription"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Activity.GetThreadSubscription(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Activity.GetThreadSubscription(ctx, "1")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActivityService_SetThreadSubscription(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &Subscription{Subscribed: Ptr(true)}

	mux.HandleFunc("/notifications/threads/1/subscription", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"ignored":true}`)
	})

	ctx := t.Context()
	sub, _, err := client.Activity.SetThreadSubscription(ctx, "1", input)
	if err != nil {
		t.Errorf("Activity.SetThreadSubscription returned error: %v", err)
	}

	want := &Subscription{Ignored: Ptr(true)}
	if !cmp.Equal(sub, want) {
		t.Errorf("Activity.SetThreadSubscription returned %+v, want %+v", sub, want)
	}

	const methodName = "SetThreadSubscription"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Activity.SetThreadSubscription(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Activity.SetThreadSubscription(ctx, "1", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActivityService_DeleteThreadSubscription(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/notifications/threads/1/subscription", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Activity.DeleteThreadSubscription(ctx, "1")
	if err != nil {
		t.Errorf("Activity.DeleteThreadSubscription returned error: %v", err)
	}

	const methodName = "DeleteThreadSubscription"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Activity.DeleteThreadSubscription(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Activity.DeleteThreadSubscription(ctx, "1")
	})
}
