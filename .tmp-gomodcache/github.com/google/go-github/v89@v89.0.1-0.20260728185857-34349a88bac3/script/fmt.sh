#!/bin/sh
#/ script/fmt.sh runs formatting on all Go files in the project.
#/ It uses custom golangci-lint to format the code.

set -e

CUSTOM_GCL="$(script/setup-custom-gcl.sh)"

CDPATH="" cd -- "$(dirname -- "$0")/.."

MOD_DIRS="$(git ls-files '*go.mod' | xargs dirname | sort)"

for dir in $MOD_DIRS; do
  (
    cd "$dir"
    "$CUSTOM_GCL" fmt
  )
done
