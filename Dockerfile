FROM golang:1.25 AS build

WORKDIR /app

COPY control-plane/ .

RUN go mod download
RUN go build -o control-plane .

FROM ghcr.io/netcracker/qubership/core-base:1.2.0 AS run

COPY --chown=10001:0 --chmod=555 --from=build app/control-plane /app/control-plane
COPY --chown=10001:0 --chmod=444 --from=build app/application.yaml /app/
COPY --chown=10001:0 --chmod=444 --from=build app/docs/swagger.json /app/
COPY --chown=10001:0 --chmod=444 --from=build app/docs/swagger.yaml /app/

WORKDIR /app

USER 10001:10001

CMD ["/app/control-plane"]