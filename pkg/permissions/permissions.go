// Package permissions provides a declarative, typed model for the fine-grained
// permissions (FGP) that GitHub fine-grained tokens carry. It mirrors the
// pkg/scopes subsystem: pkg/scopes models classic OAuth scopes, while this
// package models the fine-grained permission requirements of MCP tools.
//
// The typed permission catalog (one Permission constant per public permission,
// its scope, and its valid levels) is generated from the PUBLIC
// github/rest-api-description OpenAPI description (the components/schemas/
// app-permissions schema). See gen.go. Per-tool requirements are hand-authored
// at the tool definition site, exactly like RequiredScopes.
//
// This package contains ONLY public data: permission names and levels are
// published in the REST API documentation and in the public OpenAPI schema.
package permissions

//go:generate go run gen.go

import (
	"slices"
	"sort"
	"strings"
)

// Permission is a typed GitHub fine-grained permission name, e.g. "issues" or
// "organization_administration". The set of valid values is enumerated in
// catalog_generated.go.
type Permission string

// Level is an access level for a permission. Levels form an ordered lattice
// (read < write < admin); a higher level satisfies a requirement for a lower
// level (holding "write" satisfies a "read" requirement).
type Level int

const (
	// LevelNone means no access (the zero value).
	LevelNone Level = iota
	// LevelRead grants read access.
	LevelRead
	// LevelWrite grants write access (implies read).
	LevelWrite
	// LevelAdmin grants admin access (implies write and read).
	LevelAdmin
)

// String returns the lowercase API name of the level (read/write/admin), or
// "none" for LevelNone.
func (l Level) String() string {
	switch l {
	case LevelRead:
		return "read"
	case LevelWrite:
		return "write"
	case LevelAdmin:
		return "admin"
	default:
		return "none"
	}
}

// ParseLevel converts a level name from the public schema (read/write/admin)
// into a Level. Unknown names map to LevelNone.
func ParseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "read":
		return LevelRead
	case "write":
		return LevelWrite
	case "admin":
		return LevelAdmin
	default:
		return LevelNone
	}
}

// Scope describes where a permission is granted. It is part of the typed
// permission value: bare permission names are granted at the repository or
// account level, while organization_* permissions are granted at the
// organization level.
type Scope int

const (
	// ScopeRepo is a repository-level permission (the common case).
	ScopeRepo Scope = iota
	// ScopeOrg is an organization-level permission.
	ScopeOrg
	// ScopeAccount is a user-account-level permission.
	ScopeAccount
)

// String returns a human-readable name for the scope.
func (s Scope) String() string {
	switch s {
	case ScopeOrg:
		return "organization"
	case ScopeAccount:
		return "account"
	default:
		return "repository"
	}
}

// Read returns a PermReq requiring at least read access to the permission.
func (p Permission) Read() PermReq { return PermReq{Perm: p, Min: LevelRead} }

// Write returns a PermReq requiring at least write access to the permission.
func (p Permission) Write() PermReq { return PermReq{Perm: p, Min: LevelWrite} }

// Admin returns a PermReq requiring admin access to the permission.
func (p Permission) Admin() PermReq { return PermReq{Perm: p, Min: LevelAdmin} }

// PermReq is a single permission requirement: a permission together with the
// minimum level needed to satisfy it.
type PermReq struct {
	Perm Permission
	Min  Level
}

// String renders the requirement as "permission:level", e.g. "issues:write".
func (r PermReq) String() string {
	return string(r.Perm) + ":" + r.Min.String()
}

// Requirement expresses the fine-grained permissions a tool needs. It is an
// OR of AND-sets ("any of these alternatives, where an alternative requires
// all of its permissions"):
//
//	anyOf = [ [a AND b], [c] ]  =>  (a AND b) OR (c)
//
// The zero value is an empty requirement, which means "no gate": a tool with a
// zero-value Requirement is always shown.
type Requirement struct {
	// anyOf holds the alternatives. Each inner slice is an AND-set of
	// permission requirements. Unexported so the structure can only be built
	// through the combinators below, keeping invariants (sorted, deduped).
	anyOf [][]PermReq
}

// IsZero reports whether the requirement is empty (no gate).
func (r Requirement) IsZero() bool {
	return len(r.anyOf) == 0
}

// Require builds a requirement satisfied when ALL of the given permission
// requirements are held. This is the idiomatic constructor for the common
// single-endpoint tool, e.g. Require(Issues.Write()).
func Require(rs ...PermReq) Requirement {
	return newRequirement(rs)
}

// AllOf is a synonym for Require, reading more naturally when listing several
// permissions that are all required together.
func AllOf(rs ...PermReq) Requirement {
	return newRequirement(rs)
}

