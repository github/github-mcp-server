package skills

import (
	"context"
	"fmt"
	"path"
	"slices"
	"sort"
	"strings"
)

// Source supplies the skills a server publishes.
//
// A listing may be empty or partial — a server with a large, generated, or
// otherwise unenumerable catalog is not required to enumerate it — but a source
// MUST still answer Get for every skill it serves, including skills that never
// appear in a listing.
type Source interface {
	// List returns the entries the server publishes, in a stable order. The
	// order must not change between paginated calls, or an atomic page
	// boundary would not be reproducible.
	List(ctx context.Context) ([]Entry, error)
	// Get returns the entry for the skill whose SKILL.md is at uri. The
	// boolean reports whether the server serves that skill at all; an unknown
	// URI is reported as not found rather than as an error.
	Get(ctx context.Context, uri string) (Entry, bool, error)
}

// DirectorySource is implemented by a Source that can enumerate the children of
// a directory in its namespace. A server declares directoryRead only when its
// source implements this, so that the capability reflects a genuine
// implementation rather than an aspiration.
type DirectorySource interface {
	Source
	// ReadDirectory returns the direct children of a directory URI, in a
	// stable order. It reports false when the URI does not exist or exists but
	// is not a directory; both cases are invalid params to the caller. An
	// existing but empty directory returns an empty slice and true.
	ReadDirectory(ctx context.Context, uri string) ([]Child, bool, error)
}

// ContentSource is implemented by a Source that holds file content and can
// therefore have its files registered as ordinary MCP resources.
type ContentSource interface {
	Source
	// Files returns every file the source serves, as resource descriptors
	// paired with their content, in a stable order.
	Files(ctx context.Context) ([]ResourceFile, error)
}

// Child is one entry of a directory listing: either a file or a subdirectory.
type Child struct {
	// URI is the child's resource URI, written without a trailing slash for
	// directories.
	URI string
	// Name is the child's final path segment.
	Name string
	// Description is optional resource metadata.
	Description string
	// MIMEType is DirectoryMIMEType for a subdirectory, or the file's media
	// type otherwise.
	MIMEType string
	// Size is the file's raw byte length, and zero for a directory.
	Size int64
}

// IsDir reports whether the child is a subdirectory.
func (c Child) IsDir() bool { return c.MIMEType == DirectoryMIMEType }

// ResourceFile pairs a skill file with the URI it is served at.
type ResourceFile struct {
	// URI is the file's resource URI.
	URI string
	// SkillPath is the path of the skill the file belongs to.
	SkillPath string
	// File is the file itself.
	File File
	// Name and Description carry the resource metadata for the file. For a
	// SKILL.md these come from the skill's frontmatter.
	Name        string
	Description string
}

// Registry is an in-memory Source holding a fixed set of validated skills. It
// implements DirectorySource and ContentSource, so a server built on it serves
// the whole extension.
//
// A Registry is immutable once built and is safe for concurrent use.
type Registry struct {
	order    []string // skill paths, sorted
	skills   map[string]*Skill
	byURI    map[string]*Skill // SKILL.md URI to skill
	files    map[string]ResourceFile
	children map[string][]Child // directory URI to sorted children
}

