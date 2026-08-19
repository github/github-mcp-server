package sanitize

import (
	"strings"
	"sync"
	"unicode"

	"github.com/microcosm-cc/bluemonday"
)

var policy *bluemonday.Policy
var policyOnce sync.Once

func Sanitize(input string) string {
	cleaned := FilterCodeFenceMetadata(FilterInvisibleCharacters(input))
	protected := protectCodeAngleBrackets(cleaned)
	sanitized := FilterHTMLTags(protected)
	return restoreCodeAngleBrackets(sanitized)
}

// FilterInvisibleCharacters removes invisible or control characters that should not appear
// in user-facing titles or bodies. This includes:
// - Unicode tag characters: U+E0001, U+E0020–U+E007F
// - BiDi control characters: U+202A–U+202E, U+2066–U+2069
// - Hidden modifier characters: U+200B, U+200C, U+200E, U+200F, U+00AD, U+FEFF, U+180E, U+2060–U+2064
func FilterInvisibleCharacters(input string) string {
	if input == "" {
		return input
	}

	// Filter runes
	out := make([]rune, 0, len(input))
	for _, r := range input {
		if !shouldRemoveRune(r) {
			out = append(out, r)
		}
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

// Sentinels used to protect angle brackets inside code from HTML sanitization.
// NUL bytes are stripped by FilterInvisibleCharacters before protectCodeAngleBrackets
// runs, preventing sentinel collision attacks.
const (
	ltSentinel = "\x00LT\x00"
	gtSentinel = "\x00GT\x00"
)

// protectCodeAngleBrackets replaces < and > inside fenced and inline code with
// sentinels so bluemonday does not strip them as HTML tags.
func protectCodeAngleBrackets(input string) string {
	if input == "" {
		return input
	}

	lines := strings.Split(input, "\n")
	insideFence := false
	currentFenceLen := 0

	for i, line := range lines {
		if toggled, fenceLen := toggleCodeFence(line, insideFence, currentFenceLen); toggled {
			insideFence = !insideFence
			if insideFence {
				currentFenceLen = fenceLen
			} else {
				currentFenceLen = 0
			}
			continue
		}

		if insideFence {
			lines[i] = replaceAngleBrackets(line)
			continue
		}
		lines[i] = protectInlineCodeAngleBrackets(line)
	}

	return strings.Join(lines, "\n")
}

func toggleCodeFence(line string, insideFence bool, currentFenceLen int) (bool, int) {
	idx := strings.Index(line, "```")
	if idx == -1 || hasNonWhitespace(line[:idx]) {
		return false, currentFenceLen
	}

	fenceEnd := idx
	for fenceEnd < len(line) && line[fenceEnd] == '`' {
		fenceEnd++
	}

	fenceLen := fenceEnd - idx
	if fenceLen < 3 {
		return false, currentFenceLen
	}

	if insideFence {
		if currentFenceLen != 0 && fenceLen < currentFenceLen {
			return false, currentFenceLen
		}
		return true, fenceLen
	}

	return true, fenceLen
}

func protectInlineCodeAngleBrackets(line string) string {
	if !strings.Contains(line, "`") {
		return line
	}

	var out strings.Builder
	out.Grow(len(line))
	i := 0
	for i < len(line) {
		if line[i] != '`' {
			out.WriteByte(line[i])
			i++
			continue
		}

		openStart := i
		openLen := 0
		for i < len(line) && line[i] == '`' {
			openLen++
			i++
		}

		contentStart := i
		closeIdx := findInlineCodeClose(line, contentStart, openLen)
		if closeIdx == -1 {
			out.WriteString(line[openStart:i])
			continue
		}

		out.WriteString(line[openStart:contentStart])
		out.WriteString(replaceAngleBrackets(line[contentStart:closeIdx]))
		out.WriteString(line[closeIdx : closeIdx+openLen])
		i = closeIdx + openLen
	}

	return out.String()
}

func findInlineCodeClose(line string, contentStart, openLen int) int {
	for i := contentStart; i < len(line); i++ {
		if line[i] != '`' {
			continue
		}

		closeLen := 0
		for j := i; j < len(line) && line[j] == '`'; j++ {
			closeLen++
		}
		if closeLen == openLen {
			return i
		}
	}

	return -1
}

func replaceAngleBrackets(s string) string {
	if !strings.ContainsAny(s, "<>") {
		return s
	}
	s = strings.ReplaceAll(s, "<", ltSentinel)
	return strings.ReplaceAll(s, ">", gtSentinel)
}

func restoreCodeAngleBrackets(input string) string {
	s := strings.ReplaceAll(input, ltSentinel, "<")
	return strings.ReplaceAll(s, gtSentinel, ">")
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
	case 0x0000, // NUL — stripped to prevent sentinel collision in protectCodeAngleBrackets
		0x200B, // ZERO WIDTH SPACE
		0x200C, // ZERO WIDTH NON-JOINER
		0x200E, // LEFT-TO-RIGHT MARK
		0x200F, // RIGHT-TO-LEFT MARK
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
