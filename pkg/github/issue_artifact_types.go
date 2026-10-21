package github

// IssueArtifact identifies a Markdown artifact by both its global ID and its
// issue-scoped number. Requests use Number, never ID.
type IssueArtifact struct {
	ID     int64  `json:"id"`
	Number int    `json:"number"`
	Type   string `json:"type"`
	Name   string `json:"name"`
	Body   string `json:"body"`
}

// IssueArtifactCoordinates identifies the issue that owns an artifact.
type IssueArtifactCoordinates struct {
	Owner       string `json:"owner" jsonschema:"Repository owner"`
	Repo        string `json:"repo" jsonschema:"Repository name"`
	IssueNumber int    `json:"issue_number" jsonschema:"Positive issue number"`
}

// CreateIssueArtifactInput creates an artifact, including an optionally empty body.
type CreateIssueArtifactInput struct {
	IssueArtifactCoordinates
	Type string `json:"type" jsonschema:"Artifact type"`
	Name string `json:"name" jsonschema:"Nonempty artifact name"`
	Body string `json:"body" jsonschema:"Markdown body; may be empty"`
}

// UpdateIssueArtifactInput preserves omitted fields; an empty body clears it.
// The artifact's type cannot be changed.
type UpdateIssueArtifactInput struct {
	IssueArtifactCoordinates
	ArtifactNumber int     `json:"artifact_number" jsonschema:"Positive issue-scoped artifact number, not its global ID"`
	Name           *string `json:"name,omitempty" jsonschema:"Replacement nonempty name"`
	Body           *string `json:"body,omitempty" jsonschema:"Replacement Markdown body; an empty string clears it"`
}

// GetIssueArtifactInput addresses one artifact for reading or deletion.
type GetIssueArtifactInput struct {
	IssueArtifactCoordinates
	ArtifactNumber int `json:"artifact_number" jsonschema:"Positive issue-scoped artifact number, not its global ID"`
}

// ListIssueArtifactsInput selects an explicit page, without relying on Link headers.
type ListIssueArtifactsInput struct {
	IssueArtifactCoordinates
	Type    string `json:"type,omitempty" jsonschema:"Optional artifact type filter"`
	Page    int    `json:"page,omitempty" jsonschema:"Page number, starting at 1 (default 1)"`
	PerPage int    `json:"perPage,omitempty" jsonschema:"Results per page, from 1 to 100 (default 30)"`
}

// ListIssueArtifactsOutput includes bodies and the requested page coordinates.
// A full page may have a successor; callers can request the next explicit page.
type ListIssueArtifactsOutput struct {
	Artifacts []IssueArtifact `json:"artifacts"`
	Page      int             `json:"page"`
	PerPage   int             `json:"perPage"`
}

// DeleteIssueArtifactOutput confirms the issue-scoped artifact that was deleted.
type DeleteIssueArtifactOutput struct {
	IssueNumber    int    `json:"issue_number"`
	ArtifactNumber int    `json:"artifact_number"`
	Message        string `json:"message"`
}
