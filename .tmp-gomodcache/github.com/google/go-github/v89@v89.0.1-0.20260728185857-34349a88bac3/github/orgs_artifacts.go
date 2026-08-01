// Copyright 2026 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// DeploymentRuntimeRisk represents the runtime risk of a deployment.
type DeploymentRuntimeRisk string

const (
	DeploymentRuntimeRiskCriticalResource DeploymentRuntimeRisk = "critical-resource"
	DeploymentRuntimeRiskInternetExposed  DeploymentRuntimeRisk = "internet-exposed"
	DeploymentRuntimeRiskLateralMovement  DeploymentRuntimeRisk = "lateral-movement"
	DeploymentRuntimeRiskSensitiveData    DeploymentRuntimeRisk = "sensitive-data"
)

// ArtifactDeploymentRecord represents a GitHub artifact deployment record.
type ArtifactDeploymentRecord struct {
	ID                  *int64                  `json:"id,omitempty"`
	Digest              *string                 `json:"digest,omitempty"`
	LogicalEnvironment  *string                 `json:"logical_environment,omitempty"`
	PhysicalEnvironment *string                 `json:"physical_environment,omitempty"`
	Cluster             *string                 `json:"cluster,omitempty"`
	DeploymentName      *string                 `json:"deployment_name,omitempty"`
	Tags                map[string]string       `json:"tags,omitempty"`
	RuntimeRisks        []DeploymentRuntimeRisk `json:"runtime_risks,omitempty"`
	AttestationID       *int64                  `json:"attestation_id,omitempty"`
	CreatedAt           *Timestamp              `json:"created_at,omitempty"`
	UpdatedAt           *Timestamp              `json:"updated_at,omitempty"`
}

// CreateArtifactDeploymentRequest represents the request body for creating a deployment record.
type CreateArtifactDeploymentRequest struct {
	Name                string                  `json:"name"`
	Digest              string                  `json:"digest"`
	Version             *string                 `json:"version,omitempty"`
	Status              string                  `json:"status"`
	LogicalEnvironment  string                  `json:"logical_environment"`
	PhysicalEnvironment *string                 `json:"physical_environment,omitempty"`
	Cluster             *string                 `json:"cluster,omitempty"`
	DeploymentName      string                  `json:"deployment_name"`
	Tags                map[string]string       `json:"tags,omitempty"`
	RuntimeRisks        []DeploymentRuntimeRisk `json:"runtime_risks,omitempty"`
	GithubRepository    *string                 `json:"github_repository,omitempty"`
}

// ArtifactDeploymentResponse represents the response for deployment records.
type ArtifactDeploymentResponse struct {
	TotalCount        *int                        `json:"total_count,omitempty"`
	DeploymentRecords []*ArtifactDeploymentRecord `json:"deployment_records,omitempty"`
}

// ClusterArtifactDeployment represents a deployment within a cluster record request.
type ClusterArtifactDeployment struct {
	Name             string                  `json:"name"`
	Digest           string                  `json:"digest"`
	Version          *string                 `json:"version,omitempty"`
	Status           string                  `json:"status"`
	DeploymentName   string                  `json:"deployment_name"`
	Tags             map[string]string       `json:"tags,omitempty"`
	RuntimeRisks     []DeploymentRuntimeRisk `json:"runtime_risks,omitempty"`
	GithubRepository *string                 `json:"github_repository,omitempty"`
}

// ClusterDeploymentRecordsRequest represents the request body for setting cluster deployment records.
type ClusterDeploymentRecordsRequest struct {
	LogicalEnvironment  string                       `json:"logical_environment"`
	PhysicalEnvironment *string                      `json:"physical_environment,omitempty"`
	Deployments         []*ClusterArtifactDeployment `json:"deployments"`
}

// ArtifactStorageRecord represents a GitHub artifact storage record.
type ArtifactStorageRecord struct {
	ID          *int64     `json:"id,omitempty"`
	Name        *string    `json:"name,omitempty"`
	Digest      *string    `json:"digest,omitempty"`
	ArtifactURL *string    `json:"artifact_url,omitempty"`
	RegistryURL *string    `json:"registry_url,omitempty"`
	Repository  *string    `json:"repository,omitempty"`
	Status      *string    `json:"status,omitempty"`
	CreatedAt   *Timestamp `json:"created_at,omitempty"`
	UpdatedAt   *Timestamp `json:"updated_at,omitempty"`
}

// CreateArtifactStorageRequest represents the request body for creating a storage record.
type CreateArtifactStorageRequest struct {
	Name             string  `json:"name"`
	Digest           string  `json:"digest"`
	Version          *string `json:"version,omitempty"`
	ArtifactURL      *string `json:"artifact_url,omitempty"`
	Path             *string `json:"path,omitempty"`
	RegistryURL      string  `json:"registry_url"`
	Repository       *string `json:"repository,omitempty"`
	Status           *string `json:"status,omitempty"`
	GithubRepository *string `json:"github_repository,omitempty"`
}

