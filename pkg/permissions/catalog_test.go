package permissions

import "testing"

func TestCatalogGenerated(t *testing.T) {
	// A representative sample of permissions seeded by tools must exist with
	// the expected scope and levels.
	checks := []struct {
		perm   Permission
		scope  Scope
		levels []Level
	}{
		{Issues, ScopeRepo, []Level{LevelRead, LevelWrite}},
		{Contents, ScopeRepo, []Level{LevelRead, LevelWrite}},
		{PullRequests, ScopeRepo, []Level{LevelRead, LevelWrite}},
		{Actions, ScopeRepo, []Level{LevelRead, LevelWrite}},
		{SecurityEvents, ScopeRepo, []Level{LevelRead, LevelWrite}},
		{SecretScanningAlerts, ScopeRepo, []Level{LevelRead, LevelWrite}},
		{VulnerabilityAlerts, ScopeRepo, []Level{LevelRead, LevelWrite}},
		{OrganizationProjects, ScopeOrg, []Level{LevelRead, LevelWrite, LevelAdmin}},
		{Profile, ScopeAccount, []Level{LevelWrite}},
	}
	for _, c := range checks {
		e, ok := Lookup(c.perm)
		if !ok {
			t.Errorf("permission %q missing from catalog", c.perm)
			continue
		}
		if e.Scope != c.scope {
			t.Errorf("%q scope = %v, want %v", c.perm, e.Scope, c.scope)
		}
		if len(e.Levels) != len(c.levels) {
			t.Errorf("%q levels = %v, want %v", c.perm, e.Levels, c.levels)
		}
	}
}

func TestCatalogExcludesEnterprise(t *testing.T) {
	for p := range Catalog {
		if len(p) >= len("enterprise_") && p[:len("enterprise_")] == "enterprise_" {
			t.Errorf("enterprise permission %q should be excluded", p)
		}
	}
}

func TestMaxLevelAndValidLevel(t *testing.T) {
	if MaxLevel(OrganizationProjects) != LevelAdmin {
		t.Errorf("MaxLevel(OrganizationProjects) = %v, want admin", MaxLevel(OrganizationProjects))
	}
	if MaxLevel("does_not_exist") != LevelNone {
		t.Error("unknown permission should have MaxLevel none")
	}
	if !IsValidLevel(Issues, LevelWrite) {
		t.Error("issues:write should be valid")
	}
	if IsValidLevel(Issues, LevelAdmin) {
		t.Error("issues:admin should be invalid")
	}
}

func TestToolPermissionMapUniquePermissions(t *testing.T) {
	m := ToolPermissionMap{
		"a": Require(Issues.Write()),
		"b": Require(Issues.Read()),
		"c": AllOf(Contents.Read(), PullRequests.Read()),
	}
	got := m.UniquePermissions()
	want := []Permission{Contents, Issues, PullRequests}
	if len(got) != len(want) {
		t.Fatalf("UniquePermissions() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("UniquePermissions()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestGlobalToolPermissionMap(t *testing.T) {
	t.Cleanup(func() { SetGlobalToolPermissionMap(nil) })
	SetGlobalToolPermissionMap(ToolPermissionMap{"x": Require(Issues.Write())})
	if got := GetToolRequirement("x"); got.String() != "issues:write" {
		t.Errorf("GetToolRequirement(x) = %q", got.String())
	}
	if !GetToolRequirement("unknown").IsZero() {
		t.Error("unknown tool should return zero requirement")
	}
}
