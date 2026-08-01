#!/bin/sh
#/ script/setup-custom-gcl.sh ensures custom golangci-lint is installed.
#/ It returns the path to the custom-gcl binary.

set -e

ROOT_DIR="$(cd -- "$(dirname -- "$0")/.." > /dev/null 2>&1 && pwd -P)"
CUSTOM_GCL_CONFIG="$ROOT_DIR/.custom-gcl.yml"
GOLANGCI_LINT_VERSION="$(
  sed -n 's/^[[:space:]]*version:[[:space:]]*//p' "$CUSTOM_GCL_CONFIG" \
  | sed '1{s/[[:space:]]*#.*$//;s/^"//;s/"$//;s/[[:space:]]*$//;p;};d'
)"

if [ -z "$GOLANGCI_LINT_VERSION" ]; then
  echo "Error: could not determine golangci-lint version from $CUSTOM_GCL_CONFIG" >&2
  exit 1
fi

BIN="$ROOT_DIR/bin"

mkdir -p "$BIN"

# install golangci-lint and custom-gcl in ./bin if they don't exist with the correct version
if ! "$BIN"/custom-gcl version --short 2> /dev/null | grep -q "$GOLANGCI_LINT_VERSION"; then
  curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b "$BIN" "$GOLANGCI_LINT_VERSION"
  "$BIN"/golangci-lint custom --name custom-gcl --destination "$BIN"
fi

echo "$BIN/custom-gcl"
