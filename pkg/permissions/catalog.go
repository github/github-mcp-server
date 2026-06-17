package permissions

import "slices"

// CatalogEntry describes a single fine-grained permission as published in the
// public OpenAPI app-permissions schema: its scope and the access levels that
// the permission accepts.
type CatalogEntry struct {
	// Permission is the typed permission name.
	Permission Permission
	// Scope is where the permission is granted (repo, org, or account).
	Scope Scope
	// Levels are the valid access levels for this permission, ascending.
	Levels []Level
}

// Lookup returns the catalog entry for a permission, if known.
func Lookup(p Permission) (CatalogEntry, bool) {
	e, ok := Catalog[p]
	return e, ok
}

// ScopeOf returns the scope of a permission, defaulting to ScopeRepo for
// permissions not present in the catalog.
func ScopeOf(p Permission) Scope {
	if e, ok := Catalog[p]; ok {
		return e.Scope
	}
	return ScopeRepo
}

// MaxLevel returns the highest level a permission can be granted at, or
// LevelNone if the permission is unknown.
func MaxLevel(p Permission) Level {
	e, ok := Catalog[p]
	if !ok || len(e.Levels) == 0 {
		return LevelNone
	}
	highest := LevelNone
	for _, l := range e.Levels {
		if l > highest {
			highest = l
		}
	}
	return highest
}

// IsValidLevel reports whether the permission accepts the given level.
func IsValidLevel(p Permission, l Level) bool {
	e, ok := Catalog[p]
	if !ok {
		return false
	}
	return slices.Contains(e.Levels, l)
}

// AllPermissions returns every permission in the catalog, sorted by name.
func AllPermissions() []Permission {
	out := make([]Permission, 0, len(Catalog))
	for p := range Catalog {
		out = append(out, p)
	}
	sortPermissions(out)
	return out
}

func sortPermissions(ps []Permission) {
	// insertion sort keeps this dependency-free and the slice is small
	for i := 1; i < len(ps); i++ {
		for j := i; j > 0 && ps[j] < ps[j-1]; j-- {
			ps[j], ps[j-1] = ps[j-1], ps[j]
		}
	}
}
