# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install git and ca-certificates for HTTPS
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with server support
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/vici-cdr-scrubber ./cmd

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/vici-cdr-scrubber .
COPY --from=builder /app/config.yaml .

# Expose port (Cloud Run requires this)
EXPOSE 8080

# Run the binary in server mode
ENTRYPOINT ["./vici-cdr-scrubber"]
CMD ["--server"]
