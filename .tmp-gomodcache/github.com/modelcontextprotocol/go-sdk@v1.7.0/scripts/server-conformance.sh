#!/bin/bash
# Copyright 2025 The Go MCP SDK Authors. All rights reserved.
# Use of this source code is governed by an MIT-style
# license that can be found in the LICENSE file.

# Run MCP conformance tests against the Go SDK conformance server.

set -e

PORT="${PORT:-3000}"
SERVER_PID=""
RESULT_DIR=""
WORKDIR=""
CONFORMANCE_REPO=""
FINAL_EXIT_CODE=0

usage() {
    echo "Usage: $0 [options]"
    echo ""
    echo "Run MCP conformance tests against the Go SDK conformance server."
    echo ""
    echo "Options:"
    echo "  --result_dir <dir>       Save results to the specified directory"
    echo "  --conformance_repo <dir> Run conformance tests from a local checkout"
    echo "                           instead of using the latest npm release"
    echo "  --help                   Show this help message"
}

# Parse arguments.
while [[ $# -gt 0 ]]; do
    case $1 in
        --result_dir)
            RESULT_DIR="$2"
            shift 2
            ;;
        --conformance_repo)
            CONFORMANCE_REPO="$2"
            shift 2
            ;;
        --help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

cleanup() {
    if [ -n "$SERVER_PID" ]; then
        echo "Stopping server..."
        kill "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

# Set up the work directory.
if [ -n "$RESULT_DIR" ]; then
    mkdir -p "$RESULT_DIR"
    WORKDIR="$RESULT_DIR"
else
    WORKDIR=$(mktemp -d)
fi

# Build the conformance server.
go build -o "$WORKDIR/conformance-server" ./conformance/everything-server

# Start the server in the background.
# -stateless=false pins the server to the stateful transport so that
# server-initiated sampling/elicitation scenarios (which are not supported on
# stateless streamable HTTP) work against the current @latest conformance
# suite. Drop this flag once the 0.2.x line is promoted to the @latest
# dist-tag on npm and the stateless leg becomes viable.
echo "Starting conformance server on localhost:$PORT..."
"$WORKDIR/conformance-server" -http="localhost:$PORT" -stateless=false &
SERVER_PID=$!

echo "Server pid is $SERVER_PID"

# Wait for server to be ready
echo "Waiting for server to be ready..."
if ! timeout 15 bash -c "until curl -s http://localhost:$PORT > /dev/null 2>&1; do sleep 0.5; done"; then
    echo "Server failed to start within 15 seconds."
    exit 1
fi

# Run conformance tests from the work directory to avoid writing results to the repo.
echo "Running conformance tests..."
if [ -n "$CONFORMANCE_REPO" ]; then
    # Run from local checkout using npm run start.
    (cd "$WORKDIR" && \
        npm --prefix "$CONFORMANCE_REPO" run start -- \
            server --url "http://localhost:$PORT" \
            --spec-version 2025-11-25 \
            ${RESULT_DIR:+--output-dir "$RESULT_DIR"}) || FINAL_EXIT_CODE=$?
else
    (cd "$WORKDIR" && \
        npx @modelcontextprotocol/conformance@latest \
        server --url "http://localhost:$PORT" \
        --spec-version 2025-11-25 \
        ${RESULT_DIR:+--output-dir "$RESULT_DIR"}) || FINAL_EXIT_CODE=$?
fi

echo ""
if [ -n "$RESULT_DIR" ]; then
    echo "See $RESULT_DIR for details."
else
    echo "Run with --result_dir to save results."
fi

exit $FINAL_EXIT_CODE
