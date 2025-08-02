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

# Build the application with executable permissions
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o crawler ./cmd/main.go && chmod +x crawler

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/crawler ./crawler

# Add non-root user for security
RUN addgroup -g 1001 -S crawler && \
    adduser -u 1001 -S crawler -G crawler && \
    chown crawler:crawler ./crawler && \
    chmod +x ./crawler

USER crawler

# Default command
ENTRYPOINT ["./crawler"]
CMD ["-interval", "300"]