#!/bin/sh
#/ script/metadata.sh runs ./tools/metadata in the repository root with the given arguments

set -e

CDPATH="" cd -- "$(dirname -- "$0")/.."
REPO_DIR="$(pwd)"

(
  cd tools/metadata
  go build -o "$REPO_DIR"/bin/metadata
)

exec bin/metadata "$@"
