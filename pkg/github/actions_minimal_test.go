package github

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-github/v92/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertToMinimalWorkflowRun(t *testing.T) {
	workflowRun := actionsTestWorkflowRun()

	minimal := convertToMinimalWorkflowRun(workflowRun)

	assert.Equal(t, workflowRun.GetID(), minimal.ID)
	assert.Equal(t, workflowRun.GetWorkflowID(), minimal.WorkflowID)
	assert.Equal(t, workflowRun.GetDisplayTitle(), minimal.DisplayTitle)
	assert.Equal(t, workflowRun.GetHeadSHA(), minimal.HeadSHA)
	assert.Equal(t, []int{42}, minimal.PullRequests)
	require.NotNil(t, minimal.HeadCommit)
	assert.Equal(t, "Reduce GitHub Actions response payloads", minimal.HeadCommit.Message)
	require.Len(t, minimal.ReferencedWorkflows, 1)
	assert.Equal(t, ".github/workflows/reusable-tests.yml", minimal.ReferencedWorkflows[0].Path)
	assert.Equal(t, "refs/tags/v3", minimal.ReferencedWorkflows[0].Ref)
	assert.Equal(t, "9f4f87d9790ab0f5c2c5ad2b74b886cab515a886", minimal.ReferencedWorkflows[0].SHA)
	require.NotNil(t, minimal.Actor)
	assert.Equal(t, "octocat", minimal.Actor.Login)
	require.NotNil(t, minimal.TriggeringActor)
	assert.Equal(t, "hubot", minimal.TriggeringActor.Login)

	payload := marshalActionsObject(t, minimal)
	assert.NotContains(t, payload, "node_id")
	assert.NotContains(t, payload, "repository")
	assert.NotContains(t, payload, "head_repository")
	assert.NotContains(t, payload, "jobs_url")
	assert.NotContains(t, payload, "logs_url")
	assert.NotContains(t, payload, "artifacts_url")
	assert.Equal(t, map[string]any{
		"message": "Reduce GitHub Actions response payloads",
	}, payload["head_commit"])
	assert.Equal(t, []any{
		map[string]any{
			"path": ".github/workflows/reusable-tests.yml",
			"sha":  "9f4f87d9790ab0f5c2c5ad2b74b886cab515a886",
			"ref":  "refs/tags/v3",
		},
	}, payload["referenced_workflows"])
}

func TestConvertToMinimalWorkflowJob(t *testing.T) {
	workflowJob := actionsTestWorkflowJob()

	minimal := convertToMinimalWorkflowJob(workflowJob)

	assert.Equal(t, workflowJob.GetID(), minimal.ID)
	assert.Equal(t, workflowJob.GetRunID(), minimal.RunID)
	assert.Equal(t, workflowJob.GetRunnerID(), minimal.RunnerID)
	assert.Equal(t, workflowJob.GetRunnerName(), minimal.RunnerName)
	assert.Equal(t, workflowJob.GetRunnerGroupID(), minimal.RunnerGroupID)
	assert.Equal(t, workflowJob.GetRunnerGroupName(), minimal.RunnerGroupName)
	assert.Equal(t, workflowJob.GetLabels(), minimal.Labels)
	require.Len(t, minimal.Steps, 2)
	assert.Equal(t, "Run tests", minimal.Steps[1].Name)
	assert.Equal(t, "failure", minimal.Steps[1].Conclusion)

	payload := marshalActionsObject(t, minimal)
	assert.NotContains(t, payload, "node_id")
	assert.NotContains(t, payload, "url")
	assert.NotContains(t, payload, "run_url")
	assert.NotContains(t, payload, "check_run_url")
	assert.Equal(t, float64(1), payload["runner_id"])
	assert.Equal(t, float64(2), payload["runner_group_id"])
	assert.Equal(t, "GitHub Actions", payload["runner_group_name"])
}

