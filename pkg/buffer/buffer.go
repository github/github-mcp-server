package buffer

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// maxLineSize is the maximum size for a single log line (10MB).
// GitHub Actions logs can contain extremely long lines (base64 content, minified JS, etc.)
const maxLineSize = 10 * 1024 * 1024

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
// Lines exceeding maxLineSize are truncated with a marker.
func ProcessResponseAsRingBufferToEnd(httpResp *http.Response, maxJobLogLines int) (string, int, *http.Response, error) {
	if maxJobLogLines > 100000 {
		maxJobLogLines = 100000
	}

	lines := make([]string, maxJobLogLines)
	validLines := make([]bool, maxJobLogLines)
	totalLines := 0
	writeIndex := 0

	scanner := bufio.NewScanner(httpResp.Body)
	// Set initial buffer to 64KB and max token size to 10MB to handle very long lines
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	for scanner.Scan() {
		line := scanner.Text()
		totalLines++

		lines[writeIndex] = line
		validLines[writeIndex] = true
		writeIndex = (writeIndex + 1) % maxJobLogLines
	}

	if err := scanner.Err(); err != nil {
		// If we hit a token too long error, fall back to byte-by-byte reading
		// with line truncation to handle extremely long lines gracefully
		if err == bufio.ErrTooLong {
			return processWithLongLineHandling(httpResp.Body, lines, validLines, totalLines, writeIndex, maxJobLogLines)
		}
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

// processWithLongLineHandling continues processing after encountering a line
// that exceeds the scanner's max token size. It reads byte-by-byte and
// truncates extremely long lines instead of failing.
func processWithLongLineHandling(body io.Reader, lines []string, validLines []bool, totalLines, writeIndex, maxJobLogLines int) (string, int, *http.Response, error) {
	// Add a marker that we encountered truncated content
	truncatedMarker := "[LINE TRUNCATED - exceeded maximum line length of 10MB]"
	lines[writeIndex] = truncatedMarker
	validLines[writeIndex] = true
	totalLines++
	writeIndex = (writeIndex + 1) % maxJobLogLines

	// Continue reading with a buffered reader, truncating long lines
	reader := bufio.NewReader(body)
	var currentLine strings.Builder
	const maxDisplayLength = 1000 // Keep first 1000 chars of truncated lines

	for {
		b, err := reader.ReadByte()
		if err == io.EOF {
			// Handle final line without newline
			if currentLine.Len() > 0 {
				line := currentLine.String()
				if len(line) > maxLineSize {
					line = line[:maxDisplayLength] + "... [TRUNCATED]"
				}
				lines[writeIndex] = line
				validLines[writeIndex] = true
				totalLines++
			}
			break
		}
		if err != nil {
			return "", 0, nil, fmt.Errorf("failed to read log content: %w", err)
		}

		if b == '\n' {
			line := currentLine.String()
			if len(line) > maxLineSize {
				line = line[:maxDisplayLength] + "... [TRUNCATED]"
			}
			lines[writeIndex] = line
			validLines[writeIndex] = true
			totalLines++
			writeIndex = (writeIndex + 1) % maxJobLogLines
			currentLine.Reset()
		} else if currentLine.Len() < maxLineSize+maxDisplayLength {
			// Stop accumulating bytes once we exceed the limit (plus buffer for truncation message)
			currentLine.WriteByte(b)
		}
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

	return strings.Join(result, "\n"), totalLines, nil, nil
}
