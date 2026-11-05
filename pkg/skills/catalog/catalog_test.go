package catalog

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/v2/pkg/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// listCatalog builds a fresh registry and returns its entries. Tests build
// rather than share so that a test mutating an entry cannot affect another.
func listCatalog(t *testing.T) []skills.Entry {
	t.Helper()
	registry, err := New()
	require.NoError(t, err, "the bundled catalog must build")
	entries, err := registry.List(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, entries, "the catalog must publish at least one skill")
	return entries
}

// skillFor resolves the loaded skill behind a published entry.
func skillFor(t *testing.T, registry *skills.Registry, entry skills.Entry) *skills.Skill {
	t.Helper()
	uri, err := skills.ParseURI(entry.URI)
	require.NoError(t, err)
	skill, ok := registry.Skill(uri.SkillPath)
	require.True(t, ok, "registry must serve the skill it listed at %q", entry.URI)
	return skill
}

func TestCatalogBuilds(t *testing.T) {
	t.Parallel()

	entries := listCatalog(t)
	for _, entry := range entries {
		uri, err := skills.ParseURI(entry.URI)
		require.NoError(t, err, "entry URI %q must parse", entry.URI)
		assert.Equal(t, Authority+"/"+uri.Name(), uri.SkillPath,
			"every bundled skill is published directly under the %s authority", Authority)
		assert.Equal(t, uri.Name(), entry.Frontmatter.Name(),
			"frontmatter name must match the final path segment")
		assert.Equal(t, skills.SkillFile, uri.FilePath, "an entry URI addresses the skill's SKILL.md")
	}
}

func TestSourceIsCachedAndEquivalentToNew(t *testing.T) {
	t.Parallel()

	first, err := Source()
	require.NoError(t, err)
	second, err := Source()
	require.NoError(t, err)
	assert.Same(t, first, second, "Source must build the catalog at most once")

	cached, err := first.List(context.Background())
	require.NoError(t, err)
	assert.Equal(t, listCatalog(t), cached, "the cached catalog must match a freshly built one")
}

// TestCatalogIsDeterministic guards the property hosts depend on: a digest
// published by one process must identify the same bytes in every other.
func TestCatalogIsDeterministic(t *testing.T) {
	t.Parallel()

	first := listCatalog(t)
	second := listCatalog(t)
	require.Equal(t, first, second, "two independent builds must produce identical entries")

	for _, entry := range first {
		require.NotEmpty(t, entry.Resources,
			"a bundled skill is static, so it must publish a manifest rather than the dynamic marker")
		assert.False(t, entry.Dynamic)
		for _, resource := range entry.Resources {
			assert.True(t, strings.HasPrefix(resource.Digest, "sha256:"),
				"resource %q must carry a sha256 digest", resource.URI)
			assert.Positive(t, resource.Size, "resource %q must have a non-zero size", resource.URI)
		}
	}
}

func TestSkillFilesResolveAndMatchTheirDigests(t *testing.T) {
	t.Parallel()

	registry, err := New()
	require.NoError(t, err)
	ctx := context.Background()

	files, err := registry.Files(ctx)
	require.NoError(t, err)
	byURI := make(map[string][]byte, len(files))
	for _, file := range files {
		byURI[file.URI] = file.File.Content
	}

	for _, entry := range listCatalog(t) {
		for _, resource := range entry.Resources {
			content, ok := byURI[resource.URI]
			require.True(t, ok, "resource %q is listed in a manifest but not served", resource.URI)
			require.NoError(t, skills.Verify(content, resource),
				"served content for %q must match its published digest and size", resource.URI)
		}
	}
}

func TestSkillNamesAreUnique(t *testing.T) {
	t.Parallel()

	seen := make(map[string]string)
	for _, entry := range listCatalog(t) {
		name := entry.Frontmatter.Name()
		previous, duplicate := seen[name]
		require.False(t, duplicate, "skill name %q is used by both %s and %s", name, previous, entry.URI)
		seen[name] = entry.URI
	}
}

// TestDescriptionsDiscriminate checks the property that decides whether a host
// loads the right skill. Descriptions must say what the skill is for, what it is
// not for, and must not be so similar to a sibling that a host has to load both
// to tell them apart.
func TestDescriptionsDiscriminate(t *testing.T) {
	t.Parallel()

	entries := listCatalog(t)
	descriptions := make(map[string]string, len(entries))
	tokens := make(map[string]map[string]struct{}, len(entries))

	for _, entry := range entries {
		name := entry.Frontmatter.Name()
		description := entry.Frontmatter.Description()

		require.NotEmpty(t, description, "skill %q must have a description", name)
		assert.LessOrEqual(t, len(description), 1024,
			"skill %q has a description too long to be scanned cheaply by a host", name)
		assert.NotContains(t, description, "\n", "skill %q must keep its description on one line", name)

		assert.True(t, strings.HasPrefix(description, "USE FOR:"),
			"skill %q must open with the triggers it applies to", name)
		assert.Contains(t, description, "NOT FOR:",
			"skill %q must name the neighbouring workflows it does not cover", name)

		previous, duplicate := descriptions[description]
		require.False(t, duplicate, "skills %q and %q share a description verbatim", previous, name)
		descriptions[description] = name

		tokens[name] = significantTokens(description)
	}

	// Compare every pair rather than only neighbours: an overlapping trigger
	// anywhere in the catalog is enough to make a host load two skills.
	names := make([]string, 0, len(tokens))
	for name := range tokens {
		names = append(names, name)
	}
	for i := range names {
		for j := i + 1; j < len(names); j++ {
			a, b := names[i], names[j]
			assert.Less(t, jaccard(tokens[a], tokens[b]), 0.5,
				"descriptions for %q and %q overlap enough that a host may load both", a, b)
		}
	}
}

