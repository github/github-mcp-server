package github

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_authorLockdownResult(t *testing.T) {
	t.Parallel()

	t.Run("missing cache returns error", func(t *testing.T) {
		result, err := authorLockdownResult(context.Background(), nil, "owner", "repo", "author", lockdownIssueRestrictedMessage)
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("empty author fails closed", func(t *testing.T) {
		cache := stubRepoAccessCache(nil, time.Minute)
		result, err := authorLockdownResult(context.Background(), cache, "owner", "repo", "", lockdownIssueRestrictedMessage)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, lockdownIssueRestrictedMessage)
	})

	t.Run("lookup failure returns tool-result error", func(t *testing.T) {
		cache := stubRepoAccessCache(nil, time.Minute)
		result, err := authorLockdownResult(context.Background(), cache, "owner", "repo", "author", lockdownIssueRestrictedMessage)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "failed to check lockdown mode")
	})
}

func Test_actorLoginWithBotSuffix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, login, typename, want string
	}{
		{name: "bot without suffix", login: "my-bot", typename: "Bot", want: "my-bot[bot]"},
		{name: "bot already suffixed", login: "my-bot[bot]", typename: "Bot", want: "my-bot[bot]"},
		{name: "user is unchanged", login: "human", typename: "User", want: "human"},
		{name: "github actions already suffixed", login: "github-actions[bot]", typename: "Bot", want: "github-actions[bot]"},
		{name: "github actions bare bot login", login: "github-actions", typename: "Bot", want: "github-actions[bot]"},
		{name: "copilot is preserved", login: "copilot", typename: "Bot", want: "copilot"},
		{name: "copilot is preserved case-insensitively", login: "Copilot", typename: "Bot", want: "Copilot"},
		{name: "empty login", login: "", typename: "Bot", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, actorLoginWithBotSuffix(tc.login, tc.typename))
		})
	}
}
