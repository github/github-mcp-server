package sanitize

import (
	"bytes"
	"html"
	"net/url"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var policy *bluemonday.Policy
var policyOnce sync.Once

// Sanitize removes hidden content and unsafe HTML from short, plain-text fields
// such as titles. Use Content for Markdown and other code-bearing text.
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

func isSimpleFencePrefix(prefix string) bool {
	if len(prefix) > 3 {
		return false
	}
	for i := range len(prefix) {
		if prefix[i] != ' ' {
			return false
		}
	}
	return true
}

// Content removes or visibly neutralizes hidden characters and suspicious
// code-fence metadata without rewriting visible content. Markdown bodies,
// comments, reviews, and commit messages may contain HTML-like prose or code
// and may be written back to GitHub, so applying the lossy HTML policy to them
// would corrupt source data.
func Content(input string) string {
	filtered := filterUnconditionalInvisibleCharacters(input)
	for range maxContentFilterPasses {
		next := filterContentInvisibleCharacters(filtered)
		next = FilterCodeFenceMetadata(next)
		next = neutralizeMarkdownHTML(next)
		next = neutralizeGitHubMarkdownExtensions(next)
		if next == filtered {
			return next
		}
		filtered = next
	}

	// Adversarially nested constructs can reveal one hidden construct per pass.
	// Bound the work, then make every remaining Markdown control inert outside
	// code and remove all fence metadata.
	filtered = strings.ReplaceAll(filtered, "&", "&amp;")
	filtered = stripAllFenceMetadata(filtered)
	return escapeMarkdownSyntaxOutsideCode(filtered)
}

const maxContentFilterPasses = 4

var markdownParser = goldmark.DefaultParser()

type sourceSpan struct {
	start int
	stop  int
}

type sourceMutation struct {
	start       int
	stop        int
	replacement string
}

func filterUnconditionalInvisibleCharacters(input string) string {
	start := -1
	for i, r := range input {
		if shouldRemoveRune(r) {
			start = i
			break
		}
	}
	if start < 0 {
		return input
	}

	var out strings.Builder
	out.Grow(len(input))
	out.WriteString(input[:start])
	for _, r := range input[start:] {
		if !shouldRemoveRune(r) {
			out.WriteRune(r)
		}
	}
	return out.String()
}

// neutralizeMarkdownHTML makes render-hidden CommonMark constructs visible
// without changing code spans or blocks.
func neutralizeMarkdownHTML(input string) string {
	if !strings.Contains(input, "<") &&
		!strings.Contains(input, "]:") &&
		!strings.Contains(input, "](") {
		return input
	}
	source := []byte(input)
	document := markdownParser.Parse(text.NewReader(source))
	mutations := markdownHiddenMutations(document, source)
	if len(mutations) == 0 {
		return input
	}
	return applySourceMutations(input, mutations)
}

func applySourceMutations(input string, mutations []sourceMutation) string {
	sort.Slice(mutations, func(i, j int) bool {
		return mutations[i].start < mutations[j].start
	})

	var out strings.Builder
	out.Grow(len(input))
	copied := 0
	for _, mutation := range mutations {
		if mutation.start < copied {
			continue
		}
		out.WriteString(input[copied:mutation.start])
		out.WriteString(mutation.replacement)
		copied = mutation.stop
	}
	out.WriteString(input[copied:])
	return out.String()
}

func markdownHiddenMutations(document ast.Node, source []byte) []sourceMutation {
	usedReferences := usedLinkReferences(document)
	seenDefinitions := map[string]struct{}{}
	var mutations []sourceMutation
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch node := node.(type) {
		case *ast.RawHTML:
			for i := range node.Segments.Len() {
				mutations = appendCharacterMutations(mutations, source, node.Segments.At(i), '<', "&lt;")
			}
		case *ast.HTMLBlock:
			for i := range node.Lines().Len() {
				mutations = appendCharacterMutations(mutations, source, node.Lines().At(i), '<', "&lt;")
			}
			if node.HasClosure() {
				mutations = appendCharacterMutations(mutations, source, node.ClosureLine, '<', "&lt;")
			}
		case *ast.LinkReferenceDefinition:
			label := util.ToLinkReference(node.Label)
			_, duplicate := seenDefinitions[label]
			seenDefinitions[label] = struct{}{}
			_, used := usedReferences[label]
			if (duplicate || !used) && node.Lines().Len() > 0 {
				mutations = appendFirstCharacterMutation(
					mutations,
					source,
					node.Lines().At(0),
					'[',
					`\[`,
				)
			}
		case *ast.Link:
			if len(node.Title) > 0 ||
				!linkHasVisibleLabel(node, source) ||
				!linkDestinationIsSafe(node.Destination) ||
				(node.Reference != nil && node.Reference.Type == ast.ReferenceLinkFull) {
				mutations = appendLinkOpeningMutation(mutations, source, node.Pos())
			}
		case *ast.Image:
			mutations = appendLinkOpeningMutation(mutations, source, node.Pos())
		}
		return ast.WalkContinue, nil
	})
	return mutations
}

