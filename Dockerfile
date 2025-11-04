# Build stage
FROM golang:1.21-alpine AS builder

# Build arguments for version information
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown

# Install git for build info
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.gitCommit=${GIT_COMMIT} -X main.buildTime=${BUILD_TIME}" \
    -a -o mount-exporter .

# Final stage
FROM alpine:latest

# Labels
LABEL maintainer="mount-exporter" \
      description="Prometheus exporter for filesystem mount point availability" \
      version="${VERSION}" \
      org.opencontainers.image.title="Mount Exporter" \
      org.opencontainers.image.description="Prometheus exporter for monitoring filesystem mount points" \
      org.opencontainers.image.vendor="mount-exporter" \
      org.opencontainers.image.licenses="MIT"

# Install ca-certificates for HTTPS requests, findmnt for mount checking, and curl for health checks
RUN apk --no-cache add ca-certificates util-linux curl

# Create non-root user
RUN addgroup -g 1001 -S mount-exporter && \
    adduser -u 1001 -S mount-exporter -G mount-exporter

# Set working directory
WORKDIR /

# Copy binary from builder stage
COPY --from=builder /app/mount-exporter /usr/local/bin/mount-exporter

# Create config directory
RUN mkdir -p /etc/mount-exporter

# Copy example config
COPY examples/config.yaml /etc/mount-exporter/config.yaml.example

# Change ownership
RUN chown -R mount-exporter:mount-exporter /usr/local/bin/mount-exporter /etc/mount-exporter

# Switch to non-root user
USER mount-exporter

# Expose metrics port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f -s http://localhost:8080/health || exit 1

# Run the binary
ENTRYPOINT ["mount-exporter"]
CMD ["-config", "/etc/mount-exporter/config.yaml"]