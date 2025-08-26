# Build stage
FROM rust:1.81-slim as builder

WORKDIR /app

# Install build dependencies
RUN apt-get update && apt-get install -y \
    pkg-config \
    libssl-dev \
    protobuf-compiler \
    && rm -rf /var/lib/apt/lists/*

# Copy proto files
COPY proto/ ../proto/

# Copy manager files
COPY manager/Cargo.toml manager/Cargo.lock ./
COPY manager/src/ src/
COPY manager/build.rs ./

# Build release binary
RUN cargo build --release

# Runtime stage
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    libssl3 \
    && rm -rf /var/lib/apt/lists/*

# Create user and directories
RUN groupadd -r shiftpod && useradd -r -g shiftpod shiftpod
RUN mkdir -p /var/lib/shiftpod/checkpoints /var/run/shiftpod
RUN chown -R shiftpod:shiftpod /var/lib/shiftpod /var/run/shiftpod

# Copy binary from builder
COPY --from=builder /app/target/release/shiftpod-manager /usr/local/bin/shiftpod-manager

# Switch to non-root user
USER shiftpod

# Set environment variables
ENV RUST_LOG=info
ENV CHECKPOINT_DIR=/var/lib/shiftpod/checkpoints
ENV SOCKET_PATH=/var/run/shiftpod/manager.sock
ENV GRPC_PORT=9090

# Expose gRPC port
EXPOSE 9090

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD test -S /var/run/shiftpod/manager.sock || exit 1

# Run the manager
ENTRYPOINT ["/usr/local/bin/shiftpod-manager"]
