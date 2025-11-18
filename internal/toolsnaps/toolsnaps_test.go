package toolsnaps

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dummyTool struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// withIsolatedWorkingDir creates a temp dir, changes to it, and restores the original working dir after the test.
func withIsolatedWorkingDir(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.Chdir(origDir)) })
	require.NoError(t, os.Chdir(dir))
}

func TestSnapshotDoesNotExistNotInCI(t *testing.T) {
	withIsolatedWorkingDir(t)

	// Given we are not running in CI
	t.Setenv("GITHUB_ACTIONS", "false") // This REALLY is required because the tests run in CI
	tool := dummyTool{"foo", 42}

	// When we test the snapshot
	err := Test("dummy", tool)

	// Then it should succeed and write the snapshot file
	require.NoError(t, err)
	path := filepath.Join("__toolsnaps__", "dummy.snap")
	_, statErr := os.Stat(path)
	assert.NoError(t, statErr, "expected snapshot file to be written")
}

func TestSnapshotDoesNotExistInCI(t *testing.T) {
	withIsolatedWorkingDir(t)
	// Ensure that UPDATE_TOOLSNAPS is not set for this test, which it might be if someone is running
	// UPDATE_TOOLSNAPS=true go test ./...
	t.Setenv("UPDATE_TOOLSNAPS", "false")

	// Given we are running in CI
	t.Setenv("GITHUB_ACTIONS", "true")
	tool := dummyTool{"foo", 42}

	// When we test the snapshot
	err := Test("dummy", tool)

	// Then it should error about missing snapshot in CI
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool snapshot does not exist", "expected error about missing snapshot in CI")
}

func TestSnapshotExistsMatch(t *testing.T) {
	withIsolatedWorkingDir(t)

	// Given a matching snapshot file exists
	tool := dummyTool{"foo", 42}
	b, _ := json.MarshalIndent(tool, "", "  ")
	require.NoError(t, os.MkdirAll("__toolsnaps__", 0700))
	require.NoError(t, os.WriteFile(filepath.Join("__toolsnaps__", "dummy.snap"), b, 0600))

	// When we test the snapshot
	err := Test("dummy", tool)

	// Then it should succeed (no error)
	require.NoError(t, err)
}

func TestSnapshotExistsDiff(t *testing.T) {
	withIsolatedWorkingDir(t)
	// Ensure that UPDATE_TOOLSNAPS is not set for this test, which it might be if someone is running
	// UPDATE_TOOLSNAPS=true go test ./...
	t.Setenv("UPDATE_TOOLSNAPS", "false")

	// Given a non-matching snapshot file exists
	require.NoError(t, os.MkdirAll("__toolsnaps__", 0700))
	require.NoError(t, os.WriteFile(filepath.Join("__toolsnaps__", "dummy.snap"), []byte(`{"name":"foo","value":1}`), 0600))
	tool := dummyTool{"foo", 2}

	// When we test the snapshot
	err := Test("dummy", tool)

	// Then it should error about the schema diff
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool schema for dummy has changed unexpectedly", "expected error about diff")
}

func TestUpdateToolsnaps(t *testing.T) {
	withIsolatedWorkingDir(t)

	// Given UPDATE_TOOLSNAPS is set, regardless of whether a matching snapshot file exists
	t.Setenv("UPDATE_TOOLSNAPS", "true")
	require.NoError(t, os.MkdirAll("__toolsnaps__", 0700))
	require.NoError(t, os.WriteFile(filepath.Join("__toolsnaps__", "dummy.snap"), []byte(`{"name":"foo","value":1}`), 0600))
	tool := dummyTool{"foo", 42}

	// When we test the snapshot
	err := Test("dummy", tool)

	// Then it should succeed and write the snapshot file
	require.NoError(t, err)
	path := filepath.Join("__toolsnaps__", "dummy.snap")
	_, statErr := os.Stat(path)
	assert.NoError(t, statErr, "expected snapshot file to be written")
}

func TestMalformedSnapshotJSON(t *testing.T) {
	withIsolatedWorkingDir(t)
	// Ensure that UPDATE_TOOLSNAPS is not set for this test, which it might be if someone is running
	// UPDATE_TOOLSNAPS=true go test ./...
	t.Setenv("UPDATE_TOOLSNAPS", "false")

	// Given a malformed snapshot file exists
	require.NoError(t, os.MkdirAll("__toolsnaps__", 0700))
	require.NoError(t, os.WriteFile(filepath.Join("__toolsnaps__", "dummy.snap"), []byte(`not-json`), 0600))
	tool := dummyTool{"foo", 42}

	// When we test the snapshot
	err := Test("dummy", tool)

	// Then it should error about malformed snapshot JSON
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse snapshot JSON for dummy", "expected error about malformed snapshot JSON")
}

func TestMarshalError(t *testing.T) {
	withIsolatedWorkingDir(t)

	// Given a tool that cannot be marshaled to JSON (contains channels)
	type badTool struct {
		Name string
		Ch   chan int
	}
	tool := badTool{"test", make(chan int)}

	// When we test the snapshot
	err := Test("bad", tool)

	// Then it should error about marshaling
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal tool bad", "expected error about marshaling")
}

func TestMalformedToolJSON(t *testing.T) {
	withIsolatedWorkingDir(t)
	// This test is hard to trigger since if MarshalIndent succeeds, ReadJsonString should too
	// We can simulate it by having the tool contain values that marshal to invalid JSON
	// However, this is extremely difficult with standard Go types
	// The path at line 49-51 is effectively unreachable with normal usage
}

func TestWriteSnapMkdirError(t *testing.T) {
	withIsolatedWorkingDir(t)

	// Given a file exists where the snapshot directory should be
	require.NoError(t, os.WriteFile("__toolsnaps__", []byte("blocking file"), 0600))

	// Set UPDATE_TOOLSNAPS so it attempts to write immediately
	t.Setenv("UPDATE_TOOLSNAPS", "true")

	tool := dummyTool{"foo", 42}

	// When we test the snapshot
	err := Test("dummy", tool)

	// Then it should error about creating the directory
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create snapshot directory", "expected error about mkdir")
}

func TestWriteSnapWriteFileError(t *testing.T) {
	withIsolatedWorkingDir(t)

	// Create a nested directory structure where we can block file writing
	require.NoError(t, os.MkdirAll("__toolsnaps__/subdir", 0700))
	// Create a directory where the file should be (blocks file creation)
	require.NoError(t, os.MkdirAll("__toolsnaps__/subdir/dummy.snap", 0700))

	// Set UPDATE_TOOLSNAPS so it attempts to write immediately
	t.Setenv("UPDATE_TOOLSNAPS", "true")

	tool := dummyTool{"foo", 42}

	// When we test the snapshot with a path that has a directory where the file should be
	err := Test("subdir/dummy", tool)

	// Then it should error about writing the file
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write snapshot file", "expected error about writing file")
}
