#!/bin/bash
# Script to complete the go-github-mock to testify migration
# Usage: ./complete_migration.sh

set -e

echo "=== Completing go-github-mock Migration ==="
echo ""
echo "This script will:"
echo "1. Migrate remaining 6 test files"
echo "2. Remove go-github-mock dependency"
echo "3. Remove raw_mock.go"
echo "4. Run tests and linter"
echo ""

# Files to migrate
FILES=(
    "pkg/github/notifications_test.go"
    "pkg/github/search_test.go"
    "pkg/github/projects_test.go"
    "pkg/github/pullrequests_test.go"
    "pkg/github/repositories_test.go"
    "pkg/github/issues_test.go"
)

echo "Files to migrate:"
for f in "${FILES[@]}"; do
    lines=$(wc -l < "$f")
    echo "  - $f ($lines lines)"
done
echo ""

read -p "Continue with migration? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Migration cancelled"
    exit 1
fi

# Backup files
echo "Creating backups..."
for f in "${FILES[@]}"; do
    cp "$f" "$f.backup"
done

# Migration function
migrate_file() {
    local file=$1
    echo "Migrating $file..."
    
    # Remove mock import
    sed -i '/github\.com\/migueleliasweb\/go-github-mock\/src\/mock/d' "$file"
    
    # Fix ID naming
    sed -i 's/ThreadId/ThreadID/g; s/GistId/GistID/g; s/GhsaId/GhsaID/g' "$file"
    sed -i 's/WorkflowId/WorkflowID/g; s/RunId/RunID/g; s/JobId/JobID/g' "$file"
    
    # Replace empty mocks
    sed -i 's/mock\.NewMockedHTTPClient()/MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})/g' "$file"
    
    echo "  Basic patterns applied. Manual review needed for:"
    echo "  - mock.WithRequestMatch patterns"
    echo "  - mock.WithRequestMatchHandler patterns"
    echo "  - Closing braces for maps"
}

# Migrate each file
for f in "${FILES[@]}"; do
    migrate_file "$f"
done

echo ""
echo "=== Migration Step 1 Complete ==="
echo ""
echo "NEXT STEPS (Manual):"
echo "1. For each file, replace mock.NewMockedHTTPClient patterns:"
echo "   - Find: mock.NewMockedHTTPClient(mock.WithRequestMatch(mock.GetX, data),)"
echo "   - Replace with: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{GetX: mockResponse(t, http.StatusOK, data),})"
echo ""
echo "2. For mock.WithRequestMatchHandler patterns:"
echo "   - Find: mock.NewMockedHTTPClient(mock.WithRequestMatchHandler(mock.GetX, handler),)"
echo "   - Replace with: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{GetX: handler,})"
echo ""
echo "3. Test each file: go test ./pkg/github -run TestName -v"
echo ""
echo "4. When all files pass tests:"
echo "   - Remove: go.mod entry for migueleliasweb/go-github-mock"
echo "   - Remove: pkg/raw/raw_mock.go"
echo "   - Run: go mod tidy"
echo "   - Run: script/licenses"
echo "   - Run: script/test"
echo "   - Run: script/lint"
echo ""
echo "Backups saved as *.backup in case you need to revert."