func TestConvertToMinimalActionsLists(t *testing.T) {
	t.Run("workflow runs", func(t *testing.T) {
		result := convertToMinimalWorkflowRuns(&github.WorkflowRuns{
			TotalCount:   new(2),
			WorkflowRuns: []*github.WorkflowRun{actionsTestWorkflowRun(), nil},
		})
		assert.Equal(t, 2, result.TotalCount)
		assert.Len(t, result.WorkflowRuns, 1)
	})

	t.Run("workflow jobs", func(t *testing.T) {
		result := convertToMinimalWorkflowJobs(&github.Jobs{
			TotalCount: new(2),
			Jobs:       []*github.WorkflowJob{actionsTestWorkflowJob(), nil},
		})
		assert.Equal(t, 2, result.TotalCount)
		assert.Len(t, result.Jobs, 1)
	})

	t.Run("nil workflow runs", func(t *testing.T) {
		result := convertToMinimalWorkflowRuns(nil)
		assert.NotNil(t, result.WorkflowRuns)
		assert.Empty(t, result.WorkflowRuns)
	})

	t.Run("nil workflow jobs", func(t *testing.T) {
		result := convertToMinimalWorkflowJobs(nil)
		assert.NotNil(t, result.Jobs)
		assert.Empty(t, result.Jobs)
	})
}

func actionsTestWorkflowRun() *github.WorkflowRun {
	repository := &github.Repository{
		ID:          new(int64(1296269)),
		NodeID:      new("MDEwOlJlcG9zaXRvcnkxMjk2MjY5"),
		Name:        new("octo-repo"),
		FullName:    new("octo-org/octo-repo"),
		Description: new("A representative repository description included in the full API response."),
		HTMLURL:     new("https://github.com/octo-org/octo-repo"),
		URL:         new("https://api.github.com/repos/octo-org/octo-repo"),
		CloneURL:    new("https://github.com/octo-org/octo-repo.git"),
		Language:    new("Go"),
		Topics:      []string{"actions", "mcp", "automation"},
	}

	return &github.WorkflowRun{
		ID:                 new(int64(30433642)),
		Name:               new("CI"),
		NodeID:             new("MDEyOldvcmtmbG93IFJ1bjI2OTI4OQ=="),
		HeadBranch:         new("feature/minimal-actions"),
		HeadSHA:            new("acb5820ced9479c074f688cc328bf03f341a511d"),
		Path:               new(".github/workflows/ci.yml"),
		RunNumber:          new(562),
		RunAttempt:         new(2),
		Event:              new("pull_request"),
		DisplayTitle:       new("Reduce GitHub Actions response payloads"),
		Status:             new("completed"),
		Conclusion:         new("failure"),
		WorkflowID:         new(int64(161335)),
		CheckSuiteID:       new(int64(42)),
		CheckSuiteNodeID:   new("MDEwOkNoZWNrU3VpdGU0Mg=="),
		URL:                new("https://api.github.com/repos/octo-org/octo-repo/actions/runs/30433642"),
		HTMLURL:            new("https://github.com/octo-org/octo-repo/actions/runs/30433642"),
		JobsURL:            new("https://api.github.com/repos/octo-org/octo-repo/actions/runs/30433642/jobs"),
		LogsURL:            new("https://api.github.com/repos/octo-org/octo-repo/actions/runs/30433642/logs"),
		CheckSuiteURL:      new("https://api.github.com/repos/octo-org/octo-repo/check-suites/42"),
		ArtifactsURL:       new("https://api.github.com/repos/octo-org/octo-repo/actions/runs/30433642/artifacts"),
		CancelURL:          new("https://api.github.com/repos/octo-org/octo-repo/actions/runs/30433642/cancel"),
		RerunURL:           new("https://api.github.com/repos/octo-org/octo-repo/actions/runs/30433642/rerun"),
		PreviousAttemptURL: new("https://api.github.com/repos/octo-org/octo-repo/actions/runs/30433642/attempts/1"),
		WorkflowURL:        new("https://api.github.com/repos/octo-org/octo-repo/actions/workflows/161335"),
		Repository:         repository,
		HeadRepository:     repository,
		Actor: &github.User{
			Login:     new("octocat"),
			ID:        new(int64(1)),
			NodeID:    new("MDQ6VXNlcjE="),
			AvatarURL: new("https://github.com/images/error/octocat_happy.gif"),
			HTMLURL:   new("https://github.com/octocat"),
			URL:       new("https://api.github.com/users/octocat"),
			Name:      new("The Octocat"),
			Bio:       new("A long biography that is not needed to identify the workflow run actor."),
		},
		TriggeringActor: &github.User{
			Login:   new("hubot"),
			ID:      new(int64(2)),
			HTMLURL: new("https://github.com/hubot"),
			URL:     new("https://api.github.com/users/hubot"),
		},
		PullRequests: []*github.PullRequest{
			{
				ID:      new(int64(1001)),
				Number:  new(42),
				Title:   new("Reduce GitHub Actions response payloads"),
				Body:    new("A pull request body that is unnecessary in a workflow run response."),
				HTMLURL: new("https://github.com/octo-org/octo-repo/pull/42"),
				Head: &github.PullRequestBranch{
					Ref:  new("feature/minimal-actions"),
					SHA:  new("acb5820ced9479c074f688cc328bf03f341a511d"),
					Repo: repository,
				},
				Base: &github.PullRequestBranch{
					Ref:  new("main"),
					SHA:  new("9a2f3ec"),
					Repo: repository,
				},
			},
		},
		HeadCommit: &github.HeadCommit{
			Message: new("Reduce GitHub Actions response payloads"),
			URL:     new("https://api.github.com/repos/octo-org/octo-repo/commits/acb5820"),
			Author: &github.CommitAuthor{
				Name:  new("The Octocat"),
				Email: new("octocat@example.com"),
			},
		},
		ReferencedWorkflows: []*github.ReferencedWorkflow{
			{
				Path: new(".github/workflows/reusable-tests.yml"),
				SHA:  new("9f4f87d9790ab0f5c2c5ad2b74b886cab515a886"),
				Ref:  new("refs/tags/v3"),
			},
			nil,
		},
		CreatedAt:    actionsTestTimestamp(),
		UpdatedAt:    actionsTestTimestamp(),
		RunStartedAt: actionsTestTimestamp(),
	}
}

