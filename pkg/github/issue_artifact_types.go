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

// IssueArtifactReadInput selects a list or get operation on an issue's artifacts.
type IssueArtifactReadInput struct {
	IssueArtifactCoordinates
	Method         string `json:"method" jsonschema:"Read operation: list or get"`
	ArtifactNumber int    `json:"artifact_number,omitempty" jsonschema:"Required for get: positive issue-scoped artifact number, not its global ID"`
	Type           string `json:"type,omitempty" jsonschema:"Optional artifact type filter for list only"`
	Page           int    `json:"page,omitempty" jsonschema:"For list only: page number, starting at 1 (default 1)"`
	PerPage        int    `json:"perPage,omitempty" jsonschema:"For list only: results per page, from 1 to 100 (default 30)"`
}

// IssueArtifactWriteInput selects a create, update, or delete operation.
type IssueArtifactWriteInput struct {
	IssueArtifactCoordinates
	Method         string  `json:"method" jsonschema:"Write operation: create, update, or delete"`
	ArtifactNumber int     `json:"artifact_number,omitempty" jsonschema:"Required for update and delete: positive issue-scoped artifact number, not its global ID"`
	Type           string  `json:"type,omitempty" jsonschema:"Required for create only: artifact type; immutable after creation"`
	Name           *string `json:"name,omitempty" jsonschema:"Required for create, optional for update: nonempty artifact name"`
	Body           *string `json:"body,omitempty" jsonschema:"Required for create, optional for update: Markdown body; an empty string clears it"`
}

// IssueArtifactReadOutput contains the result of the selected read method.
type IssueArtifactReadOutput struct {
	Artifact *IssueArtifact            `json:"artifact,omitempty" jsonschema:"Artifact returned by get"`
	List     *ListIssueArtifactsOutput `json:"list,omitempty" jsonschema:"Page of artifacts returned by list"`
}

// IssueArtifactWriteOutput contains the result of the selected write method.
type IssueArtifactWriteOutput struct {
	Artifact *IssueArtifact             `json:"artifact,omitempty" jsonschema:"Artifact returned by create or update"`
	Deleted  *DeleteIssueArtifactOutput `json:"deleted,omitempty" jsonschema:"Confirmation returned by delete"`
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
