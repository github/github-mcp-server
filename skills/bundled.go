package skills

import (
	"embed"
	"sync"
)

// The skill files ship as ordinary SKILL.md files in subdirectories of this
// package, embedded into the binary here. The pattern lists each file shape
// bundled skills currently use; a skill that grows supporting files (e.g.
// references/*.md) extends this list.
//
//go:embed */SKILL.md
var bundledFS embed.FS

// BundledPrefix is the organizational prefix under which bundled skills are
// served: skill://github/<name>/SKILL.md. The final URI segment before
// SKILL.md is the skill name, per the SEP's resource mapping.
const BundledPrefix = "github"

// Bundled returns the registry of Agent Skills embedded in this binary.
// The content is embedded at compile time and validated by tests, so a
// malformed skill is a build defect: Bundled panics rather than returning
// an error.
var Bundled = sync.OnceValue(func() *Registry {
	r, err := LoadFS(bundledFS, BundledPrefix)
	if err != nil {
		panic("skills: invalid bundled skill: " + err.Error())
	}
	return r
})
