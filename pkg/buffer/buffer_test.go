package buffer

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

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

	t.Run("handles very long line exceeding 10MB", func(t *testing.T) {
		// Create a line that exceeds maxLineSize (10MB)
		longLine := strings.Repeat("x", 11*1024*1024) // 11MB
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

	t.Run("handles line at exactly max size", func(t *testing.T) {
		// Create a line just under maxLineSize
		longLine := strings.Repeat("a", 1024*1024) // 1MB - should work fine
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
	})

	t.Run("ring buffer with long line in middle of many lines", func(t *testing.T) {
		// Create many lines with a long line in the middle
		// Ring buffer size is 5, so we should only keep the last 5 lines
		var sb strings.Builder
		for i := 1; i <= 10; i++ {
			sb.WriteString(fmt.Sprintf("line%d\n", i))
		}
		// Insert an 11MB line (exceeds maxLineSize of 10MB)
		longLine := strings.Repeat("x", 11*1024*1024)
		sb.WriteString(longLine)
		sb.WriteString("\n")
		for i := 11; i <= 20; i++ {
			sb.WriteString(fmt.Sprintf("line%d\n", i))
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
		// Create a line at exactly maxLineSize (10MB) - should be truncated
		exactLine := strings.Repeat("z", 10*1024*1024)
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
		longLine := strings.Repeat("y", 11*1024*1024)
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
