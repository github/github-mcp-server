// Copyright 2025 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPrivateRegistriesService_ListOrganizationPrivateRegistries(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/private-registries", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "X-Github-Api-Version", api20260310)
		testFormValues(t, r, values{
			"page": "2",
		})
		fmt.Fprint(w, `{
  "total_count": 1,
  "configurations": [
    {
      "name": "MAVEN_REPOSITORY_SECRET",
      "registry_type": "maven_repository",
      "username": "monalisa",
      "created_at": `+refTimeStr(1136178000)+`,
      "updated_at": `+refTimeStr(1136178001)+`,
      "visibility": "selected"
    }
  ]
}`)
	})

	opts := &ListOptions{Page: 2}
	ctx := t.Context()
	privateRegistries, _, err := client.PrivateRegistries.ListOrganizationPrivateRegistries(ctx, "o", opts)
	if err != nil {
		t.Fatalf("PrivateRegistries.ListOrganizationPrivateRegistries returned error: %v", err)
	}

	want := &PrivateRegistries{
		TotalCount: Ptr(1),
		Configurations: []*PrivateRegistry{
			{
				Name:         Ptr("MAVEN_REPOSITORY_SECRET"),
				RegistryType: Ptr(PrivateRegistryTypeMavenRepository),
				Username:     Ptr("monalisa"),
				CreatedAt:    refTimestamp(1136178000),
				UpdatedAt:    refTimestamp(1136178001),
				Visibility:   Ptr(PrivateRegistryVisibilitySelected),
			},
		},
	}
	if diff := cmp.Diff(want, privateRegistries); diff != "" {
		t.Errorf("PrivateRegistries.ListOrganizationPrivateRegistries mismatch (-want +got):\\n%v", diff)
	}

	const methodName = "ListOrganizationPrivateRegistries"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.PrivateRegistries.ListOrganizationPrivateRegistries(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.PrivateRegistries.ListOrganizationPrivateRegistries(ctx, "o", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})

	// still allow both set (no validation enforced) – ensure it does not error
	ctxBypass := context.WithValue(t.Context(), BypassRateLimitCheck, true)
	if _, _, err = client.PrivateRegistries.ListOrganizationPrivateRegistries(ctxBypass, "o", opts); err != nil {
		t.Fatalf("unexpected error when both before/after set: %v", err)
	}
}

func TestPrivateRegistriesService_CreateOrganizationPrivateRegistry(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &CreateOrganizationPrivateRegistry{
		RegistryType:          PrivateRegistryTypeMavenRepository,
		URL:                   "https://example.com/OWNER/REPOSITORY",
		Username:              Ptr("monalisa"),
		EncryptedValue:        Ptr("encrypted_value"),
		KeyID:                 Ptr("key_id"),
		Visibility:            PrivateRegistryVisibilitySelected,
		SelectedRepositoryIDs: []int64{1, 2, 3},
	}

	mux.HandleFunc("/orgs/o/private-registries", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "X-Github-Api-Version", api20260310)
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{
  "name": "MAVEN_REPOSITORY_SECRET",
  "registry_type": "maven_repository",
  "username": "monalisa",
  "visibility": "selected",
  "selected_repository_ids": [1, 2, 3],
  "created_at": `+refTimeStr(1136178000)+`,
  "updated_at": `+refTimeStr(1136178001)+`
}`)
	})

	ctx := t.Context()
	privateRegistry, _, err := client.PrivateRegistries.CreateOrganizationPrivateRegistry(ctx, "o", *input)
	if err != nil {
		t.Fatalf("PrivateRegistries.CreateOrganizationPrivateRegistry returned error: %v", err)
	}

	want := &PrivateRegistry{
		Name:                  Ptr("MAVEN_REPOSITORY_SECRET"),
		RegistryType:          Ptr(PrivateRegistryTypeMavenRepository),
		Username:              Ptr("monalisa"),
		CreatedAt:             refTimestamp(1136178000),
		UpdatedAt:             refTimestamp(1136178001),
		Visibility:            Ptr(PrivateRegistryVisibilitySelected),
		SelectedRepositoryIDs: []int64{1, 2, 3},
	}
	if diff := cmp.Diff(want, privateRegistry); diff != "" {
		t.Errorf("PrivateRegistries.CreateOrganizationPrivateRegistries mismatch (-want +got):\\n%v", diff)
	}

	const methodName = "CreateOrganizationPrivateRegistry"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.PrivateRegistries.CreateOrganizationPrivateRegistry(ctx, "\n", *input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.PrivateRegistries.CreateOrganizationPrivateRegistry(ctx, "o", *input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestPrivateRegistriesService_CreateOrganizationPrivateRegistry_OIDC(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &CreateOrganizationPrivateRegistry{
		RegistryType:          PrivateRegistryTypeMavenRepository,
		URL:                   "https://example.com/maven",
		AuthType:              Ptr("oidc_azure"),
		TenantID:              Ptr("my-tenant-id"),
		ClientID:              Ptr("my-client-id"),
		ReplacesBase:          Ptr(true),
		Visibility:            PrivateRegistryVisibilitySelected,
		SelectedRepositoryIDs: []int64{1, 2, 3},
	}

	mux.HandleFunc("/orgs/o/private-registries", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "X-Github-Api-Version", api20260310)
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{
  "name": "MAVEN_REPOSITORY_SECRET",
  "registry_type": "maven_repository",
  "visibility": "selected",
  "selected_repository_ids": [1, 2, 3],
  "created_at": `+refTimeStr(1136178000)+`,
  "updated_at": `+refTimeStr(1136178001)+`
}`)
	})

	ctx := t.Context()
	privateRegistry, _, err := client.PrivateRegistries.CreateOrganizationPrivateRegistry(ctx, "o", *input)
	if err != nil {
		t.Fatalf("PrivateRegistries.CreateOrganizationPrivateRegistry (OIDC) returned error: %v", err)
	}

	want := &PrivateRegistry{
		Name:                  Ptr("MAVEN_REPOSITORY_SECRET"),
		RegistryType:          Ptr(PrivateRegistryTypeMavenRepository),
		CreatedAt:             refTimestamp(1136178000),
		UpdatedAt:             refTimestamp(1136178001),
		Visibility:            Ptr(PrivateRegistryVisibilitySelected),
		SelectedRepositoryIDs: []int64{1, 2, 3},
	}
	if diff := cmp.Diff(want, privateRegistry); diff != "" {
		t.Errorf("PrivateRegistries.CreateOrganizationPrivateRegistry (OIDC) mismatch (-want +got):\\n%v", diff)
	}
}

