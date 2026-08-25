package github

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-github/v89/github"
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
			TotalCount:   github.Ptr(2),
			WorkflowRuns: []*github.WorkflowRun{actionsTestWorkflowRun(), nil},
		})
		assert.Equal(t, 2, result.TotalCount)
		assert.Len(t, result.WorkflowRuns, 1)
	})

	t.Run("workflow jobs", func(t *testing.T) {
		result := convertToMinimalWorkflowJobs(&github.Jobs{
			TotalCount: github.Ptr(2),
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
		ID:          github.Ptr(int64(1296269)),
		NodeID:      github.Ptr("MDEwOlJlcG9zaXRvcnkxMjk2MjY5"),
		Name:        github.Ptr("octo-repo"),
		FullName:    github.Ptr("octo-org/octo-repo"),
		Description: github.Ptr("A representative repository description included in the full API response."),
		HTMLURL:     github.Ptr("https://github.com/octo-org/octo-repo"),
		URL:         github.Ptr("https://api.github.com/repos/octo-org/octo-repo"),
		CloneURL:    github.Ptr("https://github.com/octo-org/octo-repo.git"),
		Language:    github.Ptr("Go"),
		Topics:      []string{"actions", "mcp", "automation"},
	}

	return &github.WorkflowRun{
		ID:                 github.Ptr(int64(30433642)),
		Name:               github.Ptr("CI"),
		NodeID:             github.Ptr("MDEyOldvcmtmbG93IFJ1bjI2OTI4OQ=="),
		HeadBranch:         github.Ptr("feature/minimal-actions"),
		HeadSHA:            github.Ptr("acb5820ced9479c074f688cc328bf03f341a511d"),
		Path:               github.Ptr(".github/workflows/ci.yml"),
		RunNumber:          github.Ptr(562),
		RunAttempt:         github.Ptr(2),
		Event:              github.Ptr("pull_request"),
		DisplayTitle:       github.Ptr("Reduce GitHub Actions response payloads"),
		Status:             github.Ptr("completed"),
		Conclusion:         github.Ptr("failure"),
		WorkflowID:         github.Ptr(int64(161335)),
		CheckSuiteID:       github.Ptr(int64(42)),
		CheckSuiteNodeID:   github.Ptr("MDEwOkNoZWNrU3VpdGU0Mg=="),
		URL:                github.Ptr("https://api.github.com/repos/octo-org/octo-repo/actions/runs/30433642"),
		HTMLURL:            github.Ptr("https://github.com/octo-org/octo-repo/actions/runs/30433642"),
		JobsURL:            github.Ptr("https://api.github.com/repos/octo-org/octo-repo/actions/runs/30433642/jobs"),
		LogsURL:            github.Ptr("https://api.github.com/repos/octo-org/octo-repo/actions/runs/30433642/logs"),
		CheckSuiteURL:      github.Ptr("https://api.github.com/repos/octo-org/octo-repo/check-suites/42"),
		ArtifactsURL:       github.Ptr("https://api.github.com/repos/octo-org/octo-repo/actions/runs/30433642/artifacts"),
		CancelURL:          github.Ptr("https://api.github.com/repos/octo-org/octo-repo/actions/runs/30433642/cancel"),
		RerunURL:           github.Ptr("https://api.github.com/repos/octo-org/octo-repo/actions/runs/30433642/rerun"),
		PreviousAttemptURL: github.Ptr("https://api.github.com/repos/octo-org/octo-repo/actions/runs/30433642/attempts/1"),
		WorkflowURL:        github.Ptr("https://api.github.com/repos/octo-org/octo-repo/actions/workflows/161335"),
		Repository:         repository,
		HeadRepository:     repository,
		Actor: &github.User{
			Login:     github.Ptr("octocat"),
			ID:        github.Ptr(int64(1)),
			NodeID:    github.Ptr("MDQ6VXNlcjE="),
			AvatarURL: github.Ptr("https://github.com/images/error/octocat_happy.gif"),
			HTMLURL:   github.Ptr("https://github.com/octocat"),
			URL:       github.Ptr("https://api.github.com/users/octocat"),
			Name:      github.Ptr("The Octocat"),
			Bio:       github.Ptr("A long biography that is not needed to identify the workflow run actor."),
		},
		TriggeringActor: &github.User{
			Login:   github.Ptr("hubot"),
			ID:      github.Ptr(int64(2)),
			HTMLURL: github.Ptr("https://github.com/hubot"),
			URL:     github.Ptr("https://api.github.com/users/hubot"),
		},
		PullRequests: []*github.PullRequest{
			{
				ID:      github.Ptr(int64(1001)),
				Number:  github.Ptr(42),
				Title:   github.Ptr("Reduce GitHub Actions response payloads"),
				Body:    github.Ptr("A pull request body that is unnecessary in a workflow run response."),
				HTMLURL: github.Ptr("https://github.com/octo-org/octo-repo/pull/42"),
				Head: &github.PullRequestBranch{
					Ref:  github.Ptr("feature/minimal-actions"),
					SHA:  github.Ptr("acb5820ced9479c074f688cc328bf03f341a511d"),
					Repo: repository,
				},
				Base: &github.PullRequestBranch{
					Ref:  github.Ptr("main"),
					SHA:  github.Ptr("9a2f3ec"),
					Repo: repository,
				},
			},
		},
		HeadCommit: &github.HeadCommit{
			Message: github.Ptr("Reduce GitHub Actions response payloads"),
			URL:     github.Ptr("https://api.github.com/repos/octo-org/octo-repo/commits/acb5820"),
			Author: &github.CommitAuthor{
				Name:  github.Ptr("The Octocat"),
				Email: github.Ptr("octocat@example.com"),
			},
		},
		ReferencedWorkflows: []*github.ReferencedWorkflow{
			{
				Path: github.Ptr(".github/workflows/reusable-tests.yml"),
				SHA:  github.Ptr("9f4f87d9790ab0f5c2c5ad2b74b886cab515a886"),
				Ref:  github.Ptr("refs/tags/v3"),
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
		ID:              github.Ptr(int64(399444496)),
		RunID:           github.Ptr(int64(30433642)),
		RunURL:          github.Ptr("https://api.github.com/repos/octo-org/octo-repo/actions/runs/30433642"),
		NodeID:          github.Ptr("MDEyOldvcmtmbG93IEpvYjM5OTQ0NDQ5Ng=="),
		HeadBranch:      github.Ptr("feature/minimal-actions"),
		HeadSHA:         github.Ptr("acb5820ced9479c074f688cc328bf03f341a511d"),
		URL:             github.Ptr("https://api.github.com/repos/octo-org/octo-repo/actions/jobs/399444496"),
		HTMLURL:         github.Ptr("https://github.com/octo-org/octo-repo/runs/399444496"),
		Status:          github.Ptr("completed"),
		Conclusion:      github.Ptr("failure"),
		CreatedAt:       actionsTestTimestamp(),
		StartedAt:       actionsTestTimestamp(),
		CompletedAt:     actionsTestTimestamp(),
		Name:            github.Ptr("test (ubuntu-latest, Go 1.24)"),
		CheckRunURL:     github.Ptr("https://api.github.com/repos/octo-org/octo-repo/check-runs/399444496"),
		Labels:          []string{"ubuntu-latest", "x64"},
		RunnerID:        github.Ptr(int64(1)),
		RunnerName:      github.Ptr("GitHub Actions 1"),
		RunnerGroupID:   github.Ptr(int64(2)),
		RunnerGroupName: github.Ptr("GitHub Actions"),
		RunAttempt:      github.Ptr(int64(2)),
		WorkflowName:    github.Ptr("CI"),
		Steps: []*github.TaskStep{
			{
				Name:        github.Ptr("Set up job"),
				Status:      github.Ptr("completed"),
				Conclusion:  github.Ptr("success"),
				Number:      github.Ptr(int64(1)),
				StartedAt:   actionsTestTimestamp(),
				CompletedAt: actionsTestTimestamp(),
			},
			{
				Name:        github.Ptr("Run tests"),
				Status:      github.Ptr("completed"),
				Conclusion:  github.Ptr("failure"),
				Number:      github.Ptr(int64(2)),
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