func usedLinkReferences(document ast.Node) map[string]struct{} {
	used := map[string]struct{}{}
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch node := node.(type) {
		case *ast.Link:
			if node.Reference != nil {
				used[util.ToLinkReference(node.Reference.Value)] = struct{}{}
			}
		case *ast.Image:
			if node.Reference != nil {
				used[util.ToLinkReference(node.Reference.Value)] = struct{}{}
			}
		}
		return ast.WalkContinue, nil
	})
	return used
}

func appendLinkOpeningMutation(mutations []sourceMutation, source []byte, start int) []sourceMutation {
	if start >= len(source) {
		start = len(source) - 1
	}
	for offset := start; offset < min(len(source), start+2); offset++ {
		if source[offset] == '[' {
			return append(mutations, sourceMutation{start: offset, stop: offset + 1, replacement: `\[`})
		}
	}
	lineStart := bytes.LastIndexByte(source[:max(start, 0)], '\n') + 1
	for offset := start; offset >= lineStart; offset-- {
		if source[offset] == '[' {
			return append(mutations, sourceMutation{start: offset, stop: offset + 1, replacement: `\[`})
		}
	}
	return mutations
}

func linkDestinationIsSafe(destination []byte) bool {
	if len(destination) == 0 {
		return false
	}
	decoded := util.UnescapePunctuations(destination)
	decoded = util.ResolveNumericReferences(decoded)
	decoded = util.ResolveEntityNames(decoded)
	for _, r := range string(decoded) {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}

	parsed, err := url.Parse(string(decoded))
	if err != nil {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "":
		return true
	case "http", "https":
		return parsed.Host != ""
	case "mailto":
		return parsed.Opaque != "" || parsed.Path != ""
	default:
		return false
	}
}

func linkHasVisibleLabel(link ast.Node, source []byte) bool {
	visible := false
	_ = ast.Walk(link, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || node == link {
			return ast.WalkContinue, nil
		}
		switch node := node.(type) {
		case *ast.Text:
			textNode := node
			label := util.UnescapePunctuations(textNode.Segment.Value(source))
			label = util.ResolveNumericReferences(label)
			label = util.ResolveEntityNames(label)
			if textNode.HardLineBreak() {
				label = bytes.TrimSuffix(label, []byte{'\\'})
			}
			if labelHasVisibleRune(label) {
				visible = true
				return ast.WalkStop, nil
			}
			return ast.WalkContinue, nil
		case *ast.CodeSpan:
			if !node.IsBlank(source) {
				visible = true
				return ast.WalkStop, nil
			}
			return ast.WalkSkipChildren, nil
		case *ast.Emphasis:
			return ast.WalkContinue, nil
		}
		visible = true
		return ast.WalkStop, nil
	})
	return visible
}

func labelHasVisibleRune(label []byte) bool {
	for _, r := range string(label) {
		if unicode.IsSpace(r) ||
			unicode.IsControl(r) ||
			unicode.IsMark(r) ||
			r == '*' ||
			r == '_' ||
			r == '~' ||
			shouldRemoveRune(r) ||
			isVariationSelector(r) ||
			r == 0x200D ||
			r == 0x115F ||
			r == 0x1160 ||
			r == 0x2800 ||
			r == 0x3164 ||
			r == 0xFFA0 ||
			r == 0x13441 ||
			r == 0x13442 {
			continue
		}
		return unicode.IsGraphic(r)
	}
	return false
}

