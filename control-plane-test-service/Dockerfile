FROM golang:1.25 AS build

WORKDIR /app

COPY control-plane-test-service/ .

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on GOSUMDB=off GOTOOLCHAIN=local go build -v -o control-plane-test-service .

FROM alpine:3.22

RUN apk add --no-cache curl && apk add --no-cache openssl

#RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2
COPY --chmod=777  --from=build app/control-plane-test-service /app/control-plane-test-service
COPY --from=build app/application.yaml /app/
COPY --from=build app/localhost.key /app/

RUN openssl req -new -x509 -nodes -sha256 -days 365 -key /app/localhost.key -out /app/localhost.crt -subj "/C=US/ST=State/L=City/O=Qubership/CN=TestCert"

RUN openssl req -new -x509 -nodes -sha256 -days 365 -key /app/localhost.key -out /app/localhostclient.crt -subj "/C=US/ST=State/L=City/O=Qubership/CN=TestCert"

WORKDIR /app
ENTRYPOINT /app/control-plane-test-service
