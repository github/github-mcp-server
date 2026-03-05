package github

// app_auth.go provides GitHub App authentication support using ghinstallation.
// This file establishes the dependency; full implementation follows in subsequent tasks.

import (
	// ghinstallation provides GitHub App installation token transport.
	// Used for authenticating as a GitHub App installation.
	_ "github.com/bradleyfalzon/ghinstallation/v2"
)