// NewRegistry builds a registry from already-validated skills.
//
// Two skills may share a name — names are labels, not identifiers — but two
// skills may not share a path, because the path is what identifies a skill
// within a server.
//
// Skills may nest. From an enclosing skill's perspective a nested skill's files
// are ordinary supporting files, so the same URI is legitimately claimed by both
// skills; the registry serves it once. Two skills claiming one URI with
// different content is an inconsistency and is rejected.
func NewRegistry(skills ...*Skill) (*Registry, error) {
	r := &Registry{
		skills:   make(map[string]*Skill, len(skills)),
		byURI:    make(map[string]*Skill, len(skills)),
		files:    make(map[string]ResourceFile),
		children: make(map[string][]Child),
	}
	dirSets := make(map[string]map[string]Child)

	for _, s := range skills {
		if s == nil {
			return nil, fmt.Errorf("nil skill")
		}
		if _, dup := r.skills[s.SkillPath]; dup {
			return nil, fmt.Errorf("duplicate skill path %q", s.SkillPath)
		}
		r.skills[s.SkillPath] = s
		r.byURI[s.URI()] = s
		r.order = append(r.order, s.SkillPath)
		indexAncestors(dirSets, s.SkillPath)

		for _, f := range s.Files() {
			uri := SkillURI(s.SkillPath, f.Path)
			rf := ResourceFile{URI: uri, SkillPath: s.SkillPath, File: f, Name: path.Base(f.Path)}
			if f.Path == SkillFile {
				rf.Name = s.Name()
				rf.Description = s.Description()
			}
			if existing, dup := r.files[uri]; dup {
				if Digest(existing.File.Content) != Digest(f.Content) {
					return nil, fmt.Errorf("resource %q is served with different content by %q and %q", uri, existing.SkillPath, s.SkillPath)
				}
				// The more deeply nested skill roots at this file's directory,
				// so its metadata describes the file most accurately.
				if len(existing.SkillPath) >= len(s.SkillPath) {
					continue
				}
			}
			r.files[uri] = rf
			indexFile(dirSets, s.SkillPath, f)
		}
	}

	sort.Strings(r.order)
	for dir, set := range dirSets {
		children := make([]Child, 0, len(set))
		for _, c := range set {
			children = append(children, c)
		}
		sort.Slice(children, func(i, j int) bool { return children[i].URI < children[j].URI })
		r.children[dir] = children
	}
	return r, nil
}

// indexAncestors records the directories above a skill root, so that every
// directory in the served namespace answers a directory read rather than only
// those that happen to contain a file directly.
func indexAncestors(dirSets map[string]map[string]Child, skillPath string) {
	segments := strings.Split(skillPath, "/")
	dir := URIScheme + "://" + segments[0]
	if _, ok := dirSets[dir]; !ok {
		dirSets[dir] = make(map[string]Child)
	}
	for _, seg := range segments[1:] {
		childURI := dir + "/" + seg
		dirSets[dir][childURI] = Child{URI: childURI, Name: seg, MIMEType: DirectoryMIMEType}
		if _, ok := dirSets[childURI]; !ok {
			dirSets[childURI] = make(map[string]Child)
		}
		dir = childURI
	}
}

// indexFile records a file and every directory level above it, so that a
// directory read can answer for the skill root and each subdirectory.
func indexFile(dirSets map[string]map[string]Child, skillPath string, f File) {
	add := func(dir string, c Child) {
		set, ok := dirSets[dir]
		if !ok {
			set = make(map[string]Child)
			dirSets[dir] = set
		}
		set[c.URI] = c
	}

	segments := strings.Split(f.Path, "/")
	dir := SkillURI(skillPath, "")
	for i, seg := range segments {
		childURI := dir + "/" + seg
		if i == len(segments)-1 {
			add(dir, Child{URI: childURI, Name: seg, MIMEType: f.MediaType(), Size: int64(len(f.Content))})
			break
		}
		add(dir, Child{URI: childURI, Name: seg, MIMEType: DirectoryMIMEType})
		// Ensure intermediate directories exist even when empty of files.
		if _, ok := dirSets[childURI]; !ok {
			dirSets[childURI] = make(map[string]Child)
		}
		dir = childURI
	}
}

// Len returns the number of skills in the registry.
func (r *Registry) Len() int { return len(r.order) }

// WithSkills returns a new registry containing this catalog and additional
// skills. Duplicate paths and conflicting resource bytes are rejected.
func (r *Registry) WithSkills(additional ...*Skill) (*Registry, error) {
	all := make([]*Skill, 0, len(r.order)+len(additional))
	for _, p := range r.order {
		all = append(all, r.skills[p])
	}
	return NewRegistry(append(all, additional...)...)
}

