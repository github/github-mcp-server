// Package skills implements the io.modelcontextprotocol/skills MCP extension
// (SEP-2640) for the Agent Skills this server ships.
//
// The skill content lives as ordinary SKILL.md files in subdirectories of this
// package — readable by any agent-skills consumer that scans repositories for
// skills — and is embedded into the server binary for delivery over MCP. Each
// skill file is served as an individually addressable resource under
// skill://github/<name>/..., per the SEP's resource mapping, and the package
// registers the extension's three protocol methods: skills/list, skills/get,
// and resources/directory/read.
package skills

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"mime"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const (
	// ExtensionID is the extension identifier declared in the server's
	// initialize response capabilities (SEP-2133 negotiation key).
	ExtensionID = "io.modelcontextprotocol/skills"

	// SkillFile is the required entry file of every skill directory.
	SkillFile = "SKILL.md"

	// DirectoryMIMEType marks directory resources in
	// resources/directory/read listings.
	DirectoryMIMEType = "inode/directory"
)

// File is a single file of a skill, addressed relative to the skill directory.
type File struct {
	// Path is the file's path relative to the skill directory root,
	// "/"-separated; the skill's entry file is always "SKILL.md".
	Path string
	// Content is the file's raw bytes.
	Content []byte
	// Digest is the SHA-256 digest of Content, "sha256:" + 64 lowercase hex.
	Digest string
	// MIMEType is the content type served for the file.
	MIMEType string
}

// Skill is one Agent Skill served over MCP.
type Skill struct {
	// Path is the full skill-path of the URI form skill://<skill-path>/SKILL.md.
	// The final segment is the skill name; preceding segments are the server's
	// organizational prefix (e.g. "github/review-pr").
	Path string
	// Name is the skill name — the final Path segment, equal to the
	// frontmatter `name` field per the SEP's resource mapping.
	Name string
	// Frontmatter is the SKILL.md YAML frontmatter, verbatim. skills/list and
	// skills/get entries carry it unmodified so hosts can build their skill
	// registry without fetching each SKILL.md.
	Frontmatter map[string]any
	// Description is the frontmatter `description` field.
	Description string
	// Files holds the skill's files: SKILL.md first, supporting files after,
	// sorted by path.
	Files []File
}

// URI returns the resource URI of the skill's SKILL.md.
func (s *Skill) URI() string { return "skill://" + s.Path + "/" + SkillFile }

// RootURI returns the skill's directory resource URI (no trailing slash).
func (s *Skill) RootURI() string { return "skill://" + s.Path }

// FileURI returns the resource URI of a file within the skill directory.
func (s *Skill) FileURI(relPath string) string { return "skill://" + s.Path + "/" + relPath }

// EntryResource is one {uri, digest} pair of a skill entry's resources set.
type EntryResource struct {
	URI    string `json:"uri"`
	Digest string `json:"digest"`
}

// Entry is a skill entry as carried by skills/list and skills/get results.
type Entry struct {
	// URI is the resource URI of the skill's SKILL.md.
	URI string `json:"uri"`
	// Frontmatter is the SKILL.md YAML frontmatter rendered verbatim as JSON.
	Frontmatter map[string]any `json:"frontmatter"`
	// Resources enumerates every file of the skill with its digest — the unit
	// of content a host verifies and binds approval to. Omitted only for
	// dynamically generated skills that cannot publish stable digests.
	Resources []EntryResource `json:"resources,omitempty"`
}

// Entry returns the skill's wire entry.
func (s *Skill) Entry() Entry {
	resources := make([]EntryResource, 0, len(s.Files))
	for _, f := range s.Files {
		resources = append(resources, EntryResource{URI: s.FileURI(f.Path), Digest: f.Digest})
	}
	return Entry{URI: s.URI(), Frontmatter: s.Frontmatter, Resources: resources}
}

