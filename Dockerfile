# Build stage
FROM golang:1.24.5-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o executr ./cmd/executr

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates curl

# Create directories
RUN mkdir -p /cache /work /data

# Copy binary from builder
COPY --from=builder /app/executr /usr/local/bin/executr

# Set working directory
WORKDIR /data

# Expose port for server
EXPOSE 8080

# Default command (can be overridden)
ENTRYPOINT ["executr"]
CMD ["server"]