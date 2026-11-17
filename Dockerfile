# syntax=docker/dockerfile:1

ARG GO_BUILDER_IMAGE=golang:1.27.3@sha256:b76020929b709ddb6202afc05410a0faefae20ae05e27ad548b3870092688a69
ARG RUNTIME_IMAGE=ghcr.io/netcracker/qubership-core-base:alpine-2.5.9@sha256:ab860790e4ecce56f8df673495cd5745ce587fffa7d7eb210d6d098563ccb26d
FROM --platform=$BUILDPLATFORM ${GO_BUILDER_IMAGE} AS build
ARG TARGETOS
ARG TARGETARCH
ARG SOURCE_DIR=.
# Private entrypoints are included only when explicitly supplied by the caller.
ARG BOOTSTRAP_DIR=
ARG GOPROXY=https://proxy.golang.org,direct
ARG GOSUMDB=sum.golang.org
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GONOPROXY=none \
    GOMAXPROCS=2 GOMEMLIMIT=1GiB GOFLAGS=-p=2
WORKDIR /src/source
COPY ${SOURCE_DIR}/ ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,id=control-plane-go-build,target=/root/.cache/go-build \
    bash .docker/test.sh
RUN --mount=type=bind,target=/build-context \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,id=control-plane-go-build,target=/root/.cache/go-build \
    bash .docker/build.sh

FROM scratch AS artifact
COPY --from=build /reports/ /test-results/

FROM ${RUNTIME_IMAGE} AS run
WORKDIR /app
COPY --from=build --chown=10001:0 /out/ /app/
USER 10001
EXPOSE 8080 8443 15010
CMD ["/app/control-plane"]
