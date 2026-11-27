package buffer

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// MaxLogBytes bounds retained log content, including truncation markers.
const MaxLogBytes = 1 << 20 // 1 MiB

const (
	maxLineSize      = 16 << 10 // 16 KiB per line
	truncationMarker = "... [TRUNCATED]"
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
// Lines exceeding maxLineSize are truncated with a marker. Oldest lines are
// also evicted when necessary to keep the output within MaxLogBytes.
func ProcessResponseAsRingBufferToEnd(httpResp *http.Response, maxJobLogLines int) (string, int, *http.Response, error) {
	if maxJobLogLines <= 0 {
		maxJobLogLines = 500
	}
	if maxJobLogLines > 100000 {
		maxJobLogLines = 100000
	}

	lines := make([]string, maxJobLogLines)
	totalLines := 0
	first, count, retainedBytes := 0, 0, 0
	byteTruncated := false
	// Reserve room for the marker and its newline if the byte budget evicts lines.
	contentBudget := MaxLogBytes - len(truncationMarker) - 1
	evict := func() {
		retainedBytes -= len(lines[first]) + 1
		lines[first] = ""
		first = (first + 1) % maxJobLogLines
		count--
	}

	const readBufferSize = 64 * 1024 // 64KB read buffer

	readBuf := make([]byte, readBufferSize)
	var currentLine strings.Builder
	lineTruncated := false

	// storeLine saves the current line to the ring buffer and resets state
	storeLine := func() {
		line := currentLine.String()
		if lineTruncated {
			line = strings.ToValidUTF8(line, "") + truncationMarker
		}
		if count == maxJobLogLines {
			evict()
		}
		for count > 0 && retainedBytes+len(line)+1 > contentBudget {
			evict()
			byteTruncated = true
		}
		lines[(first+count)%maxJobLogLines] = line
		count++
		retainedBytes += len(line) + 1
		totalLines++
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

	var result strings.Builder
	result.Grow(retainedBytes + len(truncationMarker) + 1)
	if byteTruncated {
		result.WriteString(truncationMarker)
		result.WriteByte('\n')
	}
	for i := range count {
		if i > 0 {
			result.WriteByte('\n')
		}
		result.WriteString(lines[(first+i)%maxJobLogLines])
	}

	return result.String(), totalLines, httpResp, nil
}
