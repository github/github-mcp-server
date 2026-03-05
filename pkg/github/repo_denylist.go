package github

import (
	"log/slog"
	"strings"
)

// RepoDenylist holds parsed denylist entries for efficient lookup.
// Supports exact matches ("owner/repo") and org-level wildcards ("owner/*").
type RepoDenylist struct {
	repos map[string]bool // exact "owner/repo" entries (lowercased)
	orgs  map[string]bool // org wildcard entries — "owner" when "owner/*" is configured (lowercased)
}

// NewRepoDenylist parses a slice of denylist entries into a RepoDenylist.
// Entries must be "owner/repo" (exact) or "owner/*" (org wildcard).
// Entries are normalized to lowercase. Invalid entries are silently skipped.
func NewRepoDenylist(entries []string) *RepoDenylist {
	d := &RepoDenylist{
		repos: make(map[string]bool),
		orgs:  make(map[string]bool),
	}
	for _, entry := range entries {
		entry = strings.TrimSpace(strings.ToLower(entry))
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			slog.Warn("denylist: skipping invalid entry (expected 'owner/repo' or 'owner/*')",
				"entry", entry)
			continue
		}
		owner, repo := parts[0], parts[1]
		if repo == "*" {
			d.orgs[owner] = true
		} else {
			d.repos[owner+"/"+repo] = true
		}
	}
	return d
}

// IsDenied reports whether the given owner/repo combination is on the denylist.
// Checks exact match first, then org wildcard. Case-insensitive.
// Safe to call on a nil *RepoDenylist (always returns false).
func (d *RepoDenylist) IsDenied(owner, repo string) bool {
	if d == nil {
		return false
	}
	key := strings.ToLower(owner) + "/" + strings.ToLower(repo)
	if d.repos[key] {
		return true
	}
	return d.orgs[strings.ToLower(owner)]
}

// IsOrgDenied reports whether all repos under the given org are denied
// (i.e., an "owner/*" wildcard entry exists). Used for search query pre-checks.
// Safe to call on a nil *RepoDenylist (always returns false).
func (d *RepoDenylist) IsOrgDenied(org string) bool {
	if d == nil {
		return false
	}
	return d.orgs[strings.ToLower(org)]
}

// IsEmpty reports whether the denylist has no entries.
// Safe to call on a nil *RepoDenylist (returns true).
func (d *RepoDenylist) IsEmpty() bool {
	if d == nil {
		return true
	}
	return len(d.repos) == 0 && len(d.orgs) == 0
}
