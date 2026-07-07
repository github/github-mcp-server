// Package toolsnaps provides test utilities for ensuring json schemas for tools
// have not changed unexpectedly.
package toolsnaps

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/josephburnett/jd/v2"
)

// Test checks that the JSON schema for a tool has not changed unexpectedly.
// It compares the marshaled JSON of the provided tool against a stored snapshot file.
// If the UPDATE_TOOLSNAPS environment variable is set to "true", it updates the snapshot file instead.
// If the snapshot does not exist and not running in CI, it creates the snapshot file.
// If the snapshot does not exist and running in CI (GITHUB_ACTIONS="true"), it returns an error.
// If the snapshot exists, it compares the tool's JSON to the snapshot and returns an error if they differ.
// Returns an error if marshaling, reading, or comparing fails.
func Test(toolName string, tool any) error {
	toolJSON, err := json.MarshalIndent(tool, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tool %s: %w", toolName, err)
	}

	snapPath := fmt.Sprintf("__toolsnaps__/%s.snap", toolName)

	// If UPDATE_TOOLSNAPS is set, then we write the tool JSON to the snapshot file and exit
	if os.Getenv("UPDATE_TOOLSNAPS") == "true" {
		return writeSnap(snapPath, toolJSON)
	}

	snapJSON, err := os.ReadFile(snapPath) //nolint:gosec // filepaths are controlled by the test suite, so this is safe.
	// If the snapshot file does not exist, this must be the first time this test is run.
	// We write the tool JSON to the snapshot file and exit.
	if os.IsNotExist(err) {
		// If we're running in CI, we will error if there is not snapshot because it's important that snapshots
		// are committed alongside the tests, rather than just being constructed and not committed during a CI run.
		if os.Getenv("GITHUB_ACTIONS") == "true" {
			return fmt.Errorf("tool snapshot does not exist for %s. Please run the tests with UPDATE_TOOLSNAPS=true to create it", toolName)
		}

		return writeSnap(snapPath, toolJSON)
	}

	// Otherwise we will compare the tool JSON to the snapshot JSON
	toolNode, err := jd.ReadJsonString(string(toolJSON))
	if err != nil {
		return fmt.Errorf("failed to parse tool JSON for %s: %w", toolName, err)
	}

	snapNode, err := jd.ReadJsonString(string(snapJSON))
	if err != nil {
		return fmt.Errorf("failed to parse snapshot JSON for %s: %w", toolName, err)
	}

	// jd.Set allows arrays to be compared without order sensitivity,
	// which is useful because we don't really care about this when exposing tool schemas.
	diff := toolNode.Diff(snapNode, jd.SET).Render()
	if diff != "" {
		// If there is a difference, we return an error with the diff
		return fmt.Errorf("tool schema for %s has changed unexpectedly:\n%s\nrun with `UPDATE_TOOLSNAPS=true` if this is expected", toolName, diff)
	}

	return nil
}

func writeSnap(snapPath string, contents []byte) error {
	// Sort the JSON keys recursively to ensure consistent output.
	// We do this by unmarshaling and remarshaling, which ensures Go's JSON encoder
	// sorts all map keys alphabetically at every level.
	sortedJSON, err := sortJSONKeys(contents)
	if err != nil {
		return fmt.Errorf("failed to sort JSON keys: %w", err)
	}

	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(snapPath), 0700); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	// Write the snapshot file
	if err := os.WriteFile(snapPath, sortedJSON, 0600); err != nil {
		return fmt.Errorf("failed to write snapshot file: %w", err)
	}

	return nil
}

// sortJSONKeys recursively sorts all object keys in a JSON byte array by
// unmarshaling to map[string]any and remarshaling. Go's JSON encoder
// automatically sorts map keys alphabetically.
func sortJSONKeys(jsonData []byte) ([]byte, error) {
	var data any
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, err
	}

	return json.MarshalIndent(data, "", "  ")
}
