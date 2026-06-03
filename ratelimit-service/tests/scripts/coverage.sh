#!/usr/bin/env bash
# tests/scripts/coverage.sh
#
# Runs go test with coverage, prints summary, and fails if total coverage
# falls below the threshold.
#
# Configuration via ENV:
#   COVERAGE_THRESHOLD — required percentage (default: 60).
#   COVERAGE_OUT       — output profile file (default: coverage.out).
#   COVERAGE_HTML      — output HTML report (default: coverage.html).

set -euo pipefail

THRESHOLD="${COVERAGE_THRESHOLD:-35}"
OUT="${COVERAGE_OUT:-coverage.out}"
HTML="${COVERAGE_HTML:-coverage.html}"

# Exclude packages with no test files (e.g. pkg/utils) — they trigger
# "go: no such tool covdata" on Go 1.20+ when covdata is not in PATH.
PKGS=$(go list ./pkg/... | grep -v '/utils$')

# shellcheck disable=SC2086
go test -coverprofile="$OUT" -covermode=atomic -short $PKGS

echo ""
echo "=== Per-package coverage ==="
go tool cover -func="$OUT" | tail -20

TOTAL=$(go tool cover -func="$OUT" | tail -1 | awk '{print $NF}' | sed 's/%//')

go tool cover -html="$OUT" -o "$HTML"

echo ""
echo "=== Total: ${TOTAL}% (threshold: ${THRESHOLD}%) ==="
echo "HTML report written to: $HTML"

# bash's (( )) cannot compare floats, so use awk.
PASS=$(awk -v t="$TOTAL" -v th="$THRESHOLD" 'BEGIN { print (t+0 >= th+0) }')
if [ "$PASS" != "1" ]; then
    echo "FAIL: coverage ${TOTAL}% is below threshold ${THRESHOLD}%"
    exit 1
fi
echo "PASS"
