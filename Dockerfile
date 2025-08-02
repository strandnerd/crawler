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

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o crawler ./cmd/main.go

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Add non-root user for security
RUN addgroup -g 1001 -S crawler && \
    adduser -u 1001 -S crawler -G crawler

# Copy the binary from builder stage and set permissions
COPY --from=builder --chown=crawler:crawler /app/crawler .
RUN chmod +x ./crawler

USER crawler

# Default command
ENTRYPOINT ["./crawler"]
CMD ["-interval", "300"]