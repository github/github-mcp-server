package accounts

import (
	"fmt"
	"regexp"
	"strings"
)

// Account represents a single GitHub account configuration
type Account struct {
	// Name is a friendly identifier for this account
	Name string `json:"name"`

	// Token is the GitHub PAT or OAuth token for this account
	Token string `json:"token"`

	// Matcher defines when to use this account
	Matcher AccountMatcher `json:"matcher"`

	// Default indicates if this is the fallback account
	Default bool `json:"default,omitempty"`
}

// AccountMatcher determines which repositories/orgs use this account
type AccountMatcher struct {
	// Type specifies the matching strategy: "org", "repo_pattern", or "all"
	Type string `json:"type"`

	// Values contains the list of orgs or patterns to match
	// For "org" type: list of organization names
	// For "repo_pattern" type: list of owner/repo patterns (supports wildcards)
	Values []string `json:"values,omitempty"`
}

// Config holds the multi-account configuration
type Config struct {
	// Accounts is the list of configured accounts
	Accounts []Account `json:"accounts"`
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if len(c.Accounts) == 0 {
		return fmt.Errorf("at least one account must be configured")
	}

	defaultCount := 0
	for i, account := range c.Accounts {
		if account.Name == "" {
			return fmt.Errorf("account %d: name is required", i)
		}
		if account.Token == "" {
			return fmt.Errorf("account %s: token is required", account.Name)
		}
		if account.Matcher.Type == "" {
			return fmt.Errorf("account %s: matcher type is required", account.Name)
		}

		// Validate matcher type
		switch account.Matcher.Type {
		case "org", "repo_pattern":
			if len(account.Matcher.Values) == 0 {
				return fmt.Errorf("account %s: matcher values are required for type %s", account.Name, account.Matcher.Type)
			}
		case "all":
			// "all" matcher doesn't need values
		default:
			return fmt.Errorf("account %s: invalid matcher type %s (must be 'org', 'repo_pattern', or 'all')", account.Name, account.Matcher.Type)
		}

		if account.Default {
			defaultCount++
		}
	}

	if defaultCount > 1 {
		return fmt.Errorf("only one account can be marked as default")
	}

	return nil
}

// GetDefaultAccount returns the default account, or the first account if none is marked default
func (c *Config) GetDefaultAccount() *Account {
	for i := range c.Accounts {
		if c.Accounts[i].Default {
			return &c.Accounts[i]
		}
	}

	// If no default is specified, use the first account
	if len(c.Accounts) > 0 {
		return &c.Accounts[0]
	}

	return nil
}

// Router selects the appropriate account based on repository context
type Router struct {
	config *Config
}

// NewRouter creates a new account router
func NewRouter(config *Config) *Router {
	return &Router{config: config}
}

// SelectAccount chooses the appropriate account for the given owner/repo
func (r *Router) SelectAccount(owner, repo string) *Account {
	// Try to match against all accounts
	for i := range r.config.Accounts {
		account := &r.config.Accounts[i]
		if r.matches(account, owner, repo) {
			return account
		}
	}

	// Fall back to default account
	return r.config.GetDefaultAccount()
}

// matches checks if the account matcher matches the given owner/repo
func (r *Router) matches(account *Account, owner, repo string) bool {
	switch account.Matcher.Type {
	case "org":
		// Match if owner is in the list of organizations
		for _, org := range account.Matcher.Values {
			if strings.EqualFold(owner, org) {
				return true
			}
		}
		return false

	case "repo_pattern":
		// Match against owner/repo patterns (supports wildcards)
		fullName := owner + "/" + repo
		for _, pattern := range account.Matcher.Values {
			if matchesPattern(pattern, fullName) {
				return true
			}
		}
		return false

	case "all":
		// Matches everything
		return true

	default:
		return false
	}
}

// matchesPattern checks if a full repository name matches a pattern
// Supports wildcards: "owner/*" matches all repos in owner
func matchesPattern(pattern, fullName string) bool {
	// Convert glob pattern to regex
	// Escape special regex characters except *
	escaped := regexp.QuoteMeta(pattern)
	// Replace escaped \* with .*
	regexPattern := strings.ReplaceAll(escaped, "\\*", ".*")
	// Anchor the pattern
	regexPattern = "^" + regexPattern + "$"

	matched, err := regexp.MatchString(regexPattern, fullName)
	if err != nil {
		return false
	}
	return matched
}
