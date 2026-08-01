#!/bin/sh
#/ `script/generate.sh` runs `go generate` on repo.
#/ It also runs `script/run-check-structfield-settings.sh -fix` to keep linter
#/ exceptions in `.golangci.yml` up to date.
#/ `script/generate.sh --check` checks that the generated files are up to date.

set -e

CDPATH="" cd -- "$(dirname -- "$0")/.."

CHECK_MODE=0
if [ "$1" = "--check" ]; then
  export CHECK=1
  CHECK_MODE=1
fi

go generate ./...

MOD_DIRS="$(git ls-files '*go.mod' | xargs dirname | sort)"

for dir in $MOD_DIRS; do
  (
    cd "$dir"
    if [ "$CHECK_MODE" = "1" ]; then
      if ! go mod tidy -diff; then
        echo "go.mod/go.sum are out of date in $dir"
        exit 1
      fi
    else
      go mod tidy
    fi
  )
done

script/run-check-structfield-settings.sh -fix
