package ifc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLabelSearchIssues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		visibilities     []bool
		wantConfidential Confidentiality
	}{
		{
			name:             "empty result is treated as public",
			wantConfidential: ConfidentialityPublic,
		},
		{
			name:             "single public repo",
			visibilities:     []bool{false},
			wantConfidential: ConfidentialityPublic,
		},
		{
			name:             "all public repos stay public",
			visibilities:     []bool{false, false, false},
			wantConfidential: ConfidentialityPublic,
		},
		{
			name:             "any private match flips to private",
			visibilities:     []bool{false, true, false},
			wantConfidential: ConfidentialityPrivate,
		},
		{
			name:             "all private repos stay private",
			visibilities:     []bool{true, true},
			wantConfidential: ConfidentialityPrivate,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			label := LabelSearchIssues(tc.visibilities)
			assert.Equal(t, IntegrityUntrusted, label.Integrity)
			assert.Equal(t, tc.wantConfidential, label.Confidentiality)
		})
	}
}

func TestLabelRepoMetadata(t *testing.T) {
	t.Parallel()

	t.Run("public repo metadata is trusted and public", func(t *testing.T) {
		t.Parallel()
		label := LabelRepoMetadata(false)
		assert.Equal(t, IntegrityTrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPublic, label.Confidentiality)
	})

	t.Run("private repo metadata is trusted and private", func(t *testing.T) {
		t.Parallel()
		label := LabelRepoMetadata(true)
		assert.Equal(t, IntegrityTrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPrivate, label.Confidentiality)
	})
}

func TestLabelGetMe(t *testing.T) {
	t.Parallel()

	// get_me exposes private_gists/total_private_repos/owned_private_repos,
	// which are not part of the public profile, so the result is trusted but
	// private — never public.
	label := LabelGetMe()
	assert.Equal(t, IntegrityTrusted, label.Integrity)
	assert.Equal(t, ConfidentialityPrivate, label.Confidentiality)
}

func TestLabelRelease(t *testing.T) {
	t.Parallel()

	t.Run("public repo with no draft is trusted and public", func(t *testing.T) {
		t.Parallel()
		label := LabelRelease(false, false)
		assert.Equal(t, IntegrityTrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPublic, label.Confidentiality)
	})

	t.Run("public repo with a draft release is private", func(t *testing.T) {
		t.Parallel()
		// Draft releases are visible only to push-access users, so a draft on
		// a public repo must not be labeled public.
		label := LabelRelease(false, true)
		assert.Equal(t, IntegrityTrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPrivate, label.Confidentiality)
	})

	t.Run("private repo is private regardless of draft", func(t *testing.T) {
		t.Parallel()
		for _, hasDraft := range []bool{false, true} {
			label := LabelRelease(true, hasDraft)
			assert.Equal(t, IntegrityTrusted, label.Integrity)
			assert.Equal(t, ConfidentialityPrivate, label.Confidentiality)
		}
	})
}

func TestLabelCollaboratorRoster(t *testing.T) {
	t.Parallel()

	// A collaborator roster requires push access to list, so it is never
	// world-readable — always trusted and private.
	label := LabelCollaboratorRoster()
	assert.Equal(t, IntegrityTrusted, label.Integrity)
	assert.Equal(t, ConfidentialityPrivate, label.Confidentiality)
}

func TestLabelCommitContents(t *testing.T) {
	t.Parallel()

	t.Run("public repo commit content is untrusted and public", func(t *testing.T) {
		t.Parallel()
		label := LabelCommitContents(false)
		assert.Equal(t, IntegrityUntrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPublic, label.Confidentiality)
	})

	t.Run("private repo commit content is trusted and private", func(t *testing.T) {
		t.Parallel()
		label := LabelCommitContents(true)
		assert.Equal(t, IntegrityTrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPrivate, label.Confidentiality)
	})
}

func TestLabelActionsResult(t *testing.T) {
	t.Parallel()

	t.Run("public repo actions result is untrusted and public", func(t *testing.T) {
		t.Parallel()
		label := LabelActionsResult(false)
		assert.Equal(t, IntegrityUntrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPublic, label.Confidentiality)
	})

	t.Run("private repo actions result is untrusted and private", func(t *testing.T) {
		t.Parallel()
		label := LabelActionsResult(true)
		assert.Equal(t, IntegrityUntrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPrivate, label.Confidentiality)
	})
}

