package sanitize

import (
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
)

var policy *bluemonday.Policy
var policyOnce sync.Once

func Sanitize(input string) string {
	// The invisible-character and code-fence filters both run before and after
	// HTML processing. The first pass strips raw invisible characters so they
	// don't interfere with code-fence parsing. HTML sanitization
	// (FilterHTMLTags) decodes character entities (e.g. "&#8203;" or
	// "&#x200b;" become U+200B), which can introduce invisible or
	// bidirectional characters that were not present as literal runes in the
	// original input. Those decoded characters can both survive on their own
	// and splice previously inert text into a code fence, so the second pass
	// re-applies both filters to the fully normalized output.
	normalized := FilterHTMLTags(FilterCodeFenceMetadata(FilterInvisibleCharacters(input)))
	return FilterCodeFenceMetadata(FilterInvisibleCharacters(normalized))
}

// FilterInvisibleCharacters removes invisible or control characters that should not appear
// in user-facing titles or bodies. This includes:
// - Unicode tag characters: U+E0001, U+E0020–U+E007F
// - BiDi control characters: U+202A–U+202E, U+2066–U+2069
// - BiDi/directional marks: U+200E, U+200F, U+061C
// - Hidden modifier characters: U+200B, U+200C, U+00AD, U+FEFF, U+180E, U+2060–U+2064
// - Orphaned variation selectors: U+FE00–U+FE0F, U+E0100–U+E01EF
//
// Variation selectors are filtered contextually rather than unconditionally.
// A selector that forms a plausible variation sequence with the character it
// follows is preserved, so ordinary content such as "✈️", "1️⃣" and CJK
// ideographic variation sequences survive unchanged. Selectors that cannot
// belong to such a sequence — those at the start of the input, those following
// a removed or non-graphic character, and runs of consecutive selectors — are
// removed, which is the shape used to smuggle hidden payloads.
func FilterInvisibleCharacters(input string) string {
	if input == "" {
		return input
	}

	// Filter runes
	out := make([]rune, 0, len(input))
	var prev rune
	var prevKept bool
	for _, r := range input {
		keep := false
		if isVariationSelector(r) {
			keep = prevKept && isValidVariationSequence(prev, r)
		} else {
			keep = !shouldRemoveRune(r)
		}
		if keep {
			out = append(out, r)
		}
		prev, prevKept = r, keep
	}
	return string(out)
}

func FilterHTMLTags(input string) string {
	if input == "" {
		return input
	}
	return getPolicy().Sanitize(input)
}

// FilterCodeFenceMetadata removes hidden or suspicious info strings from fenced code blocks.
func FilterCodeFenceMetadata(input string) string {
	if input == "" {
		return input
	}

	lines := strings.Split(input, "\n")
	insideFence := false
	currentFenceLen := 0
	for i, line := range lines {
		sanitized, toggled, fenceLen := sanitizeCodeFenceLine(line, insideFence, currentFenceLen)
		lines[i] = sanitized
		if toggled {
			insideFence = !insideFence
			if insideFence {
				currentFenceLen = fenceLen
			} else {
				currentFenceLen = 0
			}
		}
	}
	return strings.Join(lines, "\n")
}

const maxCodeFenceInfoLength = 48

func sanitizeCodeFenceLine(line string, insideFence bool, expectedFenceLen int) (string, bool, int) {
	idx := strings.Index(line, "```")
	if idx == -1 {
		return line, false, expectedFenceLen
	}

	if hasNonWhitespace(line[:idx]) {
		return line, false, expectedFenceLen
	}

	fenceEnd := idx
	for fenceEnd < len(line) && line[fenceEnd] == '`' {
		fenceEnd++
	}

	fenceLen := fenceEnd - idx
	if fenceLen < 3 {
		return line, false, expectedFenceLen
	}

	rest := line[fenceEnd:]

	if insideFence {
		if expectedFenceLen != 0 && fenceLen != expectedFenceLen {
			return line, false, expectedFenceLen
		}
		return line[:fenceEnd], true, fenceLen
	}

	trimmed := strings.TrimSpace(rest)

	if trimmed == "" {
		return line[:fenceEnd], true, fenceLen
	}

	if strings.IndexFunc(trimmed, unicode.IsSpace) != -1 {
		return line[:fenceEnd], true, fenceLen
	}

	if len(trimmed) > maxCodeFenceInfoLength {
		return line[:fenceEnd], true, fenceLen
	}

	if !isSafeCodeFenceToken(trimmed) {
		return line[:fenceEnd], true, fenceLen
	}

	if len(rest) > 0 && unicode.IsSpace(rune(rest[0])) {
		return line[:fenceEnd] + " " + trimmed, true, fenceLen
	}

	return line[:fenceEnd] + trimmed, true, fenceLen
}

