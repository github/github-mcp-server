package buffer

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessResponseAsRingBufferToEnd_SmallLog(t *testing.T) {
	// Test with fewer lines than the buffer size
	logContent := "line 1\nline 2\nline 3\n"
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(logContent)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 10)

	require.NoError(t, err)
	assert.Equal(t, resp, returnedResp)
	assert.Equal(t, 3, totalLines)
	assert.Equal(t, "line 1\nline 2\nline 3", result)
}

func TestProcessResponseAsRingBufferToEnd_ExactBufferSize(t *testing.T) {
	// Test with exactly the buffer size
	logContent := "line 1\nline 2\nline 3\nline 4\nline 5\n"
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(logContent)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 5)

	require.NoError(t, err)
	assert.Equal(t, resp, returnedResp)
	assert.Equal(t, 5, totalLines)
	assert.Equal(t, "line 1\nline 2\nline 3\nline 4\nline 5", result)
}

func TestProcessResponseAsRingBufferToEnd_LargeLog(t *testing.T) {
	// Test with more lines than the buffer size - should keep only last N lines
	logContent := "line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\n"
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(logContent)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 3)

	require.NoError(t, err)
	assert.Equal(t, resp, returnedResp)
	assert.Equal(t, 7, totalLines)
	// Should only contain the last 3 lines
	assert.Equal(t, "line 5\nline 6\nline 7", result)
}

func TestProcessResponseAsRingBufferToEnd_EmptyLog(t *testing.T) {
	// Test with empty content
	logContent := ""
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(logContent)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 10)

	require.NoError(t, err)
	assert.Equal(t, resp, returnedResp)
	assert.Equal(t, 0, totalLines)
	assert.Equal(t, "", result)
}

func TestProcessResponseAsRingBufferToEnd_SingleLine(t *testing.T) {
	// Test with a single line
	logContent := "single line\n"
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(logContent)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 10)

	require.NoError(t, err)
	assert.Equal(t, resp, returnedResp)
	assert.Equal(t, 1, totalLines)
	assert.Equal(t, "single line", result)
}

func TestProcessResponseAsRingBufferToEnd_NoTrailingNewline(t *testing.T) {
	// Test with content that doesn't end with a newline
	logContent := "line 1\nline 2\nline 3"
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(logContent)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 10)

	require.NoError(t, err)
	assert.Equal(t, resp, returnedResp)
	assert.Equal(t, 3, totalLines)
	assert.Equal(t, "line 1\nline 2\nline 3", result)
}

func TestProcessResponseAsRingBufferToEnd_BufferSizeOne(t *testing.T) {
	// Test with buffer size of 1
	logContent := "line 1\nline 2\nline 3\n"
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(logContent)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 1)

	require.NoError(t, err)
	assert.Equal(t, resp, returnedResp)
	assert.Equal(t, 3, totalLines)
	// Should only contain the last line
	assert.Equal(t, "line 3", result)
}

func TestProcessResponseAsRingBufferToEnd_LongLines(t *testing.T) {
	// Test with very long lines
	longLine := strings.Repeat("a", 1000)
	logContent := longLine + "\n" + longLine + "\n" + longLine + "\n"
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(logContent)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 2)

	require.NoError(t, err)
	assert.Equal(t, resp, returnedResp)
	assert.Equal(t, 3, totalLines)
	// Should contain the last 2 lines
	lines := strings.Split(result, "\n")
	assert.Equal(t, 2, len(lines))
	assert.Equal(t, 1000, len(lines[0]))
	assert.Equal(t, 1000, len(lines[1]))
}

func TestProcessResponseAsRingBufferToEnd_RingWraparound(t *testing.T) {
	// Test that ring buffer correctly wraps around
	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, "line "+string(rune('0'+i%10)))
	}
	logContent := strings.Join(lines, "\n") + "\n"
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(logContent)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 10)

	require.NoError(t, err)
	assert.Equal(t, resp, returnedResp)
	assert.Equal(t, 100, totalLines)

	// Should contain exactly the last 10 lines
	resultLines := strings.Split(result, "\n")
	assert.Equal(t, 10, len(resultLines))
}

func TestProcessResponseAsRingBufferToEnd_LargeBuffer(t *testing.T) {
	// Test with a large buffer size
	var lines []string
	for i := 1; i <= 50; i++ {
		lines = append(lines, "line number "+string(rune('0'+i%10)))
	}
	logContent := strings.Join(lines, "\n") + "\n"
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(logContent)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 1000)

	require.NoError(t, err)
	assert.Equal(t, resp, returnedResp)
	assert.Equal(t, 50, totalLines)

	// All lines should be present
	resultLines := strings.Split(result, "\n")
	assert.Equal(t, 50, len(resultLines))
}

func TestProcessResponseAsRingBufferToEnd_BlankLines(t *testing.T) {
	// Test with blank lines
	logContent := "line 1\n\nline 3\n\nline 5\n"
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(logContent)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 10)

	require.NoError(t, err)
	assert.Equal(t, resp, returnedResp)
	assert.Equal(t, 5, totalLines)
	assert.Equal(t, "line 1\n\nline 3\n\nline 5", result)
}

