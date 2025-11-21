package buffer

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ProcessResponseAsRingBufferToEnd_EmptyResponse(t *testing.T) {
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader("")),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 10)

	require.NoError(t, err)
	assert.Equal(t, "", result)
	assert.Equal(t, 0, totalLines)
	assert.Equal(t, resp, returnedResp)
}

func Test_ProcessResponseAsRingBufferToEnd_SingleLine(t *testing.T) {
	content := "single line"
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(content)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 10)

	require.NoError(t, err)
	assert.Equal(t, content, result)
	assert.Equal(t, 1, totalLines)
	assert.Equal(t, resp, returnedResp)
}

func Test_ProcessResponseAsRingBufferToEnd_FewerLinesThanBuffer(t *testing.T) {
	lines := []string{"line 1", "line 2", "line 3"}
	content := strings.Join(lines, "\n")
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(content)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 10)

	require.NoError(t, err)
	assert.Equal(t, content, result)
	assert.Equal(t, 3, totalLines)
	assert.Equal(t, resp, returnedResp)
}

func Test_ProcessResponseAsRingBufferToEnd_ExactlyBufferSize(t *testing.T) {
	lines := []string{"line 1", "line 2", "line 3", "line 4", "line 5"}
	content := strings.Join(lines, "\n")
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(content)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 5)

	require.NoError(t, err)
	assert.Equal(t, content, result)
	assert.Equal(t, 5, totalLines)
	assert.Equal(t, resp, returnedResp)
}

func Test_ProcessResponseAsRingBufferToEnd_MoreLinesThanBuffer_RingBufferWraparound(t *testing.T) {
	// Create 10 lines but buffer only holds 5 - should get last 5
	lines := []string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
		"line 6",
		"line 7",
		"line 8",
		"line 9",
		"line 10",
	}
	content := strings.Join(lines, "\n")
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(content)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 5)

	require.NoError(t, err)
	expectedLines := lines[5:] // Last 5 lines
	expected := strings.Join(expectedLines, "\n")
	assert.Equal(t, expected, result)
	assert.Equal(t, 10, totalLines)
	assert.Equal(t, resp, returnedResp)
}

func Test_ProcessResponseAsRingBufferToEnd_MuchMoreLinesThanBuffer(t *testing.T) {
	// Create 100 lines but buffer only holds 3 - should get last 3
	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, "line "+string(rune('0'+i%10)))
	}
	content := strings.Join(lines, "\n")
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(content)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 3)

	require.NoError(t, err)
	expectedLines := lines[97:] // Last 3 lines
	expected := strings.Join(expectedLines, "\n")
	assert.Equal(t, expected, result)
	assert.Equal(t, 100, totalLines)
	assert.Equal(t, resp, returnedResp)
}

func Test_ProcessResponseAsRingBufferToEnd_BufferSizeOne(t *testing.T) {
	lines := []string{"line 1", "line 2", "line 3"}
	content := strings.Join(lines, "\n")
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(content)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 1)

	require.NoError(t, err)
	assert.Equal(t, "line 3", result) // Should only get the last line
	assert.Equal(t, 3, totalLines)
	assert.Equal(t, resp, returnedResp)
}

func Test_ProcessResponseAsRingBufferToEnd_EmptyLines(t *testing.T) {
	content := "\n\n\n"
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(content)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 10)

	require.NoError(t, err)
	assert.Equal(t, "\n\n", result) // Scanner reads 3 empty lines
	assert.Equal(t, 3, totalLines)  // 3 empty lines from scanner
	assert.Equal(t, resp, returnedResp)
}

func Test_ProcessResponseAsRingBufferToEnd_MixedEmptyLines(t *testing.T) {
	lines := []string{"line 1", "", "line 3", "", "line 5"}
	content := strings.Join(lines, "\n")
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(content)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 10)

	require.NoError(t, err)
	assert.Equal(t, content, result)
	assert.Equal(t, 5, totalLines)
	assert.Equal(t, resp, returnedResp)
}

func Test_ProcessResponseAsRingBufferToEnd_LongLines(t *testing.T) {
	// Test with lines of varying lengths
	longLine := strings.Repeat("a", 2000)
	veryLongLine := strings.Repeat("b", 10000)
	lines := []string{"short", longLine, veryLongLine, "end"}
	content := strings.Join(lines, "\n")
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(content)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 10)

	require.NoError(t, err)
	assert.Equal(t, content, result)
	assert.Equal(t, 4, totalLines)
	assert.Equal(t, resp, returnedResp)
}

func Test_ProcessResponseAsRingBufferToEnd_NoTrailingNewline(t *testing.T) {
	content := "line 1\nline 2\nline 3"
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(content)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 10)

	require.NoError(t, err)
	assert.Equal(t, content, result)
	assert.Equal(t, 3, totalLines)
	assert.Equal(t, resp, returnedResp)
}

func Test_ProcessResponseAsRingBufferToEnd_WithTrailingNewline(t *testing.T) {
	content := "line 1\nline 2\nline 3\n"
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(content)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 10)

	require.NoError(t, err)
	// Scanner doesn't read empty line after trailing newline
	assert.Equal(t, "line 1\nline 2\nline 3", result)
	assert.Equal(t, 3, totalLines) // 3 lines (scanner doesn't count trailing empty)
	assert.Equal(t, resp, returnedResp)
}

func Test_ProcessResponseAsRingBufferToEnd_RingBufferCorrectOrder(t *testing.T) {
	// Test that ring buffer maintains correct order after wraparound
	lines := []string{"A", "B", "C", "D", "E", "F", "G"}
	content := strings.Join(lines, "\n")
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(content)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 3)

	require.NoError(t, err)
	// Should get last 3 lines in correct order: E, F, G
	assert.Equal(t, "E\nF\nG", result)
	assert.Equal(t, 7, totalLines)
	assert.Equal(t, resp, returnedResp)
}
