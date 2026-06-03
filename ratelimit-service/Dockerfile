# Dockerfile
FROM golang:1.26 AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ratelimit cmd/operator/main.go

# Final stage
FROM ghcr.io/netcracker/qubership-core-base:2.2.7 AS run

WORKDIR /app/

# Copy the binary from builder
COPY --from=builder /app/ratelimit .

# Expose ports
EXPOSE 8082 9090

USER 10001:10001
# Run the binary
ENTRYPOINT ["./ratelimit"]