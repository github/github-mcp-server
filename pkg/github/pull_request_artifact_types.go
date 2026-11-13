package github

import "encoding/json"

// PullRequestArtifact is an artifact or artifact version returned by the PR API.
type PullRequestArtifact struct {
	ID        json.RawMessage             `json:"id" jsonschema:"Opaque artifact ID; mutations use name instead"`
	Number    int                         `json:"number"`
	Name      string                      `json:"name"`
	Body      string                      `json:"body"`
	CommitOID *string                     `json:"commit_oid"`
	Creator   *PullRequestArtifactCreator `json:"creator"`
	CreatedAt *string                     `json:"created_at"`
	UpdatedAt *string                     `json:"updated_at"`
}

// PullRequestArtifactCreator identifies the actor who wrote an artifact version.
type PullRequestArtifactCreator struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
}

// PullRequestArtifactCoordinates identifies the target pull request.
type PullRequestArtifactCoordinates struct {
	Owner      string `json:"owner" jsonschema:"Repository owner"`
	Repo       string `json:"repo" jsonschema:"Repository name"`
	PullNumber int    `json:"pull_number" jsonschema:"Positive pull request number"`
}

// PullRequestArtifactReadInput selects a page of pull request artifact versions.
type PullRequestArtifactReadInput struct {
	PullRequestArtifactCoordinates
	Method    string `json:"method" jsonschema:"Read operation: list"`
	CommitOID string `json:"commit_oid,omitempty" jsonschema:"Commit selection: omitted or current for the current PR head, all for every commit, or an explicit commit OID"`
	Page      int    `json:"page,omitempty" jsonschema:"Page number, starting at 1 (default 1)"`
	PerPage   int    `json:"perPage,omitempty" jsonschema:"Results per page, from 1 to 100 (default 30)"`
}

// PullRequestArtifactWriteInput upserts or deletes an artifact by name.
type PullRequestArtifactWriteInput struct {
	PullRequestArtifactCoordinates
	Method    string  `json:"method" jsonschema:"Write operation: upsert or delete"`
	Name      string  `json:"name" jsonschema:"Artifact name, not its number or global ID"`
	Body      *string `json:"body,omitempty" jsonschema:"Required for upsert: artifact content; an empty string is sent unchanged. Not supported for delete"`
	CommitOID *string `json:"commit_oid,omitempty" jsonschema:"Optional commit OID for upsert only; omission preserves the API default"`
}

// PullRequestArtifactReadOutput contains a page of artifact versions.
type PullRequestArtifactReadOutput struct {
	List *ListPullRequestArtifactsOutput `json:"list"`
}

// ListPullRequestArtifactsOutput includes REST Link-based pagination.
type ListPullRequestArtifactsOutput struct {
	Artifacts []PullRequestArtifact `json:"artifacts"`
	Page      int                   `json:"page"`
	PerPage   int                   `json:"perPage"`
	NextPage  int                   `json:"next_page,omitempty" jsonschema:"Next page from the API Link header; absent when there is no next page"`
}

// PullRequestArtifactWriteOutput contains an upsert result or delete confirmation.
type PullRequestArtifactWriteOutput struct {
	Artifact *PullRequestArtifact             `json:"artifact,omitempty"`
	Deleted  *DeletePullRequestArtifactOutput `json:"deleted,omitempty"`
}

// DeletePullRequestArtifactOutput confirms the named artifact that was deleted.
type DeletePullRequestArtifactOutput struct {
	PullNumber int    `json:"pull_number"`
	Name       string `json:"name"`
	Message    string `json:"message"`
}
