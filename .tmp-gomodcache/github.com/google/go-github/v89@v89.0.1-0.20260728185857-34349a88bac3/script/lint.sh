#!/bin/sh
#/ [ CHECK_GITHUB_OPENAPI=1 ] script/lint.sh runs linters and validates generated files.
#/ When CHECK_GITHUB is set, it validates that openapi_operations.yaml is consistent with the
#/ descriptions from github.com/github/rest-api-description.

set -e

CUSTOM_GCL="$(script/setup-custom-gcl.sh)"

CDPATH="" cd -- "$(dirname -- "$0")/.."

EXIT_CODE=0

# Colors & Formatting
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
BOLD='\033[1m'
NC='\033[0m'

fail() {
  EXIT_CODE=1
}

MOD_DIRS="$(git ls-files '*go.mod' | xargs dirname | sort -u)"

# Number of module lint jobs to run concurrently.
# Override with LINT_JOBS, otherwise use detected CPU count.
: "${LINT_JOBS:=$(getconf _NPROCESSORS_ONLN 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4)}"

LINT_DIRS="$(printf '%s\n' "$MOD_DIRS")"

FAILED_COUNT=0
LINT_FAILED=0
RUNNING=0
PIDS=""
DIRS_IN_FLIGHT=""

LOG_DIR="$(mktemp -d)"
trap 'rm -rf "$LOG_DIR"' EXIT

# --- Helper Functions ---
print_header() {
  printf "${BOLD}%s${NC}\n\n" "$1"
}


wait_pids() {
  i=1
  for pid in $PIDS; do
    # Identify the directory for this PID
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
      fail
    fi
    i=$((i + 1))
  done
  PIDS=""
  DIRS_IN_FLIGHT=""
  RUNNING=0
}

print_header "Linting modules"

for dir in $LINT_DIRS; do
  log_file="$LOG_DIR/$(echo "$dir" | tr '/' '_').log"

  # Run the linter in the background and redirect output to a log file
  (cd "$dir" && "$CUSTOM_GCL" run --color always > "$log_file" 2>&1) &

  PIDS="$PIDS $!"
  DIRS_IN_FLIGHT="$DIRS_IN_FLIGHT $dir"
  RUNNING=$((RUNNING + 1))

  if [ "$RUNNING" -ge "$LINT_JOBS" ]; then
    wait_pids
  fi
done

wait_pids

if [ -n "$CHECK_GITHUB_OPENAPI" ]; then
  print_header "Validating openapi_operations.yaml"
  if script/metadata.sh update-openapi --validate; then
    printf "${GREEN}✔ openapi_operations.yaml is valid${NC}\n"
  else
    printf "${RED}✘ openapi_operations.yaml validation failed${NC}\n"
    fail
  fi
fi

print_header "Validating generated files"
if script/generate.sh --check; then
  printf "${GREEN}✔ Generated files are up to date${NC}\n"
else
  printf "${RED}✘ Generated files out of sync${NC}\n"
  fail
fi

# --- Final Summary ---
printf -- "----------------------------\n"
SUMMARY_COLOR="$GREEN"
if [ "$FAILED_COUNT" -gt 0 ]; then
  SUMMARY_COLOR="$RED"
fi

printf "%bLinting: issues found in %d module directories.%b\n" "$SUMMARY_COLOR" "$FAILED_COUNT" "$NC"
printf -- "--------------------------------------------\n"

exit "$EXIT_CODE"
