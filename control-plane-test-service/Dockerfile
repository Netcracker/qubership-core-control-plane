FROM golang:1.24 AS build

WORKDIR /app

COPY ./ .

RUN go mod download
RUN go build -o control-plane-test-service .

FROM alpine:3.21

RUN apk add --no-cache curl && apk add --no-cache openssl

#RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2
COPY --chown=10001:0 --chmod=555 --from=build /app/control-plane-test-service /app/control-plane-test-service
COPY --chown=10001:0 --chmod=444 --from=build app/application.yaml /app/
COPY --chown=10001:0 --chmod=444 --from=build app/localhost.key /app/

RUN openssl req -new -x509 -nodes -sha256 -days 365 -key /app/localhost.key -out /app/localhost.crt -subj "/C=US/ST=State/L=City/O=Qubership/CN=TestCert"

RUN openssl req -new -x509 -nodes -sha256 -days 365 -key /app/localhost.key -out /app/localhostclient.crt -subj "/C=US/ST=State/L=City/O=Qubership/CN=TestCert"

RUN chmod +x /app/control-plane-test-service

WORKDIR /app
ENTRYPOINT /app/control-plane-test-service
