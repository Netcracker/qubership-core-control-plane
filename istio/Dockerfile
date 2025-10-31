FROM golang:1.23-alpine AS builder
RUN go install github.com/rakyll/hey@latest

FROM alpine:3.22.2
ARG TEST_URL="http://istio-gateway-istio/ambient-test/health"

WORKDIR /files

# 10 KB (≈ 10,000 bytes of base64 text)
RUN base64 /dev/urandom | head -c 7500 > payload-10kb.txt

# 100 KB (≈ 100,000 bytes of base64 text)
RUN base64 /dev/urandom | head -c 75000 > payload-100kb.txt

# 8 MB (≈ 8,000,000 bytes of base64 text)
RUN base64 /dev/urandom | head -c 6000000 > payload-8mb.txt

COPY --from=builder /go/bin/hey /usr/local/bin/hey

RUN cat <<'EOF' > /run-hey.sh
#!/bin/sh
for file in /files/*; do
  echo "Running hey with payload $file"
  hey -n 100 -c 10 -D "$file" ${TEST_URL} &
done
wait
EOF
RUN chmod +x /run-hey.sh

CMD ["/run-hey.sh"]