func neutralizeGitHubMarkdownExtensions(input string) string {
	if !strings.Contains(input, "$") && !strings.Contains(input, "[^") {
		return input
	}

	codeMask := markdownCodeMask(input)
	urlMask := markdownURLMask(input)
	var mutations []sourceMutation
	for offset := range len(input) {
		if codeMask[offset] || urlMask[offset] {
			continue
		}
		if strings.HasPrefix(input[offset:], "[^") && !isBackslashEscaped(input, offset) {
			mutations = append(mutations, sourceMutation{
				start:       offset,
				stop:        offset + 1,
				replacement: `\[`,
			})
		}
	}
	for _, offset := range githubMathDelimiters(input, codeMask, urlMask) {
		mutations = append(mutations, sourceMutation{
			start:       offset,
			stop:        offset + 1,
			replacement: `\$`,
		})
	}
	if len(mutations) == 0 {
		return input
	}
	return applySourceMutations(input, mutations)
}

func githubMathDelimiters(input string, codeMask, urlMask []bool) []int {
	var (
		openings        []int
		inlineOpening   = -1
		backtickOpening = -1
		blockOpening    = -1
	)

	for offset := 0; offset < len(input); {
		if input[offset] == '\n' {
			inlineOpening = -1
			backtickOpening = -1
			offset++
			continue
		}
		if codeMask[offset] || urlMask[offset] {
			offset++
			continue
		}

		switch {
		case strings.HasPrefix(input[offset:], "`$") &&
			backtickOpening >= 0 &&
			!isBackslashEscaped(input, offset+1):
			openings = append(openings, backtickOpening, offset+1)
			backtickOpening = -1
			offset += 2
		case strings.HasPrefix(input[offset:], "$$") && !isBackslashEscaped(input, offset):
			if blockOpening < 0 {
				blockOpening = offset
			} else {
				openings = append(openings, blockOpening, blockOpening+1, offset, offset+1)
				blockOpening = -1
			}
			offset += 2
		case strings.HasPrefix(input[offset:], "$`") && !isBackslashEscaped(input, offset):
			if backtickOpening < 0 {
				backtickOpening = offset
			}
			offset += 2
		case input[offset] == '$' && !isBackslashEscaped(input, offset):
			if inlineOpening < 0 {
				inlineOpening = offset
			} else {
				openings = append(openings, inlineOpening, offset)
				inlineOpening = -1
			}
			offset++
		default:
			offset++
		}
	}
	return openings
}

func markdownURLMask(input string) []bool {
	mask := make([]bool, len(input))
	for start := 0; start < len(input); {
		for start < len(input) && unicode.IsSpace(rune(input[start])) {
			start++
		}
		stop := start
		for stop < len(input) && !unicode.IsSpace(rune(input[stop])) {
			stop++
		}
		token := input[start:stop]
		if urlStart := urlStart(token); urlStart >= 0 {
			for offset := start + urlStart; offset < stop; offset++ {
				mask[offset] = true
			}
		}
		start = stop
	}

	for start := 0; start+2 < len(input); start++ {
		if input[start] != ']' ||
			input[start+1] != '(' ||
			!hasActiveLinkOpener(input, start) {
			continue
		}
		destinationStart := start + 2
		stop := destinationStart
		for stop < len(input) &&
			input[stop] != ')' &&
			!unicode.IsSpace(rune(input[stop])) {
			stop++
		}
		if stop < len(input) && input[stop] == ')' {
			for offset := destinationStart; offset < stop; offset++ {
				mask[offset] = true
			}
		}
		start = stop
	}
	return mask
}

func hasActiveLinkOpener(input string, closingLabel int) bool {
	for offset := closingLabel - 1; offset >= 0 && input[offset] != '\n'; offset-- {
		if input[offset] == '[' {
			return !isBackslashEscaped(input, offset)
		}
	}
	return false
}

func urlStart(token string) int {
	if separator := strings.Index(token, "://"); separator > 0 {
		start := separator - 1
		for start > 0 && isURLSchemeCharacter(token[start-1]) {
			start--
		}
		if isASCIILetter(token[start]) {
			return start
		}
	}
	if start := strings.Index(token, "mailto:"); start >= 0 {
		return start
	}
	return strings.Index(token, "www.")
}

