// Copyright 2026 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// AuditLogStream represents an audit log stream configuration for an enterprise.
type AuditLogStream struct {
	ID            int64      `json:"id"`
	StreamType    string     `json:"stream_type"`
	StreamDetails string     `json:"stream_details"`
	Enabled       bool       `json:"enabled"`
	CreatedAt     Timestamp  `json:"created_at"`
	UpdatedAt     Timestamp  `json:"updated_at"`
	PausedAt      *Timestamp `json:"paused_at,omitempty"`
}

// AuditLogStreamConfig represents a configuration for creating or updating an audit log stream.
type AuditLogStreamConfig struct {
	Enabled        bool                       `json:"enabled"`
	StreamType     string                     `json:"stream_type"`
	VendorSpecific AuditLogStreamVendorConfig `json:"vendor_specific"`
}

// AuditLogStreamVendorConfig is a sealed marker interface for vendor-specific audit log
// stream configurations. Only this package can define implementations.
type AuditLogStreamVendorConfig interface {
	isAuditLogStreamVendorConfig()
}

// AuditLogStreamKey represents the public key used to encrypt secrets for audit log streaming.
type AuditLogStreamKey struct {
	KeyID string `json:"key_id"`
	Key   string `json:"key"`
}

// AzureBlobConfig represents vendor-specific config for Azure Blob Storage.
type AzureBlobConfig struct {
	KeyID           string `json:"key_id"`
	EncryptedSASURL string `json:"encrypted_sas_url"`
	Container       string `json:"container"`
}

// AzureHubConfig represents vendor-specific config for Azure Event Hubs.
type AzureHubConfig struct {
	Name                string `json:"name"`
	EncryptedConnstring string `json:"encrypted_connstring"`
	KeyID               string `json:"key_id"`
}

// AmazonS3OIDCConfig represents vendor-specific config for Amazon S3 with OIDC authentication.
type AmazonS3OIDCConfig struct {
	Bucket             string `json:"bucket"`
	Region             string `json:"region"`
	KeyID              string `json:"key_id"`
	AuthenticationType string `json:"authentication_type"` // Value: "oidc"
	ArnRole            string `json:"arn_role"`
}

// AmazonS3AccessKeysConfig represents vendor-specific config for Amazon S3 with access key authentication.
type AmazonS3AccessKeysConfig struct {
	Bucket               string `json:"bucket"`
	Region               string `json:"region"`
	KeyID                string `json:"key_id"`
	AuthenticationType   string `json:"authentication_type"` // Value: "access_keys"
	EncryptedSecretKey   string `json:"encrypted_secret_key"`
	EncryptedAccessKeyID string `json:"encrypted_access_key_id"`
}

// SplunkConfig represents vendor-specific config for Splunk.
type SplunkConfig struct {
	Domain         string `json:"domain"`
	Port           uint16 `json:"port"`
	KeyID          string `json:"key_id"`
	EncryptedToken string `json:"encrypted_token"`
	SSLVerify      bool   `json:"ssl_verify"`
}

// HecConfig represents vendor-specific config for an HTTPS Event Collector (HEC) endpoint.
type HecConfig struct {
	Domain         string `json:"domain"`
	Port           uint16 `json:"port"`
	KeyID          string `json:"key_id"`
	EncryptedToken string `json:"encrypted_token"`
	Path           string `json:"path"`
	SSLVerify      bool   `json:"ssl_verify"`
}

// GoogleCloudConfig represents vendor-specific config for Google Cloud Storage.
type GoogleCloudConfig struct {
	Bucket                   string `json:"bucket"`
	KeyID                    string `json:"key_id"`
	EncryptedJSONCredentials string `json:"encrypted_json_credentials"`
}

// DatadogConfig represents vendor-specific config for Datadog.
type DatadogConfig struct {
	EncryptedToken string `json:"encrypted_token"`
	Site           string `json:"site"` // One of: US, US3, US5, EU1, US1-FED, AP1
	KeyID          string `json:"key_id"`
}

