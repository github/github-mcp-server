package github

import (
	"context"
	"encoding/json"
	"io"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// CompletionAwareStdioServer wraps the MCP stdio server to add completion support
type CompletionAwareStdioServer struct {
	baseServer        *server.MCPServer
	completionHandler CompletionHandlerFunc
	errLogger         *log.Logger
}

// NewCompletionAwareStdioServer creates a new stdio server with completion support
func NewCompletionAwareStdioServer(mcpServer *server.MCPServer, completionHandler CompletionHandlerFunc) *CompletionAwareStdioServer {
	return &CompletionAwareStdioServer{
		baseServer:        mcpServer,
		completionHandler: completionHandler,
		errLogger:         log.New(io.Discard, "", 0), // Default to discarding errors
	}
}

// SetErrorLogger sets the error logger for the server
func (s *CompletionAwareStdioServer) SetErrorLogger(logger *log.Logger) {
	s.errLogger = logger
}

// Listen starts the completion-aware stdio server
func (s *CompletionAwareStdioServer) Listen(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	// Use the simplified approach: create a custom stdio server that mimics the real one
	// but intercepts completion requests
	
	// We'll use the real stdio server from the mcp-go library and intercept the raw messages
	realStdioServer := server.NewStdioServer(s.baseServer)
	realStdioServer.SetErrorLogger(s.errLogger)
	
	// Create pipes to intercept messages
	stdinPipe := &completionInterceptReader{
		original:          stdin,
		completionHandler: s.completionHandler,
		baseServer:        s.baseServer,
		stdout:            stdout,
		ctx:               ctx,
		errLogger:         s.errLogger,
	}
	
	return realStdioServer.Listen(ctx, stdinPipe, stdout)
}

// completionInterceptReader intercepts stdin to handle completion requests
type completionInterceptReader struct {
	original          io.Reader
	completionHandler CompletionHandlerFunc
	baseServer        *server.MCPServer
	stdout            io.Writer
	ctx               context.Context
	errLogger         *log.Logger
	buffer            []byte
	bufferPos         int
}

func (r *completionInterceptReader) Read(p []byte) (n int, err error) {
	// If we have buffered data, return that first
	if r.bufferPos < len(r.buffer) {
		n = copy(p, r.buffer[r.bufferPos:])
		r.bufferPos += n
		if r.bufferPos >= len(r.buffer) {
			r.buffer = nil
			r.bufferPos = 0
		}
		return n, nil
	}

	// Read from original source
	n, err = r.original.Read(p)
	if err != nil {
		return n, err
	}

	// Check if this contains a completion request
	data := p[:n]
	if r.isCompletionRequest(data) {
		// Handle completion request directly
		response := r.handleCompletionRequest(data)
		if response != nil {
			// Write response to stdout
			encoder := json.NewEncoder(r.stdout)
			if encErr := encoder.Encode(response); encErr != nil {
				r.errLogger.Printf("Error writing completion response: %v", encErr)
			}
		}
		// Return EOF to the real server so it doesn't process this message
		return 0, io.EOF
	}

	return n, err
}

// isCompletionRequest checks if the data contains a completion request
func (r *completionInterceptReader) isCompletionRequest(data []byte) bool {
	var baseMessage struct {
		Method string `json:"method"`
	}

	if err := json.Unmarshal(data, &baseMessage); err != nil {
		return false
	}

	return baseMessage.Method == "completion/complete"
}

// handleCompletionRequest processes completion requests
func (r *completionInterceptReader) handleCompletionRequest(data []byte) mcp.JSONRPCMessage {
	var baseMessage struct {
		JSONRPC string `json:"jsonrpc"`
		ID      any    `json:"id"`
		Method  string `json:"method"`
	}

	if err := json.Unmarshal(data, &baseMessage); err != nil {
		return createErrorResponse(baseMessage.ID, mcp.PARSE_ERROR, "Failed to parse completion request")
	}

	var request mcp.CompleteRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return createErrorResponse(baseMessage.ID, mcp.INVALID_REQUEST, "Failed to parse completion request")
	}

	result, err := r.completionHandler(r.ctx, request)
	if err != nil {
		return createErrorResponse(baseMessage.ID, mcp.INTERNAL_ERROR, err.Error())
	}

	return createResponse(baseMessage.ID, *result)
}