package main

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOrgInstallations(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string // env vars to set
		viperID  int64             // viper installation_id (0 = not set)
		expected map[string]int64
	}{
		{
			name:     "no env vars",
			envVars:  nil,
			expected: map[string]int64{},
		},
		{
			name: "single org",
			envVars: map[string]string{
				"GITHUB_INSTALLATION_ID_MYORG": "12345",
			},
			expected: map[string]int64{"myorg": 12345},
		},
		{
			name: "org with underscores normalized to dashes",
			envVars: map[string]string{
				"GITHUB_INSTALLATION_ID_MY_ORG_NAME": "67890",
			},
			expected: map[string]int64{"my-org-name": 67890},
		},
		{
			name: "multiple orgs",
			envVars: map[string]string{
				"GITHUB_INSTALLATION_ID_ORG1": "111",
				"GITHUB_INSTALLATION_ID_ORG2": "222",
			},
			expected: map[string]int64{"org1": 111, "org2": 222},
		},
		{
			name: "invalid value silently skipped",
			envVars: map[string]string{
				"GITHUB_INSTALLATION_ID_GOOD": "111",
				"GITHUB_INSTALLATION_ID_BAD":  "not-a-number",
			},
			expected: map[string]int64{"good": 111},
		},
		{
			name: "zero value installation ID silently skipped",
			envVars: map[string]string{
				"GITHUB_INSTALLATION_ID_ZERORG": "0",
				"GITHUB_INSTALLATION_ID_GOOD":   "12345",
			},
			expected: map[string]int64{"good": 12345},
		},
		{
			name:     "default installation ID from viper",
			viperID:  99999,
			expected: map[string]int64{"_default": 99999},
		},
		{
			name: "default plus per-org",
			envVars: map[string]string{
				"GITHUB_INSTALLATION_ID_MYORG": "12345",
			},
			viperID:  99999,
			expected: map[string]int64{"myorg": 12345, "_default": 99999},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Clean up any stale GITHUB_INSTALLATION_ID_ env vars
			for _, env := range os.Environ() {
				if strings.HasPrefix(env, "GITHUB_INSTALLATION_ID_") {
					key := strings.SplitN(env, "=", 2)[0]
					t.Setenv(key, "") // will be restored after test
					os.Unsetenv(key)
				}
			}

			// Set test env vars
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}

			// Set viper installation_id
			if tc.viperID != 0 {
				viper.Set("installation_id", tc.viperID)
				defer viper.Set("installation_id", int64(0))
			} else {
				viper.Set("installation_id", int64(0))
			}

			result := parseOrgInstallations()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestValidateAuthConfig(t *testing.T) {
	tests := []struct {
		name          string
		setup         func() // set viper values
		installations map[string]int64
		wantErr       bool
		errContains   string
	}{
		{
			name: "PAT only — valid",
			setup: func() {
				viper.Set("personal_access_token", "ghp_test123")
				viper.Set("app_id", int64(0))
			},
			installations: map[string]int64{},
			wantErr:       false,
		},
		{
			name: "full app auth — valid",
			setup: func() {
				viper.Set("app_id", int64(12345))
				viper.Set("private_key_file_path", "/path/to/key.pem")
				viper.Set("personal_access_token", "")
			},
			installations: map[string]int64{"_default": 67890},
			wantErr:       false,
		},
		{
			name: "app auth with multi-org — valid",
			setup: func() {
				viper.Set("app_id", int64(12345))
				viper.Set("private_key", "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----")
				viper.Set("personal_access_token", "")
			},
			installations: map[string]int64{"org1": 111, "org2": 222},
			wantErr:       false,
		},
		{
			name: "app_id only — incomplete",
			setup: func() {
				viper.Set("app_id", int64(12345))
				viper.Set("private_key_file_path", "")
				viper.Set("private_key", "")
				viper.Set("personal_access_token", "")
			},
			installations: map[string]int64{},
			wantErr:       true,
			errContains:   "incomplete GitHub App configuration",
		},
		{
			name: "private key only — incomplete",
			setup: func() {
				viper.Set("app_id", int64(0))
				viper.Set("private_key_file_path", "/path/to/key.pem")
				viper.Set("personal_access_token", "")
			},
			installations: map[string]int64{},
			wantErr:       true,
			errContains:   "incomplete GitHub App configuration",
		},
		{
			name: "no auth at all — error",
			setup: func() {
				viper.Set("app_id", int64(0))
				viper.Set("private_key_file_path", "")
				viper.Set("private_key", "")
				viper.Set("personal_access_token", "")
			},
			installations: map[string]int64{},
			wantErr:       true,
			errContains:   "no authentication method configured",
		},
		{
			name: "app auth with PAT — both valid (app auth takes precedence)",
			setup: func() {
				viper.Set("app_id", int64(12345))
				viper.Set("private_key_file_path", "/path/to/key.pem")
				viper.Set("personal_access_token", "ghp_test123")
			},
			installations: map[string]int64{"_default": 67890},
			wantErr:       false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Reset viper state
			viper.Set("app_id", int64(0))
			viper.Set("installation_id", int64(0))
			viper.Set("private_key_file_path", "")
			viper.Set("private_key", "")
			viper.Set("personal_access_token", "")

			tc.setup()

			err := validateAuthConfig(tc.installations)
			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
