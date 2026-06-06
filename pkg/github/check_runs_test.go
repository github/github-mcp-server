package github

import (
	"testing"
	"time"

	"github.com/google/go-github/v87/github"
	"github.com/stretchr/testify/assert"
)

func Test_convertWorkflowRunToMinimalCheckRun(t *testing.T) {
	startedAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	updatedAt := time.Date(2026, 1, 2, 3, 10, 0, 0, time.UTC)

	run := &github.WorkflowRun{
		ID:           github.Ptr(int64(42)),
		Name:         github.Ptr("CI"),
		Status:       github.Ptr("completed"),
		Conclusion:   github.Ptr("failure"),
		HTMLURL:      github.Ptr("https://github.com/o/r/actions/runs/42"),
		RunStartedAt: &github.Timestamp{Time: startedAt},
		UpdatedAt:    &github.Timestamp{Time: updatedAt},
	}

	result := convertWorkflowRunToMinimalCheckRun(run)

	assert.Equal(t, int64(42), result.ID)
	assert.Equal(t, "CI", result.Name)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, "failure", result.Conclusion)
	assert.Equal(t, "https://github.com/o/r/actions/runs/42", result.HTMLURL)
	assert.Equal(t, "2026-01-02T03:04:05Z", result.StartedAt)
	assert.Equal(t, "2026-01-02T03:10:00Z", result.CompletedAt)
}

func Test_convertCommitStatusToMinimalCheckRun(t *testing.T) {
	createdAt := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 2, 1, 12, 5, 0, 0, time.UTC)

	status := &github.RepoStatus{
		ID:        github.Ptr(int64(9)),
		Context:   github.Ptr("ci/build"),
		State:     github.Ptr("success"),
		TargetURL: github.Ptr("https://ci.example.com/build/9"),
		CreatedAt: &github.Timestamp{Time: createdAt},
		UpdatedAt: &github.Timestamp{Time: updatedAt},
	}

	result := convertCommitStatusToMinimalCheckRun(status)

	assert.Equal(t, int64(9), result.ID)
	assert.Equal(t, "ci/build", result.Name)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, "success", result.Conclusion)
	assert.Equal(t, "https://ci.example.com/build/9", result.DetailsURL)
}

func Test_convertCommitStatusToMinimalCheckRun_pending(t *testing.T) {
	status := &github.RepoStatus{
		ID:      github.Ptr(int64(1)),
		Context: github.Ptr("ci/build"),
		State:   github.Ptr("pending"),
	}

	result := convertCommitStatusToMinimalCheckRun(status)

	assert.Equal(t, "in_progress", result.Status)
	assert.Empty(t, result.Conclusion)
}
