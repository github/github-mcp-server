package skills

import (
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Registry holds the set of skills a server publishes and the directory tree
// derived from their file URIs. Build one with Load (or LoadFS) at server
// construction time; it is immutable afterwards and safe for concurrent use.
type Registry struct {
	skills []*Skill
	byURI  map[string]*Skill // SKILL.md URI → skill

	// dirs maps each directory resource URI (skill roots, their ancestors,
	// and subdirectories) to its direct children, sorted by URI.
	dirs map[string][]*mcp.Resource
}

// Load builds a registry from already-assembled skills.
func Load(skills ...*Skill) (*Registry, error) {
	r := &Registry{
		byURI: make(map[string]*Skill, len(skills)),
		dirs:  make(map[string][]*mcp.Resource),
	}
	for _, s := range skills {
		if _, dup := r.byURI[s.URI()]; dup {
			return nil, fmt.Errorf("skill %q: duplicate skill-path", s.Path)
		}
		r.skills = append(r.skills, s)
		r.byURI[s.URI()] = s
	}
	slices.SortFunc(r.skills, func(a, b *Skill) int { return strings.Compare(a.Path, b.Path) })
	r.buildDirs()
	return r, nil
}

// LoadFS builds a registry from a filesystem whose top-level directories are
// skill directories (each containing a SKILL.md plus any supporting files).
// Every skill is served under the given organizational prefix:
// skill://<prefix>/<dirname>/....
func LoadFS(fsys fs.FS, prefix string) (*Registry, error) {
	filesBySkill := make(map[string][]File)
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		segs := strings.SplitN(p, "/", 2)
		if len(segs) < 2 {
			// A file at the FS root is not part of any skill directory.
			return nil
		}
		content, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		filesBySkill[segs[0]] = append(filesBySkill[segs[0]], File{Path: segs[1], Content: content})
		return nil
	})
	if err != nil {
		return nil, err
	}

	skills := make([]*Skill, 0, len(filesBySkill))
	for dir, files := range filesBySkill {
		s, err := New(path.Join(prefix, dir), files)
		if err != nil {
			return nil, err
		}
		skills = append(skills, s)
	}
	return Load(skills...)
}

// Skills returns the registry's skills, sorted by skill-path.
func (r *Registry) Skills() []*Skill { return r.skills }

// Get returns the skill whose SKILL.md URI is uri.
func (r *Registry) Get(uri string) (*Skill, bool) {
	s, ok := r.byURI[uri]
	return s, ok
}

// Entries returns the wire entries for every skill, in listing order.
func (r *Registry) Entries() []Entry {
	entries := make([]Entry, 0, len(r.skills))
	for _, s := range r.skills {
		entries = append(entries, s.Entry())
	}
	return entries
}

// Directory returns the direct children of the directory resource at uri,
// or ok=false if uri is not a directory within the registry's namespace.
func (r *Registry) Directory(uri string) ([]*mcp.Resource, bool) {
	children, ok := r.dirs[strings.TrimSuffix(uri, "/")]
	return children, ok
}

// resourceMeta returns the resource metadata under which a skill file is
// registered and listed. Per the SEP, a SKILL.md's name and description come
// from its frontmatter; other files carry their base filename.
func resourceMeta(s *Skill, f File) *mcp.Resource {
	res := &mcp.Resource{
		URI:      s.FileURI(f.Path),
		Name:     path.Base(f.Path),
		MIMEType: f.MIMEType,
	}
	if f.Path == SkillFile {
		res.Name = s.Name
		res.Description = s.Description
	}
	return res
}

// buildDirs derives the directory tree from every skill file's URI: each
// proper prefix of a file's path is a directory resource, from the URI
// authority (e.g. skill://github) down to the file's parent.
func (r *Registry) buildDirs() {
	seen := make(map[string]map[string]bool) // dir URI → child URI present
	add := func(dirURI string, child *mcp.Resource) {
		if seen[dirURI] == nil {
			seen[dirURI] = make(map[string]bool)
		}
		if seen[dirURI][child.URI] {
			return
		}
		seen[dirURI][child.URI] = true
		r.dirs[dirURI] = append(r.dirs[dirURI], child)
	}

	for _, s := range r.skills {
		for _, f := range s.Files {
			segs := strings.Split(s.Path+"/"+f.Path, "/")
			for i := 1; i < len(segs); i++ {
				dirURI := "skill://" + strings.Join(segs[:i], "/")
				if i == len(segs)-1 {
					add(dirURI, resourceMeta(s, f))
					continue
				}
				add(dirURI, &mcp.Resource{
					URI:      "skill://" + strings.Join(segs[:i+1], "/"),
					Name:     segs[i],
					MIMEType: DirectoryMIMEType,
				})
			}
		}
	}
	for _, children := range r.dirs {
		slices.SortFunc(children, func(a, b *mcp.Resource) int { return strings.Compare(a.URI, b.URI) })
	}
}
