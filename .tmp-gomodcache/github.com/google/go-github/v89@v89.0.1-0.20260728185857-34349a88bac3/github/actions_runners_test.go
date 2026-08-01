// Copyright 2020 The go-github AUTHORS. All rights reserved.
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

func TestActionsService_ListRunnerApplicationDownloads(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/runners/downloads", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{"os":"osx","architecture":"x64","download_url":"https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-osx-x64-2.164.0.tar.gz","filename":"actions-runner-osx-x64-2.164.0.tar.gz"},{"os":"linux","architecture":"x64","download_url":"https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-linux-x64-2.164.0.tar.gz","filename":"actions-runner-linux-x64-2.164.0.tar.gz"},{"os": "linux","architecture":"arm","download_url":"https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-linux-arm-2.164.0.tar.gz","filename":"actions-runner-linux-arm-2.164.0.tar.gz"},{"os":"win","architecture":"x64","download_url":"https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-win-x64-2.164.0.zip","filename":"actions-runner-win-x64-2.164.0.zip"},{"os":"linux","architecture":"arm64","download_url":"https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-linux-arm64-2.164.0.tar.gz","filename":"actions-runner-linux-arm64-2.164.0.tar.gz"}]`)
	})

	ctx := t.Context()
	downloads, _, err := client.Actions.ListRunnerApplicationDownloads(ctx, "o", "r")
	if err != nil {
		t.Errorf("Actions.ListRunnerApplicationDownloads returned error: %v", err)
	}

	want := []*RunnerApplicationDownload{
		{OS: Ptr("osx"), Architecture: Ptr("x64"), DownloadURL: Ptr("https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-osx-x64-2.164.0.tar.gz"), Filename: Ptr("actions-runner-osx-x64-2.164.0.tar.gz")},
		{OS: Ptr("linux"), Architecture: Ptr("x64"), DownloadURL: Ptr("https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-linux-x64-2.164.0.tar.gz"), Filename: Ptr("actions-runner-linux-x64-2.164.0.tar.gz")},
		{OS: Ptr("linux"), Architecture: Ptr("arm"), DownloadURL: Ptr("https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-linux-arm-2.164.0.tar.gz"), Filename: Ptr("actions-runner-linux-arm-2.164.0.tar.gz")},
		{OS: Ptr("win"), Architecture: Ptr("x64"), DownloadURL: Ptr("https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-win-x64-2.164.0.zip"), Filename: Ptr("actions-runner-win-x64-2.164.0.zip")},
		{OS: Ptr("linux"), Architecture: Ptr("arm64"), DownloadURL: Ptr("https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-linux-arm64-2.164.0.tar.gz"), Filename: Ptr("actions-runner-linux-arm64-2.164.0.tar.gz")},
	}
	if !cmp.Equal(downloads, want) {
		t.Errorf("Actions.ListRunnerApplicationDownloads returned %+v, want %+v", downloads, want)
	}

	const methodName = "ListRunnerApplicationDownloads"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.ListRunnerApplicationDownloads(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.ListRunnerApplicationDownloads(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_CreateOrgJITConfig(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := CreateJITConfigRequest{Name: "test", RunnerGroupID: 1, Labels: []string{"one", "two"}}

	mux.HandleFunc("/orgs/o/actions/runners/generate-jitconfig", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"encoded_jit_config":"foo"}`)
	})

	ctx := t.Context()
	jitConfig, _, err := client.Actions.CreateOrgJITConfig(ctx, "o", input)
	if err != nil {
		t.Errorf("Actions.CreateOrgJITConfig returned error: %v", err)
	}

	want := &JITRunnerConfig{EncodedJITConfig: Ptr("foo")}
	if !cmp.Equal(jitConfig, want) {
		t.Errorf("Actions.CreateOrgJITConfig returned %+v, want %+v", jitConfig, want)
	}

	const methodName = "CreateOrgJITConfig"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.CreateOrgJITConfig(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.CreateOrgJITConfig(ctx, "o", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_CreateRepoJITConfig(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := CreateJITConfigRequest{Name: "test", RunnerGroupID: 1, Labels: []string{"one", "two"}}

	mux.HandleFunc("/repos/o/r/actions/runners/generate-jitconfig", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"encoded_jit_config":"foo"}`)
	})

	ctx := t.Context()
	jitConfig, _, err := client.Actions.CreateRepoJITConfig(ctx, "o", "r", input)
	if err != nil {
		t.Errorf("Actions.CreateRepoJITConfig returned error: %v", err)
	}

	want := &JITRunnerConfig{EncodedJITConfig: Ptr("foo")}
	if !cmp.Equal(jitConfig, want) {
		t.Errorf("Actions.CreateRepoJITConfig returned %+v, want %+v", jitConfig, want)
	}

	const methodName = "CreateRepoJITConfig"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.CreateRepoJITConfig(ctx, "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.CreateRepoJITConfig(ctx, "o", "r", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_CreateRegistrationToken(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/runners/registration-token", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{"token":"LLBF3JGZDX3P5PMEXLND6TS6FCWO6","expires_at":`+referenceTimeStr+`}`)
	})

	ctx := t.Context()
	token, _, err := client.Actions.CreateRegistrationToken(ctx, "o", "r")
	if err != nil {
		t.Errorf("Actions.CreateRegistrationToken returned error: %v", err)
	}

	want := &RegistrationToken{
		Token:     Ptr("LLBF3JGZDX3P5PMEXLND6TS6FCWO6"),
		ExpiresAt: &referenceTimestamp,
	}
	if !cmp.Equal(token, want) {
		t.Errorf("Actions.CreateRegistrationToken returned %+v, want %+v", token, want)
	}

	const methodName = "CreateRegistrationToken"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.CreateRegistrationToken(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.CreateRegistrationToken(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_ListRunners(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/runners", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"name": "MBP", "per_page": "2", "page": "2"})
		fmt.Fprint(w, `{"total_count":1,"runners":[{"id":23,"name":"MBP","os":"macos","status":"online"}]}`)
	})

	opts := &ListRunnersOptions{
		Name:        Ptr("MBP"),
		ListOptions: ListOptions{Page: 2, PerPage: 2},
	}
	ctx := t.Context()
	runners, _, err := client.Actions.ListRunners(ctx, "o", "r", opts)
	if err != nil {
		t.Errorf("Actions.ListRunners returned error: %v", err)
	}

	want := &Runners{
		TotalCount: 1,
		Runners: []*Runner{
			{ID: Ptr(int64(23)), Name: Ptr("MBP"), OS: Ptr("macos"), Status: Ptr("online")},
		},
	}
	if !cmp.Equal(runners, want) {
		t.Errorf("Actions.ListRunners returned %+v, want %+v", runners, want)
	}

	const methodName = "ListRunners"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.ListRunners(ctx, "\n", "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.ListRunners(ctx, "o", "r", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_GetRunner(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/runners/23", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":23,"name":"MBP","os":"macos","status":"online"}`)
	})

	ctx := t.Context()
	runner, _, err := client.Actions.GetRunner(ctx, "o", "r", 23)
	if err != nil {
		t.Errorf("Actions.GetRunner returned error: %v", err)
	}

	want := &Runner{
		ID:     Ptr(int64(23)),
		Name:   Ptr("MBP"),
		OS:     Ptr("macos"),
		Status: Ptr("online"),
	}
	if !cmp.Equal(runner, want) {
		t.Errorf("Actions.GetRunner returned %+v, want %+v", runner, want)
	}

	const methodName = "GetRunner"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetRunner(ctx, "\n", "\n", 23)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetRunner(ctx, "o", "r", 23)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_CreateRemoveToken(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/runners/remove-token", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{"token":"AABF3JGZDX3P5PMEXLND6TS6FCWO6","expires_at":`+referenceTimeStr+`}`)
	})

	ctx := t.Context()
	token, _, err := client.Actions.CreateRemoveToken(ctx, "o", "r")
	if err != nil {
		t.Errorf("Actions.CreateRemoveToken returned error: %v", err)
	}

	want := &RemoveToken{Token: Ptr("AABF3JGZDX3P5PMEXLND6TS6FCWO6"), ExpiresAt: &referenceTimestamp}
	if !cmp.Equal(token, want) {
		t.Errorf("Actions.CreateRemoveToken returned %+v, want %+v", token, want)
	}

	const methodName = "CreateRemoveToken"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.CreateRemoveToken(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.CreateRemoveToken(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_RemoveRunner(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/runners/21", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Actions.RemoveRunner(ctx, "o", "r", 21)
	if err != nil {
		t.Errorf("Actions.RemoveRunner returned error: %v", err)
	}

	const methodName = "RemoveRunner"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.RemoveRunner(ctx, "\n", "\n", 21)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.RemoveRunner(ctx, "o", "r", 21)
	})
}

