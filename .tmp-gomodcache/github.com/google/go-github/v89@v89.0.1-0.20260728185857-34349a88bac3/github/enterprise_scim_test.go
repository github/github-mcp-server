// Copyright 2025 The go-github AUTHORS. All rights reserved.
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

func TestEnterpriseService_ListProvisionedSCIMGroups(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/scim/v2/enterprises/ee/Groups", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeSCIM)
		testFormValues(t, r, values{
			"startIndex":         "1",
			"excludedAttributes": "members,meta",
			"count":              "3",
			"filter":             `externalId eq "914a"`,
		})
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{
			"schemas": ["`+SCIMSchemasURINamespacesListResponse+`"],
			"totalResults": 1,
			"itemsPerPage": 1,
			"startIndex": 1,
			"Resources": [{
				"schemas": ["`+SCIMSchemasURINamespacesGroups+`"],
				"id": "914a",
				"externalId": "de88",
				"displayName": "gn1",
				"meta": {
					"resourceType": "Group",
					"created": `+referenceTimeStr+`,
					"lastModified": `+referenceTimeStr+`,
					"location": "https://api.github.com/scim/v2/enterprises/ee/Groups/914a"
				},
				"members": [{
					"value": "e7f9",
					"$ref": "https://api.github.com/scim/v2/enterprises/ee/Users/e7f9",
					"display": "d1"
				}]
			}]
		}`)
	})

	ctx := t.Context()
	opts := &ListProvisionedSCIMGroupsEnterpriseOptions{
		StartIndex:         Ptr(1),
		ExcludedAttributes: Ptr("members,meta"),
		Count:              Ptr(3),
		Filter:             Ptr(`externalId eq "914a"`),
	}
	got, _, err := client.Enterprise.ListProvisionedSCIMGroups(ctx, "ee", opts)
	if err != nil {
		t.Fatalf("Enterprise.ListProvisionedSCIMGroups returned unexpected error: %v", err)
	}

	want := &SCIMEnterpriseGroups{
		Schemas:      []string{SCIMSchemasURINamespacesListResponse},
		TotalResults: Ptr(1),
		ItemsPerPage: Ptr(1),
		StartIndex:   Ptr(1),
		Resources: []*SCIMEnterpriseGroupAttributes{{
			ID: Ptr("914a"),
			Meta: &SCIMEnterpriseMeta{
				ResourceType: "Group",
				Created:      &Timestamp{referenceTime},
				LastModified: &Timestamp{referenceTime},
				Location:     Ptr("https://api.github.com/scim/v2/enterprises/ee/Groups/914a"),
			},
			DisplayName: Ptr("gn1"),
			Schemas:     []string{SCIMSchemasURINamespacesGroups},
			ExternalID:  Ptr("de88"),
			Members: []*SCIMEnterpriseDisplayReference{{
				Value:   "e7f9",
				Ref:     Ptr("https://api.github.com/scim/v2/enterprises/ee/Users/e7f9"),
				Display: Ptr("d1"),
			}},
		}},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Enterprise.ListProvisionedSCIMGroups diff mismatch (-want +got):\n%v", diff)
	}

	const methodName = "ListProvisionedSCIMGroups"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.ListProvisionedSCIMGroups(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ListProvisionedSCIMGroups(ctx, "ee", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_ListProvisionedSCIMUsers(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/scim/v2/enterprises/ee/Users", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeSCIM)
		testFormValues(t, r, values{
			"startIndex": "1",
			"count":      "3",
			"filter":     `userName eq "octocat@github.com"`,
		})
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{
			"schemas": ["`+SCIMSchemasURINamespacesListResponse+`"],
			"totalResults": 1,
			"itemsPerPage": 1,
			"startIndex": 1,
			"Resources": [
			  {
				"schemas": ["`+SCIMSchemasURINamespacesUser+`"],
				"id": "5fc0",
				"externalId": "00u1",
				"userName": "octocat@github.com",
				"displayName": "Mona Octocat",
				"name": {
				  "givenName": "Mona",
				  "familyName": "Octocat",
				  "formatted": "Mona Octocat"
				},
				"emails": [
				  {
					"value": "octocat@github.com",
					"primary": true
				  }
				],
				"active": true,
				"meta": {
				  "resourceType": "User",
				  "created": `+referenceTimeStr+`,
				  "lastModified": `+referenceTimeStr+`,
				  "location": "https://api.github.com/scim/v2/enterprises/ee/Users/5fc0"
				}
			  }
			]
		}`)
	})

	ctx := t.Context()
	opts := &ListProvisionedSCIMUsersEnterpriseOptions{
		StartIndex: Ptr(1),
		Count:      Ptr(3),
		Filter:     Ptr(`userName eq "octocat@github.com"`),
	}
	got, _, err := client.Enterprise.ListProvisionedSCIMUsers(ctx, "ee", opts)
	if err != nil {
		t.Fatalf("Enterprise.ListProvisionedSCIMUsers returned unexpected error: %v", err)
	}

	want := &SCIMEnterpriseUsers{
		Schemas:      []string{SCIMSchemasURINamespacesListResponse},
		TotalResults: Ptr(1),
		ItemsPerPage: Ptr(1),
		StartIndex:   Ptr(1),
		Resources: []*SCIMEnterpriseUserAttributes{{
			Schemas:     []string{SCIMSchemasURINamespacesUser},
			ID:          Ptr("5fc0"),
			ExternalID:  "00u1",
			UserName:    "octocat@github.com",
			DisplayName: "Mona Octocat",
			Name: &SCIMEnterpriseUserName{
				GivenName:  "Mona",
				FamilyName: "Octocat",
				Formatted:  Ptr("Mona Octocat"),
			},
			Emails: []*SCIMEnterpriseUserEmail{{
				Value:   "octocat@github.com",
				Primary: true,
			}},
			Active: true,
			Meta: &SCIMEnterpriseMeta{
				ResourceType: "User",
				Created:      &Timestamp{referenceTime},
				LastModified: &Timestamp{referenceTime},
				Location:     Ptr("https://api.github.com/scim/v2/enterprises/ee/Users/5fc0"),
			},
		}},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Enterprise.ListProvisionedSCIMUsers diff mismatch (-want +got):\n%v", diff)
	}

	const methodName = "ListProvisionedSCIMUsers"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.ListProvisionedSCIMUsers(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ListProvisionedSCIMUsers(ctx, "ee", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_SetProvisionedSCIMGroup(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := SCIMEnterpriseGroupAttributes{
		Schemas:     []string{SCIMSchemasURINamespacesGroups},
		ExternalID:  Ptr("8aa1"),
		DisplayName: Ptr("dn"),
	}

	mux.HandleFunc("/scim/v2/enterprises/ee/Groups/abcd", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testHeader(t, r, "Accept", mediaTypeSCIM)
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{
			"schemas": ["`+SCIMSchemasURINamespacesGroups+`"],
			"id": "abcd",
			"externalId": "8aa1",
			"displayName": "dn",
			"meta": {
			  "resourceType": "Group",
			  "created": `+referenceTimeStr+`,
			  "lastModified": `+referenceTimeStr+`,
			  "location": "https://api.github.localhost/scim/v2/enterprises/ee/Groups/abcd"
			}
		}`)
	})
	want := &SCIMEnterpriseGroupAttributes{
		Schemas:     []string{SCIMSchemasURINamespacesGroups},
		ID:          Ptr("abcd"),
		ExternalID:  Ptr("8aa1"),
		DisplayName: Ptr("dn"),
		Meta: &SCIMEnterpriseMeta{
			ResourceType: "Group",
			Created:      &Timestamp{referenceTime},
			LastModified: &Timestamp{referenceTime},
			Location:     Ptr("https://api.github.localhost/scim/v2/enterprises/ee/Groups/abcd"),
		},
	}

	ctx := t.Context()
	got, _, err := client.Enterprise.SetProvisionedSCIMGroup(ctx, "ee", "abcd", input)
	if err != nil {
		t.Fatalf("Enterprise.SetProvisionedSCIMGroup returned unexpected error: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Enterprise.SetProvisionedSCIMGroup diff mismatch (-want +got):\n%v", diff)
	}

	const methodName = "SetProvisionedSCIMGroup"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.SetProvisionedSCIMGroup(ctx, "\n", "\n", SCIMEnterpriseGroupAttributes{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.SetProvisionedSCIMGroup(ctx, "ee", "abcd", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_SetProvisionedSCIMUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := SCIMEnterpriseUserAttributes{
		Schemas:     []string{SCIMSchemasURINamespacesUser},
		ExternalID:  "e123",
		Active:      true,
		UserName:    "e123",
		DisplayName: "John Doe",
		Emails: []*SCIMEnterpriseUserEmail{{
			Value:   "john@example.com",
			Type:    "work",
			Primary: true,
		}},
	}

	mux.HandleFunc("/scim/v2/enterprises/ee/Users/7fce", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testHeader(t, r, "Accept", mediaTypeSCIM)
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{
			"schemas": ["`+SCIMSchemasURINamespacesUser+`"],
			"id": "7fce",
			"externalId": "e123",
			"active": true,
			"userName": "e123",
			"displayName": "John Doe",
			"emails": [{
				"value": "john@example.com",
				"type": "work",
				"primary": true
			}],
			"meta": {
				"resourceType": "User",
				"created": `+referenceTimeStr+`,
				"lastModified": `+referenceTimeStr+`,
				"location": "https://api.github.localhost/scim/v2/enterprises/ee/Users/7fce"
			}
		}`)
	})
	want := &SCIMEnterpriseUserAttributes{
		Schemas:     []string{SCIMSchemasURINamespacesUser},
		ID:          Ptr("7fce"),
		ExternalID:  "e123",
		Active:      true,
		UserName:    "e123",
		DisplayName: "John Doe",
		Emails: []*SCIMEnterpriseUserEmail{{
			Value:   "john@example.com",
			Type:    "work",
			Primary: true,
		}},
		Meta: &SCIMEnterpriseMeta{
			ResourceType: "User",
			Created:      &Timestamp{referenceTime},
			LastModified: &Timestamp{referenceTime},
			Location:     Ptr("https://api.github.localhost/scim/v2/enterprises/ee/Users/7fce"),
		},
	}

	ctx := t.Context()
	got, _, err := client.Enterprise.SetProvisionedSCIMUser(ctx, "ee", "7fce", input)
	if err != nil {
		t.Fatalf("Enterprise.SetProvisionedSCIMUser returned unexpected error: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Enterprise.SetProvisionedSCIMUser diff mismatch (-want +got):\n%v", diff)
	}

	const methodName = "SetProvisionedSCIMUser"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.SetProvisionedSCIMUser(ctx, "\n", "\n", SCIMEnterpriseUserAttributes{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.SetProvisionedSCIMUser(ctx, "ee", "7fce", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_UpdateSCIMGroupAttribute(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := SCIMEnterpriseAttribute{
		Schemas: []string{SCIMSchemasURINamespacesPatchOp},
		Operations: []*SCIMEnterpriseAttributeOperation{{
			Op:    "replace",
			Path:  Ptr("displayName"),
			Value: "Employees",
		}},
	}

	mux.HandleFunc("/scim/v2/enterprises/ee/Groups/abcd", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testHeader(t, r, "Accept", mediaTypeSCIM)
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{
			"schemas": ["`+SCIMSchemasURINamespacesGroups+`"],
			"id": "abcd",
			"externalId": "8aa1",
			"displayName": "Employees",
			"members": [{
				"value": "879d",
				"$ref": "https://api.github.localhost/scim/v2/enterprises/ee/Users/879d",
				"display": "User 1"
			}],
			"meta": {
				"resourceType": "Group",
				"created": `+referenceTimeStr+`,
				"lastModified": `+referenceTimeStr+`,
				"location": "https://api.github.localhost/scim/v2/enterprises/ee/Groups/abcd"
			}
		}`)
	})
	want := &SCIMEnterpriseGroupAttributes{
		Schemas:     []string{SCIMSchemasURINamespacesGroups},
		ID:          Ptr("abcd"),
		ExternalID:  Ptr("8aa1"),
		DisplayName: Ptr("Employees"),
		Members: []*SCIMEnterpriseDisplayReference{{
			Value:   "879d",
			Ref:     Ptr("https://api.github.localhost/scim/v2/enterprises/ee/Users/879d"),
			Display: Ptr("User 1"),
		}},
		Meta: &SCIMEnterpriseMeta{
			ResourceType: "Group",
			Created:      &Timestamp{referenceTime},
			LastModified: &Timestamp{referenceTime},
			Location:     Ptr("https://api.github.localhost/scim/v2/enterprises/ee/Groups/abcd"),
		},
	}

	ctx := t.Context()
	got, _, err := client.Enterprise.UpdateSCIMGroupAttribute(ctx, "ee", "abcd", input)
	if err != nil {
		t.Fatalf("Enterprise.UpdateSCIMGroupAttribute returned unexpected error: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Enterprise.UpdateSCIMGroupAttribute diff mismatch (-want +got):\n%v", diff)
	}

	const methodName = "UpdateSCIMGroupAttribute"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.UpdateSCIMGroupAttribute(ctx, "\n", "\n", SCIMEnterpriseAttribute{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.UpdateSCIMGroupAttribute(ctx, "ee", "abcd", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_UpdateSCIMUserAttribute(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := SCIMEnterpriseAttribute{
		Schemas: []string{SCIMSchemasURINamespacesPatchOp},
		Operations: []*SCIMEnterpriseAttributeOperation{{
			Op:    "replace",
			Path:  Ptr("emails[type eq 'work'].value"),
			Value: "updatedEmail@example.com",
		}, {
			Op:    "replace",
			Path:  Ptr("name.familyName"),
			Value: "updatedFamilyName",
		}},
	}

	mux.HandleFunc("/scim/v2/enterprises/ee/Users/7fce", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testHeader(t, r, "Accept", mediaTypeSCIM)
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{
			"schemas": ["`+SCIMSchemasURINamespacesUser+`"],
			"id": "7fce",
			"externalId": "e123",
			"active": true,
			"userName": "e123",
			"name": {
				"formatted": "John Doe X",
				"familyName": "updatedFamilyName",
				"givenName": "John",
				"middleName": "X"
			},
			"displayName": "John Doe",
			"emails": [{
				"value": "john@example.com",
				"type": "work",
				"primary": true
			}],
			"roles": [{
				"value": "User",
				"primary": false
			}],
			"meta": {
				"resourceType": "User",
				"created": `+referenceTimeStr+`,
				"lastModified": `+referenceTimeStr+`,
				"location": "https://api.github.localhost/scim/v2/enterprises/ee/Users/7fce"
			}
		}`)
	})
	want := &SCIMEnterpriseUserAttributes{
		Schemas:     []string{SCIMSchemasURINamespacesUser},
		ID:          Ptr("7fce"),
		ExternalID:  "e123",
		Active:      true,
		UserName:    "e123",
		DisplayName: "John Doe",
		Name: &SCIMEnterpriseUserName{
			Formatted:  Ptr("John Doe X"),
			FamilyName: "updatedFamilyName",
			GivenName:  "John",
			MiddleName: Ptr("X"),
		},
		Emails: []*SCIMEnterpriseUserEmail{{
			Value:   "john@example.com",
			Type:    "work",
			Primary: true,
		}},
		Roles: []*SCIMEnterpriseUserRole{{
			Value:   "User",
			Primary: Ptr(false),
		}},
		Meta: &SCIMEnterpriseMeta{
			ResourceType: "User",
			Created:      &Timestamp{referenceTime},
			LastModified: &Timestamp{referenceTime},
			Location:     Ptr("https://api.github.localhost/scim/v2/enterprises/ee/Users/7fce"),
		},
	}

	ctx := t.Context()
	got, _, err := client.Enterprise.UpdateSCIMUserAttribute(ctx, "ee", "7fce", input)
	if err != nil {
		t.Fatalf("Enterprise.UpdateSCIMUserAttribute returned unexpected error: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Enterprise.UpdateSCIMUserAttribute diff mismatch (-want +got):\n%v", diff)
	}

	const methodName = "UpdateSCIMUserAttribute"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.UpdateSCIMUserAttribute(ctx, "\n", "\n", SCIMEnterpriseAttribute{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.UpdateSCIMUserAttribute(ctx, "ee", "7fce", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_ProvisionSCIMGroup(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := SCIMEnterpriseGroupAttributes{
		Schemas:     []string{SCIMSchemasURINamespacesGroups},
		ExternalID:  Ptr("8aa1"),
		DisplayName: Ptr("dn"),
		Members: []*SCIMEnterpriseDisplayReference{{
			Value:   "879d",
			Display: Ptr("d1"),
		}, {
			Value:   "0db5",
			Display: Ptr("d2"),
		}},
	}

	mux.HandleFunc("/scim/v2/enterprises/ee/Groups", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "Accept", mediaTypeSCIM)
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{
			"schemas": ["`+SCIMSchemasURINamespacesGroups+`"],
			"id": "abcd",
			"externalId": "8aa1",
			"displayName": "dn",
			"members": [
			  {
			    "value": "879d",
			    "$ref": "https://api.github.localhost/scim/v2/enterprises/ee/Users/879d",
			    "display": "d1"
			  },
			  {
			    "value": "0db5",
			    "$ref": "https://api.github.localhost/scim/v2/enterprises/ee/Users/0db5",
			    "display": "d2"
			  }
			],
			"meta": {
			  "resourceType": "Group",
			  "created": `+referenceTimeStr+`,
			  "lastModified": `+referenceTimeStr+`,
			  "location": "https://api.github.localhost/scim/v2/enterprises/ee/Groups/abcd"
			}
		}`)
	})
	want := &SCIMEnterpriseGroupAttributes{
		Schemas:     []string{SCIMSchemasURINamespacesGroups},
		ID:          Ptr("abcd"),
		ExternalID:  Ptr("8aa1"),
		DisplayName: Ptr("dn"),
		Members: []*SCIMEnterpriseDisplayReference{{
			Value:   "879d",
			Ref:     Ptr("https://api.github.localhost/scim/v2/enterprises/ee/Users/879d"),
			Display: Ptr("d1"),
		}, {
			Value:   "0db5",
			Ref:     Ptr("https://api.github.localhost/scim/v2/enterprises/ee/Users/0db5"),
			Display: Ptr("d2"),
		}},
		Meta: &SCIMEnterpriseMeta{
			ResourceType: "Group",
			Created:      &Timestamp{referenceTime},
			LastModified: &Timestamp{referenceTime},
			Location:     Ptr("https://api.github.localhost/scim/v2/enterprises/ee/Groups/abcd"),
		},
	}

	ctx := t.Context()
	got, _, err := client.Enterprise.ProvisionSCIMGroup(ctx, "ee", input)
	if err != nil {
		t.Fatalf("Enterprise.ProvisionSCIMGroup returned unexpected error: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Enterprise.ProvisionSCIMGroup diff mismatch (-want +got):\n%v", diff)
	}

	const methodName = "ProvisionSCIMGroup"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.ProvisionSCIMGroup(ctx, "\n", SCIMEnterpriseGroupAttributes{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ProvisionSCIMGroup(ctx, "ee", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_ProvisionSCIMUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := SCIMEnterpriseUserAttributes{
		Schemas:    []string{SCIMSchemasURINamespacesUser},
		ExternalID: "e123",
		Active:     true,
		UserName:   "e123",
		Name: &SCIMEnterpriseUserName{
			Formatted:  Ptr("John Doe"),
			FamilyName: "Doe",
			GivenName:  "John",
		},
		DisplayName: "DOE John",
		Emails: []*SCIMEnterpriseUserEmail{{
			Value:   "john@example.com",
			Type:    "work",
			Primary: true,
		}},
		Roles: []*SCIMEnterpriseUserRole{{
			Value:   "User",
			Primary: Ptr(false),
		}},
	}

	mux.HandleFunc("/scim/v2/enterprises/ee/Users", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "Accept", mediaTypeSCIM)
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{
			"schemas": ["`+SCIMSchemasURINamespacesUser+`"],
			"id": "7fce",
			"externalId": "e123",
			"active": true,
			"userName": "e123",
			"name": {
				"formatted": "John Doe",
				"familyName": "Doe",
				"givenName": "John"
			},
			"displayName": "DOE John",
			"emails": [{
				"value": "john@example.com",
				"type": "work",
				"primary": true
			}],
			"roles": [{
				"value": "User",
				"primary": false
			}],
			"meta": {
				"resourceType": "User",
				"created": `+referenceTimeStr+`,
				"lastModified": `+referenceTimeStr+`,
				"location": "https://api.github.localhost/scim/v2/enterprises/ee/Users/7fce"
			}
		}`)
	})
	want := &SCIMEnterpriseUserAttributes{
		Schemas:     []string{SCIMSchemasURINamespacesUser},
		ID:          Ptr("7fce"),
		ExternalID:  "e123",
		Active:      true,
		UserName:    "e123",
		DisplayName: "DOE John",
		Name: &SCIMEnterpriseUserName{
			Formatted:  Ptr("John Doe"),
			FamilyName: "Doe",
			GivenName:  "John",
		},
		Emails: []*SCIMEnterpriseUserEmail{{
			Value:   "john@example.com",
			Type:    "work",
			Primary: true,
		}},
		Roles: []*SCIMEnterpriseUserRole{{
			Value:   "User",
			Primary: Ptr(false),
		}},
		Meta: &SCIMEnterpriseMeta{
			ResourceType: "User",
			Created:      &Timestamp{referenceTime},
			LastModified: &Timestamp{referenceTime},
			Location:     Ptr("https://api.github.localhost/scim/v2/enterprises/ee/Users/7fce"),
		},
	}

	ctx := t.Context()
	got, _, err := client.Enterprise.ProvisionSCIMUser(ctx, "ee", input)
	if err != nil {
		t.Fatalf("Enterprise.ProvisionSCIMUser returned unexpected error: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Enterprise.ProvisionSCIMUser diff mismatch (-want +got):\n%v", diff)
	}

	const methodName = "ProvisionSCIMUser"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.ProvisionSCIMUser(ctx, "\n", SCIMEnterpriseUserAttributes{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ProvisionSCIMUser(ctx, "ee", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_GetProvisionedSCIMGroup(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/scim/v2/enterprises/ee/Groups/914a", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeSCIM)
		testFormValues(t, r, values{"excludedAttributes": "members,meta"})
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{
			"schemas": ["`+SCIMSchemasURINamespacesGroups+`"],
			"id": "914a",
			"externalId": "de88",
			"displayName": "gn1",
			"meta": {
				"resourceType": "Group",
				"created": `+referenceTimeStr+`,
				"lastModified": `+referenceTimeStr+`,
				"location": "https://api.github.com/scim/v2/enterprises/ee/Groups/914a"
			},
			"members": [{
				"value": "e7f9",
				"$ref": "https://api.github.com/scim/v2/enterprises/ee/Users/e7f9",
				"display": "d1"
			}]
		}`)
	})

	ctx := t.Context()
	opts := &GetProvisionedSCIMGroupEnterpriseOptions{ExcludedAttributes: Ptr("members,meta")}
	got, _, err := client.Enterprise.GetProvisionedSCIMGroup(ctx, "ee", "914a", opts)
	if err != nil {
		t.Fatalf("Enterprise.GetProvisionedSCIMGroup returned unexpected error: %v", err)
	}

	want := &SCIMEnterpriseGroupAttributes{
		ID: Ptr("914a"),
		Meta: &SCIMEnterpriseMeta{
			ResourceType: "Group",
			Created:      &Timestamp{referenceTime},
			LastModified: &Timestamp{referenceTime},
			Location:     Ptr("https://api.github.com/scim/v2/enterprises/ee/Groups/914a"),
		},
		DisplayName: Ptr("gn1"),
		Schemas:     []string{SCIMSchemasURINamespacesGroups},
		ExternalID:  Ptr("de88"),
		Members: []*SCIMEnterpriseDisplayReference{{
			Value:   "e7f9",
			Ref:     Ptr("https://api.github.com/scim/v2/enterprises/ee/Users/e7f9"),
			Display: Ptr("d1"),
		}},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Enterprise.GetProvisionedSCIMGroup diff mismatch (-want +got):\n%v", diff)
	}

	const methodName = "GetProvisionedSCIMGroup"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.GetProvisionedSCIMGroup(ctx, "ee", "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetProvisionedSCIMGroup(ctx, "ee", "914a", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_GetProvisionedSCIMUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/scim/v2/enterprises/ee/Users/5fc0", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeSCIM)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{
			"schemas": ["`+SCIMSchemasURINamespacesUser+`"],
			"id": "5fc0",
			"externalId": "00u1",
			"userName": "octocat@github.com",
			"displayName": "Mona Octocat",
			"name": {
				"givenName": "Mona",
				"familyName": "Octocat",
				"formatted": "Mona Octocat"
			},
			"emails": [
				{
				"value": "octocat@github.com",
				"primary": true
				}
			],
			"active": true,
			"meta": {
				"resourceType": "User",
				"created": `+referenceTimeStr+`,
				"lastModified": `+referenceTimeStr+`,
				"location": "https://api.github.com/scim/v2/enterprises/ee/Users/5fc0"
			}
		}`)
	})

	ctx := t.Context()
	got, _, err := client.Enterprise.GetProvisionedSCIMUser(ctx, "ee", "5fc0")
	if err != nil {
		t.Fatalf("Enterprise.GetProvisionedSCIMUser returned unexpected error: %v", err)
	}

	want := &SCIMEnterpriseUserAttributes{
		Schemas:     []string{SCIMSchemasURINamespacesUser},
		ID:          Ptr("5fc0"),
		ExternalID:  "00u1",
		UserName:    "octocat@github.com",
		DisplayName: "Mona Octocat",
		Name: &SCIMEnterpriseUserName{
			GivenName:  "Mona",
			FamilyName: "Octocat",
			Formatted:  Ptr("Mona Octocat"),
		},
		Emails: []*SCIMEnterpriseUserEmail{{
			Value:   "octocat@github.com",
			Primary: true,
		}},
		Active: true,
		Meta: &SCIMEnterpriseMeta{
			ResourceType: "User",
			Created:      &Timestamp{referenceTime},
			LastModified: &Timestamp{referenceTime},
			Location:     Ptr("https://api.github.com/scim/v2/enterprises/ee/Users/5fc0"),
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Enterprise.GetProvisionedSCIMUser diff mismatch (-want +got):\n%v", diff)
	}

	const methodName = "GetProvisionedSCIMUser"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.GetProvisionedSCIMUser(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetProvisionedSCIMUser(ctx, "ee", "5fc0")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_DeleteSCIMGroup(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/scim/v2/enterprises/ee/Groups/abcd", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeV3)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Enterprise.DeleteSCIMGroup(ctx, "ee", "abcd")
	if err != nil {
		t.Fatalf("Enterprise.DeleteSCIMGroup returned unexpected error: %v", err)
	}

	const methodName = "DeleteSCIMGroup"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Enterprise.DeleteSCIMGroup(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Enterprise.DeleteSCIMGroup(ctx, "ee", "abcd")
	})
}

func TestEnterpriseService_DeleteSCIMUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/scim/v2/enterprises/ee/Users/7fce", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeV3)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Enterprise.DeleteSCIMUser(ctx, "ee", "7fce")
	if err != nil {
		t.Fatalf("Enterprise.DeleteSCIMUser returned unexpected error: %v", err)
	}

	const methodName = "DeleteSCIMUser"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Enterprise.DeleteSCIMUser(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Enterprise.DeleteSCIMUser(ctx, "ee", "7fce")
	})
}
