#!/usr/bin/env bash
set -euo pipefail
mkdir -p /reports
cd /src/source/control-plane
go mod download
# The root package contains gateway integration tests; -short runs the unit suite.
go test -mod=readonly -short -v -count=1 -timeout=15m \
    -coverprofile=/reports/public-coverage.out ./... 2>&1 | tee /reports/public-unit-tests.log
