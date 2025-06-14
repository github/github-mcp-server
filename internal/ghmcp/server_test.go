package ghmcp

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestBuildAuthConfig(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		expectError bool
		expectType  string
	}{
		{
			name: "valid PAT",
			envVars: map[string]string{
				"personal_access_token": "ghp_test123",
			},
			expectType: "token",
		},
		{
			name: "valid GitHub App",
			envVars: map[string]string{
				"app_id":          "123456",
				"installation_id": "789012",
				"private_key_pem": "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
			},
			expectType: "app",
		},
		{
			name:        "missing auth",
			envVars:     map[string]string{},
			expectError: true,
		},
		{
			name: "conflicting auth",
			envVars: map[string]string{
				"personal_access_token": "ghp_test123",
				"app_id":                "123456",
				"installation_id":       "789012",
				"private_key_pem":       "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
			},
			expectError: true,
		},
		{
			name: "incomplete GitHub App - missing app_id",
			envVars: map[string]string{
				"installation_id": "789012",
				"private_key_pem": "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
			},
			expectError: true,
		},
		{
			name: "incomplete GitHub App - missing installation_id",
			envVars: map[string]string{
				"app_id":          "123456",
				"private_key_pem": "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
			},
			expectError: true,
		},
		{
			name: "incomplete GitHub App - missing private_key_pem",
			envVars: map[string]string{
				"app_id":          "123456",
				"installation_id": "789012",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear viper state
			viper.Reset()

			// Set up environment variables
			for k, v := range tt.envVars {
				viper.Set(k, v)
			}

			config, err := BuildAuthConfig()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify the config type based on expectType
				switch tt.expectType {
				case "token":
					assert.NotEmpty(t, config.Token)
					assert.Empty(t, config.AppID)
					assert.Empty(t, config.InstallationID)
					assert.Empty(t, config.PrivateKeyPEM)
				case "app":
					assert.Empty(t, config.Token)
					assert.NotEmpty(t, config.AppID)
					assert.NotEmpty(t, config.InstallationID)
					assert.NotEmpty(t, config.PrivateKeyPEM)
				}
			}
		})
	}
}

func TestMCPServerConfig_getAuthMethod(t *testing.T) {
	tests := []struct {
		name           string
		config         MCPServerConfig
		expectedMethod authMethod
		expectError    bool
	}{
		{
			name: "token authentication",
			config: MCPServerConfig{
				Auth: AuthConfig{
					Token: "ghp_test123",
				},
			},
			expectedMethod: authToken,
			expectError:    false,
		},
		{
			name: "GitHub App authentication",
			config: MCPServerConfig{
				Auth: AuthConfig{
					AppID:          "123456",
					InstallationID: "789012",
					PrivateKeyPEM:  "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
				},
			},
			expectedMethod: authGitHubApp,
			expectError:    false,
		},
		{
			name: "no authentication",
			config: MCPServerConfig{
				Auth: AuthConfig{},
			},
			expectError: true,
		},
		{
			name: "conflicting authentication",
			config: MCPServerConfig{
				Auth: AuthConfig{
					Token:          "ghp_test123",
					AppID:          "123456",
					InstallationID: "789012",
					PrivateKeyPEM:  "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method, err := tt.config.getAuthMethod()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedMethod, method)
			}
		})
	}
}

func TestMCPServerConfig_createGitHubAppTransport(t *testing.T) {
	tests := []struct {
		name        string
		config      MCPServerConfig
		expectError bool
	}{
		{
			name: "invalid app ID",
			config: MCPServerConfig{
				Auth: AuthConfig{
					AppID:          "not-a-number",
					InstallationID: "789012",
					PrivateKeyPEM:  "any-key",
				},
			},
			expectError: true,
		},
		{
			name: "invalid installation ID",
			config: MCPServerConfig{
				Auth: AuthConfig{
					AppID:          "123456",
					InstallationID: "not-a-number",
					PrivateKeyPEM:  "any-key",
				},
			},
			expectError: true,
		},
		{
			name: "invalid private key",
			config: MCPServerConfig{
				Auth: AuthConfig{
					AppID:          "123456",
					InstallationID: "789012",
					PrivateKeyPEM:  "invalid-key",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport, err := tt.config.createGitHubAppTransport()

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, transport)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, transport)
			}
		})
	}
}
