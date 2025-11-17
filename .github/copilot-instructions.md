# Copilot Onboarding Instructions for MCP Server Go Project

## Overview

This repository contains the MCP Server, implemented in Go. The MCP Server is responsible for managing and processing MCP (Message Control Protocol) requests, providing a backend service for clients to interact with MCP resources. It exposes a RESTful API and handles authentication, authorization, and data persistence.

- **Language:** Go (Golang)
- **Frameworks/Libraries:** Standard Go libraries, plus any listed in `go.mod`
- **Project Type:** Backend server/service
- **Repo Size:** Medium (typically <100 files, mostly Go source)

## Build Instructions

1. **Prerequisites**
   - Go version >= 1.19 (check `go.mod` for minimum required version)
   - Git
   - (Optional) Docker, if you wish to run the server in a container

2. **Bootstrap**
   - Clone the repository:  
     ```
     git clone <repo-url>
     cd <repo-directory>
     ```
   - Ensure Go modules are enabled:  
     ```
     export GO111MODULE=on
     ```
   - Download dependencies:  
     ```
     go mod tidy
     ```

3. **Build**
   - Build the server binary:  
     ```
     go build -o mcp-server ./cmd/server
     ```
   - The output binary will be `mcp-server` in the repo root.

4. **Run**
   - Start the server:  
     ```
     ./mcp-server
     ```
   - Configuration may be provided via environment variables or config files (see README or `cmd/server/main.go` for details).

5. **Lint**
   - Run linter (if configured):  
     ```
     golangci-lint run
     ```
   - Ensure `golangci-lint` is installed (`go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`)

6. **Test**
   - Run unit and integration tests:  
     ```
     go test ./...
     ```
   - For coverage:  
     ```
     go test -cover ./...
     ```

## Project Structure

- `cmd/server/main.go`: Entry point for the MCP server.
- `internal/`: Contains core server logic, handlers, and business logic.
- `pkg/`: Shared packages/utilities.
- `go.mod`, `go.sum`: Go module dependencies.
- `README.md`: Project documentation.
- `.github/workflows/`: CI/CD pipeline definitions.
- `configs/`: Configuration files (if present).
- `test/` or `*_test.go` files: Unit and integration tests.

## CI/CD and Validation

- **GitHub Actions**:  
  - Located in `.github/workflows/`
  - Typical workflow steps:
    - Checkout code
    - Set up Go environment
    - Run `go mod tidy`
    - Run `go build`
    - Run `go test ./...`
    - Optionally run linter
  - All pushes and pull requests trigger the workflow.
  - PRs must pass all checks before merging.

- **Validation Steps**:
  - Always run `go mod tidy` before building or testing.
  - Ensure all tests pass locally before pushing changes.
  - Lint code before submitting PRs.
  - Check for any required environment variables or config files in the README.

## Additional Notes

- If you encounter build or test failures, check for missing dependencies or incorrect Go version.
- For Docker usage, see any provided `Dockerfile` in the repo root or `build/` directory.
- For more details, refer to `README.md` and comments in `main.go`.

---

**Trust these instructions for onboarding and development. Only perform additional searches if information here is incomplete or found to be in error.**