func TestPrivateRegistries_UpdateOrganizationPrivateRegistry_OIDC(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &UpdateOrganizationPrivateRegistry{
		AuthType:    Ptr("oidc_aws"),
		AWSRegion:   Ptr("us-east-1"),
		AccountID:   Ptr("123456789012"),
		RoleName:    Ptr("my-role"),
		Domain:      Ptr("my-domain"),
		DomainOwner: Ptr("123456789012"),
		Audience:    Ptr("example.com"),
		Visibility:  Ptr(PrivateRegistryVisibilitySelected),
	}

	mux.HandleFunc("/orgs/o/private-registries/AWS_REGISTRY_SECRET", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testHeader(t, r, "X-Github-Api-Version", api20260310)
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.PrivateRegistries.UpdateOrganizationPrivateRegistry(ctx, "o", "AWS_REGISTRY_SECRET", *input)
	if err != nil {
		t.Fatalf("PrivateRegistries.UpdateOrganizationPrivateRegistry (OIDC) returned error: %v", err)
	}
}

func TestPrivateRegistriesService_CreateOrganizationPrivateRegistry_OIDCJFrog(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &CreateOrganizationPrivateRegistry{
		RegistryType:          PrivateRegistryTypeNpmRegistry,
		URL:                   "https://example.com/npm",
		AuthType:              Ptr("oidc_jfrog"),
		JFrogOIDCProviderName: Ptr("my-jfrog-provider"),
		Audience:              Ptr("jfrog"),
		IdentityMappingName:   Ptr("my-identity-mapping"),
		Visibility:            PrivateRegistryVisibilityPrivate,
	}

	mux.HandleFunc("/orgs/o/private-registries", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "X-Github-Api-Version", api20260310)
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{
  "name": "NPM_REGISTRY_SECRET",
  "registry_type": "npm_registry",
  "visibility": "private",
  "created_at": `+refTimeStr(1136178000)+`,
  "updated_at": `+refTimeStr(1136178001)+`
}`)
	})

	ctx := t.Context()
	privateRegistry, _, err := client.PrivateRegistries.CreateOrganizationPrivateRegistry(ctx, "o", *input)
	if err != nil {
		t.Fatalf("PrivateRegistries.CreateOrganizationPrivateRegistry (OIDC JFrog) returned error: %v", err)
	}

	want := &PrivateRegistry{
		Name:         Ptr("NPM_REGISTRY_SECRET"),
		RegistryType: Ptr(PrivateRegistryTypeNpmRegistry),
		CreatedAt:    refTimestamp(1136178000),
		UpdatedAt:    refTimestamp(1136178001),
		Visibility:   Ptr(PrivateRegistryVisibilityPrivate),
	}
	if diff := cmp.Diff(want, privateRegistry); diff != "" {
		t.Errorf("PrivateRegistries.CreateOrganizationPrivateRegistry (OIDC JFrog) mismatch (-want +got):\\n%v", diff)
	}
}

func TestPrivateRegistriesService_GetOrganizationPrivateRegistriesPublicKey(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/private-registries/public-key", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "X-Github-Api-Version", api20260310)
		fmt.Fprint(w, `{
  "key_id": "0123456789",
  "key": "public_key"
}`)
	})
	ctx := t.Context()
	publicKey, _, err := client.PrivateRegistries.GetOrganizationPrivateRegistriesPublicKey(ctx, "o")
	if err != nil {
		t.Fatalf("PrivateRegistries.GetOrganizationPrivateRegistriesPublicKey returned error: %v", err)
	}

	want := &PublicKey{
		KeyID: Ptr("0123456789"),
		Key:   Ptr("public_key"),
	}
	if diff := cmp.Diff(want, publicKey); diff != "" {
		t.Errorf("PrivateRegistries.GetOrganizationPrivateRegistriesPublicKey mismatch (-want +got):\\n%v", diff)
	}

	const methodName = "GetOrganizationPrivateRegistriesPublicKey"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.PrivateRegistries.GetOrganizationPrivateRegistriesPublicKey(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.PrivateRegistries.GetOrganizationPrivateRegistriesPublicKey(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestPrivateRegistriesService_GetOrganizationPrivateRegistry(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/private-registries/MAVEN_REPOSITORY_SECRET", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "X-Github-Api-Version", api20260310)
		fmt.Fprint(w, `{
  "name": "MAVEN_REPOSITORY_SECRET",
  "registry_type": "maven_repository",
  "username": "monalisa",
  "created_at": `+refTimeStr(1136178000)+`,
  "updated_at": `+refTimeStr(1136178001)+`,
  "visibility": "selected"
}`)
	})
	ctx := t.Context()
	privateRegistry, _, err := client.PrivateRegistries.GetOrganizationPrivateRegistry(ctx, "o", "MAVEN_REPOSITORY_SECRET")
	if err != nil {
		t.Fatalf("PrivateRegistries.GetOrganizationPrivateRegistry returned error: %v", err)
	}

	want := &PrivateRegistry{
		Name:         Ptr("MAVEN_REPOSITORY_SECRET"),
		RegistryType: Ptr(PrivateRegistryTypeMavenRepository),
		Username:     Ptr("monalisa"),
		CreatedAt:    refTimestamp(1136178000),
		UpdatedAt:    refTimestamp(1136178001),
		Visibility:   Ptr(PrivateRegistryVisibilitySelected),
	}
	if diff := cmp.Diff(want, privateRegistry); diff != "" {
		t.Errorf("PrivateRegistries.GetOrganizationPrivateRegistry mismatch (-want +got):\\n%v", diff)
	}

	const methodName = "GetOrganizationPrivateRegistry"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.PrivateRegistries.GetOrganizationPrivateRegistry(ctx, "\n", "MAVEN_REPOSITORY_SECRET")
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.PrivateRegistries.GetOrganizationPrivateRegistry(ctx, "o", "MAVEN_REPOSITORY_SECRET")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestPrivateRegistries_UpdateOrganizationPrivateRegistry(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &UpdateOrganizationPrivateRegistry{
		Username:       Ptr("monalisa"),
		EncryptedValue: Ptr("encrypted_value"),
		KeyID:          Ptr("key_id"),
		Visibility:     Ptr(PrivateRegistryVisibilitySelected),
	}

	mux.HandleFunc("/orgs/o/private-registries/MAVEN_REPOSITORY_SECRET", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testHeader(t, r, "X-Github-Api-Version", api20260310)
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.PrivateRegistries.UpdateOrganizationPrivateRegistry(ctx, "o", "MAVEN_REPOSITORY_SECRET", *input)
	if err != nil {
		t.Fatalf("PrivateRegistries.UpdateOrganizationPrivateRegistry returned error: %v", err)
	}

	const methodName = "UpdateOrganizationPrivateRegistry"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.PrivateRegistries.UpdateOrganizationPrivateRegistry(ctx, "\n", "MAVEN_REPOSITORY_SECRET", *input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.PrivateRegistries.UpdateOrganizationPrivateRegistry(ctx, "o", "MAVEN_REPOSITORY_SECRET", *input)
	})
}

func TestPrivateRegistriesService_DeleteOrganizationPrivateRegistry(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/private-registries/MAVEN_REPOSITORY_SECRET", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "X-Github-Api-Version", api20260310)
		w.WriteHeader(http.StatusNoContent)
	})
	ctx := t.Context()
	_, err := client.PrivateRegistries.DeleteOrganizationPrivateRegistry(ctx, "o", "MAVEN_REPOSITORY_SECRET")
	if err != nil {
		t.Fatalf("PrivateRegistries.DeleteOrganizationPrivateRegistry returned error: %v", err)
	}

	const methodName = "DeleteOrganizationPrivateRegistry"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.PrivateRegistries.DeleteOrganizationPrivateRegistry(ctx, "\n", "MAVEN_REPOSITORY_SECRET")
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.PrivateRegistries.DeleteOrganizationPrivateRegistry(ctx, "o", "MAVEN_REPOSITORY_SECRET")
	})
}
