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
		readers          [][]string
		wantOK           bool
		wantIntegrity    Integrity
		wantConfidential []Confidentiality
	}{
		{
			name:             "empty result is treated as public",
			wantOK:           true,
			wantIntegrity:    IntegrityUntrusted,
			wantConfidential: []Confidentiality{ConfidentialityPublic},
		},
		{
			name:             "single public repo",
			visibilities:     []bool{false},
			readers:          [][]string{nil},
			wantOK:           true,
			wantIntegrity:    IntegrityUntrusted,
			wantConfidential: []Confidentiality{ConfidentialityPublic},
		},
		{
			name:             "mixed public and private collapses to public",
			visibilities:     []bool{true, false},
			readers:          [][]string{{"alice"}, nil},
			wantOK:           true,
			wantIntegrity:    IntegrityUntrusted,
			wantConfidential: []Confidentiality{ConfidentialityPublic},
		},
		{
			name:             "two private repos with intersecting collaborators",
			visibilities:     []bool{true, true},
			readers:          [][]string{{"alice", "bob", "carol"}, {"bob", "carol", "dan"}},
			wantOK:           true,
			wantIntegrity:    IntegrityUntrusted,
			wantConfidential: []Confidentiality{"bob", "carol"},
		},
		{
			name:             "private repos with no overlap yield empty reader set",
			visibilities:     []bool{true, true},
			readers:          [][]string{{"alice"}, {"bob"}},
			wantOK:           true,
			wantIntegrity:    IntegrityUntrusted,
			wantConfidential: []Confidentiality{},
		},
		{
			name:             "intersection preserves first-set order and dedupes",
			visibilities:     []bool{true, true, true},
			readers:          [][]string{{"alice", "bob", "alice"}, {"bob", "alice"}, {"alice", "bob"}},
			wantOK:           true,
			wantIntegrity:    IntegrityUntrusted,
			wantConfidential: []Confidentiality{"alice", "bob"},
		},
		{
			name:         "mismatched slice lengths return ok=false",
			visibilities: []bool{true, true},
			readers:      [][]string{{"alice"}},
			wantOK:       false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			label, ok := LabelSearchIssues(tc.visibilities, tc.readers)
			assert.Equal(t, tc.wantOK, ok)
			if !tc.wantOK {
				return
			}
			assert.Equal(t, tc.wantIntegrity, label.Integrity)
			if len(tc.wantConfidential) == 0 {
				assert.Empty(t, label.Confidentiality)
				return
			}
			assert.Equal(t, tc.wantConfidential, label.Confidentiality)
		})
	}
}