func isURLSchemeCharacter(b byte) bool {
	return isASCIILetter(b) ||
		(b >= '0' && b <= '9') ||
		b == '+' ||
		b == '-' ||
		b == '.'
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func markdownCodeMask(input string) []bool {
	mask := make([]bool, len(input))
	spans := markdownCodeSpans(input)
	for _, span := range spans {
		for offset := span.start; offset < span.stop; offset++ {
			mask[offset] = true
		}
	}
	return mask
}

func isBackslashEscaped(input string, offset int) bool {
	backslashes := 0
	for offset--; offset >= 0 && input[offset] == '\\'; offset-- {
		backslashes++
	}
	return backslashes%2 == 1
}

func appendCharacterMutations(
	mutations []sourceMutation,
	source []byte,
	segment text.Segment,
	character byte,
	replacement string,
) []sourceMutation {
	for offset := segment.Start; offset < segment.Stop; offset++ {
		if source[offset] == character {
			mutations = append(mutations, sourceMutation{
				start:       offset,
				stop:        offset + 1,
				replacement: replacement,
			})
		}
	}
	return mutations
}

func appendFirstCharacterMutation(
	mutations []sourceMutation,
	source []byte,
	segment text.Segment,
	character byte,
	replacement string,
) []sourceMutation {
	for offset := segment.Start; offset < segment.Stop; offset++ {
		if source[offset] == character {
			return append(mutations, sourceMutation{
				start:       offset,
				stop:        offset + 1,
				replacement: replacement,
			})
		}
	}
	return mutations
}

func markdownHiddenSpans(document ast.Node, source []byte) []sourceSpan {
	usedReferences := usedLinkReferences(document)
	seenDefinitions := map[string]struct{}{}
	var spans []sourceSpan
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch node := node.(type) {
		case *ast.RawHTML:
			for i := range node.Segments.Len() {
				segment := node.Segments.At(i)
				spans = append(spans, sourceSpan{start: segment.Start, stop: segment.Stop})
			}
		case *ast.HTMLBlock:
			spans = appendSegmentSpans(spans, node.Lines())
			if node.HasClosure() {
				spans = append(spans, sourceSpan{
					start: node.ClosureLine.Start,
					stop:  node.ClosureLine.Stop,
				})
			}
		case *ast.LinkReferenceDefinition:
			label := util.ToLinkReference(node.Label)
			_, duplicate := seenDefinitions[label]
			seenDefinitions[label] = struct{}{}
			_, used := usedReferences[label]
			if duplicate || !used {
				spans = appendSegmentSpans(spans, node.Lines())
			}
		case *ast.Link:
			if len(node.Title) > 0 ||
				!linkHasVisibleLabel(node, source) ||
				!linkDestinationIsSafe(node.Destination) ||
				(node.Reference != nil && node.Reference.Type == ast.ReferenceLinkFull) {
				spans = append(spans, sourceSpan{start: node.Pos(), stop: node.Pos() + 1})
			}
		case *ast.Image:
			spans = append(spans, sourceSpan{start: node.Pos(), stop: node.Pos() + 1})
		}
		return ast.WalkContinue, nil
	})
	return spans
}

func escapeMarkdownSyntaxOutsideCode(input string) string {
	codeSpans := markdownCodeSpans(input)
	codeSpanIndex := 0
	var out strings.Builder
	copied := 0

	for offset := range len(input) {
		for codeSpanIndex < len(codeSpans) && offset >= codeSpans[codeSpanIndex].stop {
			codeSpanIndex++
		}
		inCode := codeSpanIndex < len(codeSpans) &&
			offset >= codeSpans[codeSpanIndex].start &&
			offset < codeSpans[codeSpanIndex].stop
		if inCode ||
			(input[offset] != '<' && input[offset] != '[' && input[offset] != '$') ||
			((input[offset] == '[' || input[offset] == '$') && isBackslashEscaped(input, offset)) {
			continue
		}

		if out.Cap() == 0 {
			out.Grow(len(input))
		}
		out.WriteString(input[copied:offset])
		switch input[offset] {
		case '<':
			out.WriteString("&lt;")
		case '[':
			out.WriteString(`\[`)
		case '$':
			out.WriteString(`\$`)
		}
		copied = offset + 1
	}

	if out.Cap() == 0 {
		return input
	}
	out.WriteString(input[copied:])
	return out.String()
}

