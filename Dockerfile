# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install git for module dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Copy tenants.yml if it exists
COPY tenants.yml* ./

# Build the application with executable permissions
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o crawler ./cmd/main.go && chmod +x crawler

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Create working directory where tenants.yml will be mounted
WORKDIR /app

# Copy the binary from builder stage to /usr/local/bin
COPY --from=builder /app/crawler /usr/local/bin/crawler

# Add non-root user for security and set permissions
RUN addgroup -g 1001 -S crawler && \
    adduser -u 1001 -S crawler -G crawler && \
    chmod +x /usr/local/bin/crawler && \
    chown crawler:crawler /app

USER crawler

# Default command
ENTRYPOINT ["/usr/local/bin/crawler"]
CMD ["-interval", "300"]
