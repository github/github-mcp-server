//go:build ignore

// Command gen generates catalog_generated.go: the typed catalog of GitHub
// fine-grained permissions (one Permission constant per permission, plus a
// Catalog map giving each permission's scope and valid levels).
//
// It reads ONLY the PUBLIC github/rest-api-description OpenAPI description and,
// specifically, the components/schemas/app-permissions schema, which is the
// authoritative public enumeration of fine-grained permission names and the
// access levels (read/write/admin) each one accepts. No internal sources are
// read or referenced.
//
// Per-endpoint / per-tool permission requirements are NOT derived here; those
// are hand-authored at each tool definition site (mirroring RequiredScopes).
//
// Usage:
//
//	go generate ./pkg/permissions
//	go run gen.go                         # fetch the public spec over HTTP
//	go run gen.go -spec path/to/spec.json # use a local copy of the public spec
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// defaultSpecURL is the public, bundled GitHub REST API OpenAPI description.
const defaultSpecURL = "https://raw.githubusercontent.com/github/rest-api-description/main/descriptions/api.github.com/api.github.com.json"

// orgPermissions are organization-scoped permissions whose names do not carry
// the organization_ prefix.
var orgPermissions = map[string]bool{
	"members":                             true,
	"custom_properties_for_organizations": true,
}

// accountPermissions are user-account-scoped permissions (bare names that apply
// to the authenticated user rather than a repository).
var accountPermissions = map[string]bool{
	"email_addresses":    true,
	"followers":          true,
	"git_ssh_keys":       true,
	"gpg_keys":           true,
	"interaction_limits": true,
	"profile":            true,
	"starring":           true,
}

// acronyms maps lowercase tokens to their canonical capitalization, following
// the repository's Go acronym conventions.
var acronyms = map[string]string{
	"ssh": "SSH",
	"gpg": "GPG",
	"id":  "ID",
	"api": "API",
	"url": "URL",
}

type openAPISpec struct {
	Components struct {
		Schemas struct {
			AppPermissions struct {
				Properties map[string]struct {
					Enum []string `json:"enum"`
				} `json:"properties"`
			} `json:"app-permissions"`
		} `json:"schemas"`
	} `json:"components"`
}

type entry struct {
	name   string
	cons   string
	scope  string // "ScopeRepo" | "ScopeOrg" | "ScopeAccount"
	levels []string
}

func main() {
	specPath := flag.String("spec", "", "path to a local copy of the public api.github.com.json (defaults to fetching over HTTP)")
	out := flag.String("out", "catalog_generated.go", "output file path")
	flag.Parse()

	raw, source, err := loadSpec(*specPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gen: %v\n", err)
		os.Exit(1)
	}

	var spec openAPISpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		fmt.Fprintf(os.Stderr, "gen: parse spec: %v\n", err)
		os.Exit(1)
	}

	props := spec.Components.Schemas.AppPermissions.Properties
	if len(props) == 0 {
		fmt.Fprintln(os.Stderr, "gen: no app-permissions properties found in spec")
		os.Exit(1)
	}

	var entries []entry
	for name, prop := range props {
		// Exclude enterprise permissions: they are not repo/org MCP tooling.
		if strings.HasPrefix(name, "enterprise_") {
			continue
		}
		levels := make([]string, 0, len(prop.Enum))
		for _, lvl := range prop.Enum {
			switch lvl {
			case "read":
				levels = append(levels, "LevelRead")
			case "write":
				levels = append(levels, "LevelWrite")
			case "admin":
				levels = append(levels, "LevelAdmin")
			}
		}
		entries = append(entries, entry{
			name:   name,
			cons:   constName(name),
			scope:  scopeFor(name),
			levels: levels,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })

	src := render(source, entries)
	formatted, err := format.Source([]byte(src))
	if err != nil {
		fmt.Fprintf(os.Stderr, "gen: gofmt: %v\n%s", err, src)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, formatted, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "gen: write %s: %v\n", *out, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d permissions) from %s\n", *out, len(entries), source)
}

func loadSpec(path string) (data []byte, source string, err error) {
	if path != "" {
		b, err := os.ReadFile(path) //#nosec G304 -- developer-supplied path for code generation
		if err != nil {
			return nil, "", fmt.Errorf("read spec %s: %w", path, err)
		}
		return b, path, nil
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(defaultSpecURL)
	if err != nil {
		return nil, "", fmt.Errorf("fetch spec: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("fetch spec: unexpected status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read spec body: %w", err)
	}
	return b, defaultSpecURL, nil
}

func scopeFor(name string) string {
	switch {
	case strings.HasPrefix(name, "organization_"), orgPermissions[name]:
		return "ScopeOrg"
	case accountPermissions[name]:
		return "ScopeAccount"
	default:
		return "ScopeRepo"
	}
}

// constName converts a snake_case permission name into an exported Go
// identifier, applying the repository's acronym conventions.
func constName(name string) string {
	parts := strings.Split(name, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		if up, ok := acronyms[p]; ok {
			b.WriteString(up)
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(p[1:])
	}
	return b.String()
}

func render(source string, entries []entry) string {
	var b strings.Builder
	b.WriteString("// Code generated by gen.go; DO NOT EDIT.\n")
	b.WriteString("//\n")
	b.WriteString("// Source: " + source + "\n")
	b.WriteString("// Generated from the PUBLIC components/schemas/app-permissions schema of the\n")
	b.WriteString("// github/rest-api-description OpenAPI description. Regenerate with:\n")
	b.WriteString("//\n")
	b.WriteString("//\tgo generate ./pkg/permissions\n")
	b.WriteString("\n")
	b.WriteString("package permissions\n\n")

	b.WriteString("// Typed permission constants, one per public fine-grained permission.\n")
	b.WriteString("const (\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "\t%s Permission = %q\n", e.cons, e.name)
	}
	b.WriteString(")\n\n")

	b.WriteString("// Catalog maps each known permission to its scope and valid access levels.\n")
	b.WriteString("var Catalog = map[Permission]CatalogEntry{\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "\t%s: {Permission: %s, Scope: %s, Levels: []Level{%s}},\n",
			e.cons, e.cons, e.scope, strings.Join(e.levels, ", "))
	}
	b.WriteString("}\n")
	return b.String()
}
