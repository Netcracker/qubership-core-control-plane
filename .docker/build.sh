#!/usr/bin/env bash
set -euo pipefail
mkdir -p /reports /out
public_module=/src/source/control-plane
cd "$public_module"
build_module="$public_module"
if [[ -n "$BOOTSTRAP_DIR" ]]; then
    cp -a "/build-context/$BOOTSTRAP_DIR" /src/private
    build_module=/src/private
    cd "$build_module"
    go mod download
    go test -mod=readonly -short -v -count=1 -timeout=15m \
        -coverprofile=/reports/bootstrap-coverage.out ./... 2>&1 | tee /reports/bootstrap-unit-tests.log
    mkdir -p /reports/bootstrap-module
    cp go.mod go.sum /reports/bootstrap-module/
fi
cd "$build_module"
GOOS="$TARGETOS" GOARCH="$TARGETARCH" go build -mod=readonly -o /out/control-plane .
go version -m /out/control-plane > /reports/build-info.txt
cp "$public_module/application.yaml" "$public_module/docs/swagger.json" "$public_module/docs/swagger.yaml" /out/
if [[ -n "$BOOTSTRAP_DIR" ]]; then
    cp "$build_module/application.yaml" "$build_module/policies.conf" "$build_module/api-version-info.json" /out/
fi
chmod 555 /out/control-plane
find /out -type f ! -name control-plane -exec chmod 444 {} +
