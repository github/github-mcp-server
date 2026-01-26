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

	for {
		n, err := httpResp.Body.Read(readBuf)
		if n > 0 {
			chunk := readBuf[:n]
			for len(chunk) > 0 {
				// Find the next newline in the chunk
				newlineIdx := bytes.IndexByte(chunk, '\n')

				if newlineIdx >= 0 {
					// Found a newline - complete the current line
					if !lineTruncated {
						remaining := maxLineSize - currentLine.Len()
						if remaining > newlineIdx {
							remaining = newlineIdx
						}
						if remaining > 0 {
							currentLine.Write(chunk[:remaining])
						}
						if currentLine.Len() >= maxLineSize {
							lineTruncated = true
						}
					}

					// Store the completed line
					line := currentLine.String()
					if lineTruncated {
						// Only keep first maxDisplayLength chars for truncated lines
						if len(line) > maxDisplayLength {
							line = line[:maxDisplayLength]
						}
						line += "... [TRUNCATED]"
					}
					lines[writeIndex] = line
					validLines[writeIndex] = true
					totalLines++
					writeIndex = (writeIndex + 1) % maxJobLogLines

					// Reset for next line
					currentLine.Reset()
					lineTruncated = false
					chunk = chunk[newlineIdx+1:]
				} else {
					// No newline in remaining chunk - accumulate if not truncated
					if !lineTruncated {
						remaining := maxLineSize - currentLine.Len()
						if remaining > len(chunk) {
							remaining = len(chunk)
						}
						if remaining > 0 {
							currentLine.Write(chunk[:remaining])
						}
						if currentLine.Len() >= maxLineSize {
							lineTruncated = true
						}
					}
					break
				}
			}
		}

		if err == io.EOF {
			// Handle final line without newline
			if currentLine.Len() > 0 {
				line := currentLine.String()
				if lineTruncated {
					if len(line) > maxDisplayLength {
						line = line[:maxDisplayLength]
					}
					line += "... [TRUNCATED]"
				}
				lines[writeIndex] = line
				validLines[writeIndex] = true
				totalLines++
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
