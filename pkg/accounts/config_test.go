package accounts

import (
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with org matcher",
			config: Config{
				Accounts: []Account{
					{
						Name:  "work",
						Token: "ghp_work",
						Matcher: AccountMatcher{
							Type:   "org",
							Values: []string{"my-company"},
						},
					},
					{
						Name:  "personal",
						Token: "ghp_personal",
						Matcher: AccountMatcher{
							Type:   "org",
							Values: []string{"drtootsie"},
						},
						Default: true,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with repo_pattern matcher",
			config: Config{
				Accounts: []Account{
					{
						Name:  "work",
						Token: "ghp_work",
						Matcher: AccountMatcher{
							Type:   "repo_pattern",
							Values: []string{"my-company/*", "other-org/specific-repo"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with all matcher",
			config: Config{
				Accounts: []Account{
					{
						Name:  "default",
						Token: "ghp_default",
						Matcher: AccountMatcher{
							Type: "all",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "no accounts",
			config:  Config{Accounts: []Account{}},
			wantErr: true,
			errMsg:  "at least one account must be configured",
		},
		{
			name: "missing account name",
			config: Config{
				Accounts: []Account{
					{
						Token: "ghp_token",
						Matcher: AccountMatcher{
							Type:   "org",
							Values: []string{"myorg"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "missing token",
			config: Config{
				Accounts: []Account{
					{
						Name: "work",
						Matcher: AccountMatcher{
							Type:   "org",
							Values: []string{"myorg"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "token is required",
		},
		{
			name: "missing matcher type",
			config: Config{
				Accounts: []Account{
					{
						Name:  "work",
						Token: "ghp_work",
						Matcher: AccountMatcher{
							Values: []string{"myorg"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "matcher type is required",
		},
		{
			name: "invalid matcher type",
			config: Config{
				Accounts: []Account{
					{
						Name:  "work",
						Token: "ghp_work",
						Matcher: AccountMatcher{
							Type:   "invalid",
							Values: []string{"myorg"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid matcher type",
		},
		{
			name: "org matcher without values",
			config: Config{
				Accounts: []Account{
					{
						Name:  "work",
						Token: "ghp_work",
						Matcher: AccountMatcher{
							Type: "org",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "matcher values are required",
		},
		{
			name: "multiple default accounts",
			config: Config{
				Accounts: []Account{
					{
						Name:  "work",
						Token: "ghp_work",
						Matcher: AccountMatcher{
							Type:   "org",
							Values: []string{"work"},
						},
						Default: true,
					},
					{
						Name:  "personal",
						Token: "ghp_personal",
						Matcher: AccountMatcher{
							Type:   "org",
							Values: []string{"personal"},
						},
						Default: true,
					},
				},
			},
			wantErr: true,
			errMsg:  "only one account can be marked as default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && !contains(err.Error(), tt.errMsg) {
				t.Errorf("Config.Validate() error = %v, want error containing %q", err, tt.errMsg)
			}
		})
	}
}

func TestConfig_GetDefaultAccount(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   string // account name
	}{
		{
			name: "explicit default",
			config: Config{
				Accounts: []Account{
					{Name: "work", Token: "ghp_work", Matcher: AccountMatcher{Type: "all"}},
					{Name: "personal", Token: "ghp_personal", Matcher: AccountMatcher{Type: "all"}, Default: true},
				},
			},
			want: "personal",
		},
		{
			name: "no explicit default, uses first",
			config: Config{
				Accounts: []Account{
					{Name: "work", Token: "ghp_work", Matcher: AccountMatcher{Type: "all"}},
					{Name: "personal", Token: "ghp_personal", Matcher: AccountMatcher{Type: "all"}},
				},
			},
			want: "work",
		},
		{
			name:   "empty config",
			config: Config{Accounts: []Account{}},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := tt.config.GetDefaultAccount()
			if tt.want == "" {
				if account != nil {
					t.Errorf("Config.GetDefaultAccount() = %v, want nil", account)
				}
			} else {
				if account == nil || account.Name != tt.want {
					var got string
					if account != nil {
						got = account.Name
					}
					t.Errorf("Config.GetDefaultAccount() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestRouter_SelectAccount(t *testing.T) {
	config := Config{
		Accounts: []Account{
			{
				Name:  "work",
				Token: "ghp_work",
				Matcher: AccountMatcher{
					Type:   "org",
					Values: []string{"my-company", "work-org"},
				},
			},
			{
				Name:  "personal",
				Token: "ghp_personal",
				Matcher: AccountMatcher{
					Type:   "org",
					Values: []string{"drtootsie", "my-personal-org"},
				},
				Default: true,
			},
			{
				Name:  "specific-repos",
				Token: "ghp_specific",
				Matcher: AccountMatcher{
					Type:   "repo_pattern",
					Values: []string{"some-org/specific-repo", "other-org/*"},
				},
			},
		},
	}

	router := NewRouter(&config)

	tests := []struct {
		name      string
		owner     string
		repo      string
		wantAcct  string
	}{
		{
			name:     "matches work org",
			owner:    "my-company",
			repo:     "some-repo",
			wantAcct: "work",
		},
		{
			name:     "matches work org (second value)",
			owner:    "work-org",
			repo:     "another-repo",
			wantAcct: "work",
		},
		{
			name:     "matches personal org",
			owner:    "drtootsie",
			repo:     "my-repo",
			wantAcct: "personal",
		},
		{
			name:     "matches specific repo",
			owner:    "some-org",
			repo:     "specific-repo",
			wantAcct: "specific-repos",
		},
		{
			name:     "matches wildcard pattern",
			owner:    "other-org",
			repo:     "any-repo",
			wantAcct: "specific-repos",
		},
		{
			name:     "no match, uses default",
			owner:    "unknown-org",
			repo:     "unknown-repo",
			wantAcct: "personal",
		},
		{
			name:     "case insensitive org match",
			owner:    "MY-COMPANY",
			repo:     "repo",
			wantAcct: "work",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := router.SelectAccount(tt.owner, tt.repo)
			if account == nil {
				t.Errorf("Router.SelectAccount() returned nil")
				return
			}
			if account.Name != tt.wantAcct {
				t.Errorf("Router.SelectAccount() = %v, want %v", account.Name, tt.wantAcct)
			}
		})
	}
}

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		fullName string
		want     bool
	}{
		{
			name:     "exact match",
			pattern:  "owner/repo",
			fullName: "owner/repo",
			want:     true,
		},
		{
			name:     "wildcard owner",
			pattern:  "owner/*",
			fullName: "owner/repo",
			want:     true,
		},
		{
			name:     "wildcard owner, different repo",
			pattern:  "owner/*",
			fullName: "owner/another-repo",
			want:     true,
		},
		{
			name:     "wildcard owner, wrong owner",
			pattern:  "owner/*",
			fullName: "other/repo",
			want:     false,
		},
		{
			name:     "partial match (not anchored)",
			pattern:  "owner",
			fullName: "owner/repo",
			want:     false,
		},
		{
			name:     "wildcard in middle",
			pattern:  "owner/*/subdir",
			fullName: "owner/something/subdir",
			want:     true,
		},
		{
			name:     "special regex characters escaped",
			pattern:  "owner/repo.test",
			fullName: "owner/repo.test",
			want:     true,
		},
		{
			name:     "special regex characters not matching",
			pattern:  "owner/repo.test",
			fullName: "owner/repoXtest",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesPattern(tt.pattern, tt.fullName); got != tt.want {
				t.Errorf("matchesPattern(%q, %q) = %v, want %v", tt.pattern, tt.fullName, got, tt.want)
			}
		})
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
