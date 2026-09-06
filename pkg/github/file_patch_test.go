package github

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyExactTextEdits(t *testing.T) {
	t.Run("applies ordered exact replacements", func(t *testing.T) {
		got, err := applyExactTextEdits("alpha beta beta gamma", []exactTextEdit{
			{OldText: "alpha", NewText: "ALPHA", ExpectedOccurrences: 1},
			{OldText: "beta", NewText: "BETA", ExpectedOccurrences: 2},
		})
		require.NoError(t, err)
		assert.Equal(t, "ALPHA BETA BETA gamma", got)
	})

	t.Run("fails when exact occurrence count differs", func(t *testing.T) {
		_, err := applyExactTextEdits("one two two", []exactTextEdit{
			{OldText: "two", NewText: "TWO", ExpectedOccurrences: 1},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected 1 exact matches, found 2")
	})

	t.Run("rejects empty old text", func(t *testing.T) {
		_, err := applyExactTextEdits("abc", []exactTextEdit{{OldText: "", NewText: "x", ExpectedOccurrences: 1}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "old_text must not be empty")
	})

	t.Run("rejects no-op patch", func(t *testing.T) {
		_, err := applyExactTextEdits("abc", []exactTextEdit{{OldText: "abc", NewText: "abc", ExpectedOccurrences: 1}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no content change")
	})

	t.Run("bounds patch payload", func(t *testing.T) {
		tooLarge := strings.Repeat("x", maxPatchInputBytes+1)
		_, err := applyExactTextEdits(tooLarge, []exactTextEdit{{OldText: tooLarge, NewText: "y", ExpectedOccurrences: 1}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "patch input exceeds")
	})
}