func hasNonWhitespace(segment string) bool {
	for _, r := range segment {
		if !unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

func isSafeCodeFenceToken(token string) bool {
	for _, r := range token {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		switch r {
		case '+', '-', '_', '#', '.':
			continue
		}
		return false
	}
	return true
}

func getPolicy() *bluemonday.Policy {
	policyOnce.Do(func() {
		p := bluemonday.StrictPolicy()

		p.AllowElements(
			"b", "blockquote", "br", "code", "em",
			"h1", "h2", "h3", "h4", "h5", "h6",
			"hr", "i", "li", "ol", "p", "pre",
			"strong", "sub", "sup", "table", "tbody",
			"td", "th", "thead", "tr", "ul",
			"a", "img",
		)

		p.AllowAttrs("href").OnElements("a")
		p.AllowURLSchemes("http", "https")
		p.RequireParseableURLs(true)
		p.RequireNoFollowOnLinks(true)
		p.RequireNoReferrerOnLinks(true)
		p.AddTargetBlankToFullyQualifiedLinks(true)

		p.AllowImages()
		p.AllowAttrs("src", "alt", "title").OnElements("img")

		policy = p
	})
	return policy
}

func shouldRemoveRune(r rune) bool {
	switch r {
	case 0x200B, // ZERO WIDTH SPACE
		0x200C, // ZERO WIDTH NON-JOINER
		0x200E, // LEFT-TO-RIGHT MARK
		0x200F, // RIGHT-TO-LEFT MARK
		0x061C, // ARABIC LETTER MARK
		0x00AD, // SOFT HYPHEN
		0xFEFF, // ZERO WIDTH NO-BREAK SPACE
		0x180E: // MONGOLIAN VOWEL SEPARATOR
		return true
	case 0xE0001: // TAG
		return true
	}

	// Ranges
	// Unicode tags: U+E0020–U+E007F
	if r >= 0xE0020 && r <= 0xE007F {
		return true
	}
	// BiDi controls: U+202A–U+202E
	if r >= 0x202A && r <= 0x202E {
		return true
	}
	// BiDi isolates: U+2066–U+2069
	if r >= 0x2066 && r <= 0x2069 {
		return true
	}
	// Hidden modifiers: U+2060–U+2064
	if r >= 0x2060 && r <= 0x2064 {
		return true
	}

	return false
}

// isVariationSelector reports whether r is a Unicode variation selector, either
// from the Variation Selectors block (VS1–VS16) or the Variation Selectors
// Supplement (VS17–VS256).
func isVariationSelector(r rune) bool {
	return (r >= 0xFE00 && r <= 0xFE0F) || (r >= 0xE0100 && r <= 0xE01EF)
}

// isValidVariationSequence reports whether selector can legitimately apply to
// the base character it immediately follows.
//
// A base may carry at most one selector, so a selector following another
// selector is always rejected; consecutive selectors carry no rendering meaning
// and are the primary way arbitrary data is hidden in text.
func isValidVariationSequence(base, selector rune) bool {
	if isVariationSelector(base) || !unicode.IsGraphic(base) || unicode.IsSpace(base) {
		return false
	}

	// The Ideographic Variation Database only registers sequences whose base is
	// a CJK ideograph, so supplement selectors are meaningless elsewhere.
	if selector >= 0xE0100 {
		return unicode.Is(unicode.Han, base)
	}

	// Standardized variation sequences use non-ASCII bases, except for the
	// keycap bases '#', '*' and the ASCII digits, which take a presentation
	// selector (VS15/VS16) only.
	if base < utf8.RuneSelf {
		if base != '#' && base != '*' && (base < '0' || base > '9') {
			return false
		}
		return selector == 0xFE0E || selector == 0xFE0F
	}

	return true
}
