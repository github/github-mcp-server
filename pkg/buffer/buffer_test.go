package buffer

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessResponseAsRingBufferToEnd(t *testing.T) {
	t.Run("normal lines", func(t *testing.T) {
		body := "line1\nline2\nline3\n"
		resp := &http.Response{
			Body: io.NopCloser(strings.NewReader(body)),
		}

		result, totalLines, respOut, err := ProcessResponseAsRingBufferToEnd(resp, 10)
		if respOut != nil && respOut.Body != nil {
			defer respOut.Body.Close()
		}
		require.NoError(t, err)
		assert.Equal(t, 3, totalLines)
		assert.Equal(t, "line1\nline2\nline3", result)
	})

	t.Run("ring buffer keeps last N lines", func(t *testing.T) {
		body := "line1\nline2\nline3\nline4\nline5\n"
		resp := &http.Response{
			Body: io.NopCloser(strings.NewReader(body)),
		}

		result, totalLines, respOut, err := ProcessResponseAsRingBufferToEnd(resp, 3)
		if respOut != nil && respOut.Body != nil {
			defer respOut.Body.Close()
		}
		require.NoError(t, err)
		assert.Equal(t, 5, totalLines)
		assert.Equal(t, "line3\nline4\nline5", result)
	})

	t.Run("handles very long line exceeding limit", func(t *testing.T) {
		longLine := strings.Repeat("x", 2*maxLineSize)
		body := "line1\n" + longLine + "\nline3\n"
		resp := &http.Response{
			Body: io.NopCloser(strings.NewReader(body)),
		}

		result, totalLines, respOut, err := ProcessResponseAsRingBufferToEnd(resp, 100)
		if respOut != nil && respOut.Body != nil {
			defer respOut.Body.Close()
		}
		require.NoError(t, err)
		// Should have processed lines with truncation marker
		assert.Greater(t, totalLines, 0)
		assert.Contains(t, result, "TRUNCATED")
	})

	t.Run("preserves line below limit", func(t *testing.T) {
		longLine := strings.Repeat("a", maxLineSize-1)
		body := "start\n" + longLine + "\nend\n"
		resp := &http.Response{
			Body: io.NopCloser(strings.NewReader(body)),
		}

		result, totalLines, respOut, err := ProcessResponseAsRingBufferToEnd(resp, 100)
		if respOut != nil && respOut.Body != nil {
			defer respOut.Body.Close()
		}
		require.NoError(t, err)
		assert.Equal(t, 3, totalLines)
		assert.Contains(t, result, "start")
		assert.Contains(t, result, "end")
		assert.Contains(t, result, longLine)
		assert.NotContains(t, result, "TRUNCATED")
	})

	t.Run("ring buffer with long line in middle of many lines", func(t *testing.T) {
		// Create many lines with a long line in the middle
		// Ring buffer size is 5, so we should only keep the last 5 lines
		var sb strings.Builder
		for i := 1; i <= 10; i++ {
			fmt.Fprintf(&sb, "line%d\n", i)
		}
		longLine := strings.Repeat("x", 2*maxLineSize)
		sb.WriteString(longLine)
		sb.WriteString("\n")
		for i := 11; i <= 20; i++ {
			fmt.Fprintf(&sb, "line%d\n", i)
		}

		resp := &http.Response{
			Body: io.NopCloser(strings.NewReader(sb.String())),
		}

		result, totalLines, respOut, err := ProcessResponseAsRingBufferToEnd(resp, 5)
		if respOut != nil && respOut.Body != nil {
			defer respOut.Body.Close()
		}
		require.NoError(t, err)
		// 10 lines before + 1 long line + 10 lines after = 21 total
		assert.Equal(t, 21, totalLines)
		// Should only have the last 5 lines (line16 through line20)
		assert.Contains(t, result, "line16")
		assert.Contains(t, result, "line17")
		assert.Contains(t, result, "line18")
		assert.Contains(t, result, "line19")
		assert.Contains(t, result, "line20")
		// Should NOT contain earlier lines
		assert.NotContains(t, result, "line1\n")
		assert.NotContains(t, result, "line10\n")
		// The truncated line should not be in the last 5
		assert.NotContains(t, result, "TRUNCATED")
	})

	t.Run("empty response body", func(t *testing.T) {
		resp := &http.Response{
			Body: io.NopCloser(strings.NewReader("")),
		}

		result, totalLines, respOut, err := ProcessResponseAsRingBufferToEnd(resp, 10)
		if respOut != nil && respOut.Body != nil {
			defer respOut.Body.Close()
		}
		require.NoError(t, err)
		assert.Equal(t, 0, totalLines)
		assert.Equal(t, "", result)
	})

	t.Run("line at exactly maxLineSize boundary", func(t *testing.T) {
		exactLine := strings.Repeat("z", maxLineSize)
		body := "before\n" + exactLine + "\nafter\n"
		resp := &http.Response{
			Body: io.NopCloser(strings.NewReader(body)),
		}

		result, totalLines, respOut, err := ProcessResponseAsRingBufferToEnd(resp, 10)
		if respOut != nil && respOut.Body != nil {
			defer respOut.Body.Close()
		}
		require.NoError(t, err)
		assert.Equal(t, 3, totalLines)
		assert.Contains(t, result, "before")
		assert.Contains(t, result, "TRUNCATED")
		assert.Contains(t, result, "after")
	})

	t.Run("ring buffer keeps truncated line when in last N", func(t *testing.T) {
		// Long line followed by only 2 more lines, with ring buffer size 5
		longLine := strings.Repeat("y", 2*maxLineSize)
		body := "line1\nline2\nline3\n" + longLine + "\nlineA\nlineB\n"
		resp := &http.Response{
			Body: io.NopCloser(strings.NewReader(body)),
		}

		result, totalLines, respOut, err := ProcessResponseAsRingBufferToEnd(resp, 5)
		if respOut != nil && respOut.Body != nil {
			defer respOut.Body.Close()
		}
		require.NoError(t, err)
		assert.Equal(t, 6, totalLines)
		// Last 5: line2, line3, truncated, lineA, lineB
		assert.Contains(t, result, "line2")
		assert.Contains(t, result, "line3")
		assert.Contains(t, result, "TRUNCATED")
		assert.Contains(t, result, "lineA")
		assert.Contains(t, result, "lineB")
		// line1 should be rotated out
		assert.NotContains(t, result, "line1")
	})
}

