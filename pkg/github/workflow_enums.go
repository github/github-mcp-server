package github

import "slices"

// WorkflowStatus is a GitHub Actions workflow-run status.
type WorkflowStatus string

var workflowStatusValues = []string{
	"queued",
	"in_progress",
	"completed",
	"requested",
	"waiting",
	"pending",
}

// Values returns the supported workflow statuses.
func (WorkflowStatus) Values() []string {
	return slices.Clone(workflowStatusValues)
}

// WorkflowStatusValues returns the supported workflow statuses.
func WorkflowStatusValues() []string {
	return (WorkflowStatus("")).Values()
}

// WorkflowConclusion is a GitHub Actions workflow-run conclusion.
type WorkflowConclusion string

var workflowConclusionValues = []string{
	"success",
	"failure",
	"neutral",
	"cancelled",
	"skipped",
	"timed_out",
	"action_required",
	"stale",
	"startup_failure",
}

// Values returns the supported workflow conclusions.
func (WorkflowConclusion) Values() []string {
	return slices.Clone(workflowConclusionValues)
}

// WorkflowConclusionValues returns the supported workflow conclusions.
func WorkflowConclusionValues() []string {
	return (WorkflowConclusion("")).Values()
}