func TestProcessResponseAsRingBufferToEnd_OnlyBlankLines(t *testing.T) {
	// Test with only blank lines
	logContent := "\n\n\n\n\n"
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(logContent)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 3)

	require.NoError(t, err)
	assert.Equal(t, resp, returnedResp)
	assert.Equal(t, 5, totalLines)
	// Should contain the last 3 blank lines
	assert.Equal(t, "\n\n", result)
}

func TestProcessResponseAsRingBufferToEnd_VeryLargeLine(t *testing.T) {
	// Test with a line larger than the default scanner buffer
	// The scanner is configured with a 1MB max token size
	megabyteLine := strings.Repeat("x", 500*1024) // 500KB line
	logContent := "start\n" + megabyteLine + "\nend\n"
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(logContent)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 10)

	require.NoError(t, err)
	assert.Equal(t, resp, returnedResp)
	assert.Equal(t, 3, totalLines)

	resultLines := strings.Split(result, "\n")
	assert.Equal(t, 3, len(resultLines))
	assert.Equal(t, "start", resultLines[0])
	assert.Equal(t, 500*1024, len(resultLines[1]))
	assert.Equal(t, "end", resultLines[2])
}

func TestProcessResponseAsRingBufferToEnd_PreservesOrder(t *testing.T) {
	// Test that order is preserved correctly
	logContent := "first\nsecond\nthird\nfourth\nfifth\nsixth\n"
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(logContent)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 4)

	require.NoError(t, err)
	assert.Equal(t, resp, returnedResp)
	assert.Equal(t, 6, totalLines)

	// Should preserve order of the last 4 lines
	assert.Equal(t, "third\nfourth\nfifth\nsixth", result)
}

func TestProcessResponseAsRingBufferToEnd_SpecialCharacters(t *testing.T) {
	// Test with special characters
	logContent := "line with spaces\nline\twith\ttabs\nline-with-dashes\nline_with_underscores\n"
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(logContent)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 10)

	require.NoError(t, err)
	assert.Equal(t, resp, returnedResp)
	assert.Equal(t, 4, totalLines)
	assert.Contains(t, result, "line with spaces")
	assert.Contains(t, result, "line\twith\ttabs")
	assert.Contains(t, result, "line-with-dashes")
	assert.Contains(t, result, "line_with_underscores")
}

func TestProcessResponseAsRingBufferToEnd_UnicodeContent(t *testing.T) {
	// Test with Unicode characters
	logContent := "Hello 世界\nこんにちは\n🎉 emoji line\n"
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(logContent)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 10)

	require.NoError(t, err)
	assert.Equal(t, resp, returnedResp)
	assert.Equal(t, 3, totalLines)
	assert.Contains(t, result, "Hello 世界")
	assert.Contains(t, result, "こんにちは")
	assert.Contains(t, result, "🎉 emoji line")
}

func TestProcessResponseAsRingBufferToEnd_OverflowScenario(t *testing.T) {
	// Test a realistic overflow scenario similar to CI/CD logs
	var lines []string
	for i := 1; i <= 1000; i++ {
		lines = append(lines, "2024-01-01 12:00:00 [INFO] Build step "+string(rune('0'+i%10)))
	}
	logContent := strings.Join(lines, "\n") + "\n"
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(logContent)),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 50)

	require.NoError(t, err)
	assert.Equal(t, resp, returnedResp)
	assert.Equal(t, 1000, totalLines)

	// Should contain exactly the last 50 lines
	resultLines := strings.Split(result, "\n")
	assert.Equal(t, 50, len(resultLines))

	// Verify the last line is correct
	assert.Contains(t, resultLines[len(resultLines)-1], "Build step")
}

func TestProcessResponseAsRingBufferToEnd_ReturnsResponseObject(t *testing.T) {
	// Verify the response object is returned correctly
	originalResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("test\n")),
		Header:     http.Header{"Content-Type": []string{"text/plain"}},
	}

	_, _, returnedResp, err := ProcessResponseAsRingBufferToEnd(originalResp, 10)

	require.NoError(t, err)
	assert.Equal(t, originalResp, returnedResp)
	assert.Equal(t, http.StatusOK, returnedResp.StatusCode)
	assert.Equal(t, "text/plain", returnedResp.Header.Get("Content-Type"))
}

// errorReader is a custom reader that always returns an error
type errorReader struct {
	errorAfterBytes int
	bytesRead       int
}

func (er *errorReader) Read(p []byte) (n int, err error) {
	if er.bytesRead >= er.errorAfterBytes {
		return 0, io.ErrUnexpectedEOF
	}
	// Return some data first
	if len(p) > 0 {
		n = copy(p, []byte("test line\n"))
		er.bytesRead += n
		return n, nil
	}
	return 0, nil
}

func TestProcessResponseAsRingBufferToEnd_ScannerError(t *testing.T) {
	// Test that scanner errors are properly handled
	resp := &http.Response{
		Body: io.NopCloser(&errorReader{errorAfterBytes: 20}),
	}

	result, totalLines, returnedResp, err := ProcessResponseAsRingBufferToEnd(resp, 10)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read log content")
	assert.Equal(t, "", result)
	assert.Equal(t, 0, totalLines)
	assert.Equal(t, resp, returnedResp)
}
