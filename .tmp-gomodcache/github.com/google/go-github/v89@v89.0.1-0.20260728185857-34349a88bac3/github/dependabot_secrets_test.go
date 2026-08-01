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

func TestDependabotService_GetRepoPublicKey(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/dependabot/secrets/public-key", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"key_id":"1234","key":"2Sg8iYjAxxmI2LvUXpJjkYrMxURPc8r+dB7TJyvv1234"}`)
	})

	ctx := t.Context()
	key, _, err := client.Dependabot.GetRepoPublicKey(ctx, "o", "r")
	if err != nil {
		t.Errorf("Dependabot.GetRepoPublicKey returned error: %v", err)
	}

	want := &PublicKey{KeyID: Ptr("1234"), Key: Ptr("2Sg8iYjAxxmI2LvUXpJjkYrMxURPc8r+dB7TJyvv1234")}
	if !cmp.Equal(key, want) {
		t.Errorf("Dependabot.GetRepoPublicKey returned %+v, want %+v", key, want)
	}

	const methodName = "GetRepoPublicKey"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Dependabot.GetRepoPublicKey(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Dependabot.GetRepoPublicKey(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestDependabotService_GetRepoPublicKeyNumeric(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/dependabot/secrets/public-key", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"key_id":1234,"key":"2Sg8iYjAxxmI2LvUXpJjkYrMxURPc8r+dB7TJyvv1234"}`)
	})

	ctx := t.Context()
	key, _, err := client.Dependabot.GetRepoPublicKey(ctx, "o", "r")
	if err != nil {
		t.Errorf("Dependabot.GetRepoPublicKey returned error: %v", err)
	}

	want := &PublicKey{KeyID: Ptr("1234"), Key: Ptr("2Sg8iYjAxxmI2LvUXpJjkYrMxURPc8r+dB7TJyvv1234")}
	if !cmp.Equal(key, want) {
		t.Errorf("Dependabot.GetRepoPublicKey returned %+v, want %+v", key, want)
	}

	const methodName = "GetRepoPublicKey"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Dependabot.GetRepoPublicKey(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Dependabot.GetRepoPublicKey(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestDependabotService_ListRepoSecrets(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/dependabot/secrets", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"per_page": "2", "page": "2"})
		fmt.Fprint(w, `{"total_count":4,"secrets":[{"name":"A","created_at":`+refTimeStr(1136178000)+`,"updated_at":`+refTimeStr(1136178001)+`},{"name":"B","created_at":`+refTimeStr(1136178002)+`,"updated_at":`+refTimeStr(1136178003)+`}]}`)
	})

	opts := &ListOptions{Page: 2, PerPage: 2}
	ctx := t.Context()
	secrets, _, err := client.Dependabot.ListRepoSecrets(ctx, "o", "r", opts)
	if err != nil {
		t.Errorf("Dependabot.ListRepoSecrets returned error: %v", err)
	}

	want := &Secrets{
		TotalCount: 4,
		Secrets: []*Secret{
			{Name: "A", CreatedAt: *refTimestamp(1136178000), UpdatedAt: *refTimestamp(1136178001)},
			{Name: "B", CreatedAt: *refTimestamp(1136178002), UpdatedAt: *refTimestamp(1136178003)},
		},
	}
	if !cmp.Equal(secrets, want) {
		t.Errorf("Dependabot.ListRepoSecrets returned %+v, want %+v", secrets, want)
	}

	const methodName = "ListRepoSecrets"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Dependabot.ListRepoSecrets(ctx, "\n", "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Dependabot.ListRepoSecrets(ctx, "o", "r", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestDependabotService_GetRepoSecret(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/dependabot/secrets/NAME", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"name":"NAME","created_at":`+refTimeStr(1136178000)+`,"updated_at":`+refTimeStr(1136178001)+`}`)
	})

	ctx := t.Context()
	secret, _, err := client.Dependabot.GetRepoSecret(ctx, "o", "r", "NAME")
	if err != nil {
		t.Errorf("Dependabot.GetRepoSecret returned error: %v", err)
	}

	want := &Secret{
		Name:      "NAME",
		CreatedAt: *refTimestamp(1136178000),
		UpdatedAt: *refTimestamp(1136178001),
	}
	if !cmp.Equal(secret, want) {
		t.Errorf("Dependabot.GetRepoSecret returned %+v, want %+v", secret, want)
	}

	const methodName = "GetRepoSecret"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Dependabot.GetRepoSecret(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Dependabot.GetRepoSecret(ctx, "o", "r", "NAME")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestDependabotService_CreateOrUpdateRepoSecret(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := SecretRequest{
		EncryptedValue: "QIv=",
		KeyID:          "1234",
	}

	mux.HandleFunc("/repos/o/r/dependabot/secrets/NAME", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testHeader(t, r, "Content-Type", "application/json")
		want := SecretRequest{
			EncryptedValue: input.EncryptedValue,
			KeyID:          input.KeyID,
		}
		testJSONBody(t, r, want)
		w.WriteHeader(http.StatusCreated)
	})

	ctx := t.Context()
	_, err := client.Dependabot.CreateOrUpdateRepoSecret(ctx, "o", "r", "NAME", input)
	if err != nil {
		t.Errorf("Dependabot.CreateOrUpdateRepoSecret returned error: %v", err)
	}

	const methodName = "CreateOrUpdateRepoSecret"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Dependabot.CreateOrUpdateRepoSecret(ctx, "\n", "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Dependabot.CreateOrUpdateRepoSecret(ctx, "o", "r", "NAME", input)
	})
}

func TestDependabotService_DeleteRepoSecret(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/dependabot/secrets/NAME", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Dependabot.DeleteRepoSecret(ctx, "o", "r", "NAME")
	if err != nil {
		t.Errorf("Dependabot.DeleteRepoSecret returned error: %v", err)
	}

	const methodName = "DeleteRepoSecret"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Dependabot.DeleteRepoSecret(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Dependabot.DeleteRepoSecret(ctx, "o", "r", "NAME")
	})
}

func TestDependabotService_GetOrgPublicKey(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/dependabot/secrets/public-key", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"key_id":"012345678","key":"2Sg8iYjAxxmI2LvUXpJjkYrMxURPc8r+dB7TJyvv1234"}`)
	})

	ctx := t.Context()
	key, _, err := client.Dependabot.GetOrgPublicKey(ctx, "o")
	if err != nil {
		t.Errorf("Dependabot.GetOrgPublicKey returned error: %v", err)
	}

	want := &PublicKey{KeyID: Ptr("012345678"), Key: Ptr("2Sg8iYjAxxmI2LvUXpJjkYrMxURPc8r+dB7TJyvv1234")}
	if !cmp.Equal(key, want) {
		t.Errorf("Dependabot.GetOrgPublicKey returned %+v, want %+v", key, want)
	}

	const methodName = "GetOrgPublicKey"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Dependabot.GetOrgPublicKey(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Dependabot.GetOrgPublicKey(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestDependabotService_ListOrgSecrets(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/dependabot/secrets", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"per_page": "2", "page": "2"})
		fmt.Fprint(w, `{"total_count":3,"secrets":[{"name":"GIST_ID","created_at":`+refTimeStr(1136178000)+`,"updated_at":`+refTimeStr(1136178001)+`,"visibility":"private"},{"name":"DEPLOY_TOKEN","created_at":`+refTimeStr(1136178002)+`,"updated_at":`+refTimeStr(1136178003)+`,"visibility":"all"},{"name":"GH_TOKEN","created_at":`+refTimeStr(1136178004)+`,"updated_at":`+refTimeStr(1136178005)+`,"visibility":"selected","selected_repositories_url":"https://api.github.com/orgs/octo-org/dependabot/secrets/SUPER_SECRET/repositories"}]}`)
	})

	opts := &ListOptions{Page: 2, PerPage: 2}
	ctx := t.Context()
	secrets, _, err := client.Dependabot.ListOrgSecrets(ctx, "o", opts)
	if err != nil {
		t.Errorf("Dependabot.ListOrgSecrets returned error: %v", err)
	}

	want := &Secrets{
		TotalCount: 3,
		Secrets: []*Secret{
			{Name: "GIST_ID", CreatedAt: *refTimestamp(1136178000), UpdatedAt: *refTimestamp(1136178001), Visibility: "private"},
			{Name: "DEPLOY_TOKEN", CreatedAt: *refTimestamp(1136178002), UpdatedAt: *refTimestamp(1136178003), Visibility: "all"},
			{Name: "GH_TOKEN", CreatedAt: *refTimestamp(1136178004), UpdatedAt: *refTimestamp(1136178005), Visibility: "selected", SelectedRepositoriesURL: "https://api.github.com/orgs/octo-org/dependabot/secrets/SUPER_SECRET/repositories"},
		},
	}
	if !cmp.Equal(secrets, want) {
		t.Errorf("Dependabot.ListOrgSecrets returned %+v, want %+v", secrets, want)
	}

	const methodName = "ListOrgSecrets"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Dependabot.ListOrgSecrets(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Dependabot.ListOrgSecrets(ctx, "o", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestDependabotService_GetOrgSecret(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/dependabot/secrets/NAME", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"name":"NAME","created_at":`+refTimeStr(1136178000)+`,"updated_at":`+refTimeStr(1136178001)+`,"visibility":"selected","selected_repositories_url":"https://api.github.com/orgs/octo-org/dependabot/secrets/SUPER_SECRET/repositories"}`)
	})

	ctx := t.Context()
	secret, _, err := client.Dependabot.GetOrgSecret(ctx, "o", "NAME")
	if err != nil {
		t.Errorf("Dependabot.GetOrgSecret returned error: %v", err)
	}

	want := &Secret{
		Name:                    "NAME",
		CreatedAt:               *refTimestamp(1136178000),
		UpdatedAt:               *refTimestamp(1136178001),
		Visibility:              "selected",
		SelectedRepositoriesURL: "https://api.github.com/orgs/octo-org/dependabot/secrets/SUPER_SECRET/repositories",
	}
	if !cmp.Equal(secret, want) {
		t.Errorf("Dependabot.GetOrgSecret returned %+v, want %+v", secret, want)
	}

	const methodName = "GetOrgSecret"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Dependabot.GetOrgSecret(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Dependabot.GetOrgSecret(ctx, "o", "NAME")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestDependabotService_CreateOrUpdateOrgSecret(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := SecretOrgRequest{
		EncryptedValue:        "QIv=",
		KeyID:                 "1234",
		Visibility:            "selected",
		SelectedRepositoryIDs: []int64{1296269, 1269280},
	}

	mux.HandleFunc("/orgs/o/dependabot/secrets/NAME", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testHeader(t, r, "Content-Type", "application/json")
		want := SecretOrgRequest{
			EncryptedValue: input.EncryptedValue,
			KeyID:          input.KeyID,
			Visibility:     input.Visibility,
		}
		testJSONBody(t, r, struct {
			SecretOrgRequest
			SelectedRepositoryIDs []string `json:"selected_repository_ids,omitempty"`
		}{
			SecretOrgRequest:      want,
			SelectedRepositoryIDs: []string{"1296269", "1269280"},
		})
		w.WriteHeader(http.StatusCreated)
	})

	ctx := t.Context()
	_, err := client.Dependabot.CreateOrUpdateOrgSecret(ctx, "o", "NAME", input)
	if err != nil {
		t.Errorf("Dependabot.CreateOrUpdateOrgSecret returned error: %v", err)
	}

	const methodName = "CreateOrUpdateOrgSecret"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Dependabot.CreateOrUpdateOrgSecret(ctx, "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Dependabot.CreateOrUpdateOrgSecret(ctx, "o", "NAME", input)
	})
}

func TestDependabotService_ListSelectedReposForOrgSecret(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/dependabot/secrets/NAME/repositories", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"total_count":1,"repositories":[{"id":1}]}`)
	})

	opts := &ListOptions{Page: 2, PerPage: 2}
	ctx := t.Context()
	repos, _, err := client.Dependabot.ListSelectedReposForOrgSecret(ctx, "o", "NAME", opts)
	if err != nil {
		t.Errorf("Dependabot.ListSelectedReposForOrgSecret returned error: %v", err)
	}

	want := &SelectedReposList{
		TotalCount: Ptr(1),
		Repositories: []*Repository{
			{ID: Ptr(int64(1))},
		},
	}
	if !cmp.Equal(repos, want) {
		t.Errorf("Dependabot.ListSelectedReposForOrgSecret returned %+v, want %+v", repos, want)
	}

	const methodName = "ListSelectedReposForOrgSecret"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Dependabot.ListSelectedReposForOrgSecret(ctx, "\n", "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Dependabot.ListSelectedReposForOrgSecret(ctx, "o", "NAME", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestDependabotService_SetSelectedReposForOrgSecret(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := []int64{64780797}

	mux.HandleFunc("/orgs/o/dependabot/secrets/NAME/repositories", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testHeader(t, r, "Content-Type", "application/json")
		testJSONBody(t, r, struct {
			SelectedIDs []int64 `json:"selected_repository_ids"`
		}{
			SelectedIDs: input,
		})
	})

	ctx := t.Context()
	_, err := client.Dependabot.SetSelectedReposForOrgSecret(ctx, "o", "NAME", input)
	if err != nil {
		t.Errorf("Dependabot.SetSelectedReposForOrgSecret returned error: %v", err)
	}

	const methodName = "SetSelectedReposForOrgSecret"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Dependabot.SetSelectedReposForOrgSecret(ctx, "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Dependabot.SetSelectedReposForOrgSecret(ctx, "o", "NAME", input)
	})
}

