package skills

import (
	"encoding/json"
	"errors"
	"fmt"
)

// DynamicResources is the literal value a skill entry carries in place of a
// resource array when its content is generated and stable digests cannot be
// published. The marker is explicit so a host can distinguish a deliberately
// unverifiable skill from a malformed entry.
const DynamicResources = "dynamic"

// ErrMissingResources reports an entry whose resources field is absent or is
// neither an array nor the literal "dynamic". Such an entry is invalid and a
// host must not load it.
var ErrMissingResources = errors.New("skill entry resources must be an array or the string \"dynamic\"")

// EntryResource is one file of a skill: its URI, the digest of its raw bytes,
// and that byte length. Together these let a host budget a skill before
// fetching anything and detect a truncated or padded read before hashing it.
type EntryResource struct {
	URI    string `json:"uri"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}

// Validate checks a single manifest entry.
func (r EntryResource) Validate() error {
	if r.URI == "" {
		return fmt.Errorf("resource URI is empty")
	}
	if err := ValidateDigest(r.Digest); err != nil {
		return fmt.Errorf("resource %q: %w", r.URI, err)
	}
	if r.Size < 0 {
		return fmt.Errorf("resource %q: negative size %d", r.URI, r.Size)
	}
	return nil
}

// Entry is a skill's published record: the URI of its SKILL.md, the verbatim
// frontmatter of that file, and its resource manifest. It is the unit returned
// by both skills/list and skills/get, identical in shape and meaning in each.
type Entry struct {
	// URI is the resource URI of the skill's SKILL.md.
	URI string `json:"uri"`
	// Frontmatter is a verbatim JSON rendering of that file's YAML frontmatter.
	Frontmatter Frontmatter `json:"frontmatter"`
	// Resources enumerates every file of the skill, each exactly once,
	// including one entry matching URI. It is nil when Dynamic is set.
	Resources []EntryResource `json:"-"`
	// Dynamic marks a skill whose content is generated, so that no stable
	// digests can be published. It is mutually exclusive with Resources.
	Dynamic bool `json:"-"`
}

// entryWire mirrors Entry with the resources union left as raw JSON so that the
// array and the "dynamic" literal can share one field.
type entryWire struct {
	URI         string          `json:"uri"`
	Frontmatter Frontmatter     `json:"frontmatter"`
	Resources   json.RawMessage `json:"resources"`
}

// MarshalJSON renders resources as either the manifest array or the literal
// "dynamic". It never omits the field: an entry with no resources is invalid.
func (e Entry) MarshalJSON() ([]byte, error) {
	w := entryWire{URI: e.URI, Frontmatter: e.Frontmatter}
	switch {
	case e.Dynamic:
		if len(e.Resources) > 0 {
			return nil, fmt.Errorf("skill %q: dynamic entry must not carry a resource manifest", e.URI)
		}
		w.Resources = json.RawMessage(`"` + DynamicResources + `"`)
	default:
		// A nil manifest still marshals as [], not null: the field is required
		// and null is not one of its two permitted forms. Construction through
		// NewSkill rejects a skill with no files, so this only guards
		// hand-built entries.
		resources := e.Resources
		if resources == nil {
			resources = []EntryResource{}
		}
		raw, err := json.Marshal(resources)
		if err != nil {
			return nil, err
		}
		w.Resources = raw
	}
	return json.Marshal(w)
}

// UnmarshalJSON parses the resources union, rejecting anything that is neither
// an array nor the literal "dynamic".
func (e *Entry) UnmarshalJSON(data []byte) error {
	var w entryWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	e.URI = w.URI
	e.Frontmatter = w.Frontmatter
	e.Resources = nil
	e.Dynamic = false

	if len(w.Resources) == 0 {
		return ErrMissingResources
	}
	var marker string
	if err := json.Unmarshal(w.Resources, &marker); err == nil {
		if marker != DynamicResources {
			return fmt.Errorf("%w: got %q", ErrMissingResources, marker)
		}
		e.Dynamic = true
		return nil
	}
	var resources []EntryResource
	if err := json.Unmarshal(w.Resources, &resources); err != nil {
		return ErrMissingResources
	}
	e.Resources = resources
	return nil
}

// TotalSize sums the manifest's sizes. It is meaningless for a dynamic entry,
// which offers nothing to count.
func (e Entry) TotalSize() int64 {
	var total int64
	for _, r := range e.Resources {
		total += r.Size
	}
	return total
}

// Validate checks the entry against the extension's structural rules and the
// per-skill limits. Because the manifest is complete, both limits are
// checkable here — from the entry alone, before any file is fetched.
func (e Entry) Validate() error {
	if e.URI == "" {
		return fmt.Errorf("skill entry URI is empty")
	}
	if err := e.Frontmatter.Validate(); err != nil {
		return fmt.Errorf("skill %q: %w", e.URI, err)
	}
	parsed, err := ParseURI(e.URI)
	if err != nil {
		return fmt.Errorf("skill %q: %w", e.URI, err)
	}
	if parsed.FilePath != SkillFile {
		return fmt.Errorf("skill %q: entry URI must address %s", e.URI, SkillFile)
	}
	if got, want := parsed.Name(), e.Frontmatter.Name(); got != want {
		return fmt.Errorf("skill %q: final path segment %q does not match frontmatter name %q", e.URI, got, want)
	}

	if e.Dynamic {
		if len(e.Resources) > 0 {
			return fmt.Errorf("skill %q: dynamic entry must not carry a resource manifest", e.URI)
		}
		return nil
	}
	if len(e.Resources) == 0 {
		return fmt.Errorf("skill %q: %w", e.URI, ErrMissingResources)
	}
	if len(e.Resources) > MaxResourcesPerSkill {
		return fmt.Errorf("skill %q: %d resources exceeds the limit of %d", e.URI, len(e.Resources), MaxResourcesPerSkill)
	}
	if total := e.TotalSize(); total > MaxSkillBytes {
		return fmt.Errorf("skill %q: total size %d bytes exceeds the limit of %d", e.URI, total, MaxSkillBytes)
	}

	seen := make(map[string]struct{}, len(e.Resources))
	var sawSkillFile bool
	for _, r := range e.Resources {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("skill %q: %w", e.URI, err)
		}
		if _, dup := seen[r.URI]; dup {
			return fmt.Errorf("skill %q: resource %q listed more than once", e.URI, r.URI)
		}
		seen[r.URI] = struct{}{}
		if r.URI == e.URI {
			sawSkillFile = true
			continue
		}
		// Origin scoping: a skill's manifest may name only that skill's own
		// files. Anything else would let one skill claim another's content.
		if !WithinSkill(parsed.SkillPath, r.URI) {
			return fmt.Errorf("skill %q: resource %q is outside the skill directory", e.URI, r.URI)
		}
	}
	if !sawSkillFile {
		return fmt.Errorf("skill %q: manifest does not include the skill's own %s", e.URI, SkillFile)
	}
	return nil
}
