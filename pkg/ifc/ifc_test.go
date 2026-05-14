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
