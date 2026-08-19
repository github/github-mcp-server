package sanitize

import (
	"strings"
	"testing"
)

// benchCorpus holds the response shapes that dominate high-throughput
// conversion: short titles, comment-sized bodies, large issue bodies, and the
// adversarial content the sanitizer exists to neutralise.
var benchCorpus = []struct {
	name  string
	input string
}{
	{"TitleASCII", benchTitleASCII},
	{"TitleUnicode", benchTitleUnicode},
	{"Comment1KiB", benchComment1KiB},
	{"Comment1KiBUnicode", benchComment1KiBUnicode},
	{"Body64KiB", benchBody64KiB},
	{"Body64KiBUnicode", benchBody64KiBUnicode},
	{"CodeFenceBody", benchCodeFenceBody},
	{"AdversarialHTML", benchAdversarialHTML},
	{"AdversarialUnicode", benchAdversarialUnicode},
	{"AdversarialMixed", benchAdversarialMixed},
}

var (
	benchTitleASCII   = "Fix flaky converter test for issue comments on large pages"
	benchTitleUnicode = "Fix flaky ✈️ converter test — 世界 for issue comments"

	benchComment1KiB        = buildClean(1024)
	benchComment1KiBUnicode = buildUnicode(1024)
	benchBody64KiB          = buildClean(64 * 1024)
	benchBody64KiBUnicode   = buildUnicode(64 * 1024)

	benchCodeFenceBody = buildFenced(4096)

	benchAdversarialHTML = strings.Repeat(
		"<script>alert(1)</script>Hello <b>bold</b> &#8203; <a href=\"https://example.com\" onclick=\"x\">link</a>\n",
		16,
	)
	benchAdversarialUnicode = strings.Repeat(
		"Hidden\u200B\u200C\u202Epayload\u202C\u2066here\u2069\uFE0F\U000E0101\U000E0102 \U0001F600\uFE0F ok\n",
		16,
	)
	benchAdversarialMixed = benchAdversarialHTML + benchAdversarialUnicode + benchCodeFenceBody
)

// buildClean produces deterministic plain markdown prose of at least n bytes,
// representative of an ordinary comment or issue body.
func buildClean(n int) string {
	const para = "The converter allocates a new slice for every field it touches, which shows up " +
		"as GC pressure once the response contains a few hundred comments. Rework the hot path so " +
		"clean text is returned as-is. See the linked issue for measurements and the plan.\n\n" +
		"- item one\n- item two\n- item three\n\n"

	var b strings.Builder
	b.Grow(n + len(para))
	for b.Len() < n {
		b.WriteString(para)
	}
	return b.String()[:n]
}

// buildUnicode produces deterministic prose of at least n bytes containing
// legitimate non-ASCII text (accents, CJK, emoji with variation selectors) that
// the sanitizer must preserve untouched.
func buildUnicode(n int) string {
	const para = "Der Konverter reserviert für jedes Feld einen neuen Puffer — 世界 — was sich als " +
		"GC-Druck zeigt. Ship it \U0001F600\uFE0F and \u2708\uFE0F today. 葛\U000E0100城 is a registered sequence.\n\n"

	var b strings.Builder
	b.Grow(n + len(para))
	for b.Len() < n {
		b.WriteString(para)
	}
	// Trim on a rune boundary so the corpus stays valid UTF-8.
	s := b.String()
	for n > 0 && n < len(s) && s[n]&0xC0 == 0x80 {
		n--
	}
	return s[:n]
}

// buildFenced produces deterministic prose of at least n bytes built from fenced
// code blocks, exercising the code-fence filter's line splitting.
func buildFenced(n int) string {
	const block = "Consider this snippet:\n\n```go\nfmt.Println(\"hi\")\nreturn nil\n```\n\nand this one:\n\n" +
		"```\nplain text block\n```\n\n"

	var b strings.Builder
	b.Grow(n + len(block))
	for b.Len() < n {
		b.WriteString(block)
	}
	return b.String()
}

func BenchmarkSanitize(b *testing.B) {
	for _, tc := range benchCorpus {
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(tc.input)))
			b.ReportAllocs()
			for b.Loop() {
				sinkString = Sanitize(tc.input)
			}
		})
	}
}

func BenchmarkFilterInvisibleCharacters(b *testing.B) {
	for _, tc := range benchCorpus {
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(tc.input)))
			b.ReportAllocs()
			for b.Loop() {
				sinkString = FilterInvisibleCharacters(tc.input)
			}
		})
	}
}

func BenchmarkFilterHTMLTags(b *testing.B) {
	for _, tc := range benchCorpus {
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(tc.input)))
			b.ReportAllocs()
			for b.Loop() {
				sinkString = FilterHTMLTags(tc.input)
			}
		})
	}
}

func BenchmarkFilterCodeFenceMetadata(b *testing.B) {
	for _, tc := range benchCorpus {
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(tc.input)))
			b.ReportAllocs()
			for b.Loop() {
				sinkString = FilterCodeFenceMetadata(tc.input)
			}
		})
	}
}

// BenchmarkSanitizeIssuePage models a 30-issue listing response: each issue
// contributes a title and a 2 KiB body.
func BenchmarkSanitizeIssuePage(b *testing.B) {
	bodies := makePage(30, 2048, buildClean)
	b.SetBytes(int64(pageBytes(bodies) + 30*len(benchTitleASCII)))
	b.ReportAllocs()
	for b.Loop() {
		for _, body := range bodies {
			sinkLen += len(Sanitize(benchTitleASCII)) + len(Sanitize(body))
		}
	}
}

// BenchmarkSanitizeCommentPage models a 100-comment listing response with 1 KiB
// bodies, the shape called out as the worst case in issue #3117.
func BenchmarkSanitizeCommentPage(b *testing.B) {
	bodies := makePage(100, 1024, buildClean)
	b.SetBytes(int64(pageBytes(bodies)))
	b.ReportAllocs()
	for b.Loop() {
		for _, body := range bodies {
			sinkLen += len(Sanitize(body))
		}
	}
}

func makePage(count, size int, build func(int) string) []string {
	page := make([]string, count)
	for i := range page {
		// Vary the offset so entries are not identical strings.
		page[i] = build(size + i)
	}
	return page
}

func pageBytes(page []string) int {
	total := 0
	for _, s := range page {
		total += len(s)
	}
	return total
}

var (
	sinkString string
	sinkLen    int
)
