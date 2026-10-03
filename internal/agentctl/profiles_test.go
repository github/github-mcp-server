package agentctl

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallNoClobberAndForce(t *testing.T) {
	base := t.TempDir()
	ids := []string{"coder", "reviewer"}
	require.NoError(t, installProfiles(base, "agents", ids, false, false, io.Discard))
	destination := filepath.Join(base, "agents", "coder.agent.md")
	require.NoError(t, os.WriteFile(destination, []byte("my custom profile"), 0o600))
	err := installProfiles(base, "agents", ids, false, false, io.Discard)
	require.ErrorContains(t, err, "--force")
	content, err := os.ReadFile(destination)
	require.NoError(t, err)
	assert.Equal(t, "my custom profile", string(content))
	require.NoError(t, installProfiles(base, "agents", ids, false, true, io.Discard))
	content, err = os.ReadFile(destination)
	require.NoError(t, err)
	expected, err := profiles.ReadFile("profiles/coder.agent.md")
	require.NoError(t, err)
	assert.Equal(t, expected, content)
}

func TestInstallDryRunAndInvalidProfiles(t *testing.T) {
	base := filepath.Join(t.TempDir(), "not-created")
	var out bytes.Buffer
	require.NoError(t, installProfiles(base, "agents", profileIDs(), true, false, &out))
	assert.Contains(t, out.String(), "Would install")
	_, err := os.Stat(base)
	assert.True(t, os.IsNotExist(err))
	for _, ids := range [][]string{{"unknown"}, {"../../escape"}, {"coder", "coder"}, {"coder", "unknown"}} {
		require.Error(t, installProfiles(base, "agents", ids, false, false, io.Discard))
		_, err = os.Stat(base)
		assert.True(t, os.IsNotExist(err))
	}
}

func TestInstallPreflightsAllDestinations(t *testing.T) {
	base := t.TempDir()
	require.NoError(t, installProfiles(base, "agents", []string{"reviewer"}, false, false, io.Discard))
	require.Error(t, installProfiles(base, "agents", []string{"coder", "reviewer"}, false, false, io.Discard))
	_, err := os.Stat(filepath.Join(base, "agents", "coder.agent.md"))
	assert.True(t, os.IsNotExist(err))
}

func TestInstallRejectsSymlinks(t *testing.T) {
	base := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(base, "agents"), 0o700))
	outside := t.TempDir()
	target := filepath.Join(outside, "target")
	require.NoError(t, os.WriteFile(target, []byte("untouched"), 0o600))
	if err := os.Symlink(target, filepath.Join(base, "agents", "coder.agent.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	require.ErrorContains(t, installProfiles(base, "agents", []string{"coder"}, false, true, io.Discard), "non-regular")
	content, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "untouched", string(content))
	require.NoError(t, os.Symlink(outside, filepath.Join(base, "escape")))
	require.Error(t, installProfiles(base, "escape", []string{"coder"}, false, false, io.Discard))
	_, err = os.Stat(filepath.Join(outside, "coder.agent.md"))
	assert.True(t, os.IsNotExist(err))
}

func TestInstallCommand(t *testing.T) {
	home := filepath.Join(t.TempDir(), "copilot-home")
	t.Setenv("COPILOT_HOME", home)
	cmd := NewCommand("test")
	cmd.SetOut(io.Discard)
	cmd.SetArgs([]string{"install", "errorfixer", "--user"})
	require.NoError(t, cmd.Execute())
	_, err := os.Stat(filepath.Join(home, "agents", "errorfixer.agent.md"))
	require.NoError(t, err)
	checkout := checkoutFixture(t, "https://github.com/owner/repo")
	cmd = NewCommand("test")
	cmd.SetOut(io.Discard)
	cmd.SetArgs([]string{"install", "reviewer", "--path", checkout})
	require.NoError(t, cmd.Execute())
	_, err = os.Stat(filepath.Join(checkout, ".github", "agents", "reviewer.agent.md"))
	require.NoError(t, err)
	for _, args := range [][]string{
		{"install"}, {"install", "--user", "--path", checkout}, {"install", "--path", ""},
		{"install", "unknown", "--user"},
	} {
		cmd = NewCommand("test")
		cmd.SetArgs(args)
		require.Error(t, cmd.Execute())
	}
}

func TestProfilesAreSelfContainedAndRestricted(t *testing.T) {
	ids := profileIDs()
	assert.Equal(t, []string{"coder", "errorfixer", "issue-planner", "researcher", "reviewer",
		"security-auditor", "summarizer", "test-writer", "workflow-debugger"}, ids)
	for _, id := range ids {
		data, err := profiles.ReadFile("profiles/" + id + ".agent.md")
		require.NoError(t, err)
		content := string(data)
		assert.Contains(t, content, "name: "+id+"\n")
		assert.Contains(t, content, "description:")
		assert.Contains(t, content, "include-custom-instructions: true")
		assert.Contains(t, content, "manifests")
		assert.Contains(t, content, "workflows")
		assert.Contains(t, content, "hypotheses")
		switch id {
		case "researcher", "reviewer", "summarizer", "security-auditor", "issue-planner":
			assert.Contains(t, content, `tools: ["read", "search"]`)
			assert.Contains(t, content, "read-only")
		default:
			assert.Contains(t, content, "user")
		}
	}
}
