FROM --platform=$BUILDPLATFORM golang:1.26 AS build

WORKDIR /app

COPY control-plane/ .

RUN go mod download
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o control-plane .

FROM ghcr.io/netcracker/qubership-core-base:2.2.13 AS run

COPY --chown=10001:0 --chmod=555 --from=build app/control-plane /app/control-plane
COPY --chown=10001:0 --chmod=444 --from=build app/application.yaml /app/
COPY --chown=10001:0 --chmod=444 --from=build app/docs/swagger.json /app/
COPY --chown=10001:0 --chmod=444 --from=build app/docs/swagger.yaml /app/

WORKDIR /app

USER 10001:10001

CMD ["/app/control-plane"]
