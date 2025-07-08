# Build stage
FROM rust:1.86-alpine AS builder

RUN apk add --no-cache \
    musl-dev \
    pkgconfig \
    openssl-dev \
    protobuf-dev

WORKDIR /app

COPY manager/Cargo.toml manager/Cargo.lock ./
COPY manager/build.rs ./
COPY manager/proto ./proto/
COPY manager/src ./src/
RUN cargo build --release

# Runtime stage
FROM alpine:latest

RUN apk add --no-cache \
    ca-certificates \
    tzdata

# Create non-root user
RUN addgroup -g 1001 -S shiftpod && \
    adduser -u 1001 -S shiftpod -G shiftpod

# Create necessary directories
RUN mkdir -p /var/lib/shiftpod/checkpoints && \
    mkdir -p /var/run/shiftpod && \
    chown -R shiftpod:shiftpod /var/lib/shiftpod && \
    chown -R shiftpod:shiftpod /var/run/shiftpod

COPY --from=builder /app/target/release/shiftpod-manager /usr/local/bin/shiftpod-manager
RUN chmod +x /usr/local/bin/shiftpod-manager
USER shiftpod

# Expose gRPC port
EXPOSE 9090
WORKDIR /var/lib/shiftpod

# Default command
ENTRYPOINT ["/usr/local/bin/shiftpod-manager"]
CMD ["--node-name", "$(NODE_NAME)", "--node-address", "$(NODE_ADDRESS)"]