// ForTools returns a catalog whose core workflows can run with available tools.
// Filtering rebuilds every index, so list/get, directories and file reads share
// the same exposure rules without changing the original registry.
func (r *Registry) ForTools(available func(string) bool) (*Registry, error) {
	if available == nil {
		return nil, fmt.Errorf("tool availability is required")
	}
	var withheld []string
	for _, p := range r.order {
		for _, group := range r.skills[p].RequiredTools {
			if len(group) == 0 {
				return nil, fmt.Errorf("skill %q has an empty required tool group", p)
			}
			if !slices.ContainsFunc(group, available) {
				withheld = append(withheld, p)
				break
			}
		}
	}
	return r.Retaining(func(s *Skill) bool {
		for _, p := range withheld {
			if s.SkillPath == p {
				return false
			}
			// A parent's manifest includes its nested skills. Withholding the
			// parent too prevents its resource reads exposing a hidden child.
			for _, f := range s.files {
				if strings.HasPrefix(s.SkillPath+"/"+f.Path, p+"/") {
					return false
				}
			}
		}
		return true
	})
}

// Retaining returns a registry holding only the skills keep accepts.
//
// A skill is guidance for driving a particular set of tools, so a server that
// registers none of them is in no position to offer it: the skill would occupy
// the menu a model chooses from, describe a workflow the model cannot begin,
// and be discovered as a dead end only after it was selected. Publishing less
// than the whole catalog is the honest response to a narrower configuration,
// not a degradation of it.
//
// The receiver is left untouched, so a process-wide catalog can be filtered per
// request without the requests interfering. Ordering, digests and file contents
// are those of the retained skills, so anything published here still matches
// what a client fetches.
func (r *Registry) Retaining(keep func(*Skill) bool) (*Registry, error) {
	if keep == nil {
		return r, nil
	}
	kept := make([]*Skill, 0, len(r.order))
	for _, skillPath := range r.order {
		if s := r.skills[skillPath]; keep(s) {
			kept = append(kept, s)
		}
	}
	if len(kept) == len(r.order) {
		return r, nil
	}
	return NewRegistry(kept...)
}

// List returns every entry, ordered by skill path so that pagination is
// reproducible.
func (r *Registry) List(context.Context) ([]Entry, error) {
	entries := make([]Entry, 0, len(r.order))
	for _, p := range r.order {
		entries = append(entries, r.skills[p].Entry())
	}
	return entries, nil
}

// Get returns the entry for a skill's SKILL.md URI.
func (r *Registry) Get(_ context.Context, uri string) (Entry, bool, error) {
	s, ok := r.byURI[uri]
	if !ok {
		return Entry{}, false, nil
	}
	return s.Entry(), true, nil
}

// ReadDirectory returns the direct children of a directory in the registry.
func (r *Registry) ReadDirectory(_ context.Context, uri string) ([]Child, bool, error) {
	// A file URI is not a directory resource, and must be reported as invalid
	// params rather than as an empty directory.
	if _, isFile := r.files[uri]; isFile {
		return nil, false, nil
	}
	children, ok := r.children[uri]
	if !ok {
		return nil, false, nil
	}
	out := make([]Child, len(children))
	copy(out, children)
	return out, true, nil
}

// Files returns every file the registry serves, ordered by URI.
func (r *Registry) Files(context.Context) ([]ResourceFile, error) {
	uris := make([]string, 0, len(r.files))
	for uri := range r.files {
		uris = append(uris, uri)
	}
	sort.Strings(uris)
	out := make([]ResourceFile, 0, len(uris))
	for _, uri := range uris {
		out = append(out, r.files[uri])
	}
	return out, nil
}

// Skill returns the skill at a path.
func (r *Registry) Skill(skillPath string) (*Skill, bool) {
	s, ok := r.skills[skillPath]
	return s, ok
}