// Implement the sealed marker interface for all vendor config types.
func (*AzureBlobConfig) isAuditLogStreamVendorConfig()          {}
func (*AzureHubConfig) isAuditLogStreamVendorConfig()           {}
func (*AmazonS3OIDCConfig) isAuditLogStreamVendorConfig()       {}
func (*AmazonS3AccessKeysConfig) isAuditLogStreamVendorConfig() {}
func (*SplunkConfig) isAuditLogStreamVendorConfig()             {}
func (*HecConfig) isAuditLogStreamVendorConfig()                {}
func (*GoogleCloudConfig) isAuditLogStreamVendorConfig()        {}
func (*DatadogConfig) isAuditLogStreamVendorConfig()            {}

// NewAzureBlobStreamConfig returns an AuditLogStreamConfig for Azure Blob Storage.
func NewAzureBlobStreamConfig(enabled bool, cfg *AzureBlobConfig) *AuditLogStreamConfig {
	return &AuditLogStreamConfig{Enabled: enabled, StreamType: "Azure Blob Storage", VendorSpecific: cfg}
}

// NewAzureHubStreamConfig returns an AuditLogStreamConfig for Azure Event Hubs.
func NewAzureHubStreamConfig(enabled bool, cfg *AzureHubConfig) *AuditLogStreamConfig {
	return &AuditLogStreamConfig{Enabled: enabled, StreamType: "Azure Event Hubs", VendorSpecific: cfg}
}

// NewAmazonS3OIDCStreamConfig returns an AuditLogStreamConfig for Amazon S3 with OIDC auth.
func NewAmazonS3OIDCStreamConfig(enabled bool, cfg *AmazonS3OIDCConfig) *AuditLogStreamConfig {
	return &AuditLogStreamConfig{Enabled: enabled, StreamType: "Amazon S3", VendorSpecific: cfg}
}

// NewAmazonS3AccessKeysStreamConfig returns an AuditLogStreamConfig for Amazon S3 with access key auth.
func NewAmazonS3AccessKeysStreamConfig(enabled bool, cfg *AmazonS3AccessKeysConfig) *AuditLogStreamConfig {
	return &AuditLogStreamConfig{Enabled: enabled, StreamType: "Amazon S3", VendorSpecific: cfg}
}

// NewSplunkStreamConfig returns an AuditLogStreamConfig for Splunk.
func NewSplunkStreamConfig(enabled bool, cfg *SplunkConfig) *AuditLogStreamConfig {
	return &AuditLogStreamConfig{Enabled: enabled, StreamType: "Splunk", VendorSpecific: cfg}
}

// NewHecStreamConfig returns an AuditLogStreamConfig for an HTTPS Event Collector endpoint.
func NewHecStreamConfig(enabled bool, cfg *HecConfig) *AuditLogStreamConfig {
	return &AuditLogStreamConfig{Enabled: enabled, StreamType: "HTTPS Event Collector", VendorSpecific: cfg}
}

// NewGoogleCloudStreamConfig returns an AuditLogStreamConfig for Google Cloud Storage.
func NewGoogleCloudStreamConfig(enabled bool, cfg *GoogleCloudConfig) *AuditLogStreamConfig {
	return &AuditLogStreamConfig{Enabled: enabled, StreamType: "Google Cloud Storage", VendorSpecific: cfg}
}

// NewDatadogStreamConfig returns an AuditLogStreamConfig for Datadog.
func NewDatadogStreamConfig(enabled bool, cfg *DatadogConfig) *AuditLogStreamConfig {
	return &AuditLogStreamConfig{Enabled: enabled, StreamType: "Datadog", VendorSpecific: cfg}
}

