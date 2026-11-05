package skills

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// frontmatterDelimiter opens and closes the YAML frontmatter block of a
// SKILL.md.
const frontmatterDelimiter = "---"

// Frontmatter is a SKILL.md's YAML frontmatter rendered as a JSON object.
//
// It is verbatim: every field the author wrote is preserved, not a curated
// subset, because a host builds its registry from the listing alone and later
// re-parses the fetched SKILL.md and compares it field-by-field against this.
// Dropping or rewriting a field here would make that comparison fail.
type Frontmatter map[string]any

// Name returns the frontmatter name field, which the Agent Skills
// specification requires.
func (f Frontmatter) Name() string { return f.string("name") }

// Description returns the frontmatter description field, which the Agent Skills
// specification requires.
func (f Frontmatter) Description() string { return f.string("description") }

func (f Frontmatter) string(key string) string {
	s, _ := f[key].(string)
	return s
}

// Validate checks the fields the Agent Skills specification requires of every
// SKILL.md.
func (f Frontmatter) Validate() error {
	if len(f) == 0 {
		return fmt.Errorf("frontmatter is empty")
	}
	if f.Name() == "" {
		return fmt.Errorf("frontmatter is missing a name")
	}
	if f.Description() == "" {
		return fmt.Errorf("frontmatter is missing a description")
	}
	return ValidateName(f.Name())
}

// ParseFrontmatter extracts the YAML frontmatter from SKILL.md content and
// returns it as a JSON-shaped object along with the markdown body that follows.
//
// Content is normalized for CRLF so that a checkout with Windows line endings
// parses identically; the digest, by contrast, always covers the raw bytes as
// served.
func ParseFrontmatter(content []byte) (Frontmatter, string, error) {
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	text = strings.TrimPrefix(text, "\uFEFF")

	rest, ok := strings.CutPrefix(text, frontmatterDelimiter+"\n")
	if !ok {
		return nil, "", fmt.Errorf("content does not begin with a %q frontmatter delimiter", frontmatterDelimiter)
	}
	block, body, ok := strings.Cut(rest, "\n"+frontmatterDelimiter)
	if !ok {
		return nil, "", fmt.Errorf("frontmatter block is not terminated by %q", frontmatterDelimiter)
	}
	body = strings.TrimPrefix(body, "\n")

	var raw map[string]any
	dec := yaml.NewDecoder(strings.NewReader(block))
	if err := dec.Decode(&raw); err != nil {
		return nil, "", fmt.Errorf("parsing frontmatter: %w", err)
	}
	fm, err := normalizeMapping(raw)
	if err != nil {
		return nil, "", fmt.Errorf("parsing frontmatter: %w", err)
	}
	return Frontmatter(fm), body, nil
}

// normalizeMapping converts a decoded YAML mapping into JSON-representable
// values while preserving every key. YAML permits non-string keys and
// non-JSON scalars; a frontmatter object that cannot round-trip through JSON
// could not be compared field-by-field by a host, so reject it rather than
// silently reshaping it.
func normalizeMapping(in map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(in))
	for k, v := range in {
		nv, err := normalizeValue(v)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", k, err)
		}
		out[k] = nv
	}
	return out, nil
}

func normalizeValue(v any) (any, error) {
	switch t := v.(type) {
	case nil, bool, string, int, int64, uint64, float64:
		return t, nil
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			ne, err := normalizeValue(e)
			if err != nil {
				return nil, fmt.Errorf("index %d: %w", i, err)
			}
			out[i] = ne
		}
		return out, nil
	case map[string]any:
		return normalizeMapping(t)
	case map[any]any:
		conv := make(map[string]any, len(t))
		for k, e := range t {
			ks, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("non-string key %v", k)
			}
			conv[ks] = e
		}
		return normalizeMapping(conv)
	default:
		return nil, fmt.Errorf("unsupported YAML value of type %T", v)
	}
}

// FrontmatterEqual reports whether two frontmatter objects are identical in
// content. Hosts perform this comparison after fetching a SKILL.md; servers use
// it to catch a listing that has drifted from the file it describes.
func FrontmatterEqual(a, b Frontmatter) bool {
	return mappingEqual(a, b)
}

func mappingEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok || !valueEqual(av, bv) {
			return false
		}
	}
	return true
}

func valueEqual(a, b any) bool {
	switch at := a.(type) {
	case map[string]any:
		bt, ok := b.(map[string]any)
		return ok && mappingEqual(at, bt)
	case []any:
		bt, ok := b.([]any)
		if !ok || len(at) != len(bt) {
			return false
		}
		for i := range at {
			if !valueEqual(at[i], bt[i]) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}
