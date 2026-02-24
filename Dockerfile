# Build stage
FROM golang:1.24-bookworm AS builder

WORKDIR /app

# Install build dependencies
RUN apt-get update && apt-get install -y git && rm -rf /var/lib/apt/lists/*

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o rag-server cmd/rag-server/main.go

# Final stage
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y ca-certificates && \
    useradd -m -u 1000 appuser && \
    rm -rf /var/lib/apt/lists/*


WORKDIR /home/appuser/app

# Copy the binary from the builder stage
COPY --from=builder --chmod=0555 /app/rag-server .
# Copy .env file if it exists (or rely on docker-compose env_file)
COPY --chmod=0444 .env .

# Switch to non-root user
USER appuser

# Expose the API port
EXPOSE 8080

# Run the application
CMD ["./rag-server"]