// ArtifactStorageResponse represents the response for storage records.
type ArtifactStorageResponse struct {
	TotalCount     *int                     `json:"total_count,omitempty"`
	StorageRecords []*ArtifactStorageRecord `json:"storage_records,omitempty"`
}

// CreateArtifactDeploymentRecord creates or updates deployment records for an artifact associated with an organization.
//
// GitHub API docs: https://docs.github.com/rest/orgs/artifact-metadata?apiVersion=2022-11-28#create-an-artifact-deployment-record
//
//meta:operation POST /orgs/{org}/artifacts/metadata/deployment-record
func (s *OrganizationsService) CreateArtifactDeploymentRecord(ctx context.Context, org string, body CreateArtifactDeploymentRequest) (*ArtifactDeploymentResponse, *Response, error) {
	u := fmt.Sprintf("orgs/%v/artifacts/metadata/deployment-record", org)
	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var v *ArtifactDeploymentResponse
	resp, err := s.client.Do(req, &v)
	if err != nil {
		return nil, resp, err
	}

	return v, resp, nil
}

// SetClusterDeploymentRecords sets deployment records for a given cluster.
//
// GitHub API docs: https://docs.github.com/rest/orgs/artifact-metadata?apiVersion=2022-11-28#set-cluster-deployment-records
//
//meta:operation POST /orgs/{org}/artifacts/metadata/deployment-record/cluster/{cluster}
func (s *OrganizationsService) SetClusterDeploymentRecords(ctx context.Context, org, cluster string, body ClusterDeploymentRecordsRequest) (*ArtifactDeploymentResponse, *Response, error) {
	u := fmt.Sprintf("orgs/%v/artifacts/metadata/deployment-record/cluster/%v", org, cluster)
	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var v *ArtifactDeploymentResponse
	resp, err := s.client.Do(req, &v)
	if err != nil {
		return nil, resp, err
	}

	return v, resp, nil
}

// CreateArtifactStorageRecord creates metadata storage records for artifacts.
//
// GitHub API docs: https://docs.github.com/rest/orgs/artifact-metadata?apiVersion=2022-11-28#create-artifact-metadata-storage-record
//
//meta:operation POST /orgs/{org}/artifacts/metadata/storage-record
func (s *OrganizationsService) CreateArtifactStorageRecord(ctx context.Context, org string, body CreateArtifactStorageRequest) (*ArtifactStorageResponse, *Response, error) {
	u := fmt.Sprintf("orgs/%v/artifacts/metadata/storage-record", org)
	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var v *ArtifactStorageResponse
	resp, err := s.client.Do(req, &v)
	if err != nil {
		return nil, resp, err
	}

	return v, resp, nil
}

// ListArtifactDeploymentRecords lists deployment records for an artifact metadata.
//
// subjectDigest is SHA256 digest of the artifact, in the form sha256:HEX_DIGEST.
//
// GitHub API docs: https://docs.github.com/rest/orgs/artifact-metadata?apiVersion=2022-11-28#list-artifact-deployment-records
//
//meta:operation GET /orgs/{org}/artifacts/{subject_digest}/metadata/deployment-records
func (s *OrganizationsService) ListArtifactDeploymentRecords(ctx context.Context, org, subjectDigest string) (*ArtifactDeploymentResponse, *Response, error) {
	u := fmt.Sprintf("orgs/%v/artifacts/%v/metadata/deployment-records", org, subjectDigest)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var v *ArtifactDeploymentResponse
	resp, err := s.client.Do(req, &v)
	if err != nil {
		return nil, resp, err
	}

	return v, resp, nil
}

// ListArtifactStorageRecords lists artifact storage records with a given subject digest.
//
// subjectDigest is SHA256 digest of the artifact, in the form sha256:HEX_DIGEST.
//
// GitHub API docs: https://docs.github.com/rest/orgs/artifact-metadata?apiVersion=2022-11-28#list-artifact-storage-records
//
//meta:operation GET /orgs/{org}/artifacts/{subject_digest}/metadata/storage-records
func (s *OrganizationsService) ListArtifactStorageRecords(ctx context.Context, org, subjectDigest string) (*ArtifactStorageResponse, *Response, error) {
	u := fmt.Sprintf("orgs/%v/artifacts/%v/metadata/storage-records", org, subjectDigest)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var v *ArtifactStorageResponse
	resp, err := s.client.Do(req, &v)
	if err != nil {
		return nil, resp, err
	}

	return v, resp, nil
}