// FilterInvisibleCharacters removes invisible or control characters that should not appear
// in user-facing titles or bodies. This includes:
// - Unicode tag characters: U+E0001, U+E0020–U+E007F
// - BiDi control characters: U+202A–U+202E, U+2066–U+2069
// - BiDi/directional marks: U+200E, U+200F, U+061C
// - Hidden modifier characters: U+200B–U+200D, U+00AD, U+FEFF, U+180E, U+2060–U+2064
// - Variation selectors: U+FE00–U+FE0F, U+E0100–U+E01EF
//
// Variation selectors are removed unconditionally. Without validating every
// pair against the Unicode variation registries, accepting them provides a
// covert alphabet that can encode arbitrary hidden content between visible
// base characters. The base characters remain, but presentation may change.
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

		keep := keepVisibleRune(r, prev, prevKept)
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

// filterContentInvisibleCharacters interprets literal runes and HTML entities
// in one stream so variation selectors are checked against the character a
// Markdown renderer would display. Benign entities remain byte-for-byte
// unchanged. Entities that decode to hidden content have their leading
// ampersand escaped, making the entity source visible instead of deleting it.
func filterContentInvisibleCharacters(input string) string {
	start := -1
	for i := range len(input) {
		if input[i] == '&' || input[i] >= utf8.RuneSelf {
			start = i
			break
		}
	}
	if start < 0 {
		return input
	}

	var codeSpans []sourceSpan
	if strings.Contains(input, "&") {
		codeSpans = markdownCodeSpans(input)
	}
	codeSpanIndex := 0
	var (
		out      strings.Builder
		prev     rune
		prevKept bool
		copied   int
		changed  bool
	)
	if start > 0 {
		prev, prevKept = rune(input[start-1]), true
	}

	for i := start; i < len(input); {
		for codeSpanIndex < len(codeSpans) && i >= codeSpans[codeSpanIndex].stop {
			codeSpanIndex++
		}
		inCode := codeSpanIndex < len(codeSpans) &&
			i >= codeSpans[codeSpanIndex].start &&
			i < codeSpans[codeSpanIndex].stop

		if input[i] == '&' && !inCode {
			if decoded, end, ok := decodeHTMLEntityAt(input, i); ok {
				last, lastKept, visible := decodedEntityIsVisible(decoded, prev, prevKept)
				if !visible {
					if !changed {
						changed = true
						out.Grow(len(input))
					}
					out.WriteString(input[copied:i])
					out.WriteString("&amp;")
					out.WriteString(input[i+1 : end])
					copied = end
					prev, prevKept = rune(input[end-1]), true
				} else {
					prev, prevKept = last, lastKept
				}
				i = end
				continue
			}
		}

		r, size := utf8.DecodeRuneInString(input[i:])
		keep := keepContentRune(r, prev, prevKept)
		prev, prevKept = r, keep
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

func markdownCodeSpans(input string) []sourceSpan {
	source := []byte(input)
	document := markdownParser.Parse(text.NewReader(source))
	var spans []sourceSpan
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch node := node.(type) {
		case *ast.CodeSpan:
			for child := node.FirstChild(); child != nil; child = child.NextSibling() {
				if textNode, ok := child.(*ast.Text); ok {
					spans = append(spans, sourceSpan{
						start: textNode.Segment.Start,
						stop:  textNode.Segment.Stop,
					})
				}
			}
		case *ast.CodeBlock:
			spans = appendSegmentSpans(spans, node.Lines())
		case *ast.FencedCodeBlock:
			spans = appendSegmentSpans(spans, node.Lines())
		}
		return ast.WalkContinue, nil
	})
	sort.Slice(spans, func(i, j int) bool {
		return spans[i].start < spans[j].start
	})
	return spans
}

func appendSegmentSpans(spans []sourceSpan, segments *text.Segments) []sourceSpan {
	for i := range segments.Len() {
		segment := segments.At(i)
		spans = append(spans, sourceSpan{start: segment.Start, stop: segment.Stop})
	}
	return spans
}

const maxNamedHTMLEntityLength = 64