// significantTokens reduces a description to the words that carry its trigger
// meaning, dropping the shared scaffolding every description uses.
func significantTokens(description string) map[string]struct{} {
	scaffolding := map[string]struct{}{
		"use": {}, "for": {}, "not": {}, "and": {}, "the": {}, "a": {}, "an": {},
		"or": {}, "to": {}, "of": {}, "on": {}, "in": {}, "one": {}, "it": {},
		"is": {}, "what": {}, "which": {}, "that": {}, "with": {}, "github": {},
	}
	word := regexp.MustCompile(`[a-z]+`)
	out := make(map[string]struct{})
	for _, token := range word.FindAllString(strings.ToLower(description), -1) {
		if _, skip := scaffolding[token]; skip || len(token) < 3 {
			continue
		}
		out[token] = struct{}{}
	}
	return out
}

func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	var intersection int
	for token := range a {
		if _, ok := b[token]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	return float64(intersection) / float64(union)
}

// hostExecutionPatterns are the routes a skill must never send a host down.
// A skill is domain guidance; anything that reaches for a host's own execution
// surface belongs to the host, not to this server.
//
// Only program names that cannot be ordinary prose are listed. Words such as
// "make" and "run" are everyday domain vocabulary ("make the change", "workflow
// run"), and "node" is GitHub's own term for a GraphQL identifier, so none of
// them can be matched without drowning the check in false positives. The shell,
// CLI, script and fenced-block patterns below already close every route that
// invoking them would need.
var hostExecutionPatterns = []struct {
	name    string
	pattern *regexp.Regexp
}{
	{"a shell", regexp.MustCompile(`(?i)\b(bash|zsh|shell|sudo|stdin|stdout)\b`)},
	{"a command line", regexp.MustCompile(`(?i)command[- ]line|\bcommand prompt\b|\bterminal\b`)},
	{"the gh CLI or another CLI", regexp.MustCompile(`(?i)\bgh\b|\bCLI\b|\bcommand-line interface\b`)},
	{"an external program", regexp.MustCompile(`(?i)\b(curl|wget|git|npm|npx|pnpm|yarn|pip|uv|uvx|docker|jq)\b`)},
	{"a language runtime", regexp.MustCompile(`(?i)\b(python3?|ruby|perl|deno|bun)\b`)},
	{"a helper script", regexp.MustCompile(`(?i)\bscripts?\b|\.(sh|bash|ps1|py|js|rb)\b`)},
	{"code mode or an interpreter", regexp.MustCompile(`(?i)code mode|\bexecute\b|\bsubprocess\b|\binterpreter\b`)},
	{"an archive", regexp.MustCompile(`(?i)\b(tarball|zipfile)\b|\.(tar|tgz|zip)\b`)},
	{"a fenced executable block", regexp.MustCompile("(?i)```(bash|sh|zsh|shell|console|powershell|python|javascript|js|ruby)")},
	{"an environment or secret file", regexp.MustCompile(`(?i)\.env\b|\benvironment variables?\b`)},
}

// TestSkillsRouteOnlyThroughTools asserts that no bundled skill offers a host an
// execution route. The catalog is portable guidance: it must read the same to a
// host with a sandbox and to one with no execution surface at all.
func TestSkillsRouteOnlyThroughTools(t *testing.T) {
	t.Parallel()

	registry, err := New()
	require.NoError(t, err)
	files, err := registry.Files(context.Background())
	require.NoError(t, err)

	for _, file := range files {
		content := string(file.File.Content)
		for _, forbidden := range hostExecutionPatterns {
			if match := forbidden.pattern.FindString(content); match != "" {
				t.Errorf("%s refers to %s via %q; skills must describe workflows in terms of MCP tools only",
					file.URI, forbidden.name, match)
			}
		}
	}
}

