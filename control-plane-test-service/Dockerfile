FROM golang:1.27.1@sha256:e0174e51e81218523251d85d248a90d24c3d5e81543b4f07a5d66229397db190 AS build

WORKDIR /app

COPY control-plane-test-service/ .

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on GOSUMDB=off GOTOOLCHAIN=local go build -v -o control-plane-test-service .

FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

RUN apk add --no-cache curl && apk add --no-cache openssl

#RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2
COPY --chmod=777  --from=build app/control-plane-test-service /app/control-plane-test-service
COPY --from=build app/application.yaml /app/
COPY --from=build app/localhost.key /app/

RUN openssl req -new -x509 -nodes -sha256 -days 365 -key /app/localhost.key -out /app/localhost.crt -subj "/C=US/ST=State/L=City/O=Qubership/CN=TestCert"

RUN openssl req -new -x509 -nodes -sha256 -days 365 -key /app/localhost.key -out /app/localhostclient.crt -subj "/C=US/ST=State/L=City/O=Qubership/CN=TestCert"

WORKDIR /app
ENTRYPOINT /app/control-plane-test-service
