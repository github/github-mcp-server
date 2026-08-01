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

func TestEnterpriseService_GetSSHKey(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/manage/v1/access/ssh", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{
				"key": "ssh-rsa 1234",
				"fingerprint": "bd"
			}]`)
	})

	ctx := t.Context()
	accessSSH, _, err := client.Enterprise.GetSSHKey(ctx)
	if err != nil {
		t.Errorf("Enterprise.GetSSHKey returned error: %v", err)
	}

	want := []*ClusterSSHKey{{
		Key:         Ptr("ssh-rsa 1234"),
		Fingerprint: Ptr("bd"),
	}}
	if !cmp.Equal(accessSSH, want) {
		t.Errorf("Enterprise.GetSSHKey returned %+v, want %+v", accessSSH, want)
	}

	const methodName = "GetSSHKey"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetSSHKey(ctx)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_DeleteSSHKey(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &SSHKeyOptions{
		Key: "ssh-rsa 1234",
	}

	mux.HandleFunc("/manage/v1/access/ssh", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `[ { "hostname": "primary", "uuid": "1b6cf518-f97c-11ed-8544-061d81f7eedb", "message": "SSH key removed successfully" } ]`)
	})

	ctx := t.Context()
	sshStatus, _, err := client.Enterprise.DeleteSSHKey(ctx, "ssh-rsa 1234")
	if err != nil {
		t.Errorf("Enterprise.DeleteSSHKey returned error: %v", err)
	}

	want := []*SSHKeyStatus{{Hostname: Ptr("primary"), UUID: Ptr("1b6cf518-f97c-11ed-8544-061d81f7eedb"), Message: Ptr("SSH key removed successfully")}}
	if diff := cmp.Diff(want, sshStatus); diff != "" {
		t.Errorf("diff mismatch (-want +got):\n%v", diff)
	}

	const methodName = "DeleteSSHKey"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.DeleteSSHKey(ctx, "ssh-rsa 1234")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_CreateSSHKey(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &SSHKeyOptions{
		Key: "ssh-rsa 1234",
	}

	mux.HandleFunc("/manage/v1/access/ssh", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `[ { "hostname": "primary", "uuid": "1b6cf518-f97c-11ed-8544-061d81f7eedb", "message": "SSH key added successfully", "modified": true } ]`)
	})

	ctx := t.Context()
	sshStatus, _, err := client.Enterprise.CreateSSHKey(ctx, "ssh-rsa 1234")
	if err != nil {
		t.Errorf("Enterprise.CreateSSHKey returned error: %v", err)
	}

	want := []*SSHKeyStatus{{Hostname: Ptr("primary"), UUID: Ptr("1b6cf518-f97c-11ed-8544-061d81f7eedb"), Message: Ptr("SSH key added successfully"), Modified: Ptr(true)}}
	if diff := cmp.Diff(want, sshStatus); diff != "" {
		t.Errorf("diff mismatch (-want +got):\n%v", diff)
	}

	const methodName = "CreateSSHKey"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.CreateSSHKey(ctx, "ssh-rsa 1234")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