func TestDependabotService_AddSelectedRepoToOrgSecret(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/dependabot/secrets/NAME/repositories/1234", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
	})

	repoID := int64(1234)
	ctx := t.Context()
	_, err := client.Dependabot.AddSelectedRepoToOrgSecret(ctx, "o", "NAME", repoID)
	if err != nil {
		t.Errorf("Dependabot.AddSelectedRepoToOrgSecret returned error: %v", err)
	}

	const methodName = "AddSelectedRepoToOrgSecret"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Dependabot.AddSelectedRepoToOrgSecret(ctx, "o", "NAME", 0)
		return err
	})
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Dependabot.AddSelectedRepoToOrgSecret(ctx, "\n", "\n", repoID)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Dependabot.AddSelectedRepoToOrgSecret(ctx, "o", "NAME", repoID)
	})
}

func TestDependabotService_RemoveSelectedRepoFromOrgSecret(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/dependabot/secrets/NAME/repositories/1234", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	repoID := int64(1234)
	ctx := t.Context()
	_, err := client.Dependabot.RemoveSelectedRepoFromOrgSecret(ctx, "o", "NAME", repoID)
	if err != nil {
		t.Errorf("Dependabot.RemoveSelectedRepoFromOrgSecret returned error: %v", err)
	}

	const methodName = "RemoveSelectedRepoFromOrgSecret"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Dependabot.RemoveSelectedRepoFromOrgSecret(ctx, "o", "NAME", 0)
		return err
	})
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Dependabot.RemoveSelectedRepoFromOrgSecret(ctx, "\n", "\n", repoID)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Dependabot.RemoveSelectedRepoFromOrgSecret(ctx, "o", "NAME", repoID)
	})
}

func TestDependabotService_DeleteOrgSecret(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/dependabot/secrets/NAME", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Dependabot.DeleteOrgSecret(ctx, "o", "NAME")
	if err != nil {
		t.Errorf("Dependabot.DeleteOrgSecret returned error: %v", err)
	}

	const methodName = "DeleteOrgSecret"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Dependabot.DeleteOrgSecret(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Dependabot.DeleteOrgSecret(ctx, "o", "NAME")
	})
}
