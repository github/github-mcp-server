package agentctl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func checkoutFixture(t *testing.T, remote string) string {
	t.Helper()
	dir := t.TempDir()
	dir, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	for _, args := range [][]string{{"init", dir}, {"-C", dir, "remote", "add", "origin", remote}} {
		cmd := exec.Command("git", args...)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s", output)
	}
	return dir
}

func fakeLookup(_ string) (string, error) {
	return "copilot-test-backend", nil
}

func TestRunForwardsArgumentsDirectoryAndIO(t *testing.T) {
	dir := checkoutFixture(t, "git@github.com:Owner/Repo.git")
	subdir := filepath.Join(dir, "src")
	require.NoError(t, os.Mkdir(subdir, 0o700))
	prompt := "First line\nSecond line; $(touch injected) --allow-all 'quoted'"
	in := strings.NewReader("terminal input")
	var out, stderr bytes.Buffer
	called := false
	cmd := newCommand("test", fakeLookup, func(ctx context.Context, process *exec.Cmd) error {
		called = true
		assert.Equal(t, []string{"copilot-test-backend", "--agent", "my-custom--role", "--prompt", prompt, "--allow-tool", "shell(go test ./...)"}, process.Args)
		assert.Equal(t, dir, process.Dir)
		assert.Same(t, in, process.Stdin)
		assert.Same(t, &out, process.Stdout)
		assert.Same(t, &stderr, process.Stderr)
		assert.NotNil(t, ctx)
		_, err := io.Copy(process.Stdout, process.Stdin)
		require.NoError(t, err)
		_, err = fmt.Fprint(process.Stderr, "native diagnostic")
		return err
	})
	cmd.SetIn(in)
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"run", "my-custom--role", "--repo", "owner/repo", "--path", subdir,
		"--prompt", prompt, "--allow-tool", "shell(go test ./...)"})
	require.NoError(t, cmd.Execute())
	assert.True(t, called)
	assert.Equal(t, "terminal input", out.String())
	assert.Equal(t, "native diagnostic", stderr.String())
	_, err := os.Stat(filepath.Join(dir, "injected"))
	assert.True(t, os.IsNotExist(err))
}

func TestRunPromptFile(t *testing.T) {
	dir := checkoutFixture(t, "https://github.com/owner/repo.git")
	file := filepath.Join(t.TempDir(), "task.txt")
	require.NoError(t, os.WriteFile(file, []byte("line 1\nline 2\n"), 0o600))
	called := false
	cmd := newCommand("test", fakeLookup, func(_ context.Context, process *exec.Cmd) error {
		called = true
		assert.Equal(t, "line 1\nline 2\n", process.Args[4])
		return nil
	})
	cmd.SetArgs([]string{"run", "coder", "--path", dir, "--prompt-file", file})
	require.NoError(t, cmd.Execute())
	assert.True(t, called)
}