func TestLogBufferBoundsContentBytes(t *testing.T) {
	for _, lineBytes := range []int{8 * 1024, 1 << 20} {
		t.Run(fmt.Sprint(lineBytes), func(t *testing.T) {
			// Smaller lines exercise total-byte eviction; larger ones exercise
			// per-line truncation as well as eviction and ring wraparound.
			const count = 200
			line := strings.Repeat("x", lineBytes) + "\n"
			body := io.MultiReader(io.LimitReader(&repeatingLogReader{line: line}, int64(len(line)*count)), strings.NewReader("last line\n"))
			resp := &http.Response{Body: io.NopCloser(body)}
			result, total, respOut, err := ProcessResponseAsRingBufferToEnd(resp, 150)
			if respOut != nil && respOut.Body != nil {
				defer respOut.Body.Close()
			}
			require.NoError(t, err)
			assert.Equal(t, count+1, total)
			assert.LessOrEqual(t, len(result), MaxLogBytes)
			assert.True(t, strings.HasPrefix(result, truncationMarker+"\n"))
			assert.True(t, strings.HasSuffix(result, "last line"))
		})
	}
}

// Stream a large input without retaining the whole fixture in memory.
type repeatingLogReader struct {
	line string
	pos  int
}

func (r *repeatingLogReader) Read(p []byte) (int, error) {
	n := copy(p, r.line[r.pos:])
	r.pos = (r.pos + n) % len(r.line)
	return n, nil
}

func TestLogBufferTruncationPreservesUTF8(t *testing.T) {
	body := "before\n" + strings.Repeat("界", maxLineSize) + "\nafter\n"
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	result, count, respOut, err := ProcessResponseAsRingBufferToEnd(resp, 3)
	if respOut != nil && respOut.Body != nil {
		defer respOut.Body.Close()
	}
	require.NoError(t, err)
	assert.Equal(t, 3, count)
	assert.True(t, utf8.ValidString(result))
	assert.Contains(t, result, truncationMarker)
	assert.True(t, strings.HasPrefix(result, "before\n"))
	assert.True(t, strings.HasSuffix(result, "\nafter"))
	assert.LessOrEqual(t, len(strings.Split(result, "\n")[1]), maxLineSize+len(truncationMarker))
}
