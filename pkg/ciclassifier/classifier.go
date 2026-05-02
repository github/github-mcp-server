package ciclassifier

import "strings"

// Check contains normalized check fields required for CI state classification.
type Check struct {
	Status     string
	Conclusion string
	Name       string
	Context    string
	Details    string
	Title      string
	Summary    string
	Text       string
}

// Classify maps raw checks into deterministic triage categories.
func Classify(checks []Check) string {
	if len(checks) == 0 {
		return "no checks"
	}

	pending := false
	failed := false
	policyBlocked := false

	for _, check := range checks {
		status := strings.ToLower(check.Status)
		conclusion := strings.ToLower(check.Conclusion)
		summary := strings.ToLower(strings.Join([]string{
			check.Name,
			check.Context,
			check.Details,
			check.Title,
			check.Summary,
			check.Text,
		}, " "))

		if strings.Contains(summary, "resource not accessible by integration") ||
			strings.Contains(summary, "insufficient permission") ||
			strings.Contains(summary, "insufficient permissions") ||
			strings.Contains(summary, "not authorized") ||
			strings.Contains(summary, "forbidden") ||
			strings.Contains(summary, "cla") {
			policyBlocked = true
		}

		switch status {
		case "pending", "queued", "in_progress", "requested", "waiting":
			pending = true
		}

		switch conclusion {
		case "failure", "timed_out", "cancelled", "action_required", "startup_failure", "stale":
			failed = true
		}
	}

	if policyBlocked {
		return "policy-blocked"
	}
	if failed {
		return "failed"
	}
	if pending {
		return "pending"
	}
	return "passed"
}
