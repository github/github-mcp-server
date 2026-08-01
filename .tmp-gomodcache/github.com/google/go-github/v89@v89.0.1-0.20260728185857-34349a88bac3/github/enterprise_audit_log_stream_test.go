// Copyright 2026 The go-github AUTHORS. All rights reserved.
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

func TestEnterpriseService_GetAuditLogStreamKey(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/audit-log/stream-key", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"key_id":"1234","key":"2Sg8iYjAxxmI2LvUXpJjkYrMxURPc8r+dB7TJyvv1234"}`)
	})

	ctx := t.Context()
	key, _, err := client.Enterprise.GetAuditLogStreamKey(ctx, "e")
	if err != nil {
		t.Errorf("Enterprise.GetAuditLogStreamKey returned error: %v", err)
	}

	want := &AuditLogStreamKey{
		KeyID: "1234",
		Key:   "2Sg8iYjAxxmI2LvUXpJjkYrMxURPc8r+dB7TJyvv1234",
	}
	if !cmp.Equal(key, want) {
		t.Errorf("Enterprise.GetAuditLogStreamKey returned %+v, want %+v", key, want)
	}

	const methodName = "GetAuditLogStreamKey"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.GetAuditLogStreamKey(ctx, "\n")
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetAuditLogStreamKey(ctx, "e")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_ListAuditLogStreams(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/audit-log/streams", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{"id":1,"stream_type":"Splunk","stream_details":"US","enabled":true}]`)
	})

	ctx := t.Context()
	streams, _, err := client.Enterprise.ListAuditLogStreams(ctx, "e")
	if err != nil {
		t.Errorf("Enterprise.ListAuditLogStreams returned error: %v", err)
	}

	want := []*AuditLogStream{
		{
			ID:            1,
			StreamType:    "Splunk",
			StreamDetails: "US",
			Enabled:       true,
		},
	}
	if !cmp.Equal(streams, want) {
		t.Errorf("Enterprise.ListAuditLogStreams returned %+v, want %+v", streams, want)
	}

	const methodName = "ListAuditLogStreams"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.ListAuditLogStreams(ctx, "\n")
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ListAuditLogStreams(ctx, "e")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_GetAuditLogStream(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/audit-log/streams/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":1,"stream_type":"Datadog","stream_details":"US","enabled":true}`)
	})

	ctx := t.Context()
	stream, _, err := client.Enterprise.GetAuditLogStream(ctx, "e", 1)
	if err != nil {
		t.Errorf("Enterprise.GetAuditLogStream returned error: %v", err)
	}

	want := &AuditLogStream{
		ID:            1,
		StreamType:    "Datadog",
		StreamDetails: "US",
		Enabled:       true,
	}
	if !cmp.Equal(stream, want) {
		t.Errorf("Enterprise.GetAuditLogStream returned %+v, want %+v", stream, want)
	}

	const methodName = "GetAuditLogStream"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.GetAuditLogStream(ctx, "\n", 1)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetAuditLogStream(ctx, "e", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_CreateAuditLogStream(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/audit-log/streams", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{"id":2,"stream_type":"Datadog","stream_details":"US3","enabled":false}`)
	})

	input := NewDatadogStreamConfig(false, &DatadogConfig{
		EncryptedToken: "ENCRYPTED",
		Site:           "US3",
		KeyID:          "v1",
	})

	ctx := t.Context()
	stream, _, err := client.Enterprise.CreateAuditLogStream(ctx, "e", *input)
	if err != nil {
		t.Errorf("Enterprise.CreateAuditLogStream returned error: %v", err)
	}

	want := &AuditLogStream{
		ID:            2,
		StreamType:    "Datadog",
		StreamDetails: "US3",
		Enabled:       false,
	}
	if !cmp.Equal(stream, want) {
		t.Errorf("Enterprise.CreateAuditLogStream returned %+v, want %+v", stream, want)
	}

	const methodName = "CreateAuditLogStream"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.CreateAuditLogStream(ctx, "\n", *input)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.CreateAuditLogStream(ctx, "e", *input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_UpdateAuditLogStream(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/audit-log/streams/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		fmt.Fprint(w, `{"id":1,"stream_type":"Splunk","stream_details":"splunk.example.com","enabled":true}`)
	})

	input := NewSplunkStreamConfig(true, &SplunkConfig{
		Domain:         "splunk.example.com",
		Port:           8089,
		KeyID:          "v1",
		EncryptedToken: "ENCRYPTED",
		SSLVerify:      true,
	})

	ctx := t.Context()
	stream, _, err := client.Enterprise.UpdateAuditLogStream(ctx, "e", 1, *input)
	if err != nil {
		t.Errorf("Enterprise.UpdateAuditLogStream returned error: %v", err)
	}

	want := &AuditLogStream{
		ID:            1,
		StreamType:    "Splunk",
		StreamDetails: "splunk.example.com",
		Enabled:       true,
	}
	if !cmp.Equal(stream, want) {
		t.Errorf("Enterprise.UpdateAuditLogStream returned %+v, want %+v", stream, want)
	}

	const methodName = "UpdateAuditLogStream"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.UpdateAuditLogStream(ctx, "\n", 1, *input)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.UpdateAuditLogStream(ctx, "e", 1, *input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_DeleteAuditLogStream(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/audit-log/streams/1", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Enterprise.DeleteAuditLogStream(ctx, "e", 1)
	if err != nil {
		t.Errorf("Enterprise.DeleteAuditLogStream returned error: %v", err)
	}

	const methodName = "DeleteAuditLogStream"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Enterprise.DeleteAuditLogStream(ctx, "\n", 1)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Enterprise.DeleteAuditLogStream(ctx, "e", 1)
	})
}