// AnyOf combines requirements with OR: the result is satisfied if any of the
// given requirements is satisfied. Empty inputs are ignored.
func AnyOf(reqs ...Requirement) Requirement {
	var out Requirement
	for _, req := range reqs {
		out.anyOf = append(out.anyOf, req.anyOf...)
	}
	out.normalize()
	return out
}

// And combines two requirements with AND. The result is the distribution of
// the two OR-of-AND forms: every alternative of r is concatenated with every
// alternative of other. This models a tool that calls multiple endpoints, each
// contributing its own permission requirement.
func (r Requirement) And(other Requirement) Requirement {
	if r.IsZero() {
		return other
	}
	if other.IsZero() {
		return r
	}
	var out Requirement
	for _, a := range r.anyOf {
		for _, b := range other.anyOf {
			combined := make([]PermReq, 0, len(a)+len(b))
			combined = append(combined, a...)
			combined = append(combined, b...)
			out.anyOf = append(out.anyOf, combined)
		}
	}
	out.normalize()
	return out
}

// SatisfiedBy reports whether the granted permissions satisfy the requirement.
// An empty requirement is always satisfied. The granted map records the level
// held for each permission; a higher granted level satisfies a lower minimum.
func (r Requirement) SatisfiedBy(granted map[Permission]Level) bool {
	if r.IsZero() {
		return true
	}
	for _, andSet := range r.anyOf {
		if andSetSatisfied(andSet, granted) {
			return true
		}
	}
	return false
}

func andSetSatisfied(andSet []PermReq, granted map[Permission]Level) bool {
	for _, req := range andSet {
		if granted[req.Perm] < req.Min {
			return false
		}
	}
	return true
}

// Permissions returns the sorted, de-duplicated set of permissions referenced
// anywhere in the requirement.
func (r Requirement) Permissions() []Permission {
	seen := make(map[Permission]struct{})
	for _, andSet := range r.anyOf {
		for _, req := range andSet {
			seen[req.Perm] = struct{}{}
		}
	}
	out := make([]Permission, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}

// String renders the requirement for documentation and CLI output. AND-sets
// are joined with " AND " and alternatives with " OR ", e.g.:
//
//	"issues:write"
//	"contents:read AND pull_requests:read"
//	"issues:write OR organization_projects:write"
//
// The empty requirement renders as an empty string.
func (r Requirement) String() string {
	if r.IsZero() {
		return ""
	}
	alts := make([]string, 0, len(r.anyOf))
	for _, andSet := range r.anyOf {
		parts := make([]string, 0, len(andSet))
		for _, req := range andSet {
			parts = append(parts, req.String())
		}
		alts = append(alts, strings.Join(parts, " AND "))
	}
	return strings.Join(alts, " OR ")
}

// newRequirement constructs a single-alternative requirement from an AND-set,
// dropping zero-level entries and normalizing.
func newRequirement(rs []PermReq) Requirement {
	andSet := make([]PermReq, 0, len(rs))
	for _, r := range rs {
		if r.Perm == "" || r.Min == LevelNone {
			continue
		}
		andSet = append(andSet, r)
	}
	if len(andSet) == 0 {
		return Requirement{}
	}
	out := Requirement{anyOf: [][]PermReq{andSet}}
	out.normalize()
	return out
}

// normalize sorts each AND-set and the list of alternatives so that equal
// requirements have an identical structure, giving deterministic output for
// docs and tests. Within an AND-set, duplicate permissions collapse to their
// highest required level.
func (r *Requirement) normalize() {
	for i, andSet := range r.anyOf {
		highest := make(map[Permission]Level)
		for _, req := range andSet {
			if req.Min > highest[req.Perm] {
				highest[req.Perm] = req.Min
			}
		}
		deduped := make([]PermReq, 0, len(highest))
		for perm, lvl := range highest {
			deduped = append(deduped, PermReq{Perm: perm, Min: lvl})
		}
		sort.Slice(deduped, func(a, b int) bool {
			if deduped[a].Perm != deduped[b].Perm {
				return deduped[a].Perm < deduped[b].Perm
			}
			return deduped[a].Min < deduped[b].Min
		})
		r.anyOf[i] = deduped
	}
	sort.Slice(r.anyOf, func(a, b int) bool {
		return andSetKey(r.anyOf[a]) < andSetKey(r.anyOf[b])
	})
}

func andSetKey(andSet []PermReq) string {
	parts := make([]string, len(andSet))
	for i, req := range andSet {
		parts[i] = req.String()
	}
	return strings.Join(parts, ",")
}
