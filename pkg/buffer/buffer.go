package buffer

import (
	"bytes"
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

	const readBufferSize = 64 * 1024 // 64KB read buffer
	const maxDisplayLength = 1000    // Keep first 1000 chars of truncated lines

	readBuf := make([]byte, readBufferSize)
	var currentLine strings.Builder
	lineTruncated := false

	// storeLine saves the current line to the ring buffer and resets state
	storeLine := func() {
		line := currentLine.String()
		if lineTruncated && len(line) > maxDisplayLength {
			line = line[:maxDisplayLength]
		}
		if lineTruncated {
			line += "... [TRUNCATED]"
		}
		lines[writeIndex] = line
		validLines[writeIndex] = true
		totalLines++
		writeIndex = (writeIndex + 1) % maxJobLogLines
		currentLine.Reset()
		lineTruncated = false
	}

	// accumulate adds bytes to currentLine up to maxLineSize, sets lineTruncated if exceeded
	accumulate := func(data []byte) {
		if lineTruncated {
			return
		}
		remaining := maxLineSize - currentLine.Len()
		if remaining <= 0 {
			lineTruncated = true
			return
		}
		if remaining > len(data) {
			remaining = len(data)
		}
		currentLine.Write(data[:remaining])
		if currentLine.Len() >= maxLineSize {
			lineTruncated = true
		}
	}

	for {
		n, err := httpResp.Body.Read(readBuf)
		if n > 0 {
			chunk := readBuf[:n]
			for len(chunk) > 0 {
				newlineIdx := bytes.IndexByte(chunk, '\n')
				if newlineIdx < 0 {
					accumulate(chunk)
					break
				}
				accumulate(chunk[:newlineIdx])
				storeLine()
				chunk = chunk[newlineIdx+1:]
			}
		}

		if err == io.EOF {
			if currentLine.Len() > 0 {
				storeLine()
			}
			break
		}
		if err != nil {
			return "", 0, httpResp, fmt.Errorf("failed to read log content: %w", err)
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

	return strings.Join(result, "\n"), totalLines, httpResp, nil
}