func decodeHTMLEntityAt(input string, start int) (string, int, bool) {
	if start+2 >= len(input) || input[start] != '&' {
		return "", start, false
	}

	// "shy" is the only named entity that both decodes without a semicolon
	// under Go's HTML rules and maps to a rune this package removes.
	if strings.HasPrefix(input[start:], "&shy") {
		return "\u00AD", start + len("&shy"), true
	}

	if input[start+1] == '#' {
		end := start + 2
		isHex := end < len(input) && (input[end] == 'x' || input[end] == 'X')
		if isHex {
			end++
		}
		digitsStart := end
		for end < len(input) && isEntityDigit(input[end], isHex) {
			end++
		}
		if end == digitsStart {
			return "", start, false
		}
		if end < len(input) && input[end] == ';' {
			end++
		}
		entity := input[start:end]
		decoded := html.UnescapeString(entity)
		return decoded, end, decoded != entity
	}

	limit := min(len(input), start+maxNamedHTMLEntityLength)
	semicolon := strings.IndexByte(input[start+1:limit], ';')
	if semicolon < 0 {
		return "", start, false
	}
	end := start + 1 + semicolon + 1
	for i := start + 1; i < end-1; i++ {
		if !isEntityNameChar(input[i]) {
			return "", start, false
		}
	}
	entity := input[start:end]
	decoded := html.UnescapeString(entity)
	return decoded, end, decoded != entity
}

func isEntityDigit(b byte, hex bool) bool {
	if b >= '0' && b <= '9' {
		return true
	}
	return hex && ((b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F'))
}

func isEntityNameChar(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func decodedEntityIsVisible(input string, prev rune, prevKept bool) (rune, bool, bool) {
	visible := true
	for _, r := range input {
		keep := keepContentRune(r, prev, prevKept)
		prev, prevKept = r, keep
		if !keep {
			visible = false
		}
	}
	return prev, prevKept, visible
}

func keepVisibleRune(r, _ rune, _ bool) bool {
	if isVariationSelector(r) {
		return false
	}
	return !shouldRemoveRune(r)
}

func keepContentRune(r, _ rune, _ bool) bool {
	if isVariationSelector(r) || r == 0x200D {
		return false
	}
	return !shouldRemoveRune(r)
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
	if needsCommonMarkFenceParsing(input) {
		return filterCommonMarkFenceMetadata(input)
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
		return filterCommonMarkFenceMetadata(input)
	}
	out.WriteString(input[copied:])
	return filterCommonMarkFenceMetadata(out.String())
}

func filterCommonMarkFenceMetadata(input string) string {
	if !needsCommonMarkFenceParsing(input) {
		return input
	}

	source := []byte(input)
	document := markdownParser.Parse(text.NewReader(source))
	var removals []sourceSpan
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		fence, ok := node.(*ast.FencedCodeBlock)
		if !entering || !ok || fence.Info == nil {
			return ast.WalkContinue, nil
		}

		if !fenceInfoIsUnsafe(fence, source) {
			return ast.WalkContinue, nil
		}

		removals = append(removals, sourceSpan{
			start: fence.Info.Segment.Start,
			stop:  fence.Info.Segment.Stop,
		})
		return ast.WalkContinue, nil
	})

	if len(removals) == 0 {
		return input
	}

	var out strings.Builder
	out.Grow(len(input))
	copied := 0
	for _, removal := range removals {
		out.WriteString(input[copied:removal.start])
		copied = removal.stop
	}
	out.WriteString(input[copied:])
	return out.String()
}

func fenceInfoIsUnsafe(fence *ast.FencedCodeBlock, source []byte) bool {
	if fence.Info == nil {
		return false
	}
	info := string(fence.Info.Segment.Value(source))
	return len(info) > maxCodeFenceInfoLength ||
		strings.IndexFunc(info, unicode.IsSpace) != -1 ||
		!isSafeCodeFenceToken(info)
}

func stripAllFenceMetadata(input string) string {
	if !strings.Contains(input, "```") && !strings.Contains(input, "~~~") {
		return input
	}

	source := []byte(input)
	document := markdownParser.Parse(text.NewReader(source))
	var removals []sourceMutation
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		fence, ok := node.(*ast.FencedCodeBlock)
		if entering && ok && fence.Info != nil {
			removals = append(removals, sourceMutation{
				start: fence.Info.Segment.Start,
				stop:  fence.Info.Segment.Stop,
			})
		}
		return ast.WalkContinue, nil
	})
	if len(removals) == 0 {
		return input
	}
	return applySourceMutations(input, removals)
}