func actionsTestWorkflowJob() *github.WorkflowJob {
	return &github.WorkflowJob{
		ID:              new(int64(399444496)),
		RunID:           new(int64(30433642)),
		RunURL:          new("https://api.github.com/repos/octo-org/octo-repo/actions/runs/30433642"),
		NodeID:          new("MDEyOldvcmtmbG93IEpvYjM5OTQ0NDQ5Ng=="),
		HeadBranch:      new("feature/minimal-actions"),
		HeadSHA:         new("acb5820ced9479c074f688cc328bf03f341a511d"),
		URL:             new("https://api.github.com/repos/octo-org/octo-repo/actions/jobs/399444496"),
		HTMLURL:         new("https://github.com/octo-org/octo-repo/runs/399444496"),
		Status:          new("completed"),
		Conclusion:      new("failure"),
		CreatedAt:       actionsTestTimestamp(),
		StartedAt:       actionsTestTimestamp(),
		CompletedAt:     actionsTestTimestamp(),
		Name:            new("test (ubuntu-latest, Go 1.24)"),
		CheckRunURL:     new("https://api.github.com/repos/octo-org/octo-repo/check-runs/399444496"),
		Labels:          []string{"ubuntu-latest", "x64"},
		RunnerID:        new(int64(1)),
		RunnerName:      new("GitHub Actions 1"),
		RunnerGroupID:   new(int64(2)),
		RunnerGroupName: new("GitHub Actions"),
		RunAttempt:      new(int64(2)),
		WorkflowName:    new("CI"),
		Steps: []*github.TaskStep{
			{
				Name:        new("Set up job"),
				Status:      new("completed"),
				Conclusion:  new("success"),
				Number:      new(int64(1)),
				StartedAt:   actionsTestTimestamp(),
				CompletedAt: actionsTestTimestamp(),
			},
			{
				Name:        new("Run tests"),
				Status:      new("completed"),
				Conclusion:  new("failure"),
				Number:      new(int64(2)),
				StartedAt:   actionsTestTimestamp(),
				CompletedAt: actionsTestTimestamp(),
			},
		},
	}
}

func actionsTestTimestamp() *github.Timestamp {
	return &github.Timestamp{Time: time.Date(2026, time.August, 6, 10, 30, 0, 0, time.UTC)}
}

func marshalActionsObject(t *testing.T, value any) map[string]any {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)

	var object map[string]any
	require.NoError(t, json.Unmarshal(data, &object))
	return object
}