func TestNewAzureBlobStreamConfig(t *testing.T) {
	t.Parallel()
	cfg := &AzureBlobConfig{
		KeyID:           "v1",
		EncryptedSASURL: "ENCRYPTED",
		Container:       "my-container",
	}

	got := NewAzureBlobStreamConfig(true, cfg)

	want := &AuditLogStreamConfig{
		Enabled:        true,
		StreamType:     "Azure Blob Storage",
		VendorSpecific: cfg,
	}
	if !cmp.Equal(got, want) {
		t.Errorf("NewAzureBlobStreamConfig = %+v, want %+v", got, want)
	}
}

func TestNewAzureHubStreamConfig(t *testing.T) {
	t.Parallel()
	cfg := &AzureHubConfig{
		Name:                "my-hub",
		EncryptedConnstring: "ENCRYPTED",
		KeyID:               "v1",
	}

	got := NewAzureHubStreamConfig(true, cfg)

	want := &AuditLogStreamConfig{
		Enabled:        true,
		StreamType:     "Azure Event Hubs",
		VendorSpecific: cfg,
	}
	if !cmp.Equal(got, want) {
		t.Errorf("NewAzureHubStreamConfig = %+v, want %+v", got, want)
	}
}

func TestNewAmazonS3OIDCStreamConfig(t *testing.T) {
	t.Parallel()
	cfg := &AmazonS3OIDCConfig{
		Bucket:             "my-bucket",
		Region:             "us-east-1",
		KeyID:              "v1",
		AuthenticationType: "oidc",
		ArnRole:            "arn:aws:iam::role/my-role",
	}

	got := NewAmazonS3OIDCStreamConfig(true, cfg)

	want := &AuditLogStreamConfig{
		Enabled:        true,
		StreamType:     "Amazon S3",
		VendorSpecific: cfg,
	}
	if !cmp.Equal(got, want) {
		t.Errorf("NewAmazonS3OIDCStreamConfig = %+v, want %+v", got, want)
	}
}

func TestNewAmazonS3AccessKeysStreamConfig(t *testing.T) {
	t.Parallel()
	cfg := &AmazonS3AccessKeysConfig{
		Bucket:               "my-bucket",
		Region:               "us-west-2",
		KeyID:                "v1",
		AuthenticationType:   "access_keys",
		EncryptedSecretKey:   "ENCRYPTED_SECRET",
		EncryptedAccessKeyID: "ENCRYPTED_KEY_ID",
	}

	got := NewAmazonS3AccessKeysStreamConfig(false, cfg)

	want := &AuditLogStreamConfig{
		Enabled:        false,
		StreamType:     "Amazon S3",
		VendorSpecific: cfg,
	}
	if !cmp.Equal(got, want) {
		t.Errorf("NewAmazonS3AccessKeysStreamConfig = %+v, want %+v", got, want)
	}
}

func TestNewSplunkStreamConfig(t *testing.T) {
	t.Parallel()
	cfg := &SplunkConfig{
		Domain:         "splunk.example.com",
		Port:           8089,
		KeyID:          "v1",
		EncryptedToken: "ENCRYPTED",
		SSLVerify:      true,
	}

	got := NewSplunkStreamConfig(true, cfg)

	want := &AuditLogStreamConfig{
		Enabled:        true,
		StreamType:     "Splunk",
		VendorSpecific: cfg,
	}
	if !cmp.Equal(got, want) {
		t.Errorf("NewSplunkStreamConfig = %+v, want %+v", got, want)
	}
}

func TestNewHecStreamConfig(t *testing.T) {
	t.Parallel()
	cfg := &HecConfig{
		Domain:         "hec.example.com",
		Port:           443,
		KeyID:          "v1",
		EncryptedToken: "ENCRYPTED",
		Path:           "/services/collector",
		SSLVerify:      true,
	}

	got := NewHecStreamConfig(false, cfg)

	want := &AuditLogStreamConfig{
		Enabled:        false,
		StreamType:     "HTTPS Event Collector",
		VendorSpecific: cfg,
	}
	if !cmp.Equal(got, want) {
		t.Errorf("NewHecStreamConfig = %+v, want %+v", got, want)
	}
}

func TestNewGoogleCloudStreamConfig(t *testing.T) {
	t.Parallel()
	cfg := &GoogleCloudConfig{
		Bucket:                   "my-gcs-bucket",
		KeyID:                    "v1",
		EncryptedJSONCredentials: "ENCRYPTED",
	}

	got := NewGoogleCloudStreamConfig(true, cfg)

	want := &AuditLogStreamConfig{
		Enabled:        true,
		StreamType:     "Google Cloud Storage",
		VendorSpecific: cfg,
	}
	if !cmp.Equal(got, want) {
		t.Errorf("NewGoogleCloudStreamConfig = %+v, want %+v", got, want)
	}
}

func TestNewDatadogStreamConfig(t *testing.T) {
	t.Parallel()
	cfg := &DatadogConfig{
		EncryptedToken: "ENCRYPTED",
		Site:           "US",
		KeyID:          "v1",
	}

	got := NewDatadogStreamConfig(false, cfg)

	want := &AuditLogStreamConfig{
		Enabled:        false,
		StreamType:     "Datadog",
		VendorSpecific: cfg,
	}
	if !cmp.Equal(got, want) {
		t.Errorf("NewDatadogStreamConfig = %+v, want %+v", got, want)
	}
}
