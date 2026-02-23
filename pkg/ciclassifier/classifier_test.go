package ciclassifier

import "testing"

func TestClassifyNoChecks(t *testing.T) {
	if got := Classify(nil); got != "no checks" {
		t.Fatalf("expected no checks, got %q", got)
	}
}

func TestClassifyPending(t *testing.T) {
	checks := []Check{{Status: "in_progress"}}
	if got := Classify(checks); got != "pending" {
		t.Fatalf("expected pending, got %q", got)
	}
}

func TestClassifyFailed(t *testing.T) {
	checks := []Check{{Conclusion: "failure"}}
	if got := Classify(checks); got != "failed" {
		t.Fatalf("expected failed, got %q", got)
	}
}

func TestClassifyPolicyBlockedWins(t *testing.T) {
	checks := []Check{{Conclusion: "failure", Summary: "Resource not accessible by integration"}}
	if got := Classify(checks); got != "policy-blocked" {
		t.Fatalf("expected policy-blocked, got %q", got)
	}
}