// TestFrontmatterCarriesOnlyKnownKeys pins the shape of what the catalog puts on
// the wire.
//
// Frontmatter is published verbatim: whatever an author writes in a SKILL.md is
// what a host receives. That is deliberate — a host re-parses the fetched file
// and compares it field-by-field against the listing, so nothing here may be
// dropped or rewritten. The cost of that fidelity is that a key pasted in from
// a host-specific skill format travels too.
//
// Most of those keys are harmless noise. Some are not: keys that grant tools,
// mark a skill user-invocable, or hint at arguments describe a host's own
// execution model, and this server has no standing to assert any of them. The
// allow-list is the boundary, and adding to it should require saying why the
// key belongs on a portable, server-published skill.
func TestFrontmatterCarriesOnlyKnownKeys(t *testing.T) {
	t.Parallel()

	known := map[string]string{
		"name":        "required by the Agent Skills specification; also the directory name",
		"description": "required by the specification; the only text a host routes on",
	}

	for _, entry := range listCatalog(t) {
		for key := range entry.Frontmatter {
			_, ok := known[key]
			assert.Truef(t, ok,
				"skill %q declares frontmatter key %q, which the catalog does not recognise; "+
					"frontmatter is published verbatim, so an unknown key reaches every host that lists the catalog",
				entry.Frontmatter.Name(), key)
		}
	}
}

// TestSkillsDoNotCrossReference keeps the catalog free of load-skill thrash: a
// skill that names a sibling invites a host to load both.
func TestSkillsDoNotCrossReference(t *testing.T) {
	t.Parallel()

	registry, err := New()
	require.NoError(t, err)
	ctx := context.Background()

	entries, err := registry.List(ctx)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Frontmatter.Name())
	}

	files, err := registry.Files(ctx)
	require.NoError(t, err)
	for _, file := range files {
		body := string(file.File.Content)
		if idx := strings.Index(body, "\n---"); idx > 0 {
			body = body[idx:]
		}
		for _, name := range names {
			if strings.Contains(file.URI, name) {
				continue
			}
			assert.NotContains(t, body, name,
				"%s names the sibling skill %q; a skill must stand alone", file.URI, name)
		}
	}
}

// TestSkillsRequireConfirmationBeforeWriting holds the catalog to the rule that
// matters most in practice: anything a host cannot undo has to be agreed first.
func TestSkillsRequireConfirmationBeforeWriting(t *testing.T) {
	t.Parallel()

	confirmation := regexp.MustCompile(`(?i)\b(confirm|agreement|agreed|ask|approval|escalate)\b`)

	registry, err := New()
	require.NoError(t, err)
	entries, err := registry.List(context.Background())
	require.NoError(t, err)

	for _, entry := range entries {
		name := entry.Frontmatter.Name()
		doc, ok := skillFor(t, registry, entry).File(skills.SkillFile)
		require.True(t, ok)
		body := string(doc.Content)

		assert.Regexp(t, `(?i)\n##+ .*confirm`, body,
			"skill %q must carry a section stating what needs confirmation before acting", name)
		assert.True(t, confirmation.MatchString(body),
			"skill %q must tell a host to seek agreement rather than act unilaterally", name)
		assert.Regexp(t, `(?i)\n##+ .*(empty|failure)`, body,
			"skill %q must say what to do when a call returns nothing or fails", name)
	}
}

// TestSkillsStayWithinPublishedLimits checks the sizes a host may refuse. The
// limits are enforced by NewSkill too, but failing here names the skill.
func TestSkillsStayWithinPublishedLimits(t *testing.T) {
	t.Parallel()

	registry, err := New()
	require.NoError(t, err)
	entries, err := registry.List(context.Background())
	require.NoError(t, err)

	for _, entry := range entries {
		name := entry.Frontmatter.Name()
		assert.LessOrEqual(t, len(entry.Resources), skills.MaxResourcesPerSkill,
			"skill %q publishes more files than a host will accept", name)

		var total int64
		for _, resource := range entry.Resources {
			total += resource.Size
		}
		assert.LessOrEqual(t, total, skills.MaxSkillBytes,
			"skill %q is larger than a host will accept", name)
	}
}

// TestSupportingResourcesAreReachable keeps every supporting file earning its
// place: it must be referenced by the SKILL.md that ships it. The reverse
// direction matters more and is easier to miss — a SKILL.md may cite a file
// that was renamed or removed, and the skill then promises the model layered
// detail it cannot fetch. Shipping a file nobody reads is dead weight; citing a
// file that is not there is a broken promise, so both are checked here.
func TestSupportingResourcesAreReachable(t *testing.T) {
	t.Parallel()

	registry, err := New()
	require.NoError(t, err)
	entries, err := registry.List(context.Background())
	require.NoError(t, err)

	citedFile := regexp.MustCompile("`([a-z0-9_/-]+\\.md)`")

	for _, entry := range entries {
		skill := skillFor(t, registry, entry)
		doc, ok := skill.File(skills.SkillFile)
		require.True(t, ok)

		shipped := map[string]bool{}
		for _, file := range skill.Files() {
			shipped[file.Path] = true
			if file.Path == skills.SkillFile {
				continue
			}
			assert.Contains(t, string(doc.Content), file.Path,
				"skill %q ships %q without referencing it", entry.Frontmatter.Name(), file.Path)
		}

		for _, cited := range citedFile.FindAllStringSubmatch(string(doc.Content), -1) {
			// Pull request templates are repository inputs, not bundled files.
			if cited[1] == "pull_request_template.md" {
				continue
			}
			assert.True(t, shipped[cited[1]],
				"skill %q cites %q, which it does not ship; a host asked for that resource would find nothing",
				entry.Frontmatter.Name(), cited[1])
		}
	}
}
