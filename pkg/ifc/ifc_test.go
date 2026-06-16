package ifc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLabelListIssues(t *testing.T) {
	t.Parallel()

	t.Run("public repo issues are untrusted and public", func(t *testing.T) {
		t.Parallel()
		label := LabelListIssues(false)
		assert.Equal(t, IntegrityUntrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPublic, label.Confidentiality)
	})

	t.Run("private repo issues are trusted and private", func(t *testing.T) {
		t.Parallel()
		label := LabelListIssues(true)
		assert.Equal(t, IntegrityTrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPrivate, label.Confidentiality)
	})
}

func TestLabelRepoUserContent(t *testing.T) {
	t.Parallel()

	t.Run("public repo user content is untrusted and public", func(t *testing.T) {
		t.Parallel()
		label := LabelRepoUserContent(false)
		assert.Equal(t, IntegrityUntrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPublic, label.Confidentiality)
	})

	t.Run("private repo user content is trusted and private", func(t *testing.T) {
		t.Parallel()
		label := LabelRepoUserContent(true)
		assert.Equal(t, IntegrityTrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPrivate, label.Confidentiality)
	})
}

func TestLabelSearchIssues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		visibilities     []bool // true == private
		wantIntegrity    Integrity
		wantConfidential Confidentiality
	}{
		{
			name:             "empty result is treated as public",
			wantIntegrity:    IntegrityUntrusted,
			wantConfidential: ConfidentialityPublic,
		},
		{
			name:             "single public repo",
			visibilities:     []bool{false},
			wantIntegrity:    IntegrityUntrusted,
			wantConfidential: ConfidentialityPublic,
		},
		{
			name:             "all public repos stay public",
			visibilities:     []bool{false, false, false},
			wantIntegrity:    IntegrityUntrusted,
			wantConfidential: ConfidentialityPublic,
		},
		{
			name:             "mixed public and private repos become untrusted private",
			visibilities:     []bool{false, true, false},
			wantIntegrity:    IntegrityUntrusted,
			wantConfidential: ConfidentialityPrivate,
		},
		{
			name:             "all private repos stay trusted private",
			visibilities:     []bool{true, true},
			wantIntegrity:    IntegrityTrusted,
			wantConfidential: ConfidentialityPrivate,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			label := LabelSearchIssues(tc.visibilities)
			assert.Equal(t, tc.wantIntegrity, label.Integrity)
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
		label := LabelGist()
		assert.Equal(t, IntegrityUntrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPublic, label.Confidentiality)
	})

	t.Run("secret gist is untrusted and public", func(t *testing.T) {
		t.Parallel()
		label := LabelGist()
		assert.Equal(t, IntegrityUntrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPublic, label.Confidentiality)
	})
}

func TestLabelGistList(t *testing.T) {
	t.Parallel()

	label := LabelGistList()
	assert.Equal(t, IntegrityUntrusted, label.Integrity)
	assert.Equal(t, ConfidentialityPublic, label.Confidentiality)
}

func TestLabelProject(t *testing.T) {
	t.Parallel()

	t.Run("public project is untrusted and public", func(t *testing.T) {
		t.Parallel()
		label := LabelProject(false)
		assert.Equal(t, IntegrityUntrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPublic, label.Confidentiality)
	})

	t.Run("private project metadata is trusted and private", func(t *testing.T) {
		t.Parallel()
		label := LabelProject(true)
		assert.Equal(t, IntegrityTrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPrivate, label.Confidentiality)
	})
}

func TestLabelProjectList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		visibilities     []bool // true == private
		wantIntegrity    Integrity
		wantConfidential Confidentiality
	}{
		{
			name:             "empty result is treated as public",
			wantIntegrity:    IntegrityUntrusted,
			wantConfidential: ConfidentialityPublic,
		},
		{
			name:             "all public projects stay public",
			visibilities:     []bool{false, false},
			wantIntegrity:    IntegrityUntrusted,
			wantConfidential: ConfidentialityPublic,
		},
		{
			name:             "mixed public and private projects become untrusted private",
			visibilities:     []bool{false, true},
			wantIntegrity:    IntegrityUntrusted,
			wantConfidential: ConfidentialityPrivate,
		},
		{
			name:             "all private projects stay trusted private",
			visibilities:     []bool{true, true},
			wantIntegrity:    IntegrityTrusted,
			wantConfidential: ConfidentialityPrivate,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			label := LabelProjectList(tc.visibilities)
			assert.Equal(t, tc.wantIntegrity, label.Integrity)
			assert.Equal(t, tc.wantConfidential, label.Confidentiality)
		})
	}
}

func TestLabelProjectContent(t *testing.T) {
	t.Parallel()

	t.Run("public project content is untrusted and public", func(t *testing.T) {
		t.Parallel()
		label := LabelProjectContent(false)
		assert.Equal(t, IntegrityUntrusted, label.Integrity)
		assert.Equal(t, ConfidentialityPublic, label.Confidentiality)
	})

	t.Run("private project content is untrusted and private", func(t *testing.T) {
		t.Parallel()
		label := LabelProjectContent(true)
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
