// Package catalog holds the skills this server bundles: portable, host-agnostic
// guidance for GitHub workflows that the MCP tools in this repository support.
//
// # What a bundled skill is
//
// A skill is domain guidance a host may read: the order to do things in, the
// decisions that actually branch the workflow, the fields worth looking at, and
// what to do when a call returns nothing or fails. A skill is not a program. It
// never tells a host to run anything, and it grants no tool access.
package catalog

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/github/github-mcp-server/v2/pkg/skills"
)

//go:embed skills
var content embed.FS

const (
	// Authority is the first segment of every bundled skill path, and so the
	// authority component of every bundled skill URI. It is a publisher scope
	// rather than a hostname: nothing dereferences it over the network.
	Authority = "github.com"

	// root is the embedded directory holding one subdirectory per skill.
	root = "skills"
)

var (
	once        sync.Once
	sharedSkill *skills.Registry
	sharedErr   error
)

const toolGuidance = `
## Tool availability and result handling

Use only tools available from this server in the current connection. If a
supporting tool is absent, skip that step and state the limitation; do not invent
a tool or claim that the missing step was completed. Where consolidated and
granular tools differ, use the available tool's declared schema.

Use list tools for broad retrieval and search tools for targeted queries. Prefer
pages of 5-10 items and request minimal output when the tool supports it and full
details are unnecessary. Follow pagination before claiming a complete result.
For search tools with separate sort and order parameters, use those parameters
rather than putting sort qualifiers in the query.
`

// Source returns the bundled catalog, building it at most once per process.
//
// Prefer this over New when wiring a server: a server may be constructed per
// request, and the catalog is identical every time.
func Source() (*skills.Registry, error) {
	once.Do(func() { sharedSkill, sharedErr = New() })
	return sharedSkill, sharedErr
}

// New builds a fresh registry over the embedded skills. Every call produces
// byte-identical content, so entry digests and sizes are stable across builds and
// across processes.
//
// Every directory is loaded before any failure is reported. A catalog that
// stopped at the first bad skill would hide the rest, and the reader of a build
// failure wants the whole list rather than one entry at a time.
func New() (*skills.Registry, error) {
	dirs, err := fs.ReadDir(content, root)
	if err != nil {
		return nil, fmt.Errorf("read bundled skills: %w", err)
	}

	built := make([]*skills.Skill, 0, len(dirs))
	var problems []error
	for _, dir := range dirs {
		if !dir.IsDir() {
			problems = append(problems, fmt.Errorf("bundled skills: %s/%s is not a skill directory", root, dir.Name()))
			continue
		}
		skill, err := load(dir.Name())
		if err != nil {
			problems = append(problems, err)
			continue
		}
		built = append(built, skill)
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}

	registry, err := skills.NewRegistry(built...)
	if err != nil {
		return nil, fmt.Errorf("bundled skills: %w", err)
	}
	return registry, nil
}

// load reads one embedded skill directory into a Skill rooted at
// Authority/<name>. The directory name is the skill name, which NewSkill checks
// against the name declared in frontmatter.
func load(name string) (*skills.Skill, error) {
	dir := path.Join(root, name)

	var files []skills.File
	err := fs.WalkDir(content, dir, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		body, err := fs.ReadFile(content, p)
		if err != nil {
			return err
		}
		// Partial tool coverage is intentional. Keep a host from treating an
		// optional step in the workflow as an instruction to call a missing tool.
		if path.Base(p) == skills.SkillFile {
			body = append(body, []byte(toolGuidance)...)
		}

		files = append(files, skills.File{
			Path:    strings.TrimPrefix(p, dir+"/"),
			Content: body,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("bundled skill %q: %w", name, err)
	}

	// WalkDir already yields lexical order; sorting keeps that guarantee local
	// and independent of how the files were read.
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	skill, err := skills.NewSkill(path.Join(Authority, name), files)
	if err != nil {
		return nil, fmt.Errorf("bundled skill %q: %w", name, err)
	}
	tools, ok := requirements[name]
	if !ok {
		return nil, fmt.Errorf("bundled skill %q has no tool availability rule", name)
	}
	skill.RequiredTools = tools.required
	skill.OptionalTools = tools.optional
	return skill, nil
}
