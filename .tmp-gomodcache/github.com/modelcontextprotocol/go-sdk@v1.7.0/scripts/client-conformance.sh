#!/bin/bash
# Copyright 2025 The Go MCP SDK Authors. All rights reserved.
# Use of this source code is governed by an MIT-style
# license that can be found in the LICENSE file.

# Run MCP conformance tests against the Go SDK conformance client.

set -e

RESULT_DIR=""
WORKDIR=""
CONFORMANCE_REPO=""
SUITE="core"
FINAL_EXIT_CODE=0

usage() {
    echo "Usage: $0 [options]"
    echo ""
    echo "Run MCP conformance tests against the Go SDK conformance client."
    echo ""
    echo "Options:"
    echo "  --result_dir <dir>       Save results to the specified directory"
    echo "  --conformance_repo <dir> Run conformance tests from a local checkout"
    echo "                           instead of using the latest npm release"
    echo "  --suite <name>           Which suite to run (default: core)"
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
        --suite)
            SUITE="$2"
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

# Set up the work directory.
if [ -n "$RESULT_DIR" ]; then
    mkdir -p "$RESULT_DIR"
    WORKDIR="$RESULT_DIR"
else
    WORKDIR=$(mktemp -d)
fi

# Build the conformance server.
go build -o "$WORKDIR/conformance-client" ./conformance/everything-client

# Run conformance tests from the work directory to avoid writing results to the repo.
echo "Running conformance tests..."
if [ -n "$CONFORMANCE_REPO" ]; then
    # Run from local checkout using npm run start.
    (cd "$WORKDIR" && \
        npm --prefix "$CONFORMANCE_REPO" run start -- \
            client --command "$WORKDIR/conformance-client" \
            --suite "$SUITE" \
            ${RESULT_DIR:+--output-dir "$RESULT_DIR"}) || FINAL_EXIT_CODE=$?
else
    (cd "$WORKDIR" && \
        npx @modelcontextprotocol/conformance@latest \
        client --command "$WORKDIR/conformance-client" \
        --suite "$SUITE" \
        ${RESULT_DIR:+--output-dir "$RESULT_DIR"}) || FINAL_EXIT_CODE=$?
fi

echo ""
if [ -n "$RESULT_DIR" ]; then
    echo "See $RESULT_DIR for details."
else
    echo "Run with --result_dir to save results."
fi

exit $FINAL_EXIT_CODE
