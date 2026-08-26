# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o hyper2kvm \
    ./cmd/hyper2kvm

# Runtime stage
FROM registry.fedoraproject.org/fedora-minimal:43

LABEL org.opencontainers.image.base.name="registry.fedoraproject.org/fedora-minimal:43"

# Install runtime dependencies
RUN microdnf install -y \
    ca-certificates \
    tzdata \
    && microdnf clean all

# Create non-root user
RUN groupadd -g 1000 hyper2kvm && \
    useradd -u 1000 -g hyper2kvm -m -d /home/hyper2kvm -s /sbin/nologin hyper2kvm

# Create directories
RUN mkdir -p /exports && \
    chown -R hyper2kvm:hyper2kvm /exports

# Copy binary from builder
COPY --from=builder /build/hyper2kvm /usr/local/bin/hyper2kvm

# Set user
USER hyper2kvm

# Set working directory
WORKDIR /exports

# Set environment variables
ENV LOG_LEVEL=info \
    DOWNLOAD_WORKERS=3 \
    RETRY_ATTEMPTS=3 \
    PROGRESS_STYLE=bar

# Health check (if needed)
# HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
#     CMD ["/usr/local/bin/hyper2kvm", "--version"] || exit 1

# Run the application
ENTRYPOINT ["/usr/local/bin/hyper2kvm"]
