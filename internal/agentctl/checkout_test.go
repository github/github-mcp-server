package agentctl

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRemote(t *testing.T) {
	for _, remote := range []string{
		"https://github.com/Owner/Repo", "https://github.com/Owner/Repo.git",
		"git@github.com:Owner/Repo.git", "ssh://git@github.com/Owner/Repo.git",
	} {
		actual, err := parseRemote(remote)
		require.NoError(t, err, remote)
		assert.Equal(t, "Owner/Repo", actual)
	}
	for _, remote := range []string{
		"", "/tmp/repo", "file:///tmp/repo", "http://github.com/owner/repo",
		"https://example.com/owner/repo", "git@example.com:owner/repo",
		"https://github.com.evil.test/owner/repo", "https://github.com:443/owner/repo",
		"https://github.com/owner", "https://github.com/owner/repo/tree/main",
		"https://github.com/owner/repo?x=1", "https://github.com/owner/repo#main",
		"https://github.com/owner/%72epo", "https://github.com//owner/repo",
		"https://token@github.com/owner/repo", "ssh://user@github.com/owner/repo",
		"git@github.com:owner/repo;touch bad", "git@github.com:owner/..",
		"git@github.com:owner/repo\nhttps://github.com/other/repo",
		"https://github.com/owner/repo/", "******github.com/owner/repo",
	} {
		_, err := parseRemote(remote)
		assert.Error(t, err, remote)
	}
}

func TestCheckoutValidation(t *testing.T) {
	dir := checkoutFixture(t, "git@github.com:owner/repo.git")
	ctx := context.Background()
	root, err := validateCheckout(ctx, dir, "OWNER/REPO")
	require.NoError(t, err)
	assert.Equal(t, dir, root)
	_, err = validateCheckout(ctx, dir, "other/repo")
	require.ErrorContains(t, err, "select the correct checkout")
	_, err = validateCheckout(ctx, dir, "owner/repo/extra")
	require.ErrorContains(t, err, "owner/repo")
	_, err = validateCheckout(ctx, t.TempDir(), "")
	require.ErrorContains(t, err, "not a Git working tree")
	_, err = validateCheckout(ctx, filepath.Join(dir, "missing"), "")
	require.ErrorContains(t, err, "existing local checkout")
	require.NoError(t, os.Mkdir(filepath.Join(dir, "nested"), 0o700))
	t.Chdir(filepath.Join(dir, "nested"))
	root, err = validateCheckout(ctx, "", "owner/repo")
	require.NoError(t, err)
	assert.Equal(t, dir, root)
}

func TestGitEnvironmentCannotRedirectCheckout(t *testing.T) {
	selected := checkoutFixture(t, "https://github.com/owner/selected")
	other := checkoutFixture(t, "https://github.com/owner/other")
	t.Setenv("GIT_DIR", filepath.Join(other, ".git"))
	t.Setenv("GIT_WORK_TREE", other)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "remote.origin.url")
	t.Setenv("GIT_CONFIG_VALUE_0", "https://github.com/owner/other")
	root, err := validateCheckout(context.Background(), selected, "owner/selected")
	require.NoError(t, err)
	assert.Equal(t, selected, root)
}

func TestOriginRequiredEvenWithoutRepoFlag(t *testing.T) {
	dir := checkoutFixture(t, "https://example.com/owner/repo")
	_, err := validateCheckout(context.Background(), dir, "")
	require.ErrorContains(t, err, "github.com")
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("[core]\nrepositoryformatversion = 0\nbare = false\n"), 0o600))
	_, err = validateCheckout(context.Background(), dir, "")
	require.ErrorContains(t, err, "origin URL")
}

func TestCheckoutCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := validateCheckout(ctx, t.TempDir(), "")
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 130, ExitCode(err))
}

func TestCheckoutRejectsMultipleOrigins(t *testing.T) {
	dir := checkoutFixture(t, "https://github.com/owner/repo")
	file, err := os.OpenFile(filepath.Join(dir, ".git", "config"), os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	_, err = file.WriteString("\n[remote \"origin\"]\nurl = https://github.com/owner/other\n")
	require.NoError(t, err)
	require.NoError(t, file.Close())
	_, err = validateCheckout(context.Background(), dir, "owner/repo")
	require.Error(t, err)
}

func TestCheckoutRejectsRedirectedWorktree(t *testing.T) {
	dir := checkoutFixture(t, "https://github.com/owner/repo")
	other := t.TempDir()
	file, err := os.OpenFile(filepath.Join(dir, ".git", "config"), os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	_, err = file.WriteString("\n[core]\nworktree = " + filepath.ToSlash(other) + "\n")
	require.NoError(t, err)
	require.NoError(t, file.Close())
	_, err = validateCheckout(context.Background(), dir, "owner/repo")
	require.ErrorContains(t, err, "outside the selected directory")
}