// GetAuditLogStreamKey retrieves the public key used to encrypt secrets for audit log streaming.
// Credentials must be encrypted with this key before being submitted via CreateAuditLogStream
// or UpdateAuditLogStream.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/enterprise-admin/audit-log?apiVersion=2022-11-28#get-the-audit-log-stream-key-for-encrypting-secrets
//
//meta:operation GET /enterprises/{enterprise}/audit-log/stream-key
func (s *EnterpriseService) GetAuditLogStreamKey(ctx context.Context, enterprise string) (*AuditLogStreamKey, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/audit-log/stream-key", enterprise)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var key *AuditLogStreamKey
	resp, err := s.client.Do(req, &key)
	if err != nil {
		return nil, resp, err
	}

	return key, resp, nil
}

// ListAuditLogStreams lists the audit log stream configurations for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/enterprise-admin/audit-log?apiVersion=2022-11-28#list-audit-log-stream-configurations-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/audit-log/streams
func (s *EnterpriseService) ListAuditLogStreams(ctx context.Context, enterprise string) ([]*AuditLogStream, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/audit-log/streams", enterprise)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var streams []*AuditLogStream
	resp, err := s.client.Do(req, &streams)
	if err != nil {
		return nil, resp, err
	}

	return streams, resp, nil
}

// GetAuditLogStream gets a single audit log stream configuration for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/enterprise-admin/audit-log?apiVersion=2022-11-28#list-one-audit-log-streaming-configuration-via-a-stream-id
//
//meta:operation GET /enterprises/{enterprise}/audit-log/streams/{stream_id}
func (s *EnterpriseService) GetAuditLogStream(ctx context.Context, enterprise string, streamID int64) (*AuditLogStream, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/audit-log/streams/%v", enterprise, streamID)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var stream *AuditLogStream
	resp, err := s.client.Do(req, &stream)
	if err != nil {
		return nil, resp, err
	}

	return stream, resp, nil
}

// CreateAuditLogStream creates an audit log streaming configuration for an enterprise.
// Credentials in the config must be encrypted using the key returned by GetAuditLogStreamKey.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/enterprise-admin/audit-log?apiVersion=2022-11-28#create-an-audit-log-streaming-configuration-for-an-enterprise
//
//meta:operation POST /enterprises/{enterprise}/audit-log/streams
func (s *EnterpriseService) CreateAuditLogStream(ctx context.Context, enterprise string, body AuditLogStreamConfig) (*AuditLogStream, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/audit-log/streams", enterprise)

	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var stream *AuditLogStream
	resp, err := s.client.Do(req, &stream)
	if err != nil {
		return nil, resp, err
	}

	return stream, resp, nil
}

// UpdateAuditLogStream updates an existing audit log stream configuration for an enterprise.
// Credentials in the config must be encrypted using the key returned by GetAuditLogStreamKey.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/enterprise-admin/audit-log?apiVersion=2022-11-28#update-an-existing-audit-log-stream-configuration
//
//meta:operation PUT /enterprises/{enterprise}/audit-log/streams/{stream_id}
func (s *EnterpriseService) UpdateAuditLogStream(ctx context.Context, enterprise string, streamID int64, body AuditLogStreamConfig) (*AuditLogStream, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/audit-log/streams/%v", enterprise, streamID)

	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, nil, err
	}

	var stream *AuditLogStream
	resp, err := s.client.Do(req, &stream)
	if err != nil {
		return nil, resp, err
	}

	return stream, resp, nil
}

// DeleteAuditLogStream deletes an audit log stream configuration for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/enterprise-admin/audit-log?apiVersion=2022-11-28#delete-an-audit-log-streaming-configuration-for-an-enterprise
//
//meta:operation DELETE /enterprises/{enterprise}/audit-log/streams/{stream_id}
func (s *EnterpriseService) DeleteAuditLogStream(ctx context.Context, enterprise string, streamID int64) (*Response, error) {
	u := fmt.Sprintf("enterprises/%v/audit-log/streams/%v", enterprise, streamID)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
