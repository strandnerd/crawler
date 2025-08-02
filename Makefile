.PHONY: help build run dev test clean deploy-build deploy-up deploy-down logs

# Default target
help:
	@echo "StrandNerd Crawler - Available commands:"
	@echo ""
	@echo "Development:"
	@echo "  build           Build the Docker image"
	@echo "  run             Run crawler once"
	@echo "  dev             Run crawler in development mode (continuous)"
	@echo "  test            Run tests (when implemented)" 
	@echo "  clean           Clean up Docker images and containers"
	@echo ""
	@echo "Production Deployment:"
	@echo "  deploy-build    Build production image"
	@echo "  deploy-up       Start production crawler"
	@echo "  deploy-down     Stop production crawler"
	@echo "  logs            View crawler logs"
	@echo ""
	@echo "Examples:"
	@echo "  make build"
	@echo "  make run"
	@echo "  make deploy-up"

# Development targets
build:
	@echo "Building crawler Docker image..."
	docker-compose build

run:
	@echo "Running crawler once..."
	docker-compose run --rm crawler -once

dev:
	@echo "Starting crawler in development mode..."
	docker-compose up

test:
	@echo "Running tests..."
	go test ./...

clean:
	@echo "Cleaning up Docker resources..."
	docker-compose down --remove-orphans
	docker image prune -f --filter label=project=strandnerd

# Production deployment targets
deploy-build:
	@echo "Building production crawler image..."
	docker build -t strandnerd-crawler:latest .

deploy-up:
	@echo "Starting production crawler..."
	docker-compose -f docker-compose.prod.yml up -d

deploy-down:
	@echo "Stopping production crawler..."
	docker-compose -f docker-compose.prod.yml down

logs:
	@echo "Showing crawler logs..."
	docker-compose -f docker-compose.prod.yml logs -f crawler

# Go development targets (for local development without Docker)
go-run:
	@echo "Running crawler locally..."
	go run ./cmd/main.go

go-build:
	@echo "Building crawler binary..."
	go build -o crawler ./cmd/main.go

go-clean:
	@echo "Cleaning Go build artifacts..."
	rm -f crawler
	go clean