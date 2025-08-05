# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

This is a standalone Go application that crawls RSS feeds and sends parsed content to the StrandNerd CMS via API. The crawler operates as an independent microservice using access token authentication to communicate with the CMS.

## Project Structure

```
crawler/
├── cmd/
│   └── main.go              # Main application entry point
├── internal/
│   ├── client/              # CMS API client
│   ├── config/              # Configuration management
│   ├── crawler/             # Core crawling service
│   ├── llm/                 # OpenAI integration for content analysis
│   ├── models/              # Data models and structures
│   └── parser/              # RSS parsing and content extraction
├── docker-compose.yml       # Development Docker setup
├── docker-compose.prod.yml  # Production Docker setup
├── Dockerfile              # Container build configuration
└── Makefile               # Development and deployment commands
```

## Development Commands

### Development Environment (Docker-based)

```bash
# Build Docker image
make build

# Run crawler once
make run

# Run continuously in development mode
make dev

# Clean Docker resources
make clean
```

### Local Go Development (without Docker)

```bash
# Run locally
make go-run

# Build binary
make go-build

# Clean build artifacts
make go-clean

# Run tests
make test
go test ./...
```

### Production Deployment

```bash
# Build production image
make deploy-build

# Start production crawler
make deploy-up

# Stop production crawler
make deploy-down

# View logs
make logs
```

## Application Architecture

### Core Components

- **Crawler Service** (`internal/crawler/service.go`): Main orchestration logic for RSS feed processing
- **CMS Client** (`internal/client/cms_client.go`): HTTP client for communicating with StrandNerd CMS API
- **RSS Parser** (`internal/parser/rss_parser.go`): RSS feed parsing and content extraction
- **LLM Client** (`internal/llm/client.go`): OpenAI integration for AI-powered content analysis
- **Configuration** (`internal/config/config.go`): Environment-based configuration management

### Key Features

- **RSS Feed Processing**: Standard RSS 2.0 feed parsing with duplicate detection via GUID matching
- **AI Content Analysis**: Optional GPT-3.5-turbo integration for detecting primary reporting and extracting source attribution
- **Concurrent Processing**: Configurable concurrent crawling with rate limiting
- **Caching**: In-memory feed configuration caching to reduce API calls
- **Access Token Authentication**: Secure CMS API communication using long-lived access tokens
- **Queue Processing**: Priority-based processing of crawler queue requests

### Command Line Interface

```bash
./crawler [options]

Options:
  -once           Run crawl once and exit
  -feed <id>      Crawl specific feed ID only
  -interval <sec> Crawl interval in seconds (default: 300)
  -help           Show help message

Examples:
  ./crawler -once                    # Run once and exit
  ./crawler -once -feed abc123       # Crawl specific feed once
  ./crawler -interval 600            # Run every 10 minutes
```

## Configuration

### Required Environment Variables
- `CMS_BASE_URL`: Base URL of the CMS API
- `ACCESS_TOKEN`: CMS access token for authentication

### Optional Environment Variables
- `OPENAI_API_KEY`: OpenAI API key for content analysis
- `ENABLE_CONTENT_ANALYSIS`: Enable GPT analysis (default: true)
- `LOG_LEVEL`: Log level (debug, info, warn, error) (default: info)
- `FEED_REFRESH_INTERVAL`: Feed cache refresh interval in minutes (default: 5)
- `REQUEST_TIMEOUT`: HTTP request timeout in seconds (default: 30)
- `MAX_CONCURRENT_CRAWLS`: Maximum concurrent feed crawls (default: 3)
- `USER_AGENT`: User agent for RSS requests (default: StrandNerd-Crawler/1.0)

## CMS API Integration

### Endpoints Used
- `GET /api/v1/crawler/inspiration_feeds`: Fetch all feeds to crawl
- `GET /api/v1/crawler/inspiration_feeds/{id}`: Get specific feed configuration
- `GET /api/v1/inspiration-posts`: Check for existing posts (duplicate detection)
- `POST /api/v1/crawler/inspiration_feed_posts`: Create new posts from crawled content
- `PUT /api/v1/crawler/inspiration_feeds/{id}/last-crawled`: Update feed crawl timestamps
- `GET /api/v1/crawler/requests/poll`: Poll for crawl requests from queue
- `DELETE /api/v1/crawler/requests/{id}`: Acknowledge crawl request completion

### Access Token Requirements
The crawler requires an access token with permissions for:
- Read access to inspiration feeds and posts
- Write access to create inspiration posts and update feed timestamps

## AI Content Analysis

When enabled with `ENABLE_CONTENT_ANALYSIS=true` and a valid `OPENAI_API_KEY`, the crawler uses GPT-3.5-turbo to analyze crawled articles for:

1. **Primary Reporting Detection**: Determines if content is original reporting vs. referencing other sources
2. **Source Attribution**: Extracts original source names when content references external sources

Results are stored as `is_primary_reporting` (boolean) and `original_source_name` (string) fields. Analysis failures don't prevent article saving.

## Docker Configuration

### Development Setup
- Uses `docker-compose.yml` with local build context
- Includes environment variable mapping from `.env` file
- Configured for continuous running with 5-minute intervals

### Production Setup
- Uses `docker-compose.prod.yml` with pre-built image
- Includes resource limits (256M memory, 0.5 CPU)
- Configured with log rotation and restart policies
- Production-optimized logging configuration

## Testing and Quality

- **Unit Tests**: `go test ./...` for all packages
- **Integration Testing**: Manual testing with `-once` flag for single runs
- **Error Handling**: Comprehensive error handling with structured logging
- **Monitoring**: Health checks via container status and log analysis

## Common Workflows

### Testing Feed Processing
```bash
# Test single feed
./crawler -once -feed <feed-id>

# Test all feeds once
./crawler -once

# Run with debug logging
LOG_LEVEL=debug ./crawler -once
```

### Debugging Issues
```bash
# Check container logs
make logs

# Run locally for debugging
make go-run

# Verify API connectivity
curl -H "Authorization: Bearer $ACCESS_TOKEN" $CMS_BASE_URL/api/v1/crawler/inspiration_feeds
```

### Deployment Process
1. Build production image: `make deploy-build`
2. Deploy to production: `make deploy-up`
3. Monitor logs: `make logs`
4. Verify processing: Check CMS for new inspiration posts

## Resource Requirements

- **Memory**: 128MB-256MB
- **CPU**: 0.25-0.5 cores
- **Storage**: Minimal (logs only, no persistent data)
- **Network**: Outbound HTTPS for RSS feeds, CMS API, and OpenAI API

## Monitoring

- **Container Health**: Monitor via `docker-compose ps`
- **Processing Logs**: Look for "Crawl summary" messages indicating successful runs
- **Error Detection**: Monitor for ERROR/FATAL level logs
- **Performance**: Track feed processing times and success rates