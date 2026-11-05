package skills

import (
	"fmt"
	"mime"
	"path"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

// File is one file of a skill, held as the raw bytes the digest covers.
type File struct {
	// Path is the file's path relative to the skill directory root, using "/"
	// as the separator.
	Path string
	// Content is the file's raw bytes.
	Content []byte
	// MIMEType overrides the type inferred from the file extension. Leave it
	// empty to infer.
	MIMEType string
}

// Skill is a complete skill held in memory: its location in the server's
// namespace, its parsed frontmatter, and its files.
//
// Construct one with NewSkill, which enforces the extension's structural rules
// and per-skill limits. A Skill obtained any other way has not been validated.
type Skill struct {
	// SkillPath locates the skill within the server's namespace. Its final
	// segment equals the frontmatter name.
	SkillPath string
	// Frontmatter is the verbatim frontmatter of the skill's SKILL.md.
	Frontmatter Frontmatter
	// Dynamic marks a skill whose content is generated, so that it publishes
	// no digests. Its files are still served, but its entry carries the
	// "dynamic" marker rather than a manifest.
	Dynamic bool

	// RequiredTools is server-only availability information, never frontmatter
	// or wire metadata. Every group must have at least one available tool;
	// alternatives accommodate consolidated and granular tool surfaces.
	RequiredTools [][]string
	// OptionalTools documents supporting tools whose absence does not prevent
	// the core workflow. It is never published as metadata or a permission grant.
	OptionalTools []string

	files []File
}

// NewSkill builds a skill from its path and files.
//
// The files must include a SKILL.md at the skill root; its frontmatter is
// parsed and its name is required to equal the final segment of skillPath. The
// per-skill limits are enforced here, so a registry never serves an entry a
// conforming host would refuse.
func NewSkill(skillPath string, files []File) (*Skill, error) {
	if err := validateSkillPath(skillPath); err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("skill %q: no files", skillPath)
	}

	normalized := make([]File, 0, len(files))
	seen := make(map[string]struct{}, len(files))
	var skillDoc *File
	var total int64
	for i := range files {
		f := files[i]
		if err := validateFilePath(f.Path); err != nil {
			return nil, fmt.Errorf("skill %q: %w", skillPath, err)
		}
		if _, dup := seen[f.Path]; dup {
			return nil, fmt.Errorf("skill %q: file %q listed more than once", skillPath, f.Path)
		}
		seen[f.Path] = struct{}{}
		if f.Path == SkillFile {
			skillDoc = &files[i]
		}
		total += int64(len(f.Content))
		normalized = append(normalized, f)
	}
	if skillDoc == nil {
		return nil, fmt.Errorf("skill %q: missing %s", skillPath, SkillFile)
	}
	if len(normalized) > MaxResourcesPerSkill {
		return nil, fmt.Errorf("skill %q: %d files exceeds the limit of %d", skillPath, len(normalized), MaxResourcesPerSkill)
	}
	if total > MaxSkillBytes {
		return nil, fmt.Errorf("skill %q: total size %d bytes exceeds the limit of %d", skillPath, total, MaxSkillBytes)
	}

	fm, _, err := ParseFrontmatter(skillDoc.Content)
	if err != nil {
		return nil, fmt.Errorf("skill %q: %w", skillPath, err)
	}
	if err := fm.Validate(); err != nil {
		return nil, fmt.Errorf("skill %q: %w", skillPath, err)
	}
	name := URI{Scheme: URIScheme, SkillPath: skillPath}.Name()
	if fm.Name() != name {
		return nil, fmt.Errorf("skill %q: frontmatter name %q does not match final path segment %q", skillPath, fm.Name(), name)
	}

	sort.Slice(normalized, func(i, j int) bool { return normalized[i].Path < normalized[j].Path })
	return &Skill{SkillPath: skillPath, Frontmatter: fm, files: normalized}, nil
}

// Name returns the skill's name: its frontmatter name, which by construction
// equals the final segment of its path.
func (s *Skill) Name() string { return s.Frontmatter.Name() }

// Description returns the skill's frontmatter description.
func (s *Skill) Description() string { return s.Frontmatter.Description() }

// URI returns the resource URI of the skill's SKILL.md.
func (s *Skill) URI() string { return SkillURI(s.SkillPath, SkillFile) }

// Root returns the skill's root directory URI, written without a trailing
// slash.
func (s *Skill) Root() string { return SkillURI(s.SkillPath, "") }

// Files returns the skill's files in path order.
func (s *Skill) Files() []File { return s.files }

// File returns the file at a path relative to the skill root.
func (s *Skill) File(relPath string) (File, bool) {
	for _, f := range s.files {
		if f.Path == relPath {
			return f, true
		}
	}
	return File{}, false
}

// Entry renders the skill's published record. A dynamic skill carries the
// "dynamic" marker in place of a manifest; every other skill carries a complete
// one, listing every file exactly once including its own SKILL.md.
func (s *Skill) Entry() Entry {
	e := Entry{URI: s.URI(), Frontmatter: s.Frontmatter}
	if s.Dynamic {
		e.Dynamic = true
		return e
	}
	e.Resources = make([]EntryResource, 0, len(s.files))
	for _, f := range s.files {
		e.Resources = append(e.Resources, EntryResource{
			URI:    SkillURI(s.SkillPath, f.Path),
			Digest: Digest(f.Content),
			Size:   int64(len(f.Content)),
		})
	}
	return e
}

// MediaType returns the media type a file is served as: its explicit override,
// the type inferred from its extension, or a conservative default.
func (f File) MediaType() string {
	if f.MIMEType != "" {
		return f.MIMEType
	}
	return FileMIMEType(f.Path, f.Content)
}

// FileMIMEType infers a media type from a file's extension, falling back to a
// text or binary default based on the content itself.
func FileMIMEType(filePath string, content []byte) string {
	if path.Base(filePath) == SkillFile {
		return SkillMIMEType
	}
	switch strings.ToLower(path.Ext(filePath)) {
	case ".md", ".markdown":
		return SkillMIMEType
	case ".txt":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "application/yaml"
	case ".py":
		return "text/x-python"
	case ".sh":
		return "text/x-shellscript"
	}
	if t := mime.TypeByExtension(path.Ext(filePath)); t != "" {
		return t
	}
	if IsText(content) {
		return "text/plain"
	}
	return "application/octet-stream"
}

// IsText reports whether content can be served as text: valid UTF-8 with no NUL
// bytes. Resource contents that are not text must be sent base64-encoded.
func IsText(content []byte) bool {
	return utf8.Valid(content) && !slices.Contains(content, 0x00)
}