func TestLabelSecurityAlert(t *testing.T) {
	t.Parallel()
	label := LabelSecurityAlert()
	assert.Equal(t, IntegrityUntrusted, label.Integrity)
	assert.Equal(t, ConfidentialityPrivate, label.Confidentiality,
		"security alerts are access-restricted regardless of repo visibility")
}

func TestLabelGlobalSecurityAdvisory(t *testing.T) {
	t.Parallel()
	label := LabelGlobalSecurityAdvisory()
	assert.Equal(t, IntegrityUntrusted, label.Integrity)
	assert.Equal(t, ConfidentialityPublic, label.Confidentiality)
}

func TestLabelRepositorySecurityAdvisory(t *testing.T) {
	t.Parallel()

	t.Run("public repo with all published advisories is untrusted and public", func(t *testing.T) {
		t.Parallel()
		label := LabelRepositorySecurityAdvisory(false, true)
		assert.Equal(t, IntegrityUntrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPublic, label.Confidentiality)
	})

	t.Run("public repo with an unpublished advisory is untrusted and private", func(t *testing.T) {
		t.Parallel()
		// draft/triage/closed advisories are not world-readable even on a
		// public repo, so confidentiality must be private.
		label := LabelRepositorySecurityAdvisory(false, false)
		assert.Equal(t, IntegrityUntrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPrivate, label.Confidentiality)
	})

	t.Run("private repo advisory is untrusted and private", func(t *testing.T) {
		t.Parallel()
		label := LabelRepositorySecurityAdvisory(true, true)
		assert.Equal(t, IntegrityUntrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPrivate, label.Confidentiality)
	})

	t.Run("private repo with unpublished advisory is untrusted and private", func(t *testing.T) {
		t.Parallel()
		label := LabelRepositorySecurityAdvisory(true, false)
		assert.Equal(t, IntegrityUntrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPrivate, label.Confidentiality)
	})
}

func TestLabelGist(t *testing.T) {
	t.Parallel()

	t.Run("public gist is untrusted and public", func(t *testing.T) {
		t.Parallel()
		label := LabelGist(true)
		assert.Equal(t, IntegrityUntrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPublic, label.Confidentiality)
	})

	t.Run("secret gist is untrusted and private", func(t *testing.T) {
		t.Parallel()
		label := LabelGist(false)
		assert.Equal(t, IntegrityUntrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPrivate, label.Confidentiality)
	})
}

func TestLabelGistList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		visibilities     []bool // true == public
		wantConfidential Confidentiality
	}{
		{
			name:             "empty result is treated as public",
			wantConfidential: ConfidentialityPublic,
		},
		{
			name:             "all public gists stay public",
			visibilities:     []bool{true, true},
			wantConfidential: ConfidentialityPublic,
		},
		{
			name:             "any secret gist flips to private",
			visibilities:     []bool{true, false, true},
			wantConfidential: ConfidentialityPrivate,
		},
		{
			name:             "all secret gists stay private",
			visibilities:     []bool{false, false},
			wantConfidential: ConfidentialityPrivate,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			label := LabelGistList(tc.visibilities)
			assert.Equal(t, IntegrityUntrusted, label.Integrity)
			assert.Equal(t, tc.wantConfidential, label.Confidentiality)
		})
	}
}

func TestLabelProject(t *testing.T) {
	t.Parallel()

	t.Run("public project is untrusted and public", func(t *testing.T) {
		t.Parallel()
		label := LabelProject(true)
		assert.Equal(t, IntegrityUntrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPublic, label.Confidentiality)
	})

	t.Run("private project is untrusted and private", func(t *testing.T) {
		t.Parallel()
		label := LabelProject(false)
		assert.Equal(t, IntegrityUntrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPrivate, label.Confidentiality)
	})
}

func TestLabelTeam(t *testing.T) {
	t.Parallel()
	label := LabelTeam()
	assert.Equal(t, IntegrityTrusted, label.Integrity)
	assert.Equal(t, ConfidentialityPrivate, label.Confidentiality)
}

func TestLabelNotificationDetails(t *testing.T) {
	t.Parallel()
	label := LabelNotificationDetails()
	assert.Equal(t, IntegrityUntrusted, label.Integrity)
	assert.Equal(t, ConfidentialityPrivate, label.Confidentiality)
}