func TestActionsService_ListOrganizationRunnerApplicationDownloads(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/runners/downloads", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{"os":"osx","architecture":"x64","download_url":"https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-osx-x64-2.164.0.tar.gz","filename":"actions-runner-osx-x64-2.164.0.tar.gz"},{"os":"linux","architecture":"x64","download_url":"https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-linux-x64-2.164.0.tar.gz","filename":"actions-runner-linux-x64-2.164.0.tar.gz"},{"os": "linux","architecture":"arm","download_url":"https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-linux-arm-2.164.0.tar.gz","filename":"actions-runner-linux-arm-2.164.0.tar.gz"},{"os":"win","architecture":"x64","download_url":"https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-win-x64-2.164.0.zip","filename":"actions-runner-win-x64-2.164.0.zip"},{"os":"linux","architecture":"arm64","download_url":"https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-linux-arm64-2.164.0.tar.gz","filename":"actions-runner-linux-arm64-2.164.0.tar.gz"}]`)
	})

	ctx := t.Context()
	downloads, _, err := client.Actions.ListOrganizationRunnerApplicationDownloads(ctx, "o")
	if err != nil {
		t.Errorf("Actions.ListOrganizationRunnerApplicationDownloads returned error: %v", err)
	}

	want := []*RunnerApplicationDownload{
		{OS: Ptr("osx"), Architecture: Ptr("x64"), DownloadURL: Ptr("https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-osx-x64-2.164.0.tar.gz"), Filename: Ptr("actions-runner-osx-x64-2.164.0.tar.gz")},
		{OS: Ptr("linux"), Architecture: Ptr("x64"), DownloadURL: Ptr("https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-linux-x64-2.164.0.tar.gz"), Filename: Ptr("actions-runner-linux-x64-2.164.0.tar.gz")},
		{OS: Ptr("linux"), Architecture: Ptr("arm"), DownloadURL: Ptr("https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-linux-arm-2.164.0.tar.gz"), Filename: Ptr("actions-runner-linux-arm-2.164.0.tar.gz")},
		{OS: Ptr("win"), Architecture: Ptr("x64"), DownloadURL: Ptr("https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-win-x64-2.164.0.zip"), Filename: Ptr("actions-runner-win-x64-2.164.0.zip")},
		{OS: Ptr("linux"), Architecture: Ptr("arm64"), DownloadURL: Ptr("https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-linux-arm64-2.164.0.tar.gz"), Filename: Ptr("actions-runner-linux-arm64-2.164.0.tar.gz")},
	}
	if !cmp.Equal(downloads, want) {
		t.Errorf("Actions.ListOrganizationRunnerApplicationDownloads returned %+v, want %+v", downloads, want)
	}

	const methodName = "ListOrganizationRunnerApplicationDownloads"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.ListOrganizationRunnerApplicationDownloads(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.ListOrganizationRunnerApplicationDownloads(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_CreateOrganizationRegistrationToken(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/runners/registration-token", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{"token":"LLBF3JGZDX3P5PMEXLND6TS6FCWO6","expires_at":`+referenceTimeStr+`}`)
	})

	ctx := t.Context()
	token, _, err := client.Actions.CreateOrganizationRegistrationToken(ctx, "o")
	if err != nil {
		t.Errorf("Actions.CreateOrganizationRegistrationToken returned error: %v", err)
	}

	want := &RegistrationToken{
		Token:     Ptr("LLBF3JGZDX3P5PMEXLND6TS6FCWO6"),
		ExpiresAt: &referenceTimestamp,
	}
	if !cmp.Equal(token, want) {
		t.Errorf("Actions.CreateOrganizationRegistrationToken returned %+v, want %+v", token, want)
	}

	const methodName = "CreateOrganizationRegistrationToken"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.CreateOrganizationRegistrationToken(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.CreateOrganizationRegistrationToken(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_ListOrganizationRunners(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/runners", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"per_page": "2", "page": "2"})
		fmt.Fprint(w, `{"total_count":2,"runners":[{"id":23,"name":"MBP","os":"macos","status":"online"},{"id":24,"name":"iMac","os":"macos","status":"offline"}]}`)
	})

	opts := &ListRunnersOptions{
		ListOptions: ListOptions{Page: 2, PerPage: 2},
	}
	ctx := t.Context()
	runners, _, err := client.Actions.ListOrganizationRunners(ctx, "o", opts)
	if err != nil {
		t.Errorf("Actions.ListOrganizationRunners returned error: %v", err)
	}

	want := &Runners{
		TotalCount: 2,
		Runners: []*Runner{
			{ID: Ptr(int64(23)), Name: Ptr("MBP"), OS: Ptr("macos"), Status: Ptr("online")},
			{ID: Ptr(int64(24)), Name: Ptr("iMac"), OS: Ptr("macos"), Status: Ptr("offline")},
		},
	}
	if !cmp.Equal(runners, want) {
		t.Errorf("Actions.ListOrganizationRunners returned %+v, want %+v", runners, want)
	}

	const methodName = "ListOrganizationRunners"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.ListOrganizationRunners(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.ListOrganizationRunners(ctx, "o", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_GetOrganizationRunner(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/runners/23", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":23,"name":"MBP","os":"macos","status":"online"}`)
	})

	ctx := t.Context()
	runner, _, err := client.Actions.GetOrganizationRunner(ctx, "o", 23)
	if err != nil {
		t.Errorf("Actions.GetOrganizationRunner returned error: %v", err)
	}

	want := &Runner{
		ID:     Ptr(int64(23)),
		Name:   Ptr("MBP"),
		OS:     Ptr("macos"),
		Status: Ptr("online"),
	}
	if !cmp.Equal(runner, want) {
		t.Errorf("Actions.GetOrganizationRunner returned %+v, want %+v", runner, want)
	}

	const methodName = "GetOrganizationRunner"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetOrganizationRunner(ctx, "\n", 23)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetOrganizationRunner(ctx, "o", 23)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_CreateOrganizationRemoveToken(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/runners/remove-token", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{"token":"AABF3JGZDX3P5PMEXLND6TS6FCWO6","expires_at":`+referenceTimeStr+`}`)
	})

	ctx := t.Context()
	token, _, err := client.Actions.CreateOrganizationRemoveToken(ctx, "o")
	if err != nil {
		t.Errorf("Actions.CreateOrganizationRemoveToken returned error: %v", err)
	}

	want := &RemoveToken{Token: Ptr("AABF3JGZDX3P5PMEXLND6TS6FCWO6"), ExpiresAt: &referenceTimestamp}
	if !cmp.Equal(token, want) {
		t.Errorf("Actions.CreateOrganizationRemoveToken returned %+v, want %+v", token, want)
	}

	const methodName = "CreateOrganizationRemoveToken"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.CreateOrganizationRemoveToken(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.CreateOrganizationRemoveToken(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_RemoveOrganizationRunner(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/actions/runners/21", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Actions.RemoveOrganizationRunner(ctx, "o", 21)
	if err != nil {
		t.Errorf("Actions.RemoveOrganizationRunner returned error: %v", err)
	}

	const methodName = "RemoveOrganizationRunner"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.RemoveOrganizationRunner(ctx, "\n", 21)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.RemoveOrganizationRunner(ctx, "o", 21)
	})
}
