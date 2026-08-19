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
	filtered := FilterCodeFenceMetadata(FilterInvisibleCharacters(input))
	normalized := FilterHTMLTags(filtered)

	// HTML processing is the only stage that can introduce a character its input
	// did not contain, so when it returns that input byte for byte there is
	// nothing new for the second pass to find. Both filters are fixed points on
	// the first pass's output, so the second pass is provably the identity here;
	// see TestSecondSanitizePassIsRedundantWhenHTMLIsUnchanged.
	if normalized == filtered {
		return normalized
	}
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
//
// The scan is copy-on-first-match: clean input is returned unchanged with no
// allocation.
func FilterInvisibleCharacters(input string) string {
	// Every filtered rune is non-ASCII, so a run of ASCII bytes can be skipped
	// without decoding it and an all-ASCII string needs no further work.
	for i := range len(input) {
		if input[i] >= utf8.RuneSelf {
			return filterInvisibleFrom(input, i)
		}
	}
	return input
}

// filterInvisibleFrom resumes FilterInvisibleCharacters at start, the first byte
// that could need filtering. It buffers output only once a rune actually
// changes, so input that turns out to be clean is still returned as-is.
func filterInvisibleFrom(input string, start int) string {
	var (
		out      strings.Builder
		prev     rune
		prevKept bool
		copied   int
		changed  bool
	)
	if start > 0 {
		// Everything before start is ASCII, which is never filtered, so the
		// preceding byte is both the previous rune and known to have been kept.
		prev, prevKept = rune(input[start-1]), true
	}

	for i := start; i < len(input); {
		r, size := utf8.DecodeRuneInString(input[i:])

		keep := true
		if isVariationSelector(r) {
			keep = prevKept && isValidVariationSequence(prev, r)
		} else if shouldRemoveRune(r) {
			keep = false
		}
		prev, prevKept = r, keep

		// An invalid UTF-8 byte decodes to U+FFFD. The rune-wise filter this
		// replaced re-encoded every rune it kept, turning such bytes into
		// U+FFFD, so reproduce that instead of passing the raw byte through.
		invalid := r == utf8.RuneError && size == 1
		if keep && !invalid {
			i += size
			continue
		}

		if !changed {
			changed = true
			out.Grow(len(input))
		}
		out.WriteString(input[copied:i])
		if keep {
			out.WriteRune(utf8.RuneError)
		}
		i += size
		copied = i
	}

	if !changed {
		return input
	}
	out.WriteString(input[copied:])
	return out.String()
}

// FilterHTMLTags applies the HTML allowlist policy to input.
func FilterHTMLTags(input string) string {
	if input == "" || isHTMLInert(input) {
		return input
	}
	return getPolicy().Sanitize(input)
}

// isHTMLInert reports whether input is provably a fixed point of the HTML
// policy, letting the caller skip it. It is a sufficient condition, deliberately
// narrow, not a description of every fixed point.
//
// The policy tokenizes input as HTML and re-emits text through
// html.EscapeString, so anything it can rewrite must contain at least one of:
//   - one of the five characters EscapeString rewrites (ampersand, apostrophe,
//     quote, less-than, greater-than), which are also the only way to open a
//     tag, comment, doctype or entity;
//   - a byte the tokenizer itself rewrites: NUL becomes U+FFFD, CR folds into LF;
//   - a byte outside ASCII, which may be part of a malformed UTF-8 sequence.
//
// Printable ASCII minus those five characters, plus TAB and LF, excludes all of
// them. Every accepted byte is checked against the live policy in
// TestHTMLInertBytesAreFixedPointsOfThePolicy.
func isHTMLInert(input string) bool {
	for i := range len(input) {
		if !htmlInertBytes[input[i]] {
			return false
		}
	}
	return true
}

var htmlInertBytes = func() (table [256]bool) {
	for c := 0x20; c <= 0x7E; c++ {
		table[c] = true
	}
	table['\t'] = true
	table['\n'] = true
	for _, c := range []byte{'&', '\'', '"', '<', '>'} {
		table[c] = false
	}
	return table
}()

// FilterCodeFenceMetadata removes hidden or suspicious info strings from fenced code blocks.
//
// Like FilterInvisibleCharacters this is copy-on-first-match: input whose lines
// all survive unchanged is returned without allocating.
func FilterCodeFenceMetadata(input string) string {
	if input == "" {
		return input
	}

	var (
		out             strings.Builder
		changed         bool
		copied          int
		insideFence     bool
		currentFenceLen int
	)

	// Walks the same lines strings.Split(input, "\n") would yield, without
	// materialising them.
	for start := 0; start <= len(input); {
		line := input[start:]
		if nl := strings.IndexByte(line, '\n'); nl >= 0 {
			line = line[:nl]
		}

		sanitized, toggled, fenceLen := sanitizeCodeFenceLine(line, insideFence, currentFenceLen)
		if toggled {
			insideFence = !insideFence
			if insideFence {
				currentFenceLen = fenceLen
			} else {
				currentFenceLen = 0
			}
		}
		if sanitized != line {
			if !changed {
				changed = true
				out.Grow(len(input))
			}
			out.WriteString(input[copied:start])
			out.WriteString(sanitized)
			copied = start + len(line)
		}

		start += len(line) + 1
	}

	if !changed {
		return input
	}
	out.WriteString(input[copied:])
	return out.String()
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

	// Reconstructing the line would allocate a copy of what is already there,
	// so return the original when normalization is a no-op.
	if rest == trimmed {
		return line, true, fenceLen
	}

	if len(rest) > 0 && unicode.IsSpace(rune(rest[0])) {
		if rest[0] == ' ' && len(rest) == len(trimmed)+1 {
			return line, true, fenceLen
		}
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
