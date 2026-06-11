package permissions

import (
	"reflect"
	"testing"
)

func TestLevelOrdering(t *testing.T) {
	if LevelNone >= LevelRead || LevelRead >= LevelWrite || LevelWrite >= LevelAdmin {
		t.Fatalf("levels are not strictly ordered")
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]Level{
		"read": LevelRead, "WRITE": LevelWrite, "admin": LevelAdmin, "nope": LevelNone, "": LevelNone,
	}
	for in, want := range cases {
		if got := ParseLevel(in); got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", in, got, want)
		}
	}
	if got := ParseLevel("  admin  "); got != LevelAdmin {
		t.Errorf("ParseLevel trims surrounding whitespace, got %v", got)
	}
}

func TestRequireSatisfiedBy(t *testing.T) {
	req := Require(Issues.Write())

	tests := []struct {
		name    string
		granted map[Permission]Level
		want    bool
	}{
		{"exact write", map[Permission]Level{Issues: LevelWrite}, true},
		{"admin satisfies write", map[Permission]Level{Issues: LevelAdmin}, true},
		{"read does not satisfy write", map[Permission]Level{Issues: LevelRead}, false},
		{"missing permission", map[Permission]Level{Contents: LevelWrite}, false},
		{"empty grant", map[Permission]Level{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := req.SatisfiedBy(tc.granted); got != tc.want {
				t.Errorf("SatisfiedBy(%v) = %v, want %v", tc.granted, got, tc.want)
			}
		})
	}
}

func TestZeroRequirementAlwaysSatisfied(t *testing.T) {
	var zero Requirement
	if !zero.IsZero() {
		t.Fatal("expected zero value to be IsZero")
	}
	if !zero.SatisfiedBy(nil) {
		t.Fatal("zero requirement must be satisfied by anything (no gate)")
	}
}

func TestAllOfIsAnd(t *testing.T) {
	req := AllOf(Contents.Read(), PullRequests.Read())
	if req.SatisfiedBy(map[Permission]Level{Contents: LevelRead}) {
		t.Error("AllOf must require every permission")
	}
	if !req.SatisfiedBy(map[Permission]Level{Contents: LevelRead, PullRequests: LevelWrite}) {
		t.Error("AllOf satisfied when all permissions held")
	}
}

func TestAnyOfIsOr(t *testing.T) {
	req := AnyOf(Require(Issues.Write()), Require(OrganizationProjects.Write()))
	if !req.SatisfiedBy(map[Permission]Level{Issues: LevelWrite}) {
		t.Error("AnyOf satisfied by first alternative")
	}
	if !req.SatisfiedBy(map[Permission]Level{OrganizationProjects: LevelWrite}) {
		t.Error("AnyOf satisfied by second alternative")
	}
	if req.SatisfiedBy(map[Permission]Level{Contents: LevelWrite}) {
		t.Error("AnyOf not satisfied by unrelated permission")
	}
}

func TestAndDistributes(t *testing.T) {
	// (issues:read) AND (contents:read OR pull_requests:read)
	req := Require(Issues.Read()).And(AnyOf(Require(Contents.Read()), Require(PullRequests.Read())))
	if !req.SatisfiedBy(map[Permission]Level{Issues: LevelRead, Contents: LevelRead}) {
		t.Error("expected satisfied via contents branch")
	}
	if !req.SatisfiedBy(map[Permission]Level{Issues: LevelRead, PullRequests: LevelRead}) {
		t.Error("expected satisfied via pull_requests branch")
	}
	if req.SatisfiedBy(map[Permission]Level{Contents: LevelRead}) {
		t.Error("missing required issues:read should fail")
	}
}

func TestAndWithZeroIsIdentity(t *testing.T) {
	req := Require(Issues.Write())
	var zero Requirement
	if !reflect.DeepEqual(req.And(zero), req) {
		t.Error("AND with zero on the right should be identity")
	}
	if !reflect.DeepEqual(zero.And(req), req) {
		t.Error("AND with zero on the left should be identity")
	}
}

func TestRequirementString(t *testing.T) {
	tests := []struct {
		name string
		req  Requirement
		want string
	}{
		{"single", Require(Issues.Write()), "issues:write"},
		{"and", AllOf(Contents.Read(), PullRequests.Read()), "contents:read AND pull_requests:read"},
		{"or", AnyOf(Require(Issues.Write()), Require(OrganizationProjects.Write())), "issues:write OR organization_projects:write"},
		{"zero", Requirement{}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.req.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPermissions(t *testing.T) {
	req := AnyOf(Require(Issues.Write()), AllOf(Contents.Read(), Issues.Read()))
	got := req.Permissions()
	want := []Permission{Contents, Issues}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Permissions() = %v, want %v", got, want)
	}
}

func TestNormalizeCollapsesDuplicateToHighest(t *testing.T) {
	req := Require(Issues.Read(), Issues.Write())
	if got := req.String(); got != "issues:write" {
		t.Errorf("duplicate permission should collapse to highest level, got %q", got)
	}
}
