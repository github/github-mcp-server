package utils //nolint:revive //TODO: figure out a better name for this package

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAPIHost(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantRestURL string
		wantErr     bool
		errContains string
	}{
		{
			name:        "empty string defaults to dotcom",
			input:       "",
			wantRestURL: "https://api.github.com/",
		},
		{
			name:        "github.com hostname",
			input:       "https://github.com",
			wantRestURL: "https://api.github.com/",
		},
		{
			name:        "subdomain of github.com",
			input:       "https://foo.github.com",
			wantRestURL: "https://api.github.com/",
		},
		{
			name:        "hostname ending in github.com but not a subdomain",
			input:       "https://mycompanygithub.com",
			wantRestURL: "https://mycompanygithub.com/api/v3/",
		},
		{
			name:        "hostname ending in notgithub.com",
			input:       "https://notgithub.com",
			wantRestURL: "https://notgithub.com/api/v3/",
		},
		{
			name:        "ghe.com hostname",
			input:       "https://ghe.com",
			wantRestURL: "https://api.ghe.com/",
		},
		{
			name:        "subdomain of ghe.com",
			input:       "https://mycompany.ghe.com",
			wantRestURL: "https://api.mycompany.ghe.com/",
		},
		{
			name:        "hostname ending in ghe.com but not a subdomain",
			input:       "https://myghe.com",
			wantRestURL: "https://myghe.com/api/v3/",
		},
		{
			name:    "missing scheme",
			input:   "github.com",
			wantErr: true,
		},
		{
			name:        "http GHES rejected to avoid cleartext credentials",
			input:       "http://ghes.example.com",
			wantErr:     true,
			errContains: "host must use https",
		},
		{
			name:        "http loopback allowed for local development",
			input:       "http://localhost",
			wantRestURL: "http://localhost/api/v3/",
		},
		{
			name:        "http 127.0.0.1 loopback allowed for local development",
			input:       "http://127.0.0.1",
			wantRestURL: "http://127.0.0.1/api/v3/",
		},
		{
			name:        "http loopback preserves port for local development",
			input:       "http://localhost:3000",
			wantRestURL: "http://localhost:3000/api/v3/",
		},
		{
			name:        "http ipv6 loopback preserves brackets",
			input:       "http://[::1]",
			wantRestURL: "http://[::1]/api/v3/",
		},
		{
			name:        "http ipv6 loopback preserves brackets and port",
			input:       "http://[::1]:8080",
			wantRestURL: "http://[::1]:8080/api/v3/",
		},
		{
			name:        "http remote host rejected",
			input:       "http://notgithub.com",
			wantErr:     true,
			errContains: "host must use https",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			host, err := parseAPIHost(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantRestURL, host.restURL.String())
		})
	}
}
