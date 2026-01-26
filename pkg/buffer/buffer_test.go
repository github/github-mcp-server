package buffer

import (
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
}
