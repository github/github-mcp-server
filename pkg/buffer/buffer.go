package buffer

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"
)

// ProcessResponseAsRingBufferToEnd reads the body of an HTTP response line by line,
// storing only the last maxJobLogLines lines using a ring buffer (sliding window).
// This efficiently retains the most recent lines, overwriting older ones as needed.
//
// Parameters:
//
//	httpResp:        The HTTP response whose body will be read.
//	maxJobLogLines:  The maximum number of log lines to retain.
//
// Returns:
//
//	string:          The concatenated log lines (up to maxJobLogLines), separated by newlines.
//	int:             The total number of lines read from the response.
//	*http.Response:  The original HTTP response.
//	error:           Any error encountered during reading.
//
// The function uses a ring buffer to efficiently store only the last maxJobLogLines lines.
// If the response contains more lines than maxJobLogLines, only the most recent lines are kept.
func ProcessResponseAsRingBufferToEnd(httpResp *http.Response, maxJobLogLines int) (string, int, *http.Response, error) {
	if maxJobLogLines > 100000 {
		maxJobLogLines = 100000
	}

	lines := make([]string, maxJobLogLines)
	validLines := make([]bool, maxJobLogLines)
	totalLines := 0
	writeIndex := 0

	scanner := bufio.NewScanner(httpResp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		totalLines++

		lines[writeIndex] = line
		validLines[writeIndex] = true
		writeIndex = (writeIndex + 1) % maxJobLogLines
	}

	if err := scanner.Err(); err != nil {
		return "", 0, httpResp, fmt.Errorf("failed to read log content: %w", err)
	}

	var result []string
	linesInBuffer := totalLines
	if linesInBuffer > maxJobLogLines {
		linesInBuffer = maxJobLogLines
	}

	startIndex := 0
	if totalLines > maxJobLogLines {
		startIndex = writeIndex
	}

	for i := 0; i < linesInBuffer; i++ {
		idx := (startIndex + i) % maxJobLogLines
		if validLines[idx] {
			result = append(result, lines[idx])
		}
	}

	return strings.Join(result, "\n"), totalLines, httpResp, nil
}
