#!/bin/sh
#/ script/test.sh runs tests on each go module in go-github in parallel.
#/ Arguments are passed to each go test invocation.
#/ "-race -covermode atomic ./..." is used when no arguments are given.
#/
#/ When UPDATE_GOLDEN is set, all directories named "golden" are removed before running tests.
#/
#/ TEST_JOBS can be set to control parallelism (defaults to detected CPU count).

set -e

CDPATH="" cd -- "$(dirname -- "$0")/.."

if [ "$#" = "0" ]; then
  set -- -race -covermode atomic ./...
fi

if [ -n "$UPDATE_GOLDEN" ]; then
  find . -name golden -type d -exec rm -rf {} +
fi

MOD_DIRS="$(git ls-files '*go.mod' | xargs dirname | sort -u)"

GREEN='\033[0;32m'
RED='\033[0;31m'
BOLD='\033[1m'
NC='\033[0m'

EXIT_CODE=0
FAILED_COUNT=0
RUNNING=0
PIDS=""
DIRS_IN_FLIGHT=""

LOG_DIR="$(mktemp -d)"
trap 'rm -rf "$LOG_DIR"' EXIT

: "${TEST_JOBS:=$(getconf _NPROCESSORS_ONLN 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4)}"

print_header() {
  printf "${BOLD}%s${NC}\n\n" "$1"
}

wait_pids() {
  i=1
  for pid in $PIDS; do
    dir=$(echo "$DIRS_IN_FLIGHT" | awk -v i="$i" '{print $i}')
    log_file="$LOG_DIR/$(echo "$dir" | tr '/' '_').log"

    if wait "$pid"; then
      printf "${GREEN}✔ %-40s [ PASS ]${NC}\n" "$dir"
    else
      printf "${RED}✘ %-40s [ FAIL ]${NC}\n" "$dir"
      if [ -f "$log_file" ]; then
        sed 's/^/    /' "$log_file"
      fi
      FAILED_COUNT=$((FAILED_COUNT + 1))
      EXIT_CODE=1
    fi
    i=$((i + 1))
  done
  PIDS=""
  DIRS_IN_FLIGHT=""
  RUNNING=0
}

print_header "Testing modules"

for dir in $MOD_DIRS; do
  log_file="$LOG_DIR/$(echo "$dir" | tr '/' '_').log"

  (cd "$dir" && go test "$@" > "$log_file" 2>&1) &

  PIDS="$PIDS $!"
  DIRS_IN_FLIGHT="$DIRS_IN_FLIGHT $dir"
  RUNNING=$((RUNNING + 1))

  if [ "$RUNNING" -ge "$TEST_JOBS" ]; then
    wait_pids
  fi
done

wait_pids

printf -- "----------------------------\n"
SUMMARY_COLOR="$GREEN"
if [ "$FAILED_COUNT" -gt 0 ]; then
  SUMMARY_COLOR="$RED"
fi

printf "%bTesting: issues found in %d module directories.%b\n" "$SUMMARY_COLOR" "$FAILED_COUNT" "$NC"
printf -- "--------------------------------------------\n"

exit "$EXIT_CODE"