// Digest returns the SEP-2640 digest of content: the SHA-256 hash of its raw
// bytes formatted as "sha256:" + 64 lowercase hexadecimal characters.
func Digest(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// nameRE enforces the Agent Skills specification's naming rules: lowercase
// alphanumeric segments separated by single hyphens.
var nameRE = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// New assembles and validates a Skill from its skill-path and files. It
// parses the SKILL.md frontmatter and enforces the SEP's structural rules:
// a SKILL.md must be present, the frontmatter must carry non-empty `name`
// and `description`, and `name` must equal the final skill-path segment.
func New(skillPath string, files []File) (*Skill, error) {
	name := path.Base(skillPath)
	if !nameRE.MatchString(name) || len(name) > 64 {
		return nil, fmt.Errorf("skill %q: name does not satisfy agentskills.io naming rules", skillPath)
	}

	var skillMD *File
	sorted := make([]File, 0, len(files))
	for i := range files {
		f := files[i]
		if f.Digest == "" {
			f.Digest = Digest(f.Content)
		}
		if f.MIMEType == "" {
			f.MIMEType = FileMIMEType(f.Path)
		}
		if f.Path == SkillFile {
			skillMD = &f
			continue
		}
		sorted = append(sorted, f)
	}
	if skillMD == nil {
		return nil, fmt.Errorf("skill %q: missing %s", skillPath, SkillFile)
	}
	slices.SortFunc(sorted, func(a, b File) int { return strings.Compare(a.Path, b.Path) })
	ordered := append([]File{*skillMD}, sorted...)

	fm, err := ParseFrontmatter(skillMD.Content)
	if err != nil {
		return nil, fmt.Errorf("skill %q: %w", skillPath, err)
	}
	fmName, _ := fm["name"].(string)
	if fmName != name {
		return nil, fmt.Errorf("skill %q: frontmatter name %q does not match final skill-path segment %q", skillPath, fmName, name)
	}
	description, _ := fm["description"].(string)
	if description == "" {
		return nil, fmt.Errorf("skill %q: frontmatter is missing a description", skillPath)
	}

	return &Skill{
		Path:        skillPath,
		Name:        name,
		Frontmatter: fm,
		Description: description,
		Files:       ordered,
	}, nil
}

// ParseFrontmatter extracts and decodes the YAML frontmatter block of a
// SKILL.md. The document must begin with a "---" line; the frontmatter runs
// to the next "---" line. Every field the author wrote is returned — the SEP
// requires listings to carry frontmatter verbatim, not a curated subset.
func ParseFrontmatter(content []byte) (map[string]any, error) {
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	rest, ok := bytes.CutPrefix(normalized, []byte("---\n"))
	if !ok {
		return nil, fmt.Errorf("%s does not begin with YAML frontmatter", SkillFile)
	}
	end := bytes.Index(rest, []byte("\n---"))
	if end < 0 {
		return nil, fmt.Errorf("%s frontmatter is not terminated", SkillFile)
	}
	var fm map[string]any
	if err := yaml.Unmarshal(rest[:end+1], &fm); err != nil {
		return nil, fmt.Errorf("invalid %s frontmatter: %w", SkillFile, err)
	}
	if fm == nil {
		return nil, fmt.Errorf("%s frontmatter is empty", SkillFile)
	}
	return fm, nil
}

// FileMIMEType picks the content type served for a skill file. SKILL.md and
// other markdown files are text/markdown per the SEP's resource metadata
// guidance; everything else resolves by extension.
func FileMIMEType(relPath string) string {
	ext := strings.ToLower(path.Ext(relPath))
	switch ext {
	case ".md", ".markdown":
		return "text/markdown"
	case ".py":
		return "text/x-python"
	case ".sh":
		return "text/x-shellscript"
	}
	if t := mime.TypeByExtension(ext); t != "" {
		return t
	}
	return "text/plain"
}

// IsText reports whether content can be served as a text resource.
func IsText(content []byte) bool { return utf8.Valid(content) }