func TestRunValidation(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no agent", []string{"run"}, "accepts 1 arg"},
		{"too many agents", []string{"run", "coder", "reviewer"}, "accepts 1 arg"},
		{"no prompt", []string{"run", "coder"}, "exactly one"},
		{"both prompts", []string{"run", "coder", "--prompt", "task", "--prompt-file", "task.txt"}, "exactly one"},
		{"empty prompt", []string{"run", "coder", "--prompt", " \n "}, "nonempty"},
		{"empty filename", []string{"run", "coder", "--prompt-file", ""}, "name a file"},
		{"missing file", []string{"run", "coder", "--prompt-file", filepath.Join(t.TempDir(), "missing")}, "read prompt file"},
		{"NUL prompt", []string{"run", "coder", "--prompt", "a\x00b"}, "NUL"},
		{"injection agent", []string{"run", "coder;touch bad", "--prompt", "task"}, "custom-agent ID"},
		{"flag agent", []string{"run", "--prompt", "task", "--", "--allow-all"}, "custom-agent ID"},
		{"allow all", []string{"run", "coder", "--prompt", "task", "--allow-all"}, "unknown flag"},
		{"wildcard grant", []string{"run", "coder", "--prompt", "task", "--allow-tool", "*"}, "exact command"},
		{"broad shell", []string{"run", "coder", "--prompt", "task", "--allow-tool", "shell"}, "exact command"},
		{"broad write", []string{"run", "coder", "--prompt", "task", "--allow-tool", "write"}, "exact command"},
		{"directory write", []string{"run", "coder", "--prompt", "task", "--allow-tool", "write(.)"}, "exact command"},
		{"wildcard shell", []string{"run", "coder", "--prompt", "task", "--allow-tool", "shell(git:*)"}, "exact command"},
		{"combined grants", []string{"run", "coder", "--prompt", "task", "--allow-tool", "read,write"}, "exact command"},
		{"empty shell", []string{"run", "coder", "--prompt", "task", "--allow-tool", "shell()"}, "exact command"},
		{"newline grant", []string{"run", "coder", "--prompt", "task", "--allow-tool", "shell(ls\npwd)"}, "exact command"},
		{"bad repo", []string{"run", "coder", "--prompt", "task", "--repo", "https://github.com/owner/repo"}, "owner/repo"},
		{"empty repo", []string{"run", "coder", "--prompt", "task", "--repo", ""}, "owner/repo"},
		{"empty path", []string{"run", "coder", "--prompt", "task", "--path", ""}, "name a checkout"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newCommand("test", fakeLookup, func(_ context.Context, _ *exec.Cmd) error {
				t.Fatal("must not invoke Copilot")
				return nil
			})
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			require.ErrorContains(t, err, tt.want)
			assert.Equal(t, 1, ExitCode(err))
		})
	}
}

func TestMissingBinary(t *testing.T) {
	dir := checkoutFixture(t, "https://github.com/owner/repo")
	cmd := newCommand("test", func(name string) (string, error) {
		assert.Equal(t, "copilot", name)
		return "", exec.ErrNotFound
	}, func(_ context.Context, _ *exec.Cmd) error {
		t.Fatal("must not execute")
		return nil
	})
	cmd.SetArgs([]string{"run", "coder", "--path", dir, "--prompt", "task"})
	err := cmd.Execute()
	require.ErrorContains(t, err, "not found on PATH")
	assert.ErrorIs(t, err, exec.ErrNotFound)
}

// TestCopilotProcess is an offline subprocess backend; it never calls Copilot.
func TestCopilotProcess(_ *testing.T) {
	mode := os.Getenv("AGENTCTL_TEST_PROCESS")
	if mode == "" {
		return
	}
	switch mode {
	case "failure":
		fmt.Fprintln(os.Stderr, "authentication or quota failure from backend")
		os.Exit(23)
	case "cancel":
		fmt.Fprintln(os.Stdout, "ready")
		time.Sleep(time.Hour)
	case "io":
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			os.Exit(2)
		}
		dir, err := os.Getwd()
		if err != nil {
			os.Exit(2)
		}
		if err := json.NewEncoder(os.Stdout).Encode([]string{dir, string(input)}); err != nil {
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, "backend stderr")
	}
	os.Exit(0)
}

func helperExecutor(t *testing.T, mode string) executor {
	t.Helper()
	binary, err := os.Executable()
	require.NoError(t, err)
	return func(ctx context.Context, process *exec.Cmd) error {
		helper := exec.CommandContext(ctx, binary, "-test.run=^TestCopilotProcess$")
		helper.Env = process.Environ()
		helper.Env = append(helper.Env, "AGENTCTL_TEST_PROCESS="+mode)
		helper.Dir = process.Dir
		helper.Stdin, helper.Stdout, helper.Stderr = process.Stdin, process.Stdout, process.Stderr
		return helper.Run()
	}
}

