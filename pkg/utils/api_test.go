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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			host, err := parseAPIHost(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantRestURL, host.restURL.String())
		})
	}
}

func TestParseHostType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		host string
		want HostType
	}{
		{name: "GitHub.com is case insensitive", host: "https://API.GITHUB.COM", want: HostTypeDotcom},
		{name: "GHEC is case insensitive", host: "https://API.EXAMPLE.GHE.COM", want: HostTypeGHEC},
		{name: "lookalike domain is GHES", host: "https://api.github.com.example.org", want: HostTypeGHES},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseHostType(tt.host)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