func needsCommonMarkFenceParsing(input string) bool {
	if strings.Contains(input, "~~~") {
		return true
	}

	for start := 0; start <= len(input); {
		line := input[start:]
		if nl := strings.IndexByte(line, '\n'); nl >= 0 {
			line = line[:nl]
		}
		if before, _, found := strings.Cut(line, "```"); found && !isSimpleFencePrefix(before) {
			return true
		}
		start += len(line) + 1
	}
	return false
}

const maxCodeFenceInfoLength = 48

func sanitizeCodeFenceLine(line string, insideFence bool, expectedFenceLen int) (string, bool, int) {
	idx := strings.Index(line, "```")
	if idx == -1 {
		return line, false, expectedFenceLen
	}

	if !isSimpleFencePrefix(line[:idx]) {
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
		if expectedFenceLen != 0 && fenceLen < expectedFenceLen {
			return line, false, expectedFenceLen
		}
		if strings.Trim(rest, " \t\r") != "" {
			return line, false, expectedFenceLen
		}
		return line[:fenceEnd], true, fenceLen
	}

	trimmed := strings.Trim(rest, " \t\r")

	if trimmed == "" {
		return line[:fenceEnd], true, fenceLen
	}

	if strings.Contains(trimmed, "`") {
		return line, false, expectedFenceLen
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

func isSafeCodeFenceToken(token string) bool {
	_, ok := safeCodeFenceLanguages[strings.ToLower(token)]
	return ok
}

var safeCodeFenceLanguages = map[string]struct{}{
	"astro": {}, "bash": {}, "c": {}, "c#": {}, "c++": {}, "clojure": {},
	"console": {}, "cpp": {}, "csharp": {}, "cs": {}, "css": {}, "dart": {},
	"diff": {}, "dockerfile": {}, "elixir": {}, "erlang": {}, "ex": {}, "fish": {},
	"go": {}, "golang": {}, "graphql": {}, "haskell": {}, "hcl": {}, "html": {},
	"env": {}, "http": {}, "ini": {}, "java": {}, "javascript": {}, "js": {},
	"json": {}, "jsonc": {},
	"jsx": {}, "kotlin": {}, "kt": {}, "less": {}, "lisp": {}, "lua": {},
	"makefile": {}, "markdown": {}, "md": {}, "nix": {}, "objective-c": {},
	"ocaml": {}, "patch": {}, "perl": {}, "php": {}, "pl": {}, "plaintext": {},
	"powershell": {}, "properties": {}, "ps1": {}, "py": {}, "python": {}, "r": {},
	"rb": {}, "rs": {}, "ruby": {}, "rust": {}, "scala": {}, "scheme": {},
	"scss": {}, "sh": {}, "shell": {}, "shell-session": {}, "solidity": {},
	"sql": {}, "suggestion": {}, "svelte": {}, "swift": {}, "terraform": {},
	"text": {}, "toml": {}, "ts": {}, "tsx": {}, "typescript": {}, "vue": {},
	"wasm": {}, "xml": {},
	"yaml": {}, "yml": {}, "zsh": {},
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
		0x200C,  // ZERO WIDTH NON-JOINER
		0x200D,  // ZERO WIDTH JOINER
		0x200E,  // LEFT-TO-RIGHT MARK
		0x200F,  // RIGHT-TO-LEFT MARK
		0x061C,  // ARABIC LETTER MARK
		0x00AD,  // SOFT HYPHEN
		0xFEFF,  // ZERO WIDTH NO-BREAK SPACE
		0x180E,  // MONGOLIAN VOWEL SEPARATOR
		0x13441, // EGYPTIAN HIEROGLYPH FULL BLANK
		0x13442: // EGYPTIAN HIEROGLYPH HALF BLANK
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
	return (r >= 0x180B && r <= 0x180D) ||
		r == 0x180F ||
		(r >= 0xFE00 && r <= 0xFE0F) ||
		(r >= 0xE0100 && r <= 0xE01EF)
}
