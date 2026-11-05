package skills

import (
	"fmt"
	"regexp"
	"strings"
)

// nameRe is the Agent Skills naming rule that the final <skill-path> segment
// must satisfy: lowercase letters and digits in hyphen-separated groups.
var nameRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// regNameRe approximates RFC 3986 reg-name: unreserved / pct-encoded /
// sub-delims. The first <skill-path> segment occupies the authority component
// and SHOULD satisfy it.
var regNameRe = regexp.MustCompile(`^(?:[A-Za-z0-9\-._~!$&'()*+,;=]|%[0-9A-Fa-f]{2})+$`)

// MaxNameLength is the Agent Skills limit on a skill name.
const MaxNameLength = 64

// URI is a parsed skill:// resource URI. The zero value is not meaningful; use
// ParseURI.
type URI struct {
	// Scheme is the URI scheme, always URIScheme for URIs this package mints.
	Scheme string
	// SkillPath locates the skill directory within the server's namespace, as
	// a "/"-separated path of one or more segments. Its final segment is the
	// skill name; any preceding segments are a server-chosen organizational
	// prefix.
	SkillPath string
	// FilePath is the file's path relative to the skill directory root. It is
	// empty for the skill root URI.
	FilePath string
}

// Name returns the skill name: the final segment of the skill path.
func (u URI) Name() string {
	if i := strings.LastIndex(u.SkillPath, "/"); i >= 0 {
		return u.SkillPath[i+1:]
	}
	return u.SkillPath
}

// Root returns the skill's root directory URI: the SKILL.md suffix removed, no
// trailing slash.
func (u URI) Root() string {
	return u.Scheme + "://" + u.SkillPath
}

// String renders the URI. A URI with no file path renders as the skill root.
func (u URI) String() string {
	if u.FilePath == "" {
		return u.Root()
	}
	return u.Root() + "/" + u.FilePath
}

// SkillURI builds the URI of a file within a skill. An empty filePath yields
// the skill root.
func SkillURI(skillPath, filePath string) string {
	u := URI{Scheme: URIScheme, SkillPath: skillPath, FilePath: filePath}
	return u.String()
}

// ParseURI parses a skill:// URI into its skill path and file path. It does not
// validate that the referenced skill exists.
//
// A URI naming only the skill path — no file path — parses as the skill root.
// Callers that require a file, such as the SKILL.md URI in a skills/get
// request, must check FilePath themselves.
func ParseURI(raw string) (URI, error) {
	rest, ok := strings.CutPrefix(raw, URIScheme+"://")
	if !ok {
		return URI{}, fmt.Errorf("not a %s:// URI: %q", URIScheme, raw)
	}
	if rest == "" {
		return URI{}, fmt.Errorf("missing skill path: %q", raw)
	}
	if strings.HasSuffix(rest, "/") {
		return URI{}, fmt.Errorf("trailing slash is not permitted: %q", raw)
	}
	if strings.Contains(rest, "//") {
		return URI{}, fmt.Errorf("empty path segment: %q", raw)
	}
	// The final SKILL.md, when present, terminates the skill path: everything
	// before it is the skill path and everything after would be a file inside
	// a nested skill, which is addressed under its own skill path.
	if skillPath, ok := strings.CutSuffix(rest, "/"+SkillFile); ok {
		u := URI{Scheme: URIScheme, SkillPath: skillPath, FilePath: SkillFile}
		return u, validateSkillPath(u.SkillPath)
	}
	// Otherwise the caller has not told us where the skill path ends. Treat the
	// whole thing as a skill root; ParseFileURI resolves file URIs against a
	// known skill path instead of guessing.
	u := URI{Scheme: URIScheme, SkillPath: rest}
	return u, validateSkillPath(u.SkillPath)
}

// validateSkillPath enforces the structural rules on <skill-path>: a non-empty
// "/"-separated path whose final segment is a valid skill name and whose first
// segment is a plausible RFC 3986 reg-name.
func validateSkillPath(skillPath string) error {
	if skillPath == "" {
		return fmt.Errorf("empty skill path")
	}
	segments := strings.Split(skillPath, "/")
	for _, s := range segments {
		if s == "" {
			return fmt.Errorf("empty segment in skill path %q", skillPath)
		}
		if s == "." || s == ".." {
			return fmt.Errorf("relative segment %q in skill path %q", s, skillPath)
		}
	}
	if err := ValidateName(segments[len(segments)-1]); err != nil {
		return fmt.Errorf("skill path %q: %w", skillPath, err)
	}
	// The authority component carries no semantics and clients must never
	// resolve it, but a malformed authority makes the URI unparseable by
	// generic URI machinery, so hold servers to it.
	if !regNameRe.MatchString(segments[0]) {
		return fmt.Errorf("skill path %q: first segment %q is not a valid reg-name", skillPath, segments[0])
	}
	return nil
}

// ValidateName reports whether a skill name satisfies the Agent Skills naming
// rules. The final segment of a skill path must equal the skill's frontmatter
// name, so both are checked against this.
func ValidateName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("skill name is empty")
	case len(name) > MaxNameLength:
		return fmt.Errorf("skill name %q exceeds %d characters", name, MaxNameLength)
	case !nameRe.MatchString(name):
		return fmt.Errorf("skill name %q must be lowercase alphanumeric segments separated by hyphens", name)
	}
	return nil
}

// validateFilePath enforces that a supporting file path stays inside the skill
// directory. Escaping it would let a skill's manifest claim files that are not
// the skill's, which the origin-scoping rules forbid.
func validateFilePath(filePath string) error {
	switch {
	case filePath == "":
		return fmt.Errorf("empty file path")
	case strings.HasPrefix(filePath, "/"), strings.HasSuffix(filePath, "/"):
		return fmt.Errorf("file path %q must not begin or end with a separator", filePath)
	case strings.Contains(filePath, "//"):
		return fmt.Errorf("file path %q contains an empty segment", filePath)
	}
	for s := range strings.SplitSeq(filePath, "/") {
		if s == "." || s == ".." {
			return fmt.Errorf("file path %q must not contain relative segments", filePath)
		}
	}
	return nil
}

// WithinSkill reports whether uri addresses the skill rooted at skillPath —
// either its SKILL.md or a file within its directory. It is the check behind
// the origin-scoping rule that a skill's resources may only name that skill's
// own files.
func WithinSkill(skillPath, uri string) bool {
	root := SkillURI(skillPath, "")
	if uri == root {
		return false // the root is a directory, not a file of the skill
	}
	suffix, ok := strings.CutPrefix(uri, root+"/")
	return ok && validateFilePath(suffix) == nil
}
