package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunListScopesReadOnly(t *testing.T) {
	initConfig()
	for _, value := range []string{"yes", "no", "on", "off", "tru", "   "} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("GITHUB_READ_ONLY", value)
			viper.Set("list-scopes-output", "json")
			t.Cleanup(func() { viper.Set("list-scopes-output", nil) })

			output, err := os.CreateTemp(t.TempDir(), "scopes")
			require.NoError(t, err)
			stdout := os.Stdout
			os.Stdout = output
			t.Cleanup(func() {
				os.Stdout = stdout
				assert.NoError(t, output.Close())
			})

			err = runListScopes()
			if value == "tru" || value == "   " {
				require.ErrorContains(t, err, "GITHUB_READ_ONLY")
				info, err := output.Stat()
				require.NoError(t, err)
				assert.Zero(t, info.Size())
				return
			}
			require.NoError(t, err)
			_, err = output.Seek(0, 0)
			require.NoError(t, err)
			var result ScopesOutput
			require.NoError(t, json.NewDecoder(output).Decode(&result))
			readOnly := value == "yes" || value == "on"
			assert.Equal(t, readOnly, result.ReadOnly)
			require.NotEmpty(t, result.Tools)
			hasWriteTool := false
			for _, tool := range result.Tools {
				if !tool.ReadOnly {
					hasWriteTool = true
				}
			}
			assert.Equal(t, !readOnly, hasWriteTool)
		})
	}
}