func TestActualSubprocessIOAndFailure(t *testing.T) {
	dir := checkoutFixture(t, "ssh://git@github.com/owner/repo.git")
	for _, mode := range []string{"io", "failure"} {
		t.Run(mode, func(t *testing.T) {
			var out, stderr bytes.Buffer
			cmd := newCommand("test", fakeLookup, helperExecutor(t, mode))
			cmd.SetIn(strings.NewReader("inherited input\n"))
			cmd.SetOut(&out)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{"run", "coder", "--path", dir, "--prompt", "task"})
			err := cmd.Execute()
			if mode == "failure" {
				require.ErrorContains(t, err, "copilot CLI failed")
				assert.Equal(t, 23, ExitCode(err))
				assert.Contains(t, stderr.String(), "authentication or quota failure from backend")
			} else {
				require.NoError(t, err)
				var result []string
				require.NoError(t, json.Unmarshal(out.Bytes(), &result))
				assert.Equal(t, []string{dir, "inherited input\n"}, result)
				assert.Equal(t, "backend stderr\n", stderr.String())
				assert.Equal(t, 0, ExitCode(nil))
			}
		})
	}
}

type readyWriter struct {
	cancel context.CancelFunc
}

func (w readyWriter) Write(p []byte) (int, error) {
	w.cancel()
	return len(p), nil
}

func TestRunCancellation(t *testing.T) {
	dir := checkoutFixture(t, "https://github.com/owner/repo")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := newCommand("test", fakeLookup, helperExecutor(t, "cancel"))
	cmd.SetOut(readyWriter{cancel: cancel})
	cmd.SetArgs([]string{"run", "coder", "--path", dir, "--prompt", "task"})
	err := cmd.ExecuteContext(ctx)
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 130, ExitCode(err))
	assert.Equal(t, 130, ExitCode(context.DeadlineExceeded))
	assert.Equal(t, 1, ExitCode(errors.New("start failed")))
}

func TestHelpVersionAndList(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"--version"}, {"list"}, {"run", "--help"}, {"install", "--help"}} {
		cmd := newCommand("v-test", fakeLookup, nil)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs(args)
		require.NoError(t, cmd.Execute())
		assert.NotEmpty(t, out.String())
		if args[0] == "list" {
			for _, id := range profileIDs() {
				assert.Contains(t, out.String(), id+"\n")
			}
		}
		if args[0] == "--version" {
			assert.Contains(t, out.String(), "v-test")
		}
	}
}

func TestNarrowPermissions(t *testing.T) {
	for _, tool := range []string{"read", "shell(go test ./...)", "write(README.md)", "write(src/main.go)"} {
		assert.True(t, narrowPermission(tool), tool)
	}
	for _, tool := range []string{"*", "shell", "write", "write(*)", "write(..)", "write(src/)", "github(*)",
		"read,write", "shell(ls; rm -rf /)", "shell(ls && pwd)", "shell(ls | sh)", "shell(echo $(id))"} {
		assert.False(t, narrowPermission(tool), tool)
	}
}

func TestRunCannotInheritGitRepositoryOverrides(t *testing.T) {
	dir := checkoutFixture(t, "https://github.com/owner/repo")
	other := checkoutFixture(t, "https://github.com/owner/other")
	t.Setenv("GIT_DIR", filepath.Join(other, ".git"))
	t.Setenv("GIT_WORK_TREE", other)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "remote.origin.url")
	t.Setenv("GIT_CONFIG_VALUE_0", "https://github.com/owner/other")
	home := t.TempDir()
	t.Setenv("COPILOT_HOME", home)
	called := false
	cmd := newCommand("test", fakeLookup, func(_ context.Context, process *exec.Cmd) error {
		called = true
		assert.Equal(t, dir, process.Dir)
		for _, variable := range process.Env {
			key, _, _ := strings.Cut(variable, "=")
			assert.False(t, strings.HasPrefix(key, "GIT_"), variable)
		}
		assert.Contains(t, process.Env, "COPILOT_HOME="+home)
		return nil
	})
	cmd.SetArgs([]string{"run", "coder", "--path", dir, "--repo", "owner/repo", "--prompt", "task"})
	require.NoError(t, cmd.Execute())
	assert.True(t, called)
}
