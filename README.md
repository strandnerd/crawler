# StrandNerd Crawler

A standalone Go application that crawls RSS feeds and sends the parsed content to the StrandNerd CMS via API. This crawler is designed to be a separate microservice that communicates with the CMS using access tokens.

## Overview

The crawler fetches inspiration feeds from the CMS, parses RSS content, and creates new inspiration feed posts. It operates independently from the CMS and can be deployed on a separate server or container.

## Features

- **RSS Feed Parsing**: Supports standard RSS 2.0 feeds
- **Duplicate Detection**: Prevents duplicate posts using GUID matching
- **AI Content Analysis**: Uses GPT-3.5-turbo to detect primary reporting and extract original sources
- **Concurrent Processing**: Configurable concurrent crawling with rate limiting
- **Caching**: In-memory caching of feed configurations to reduce API calls
- **Retry Logic**: Built-in error handling and retry mechanisms
- **Access Token Authentication**: Secure communication with CMS using access tokens
- **Docker Support**: Containerized deployment with Docker Compose
- **CI/CD Ready**: GitHub Actions workflow for automated deployment

## Architecture

```
┌─────────────────┐    HTTP API     ┌─────────────────┐
│                 │   (Access Token) │                 │
│   Crawler       │ ◄──────────────► │   CMS API       │
│   Service       │                  │                 │
└─────────────────┘                  └─────────────────┘
         │                                     │
         ▼                                     ▼
┌─────────────────┐                  ┌─────────────────┐
│   RSS Feeds     │                  │   PostgreSQL    │
│   (External)    │                  │   Database      │
└─────────────────┘                  └─────────────────┘
```

## Configuration

The crawler supports multiple configuration methods:

1. **YAML Configuration (Recommended)**: Use `tenants.yml` for multi-tenant setup
2. **Environment Variables**: Legacy support for single tenant or environment-based config

### YAML Configuration (Multi-tenant)

Create a `tenants.yml` file in the project root. This file is ignored by git, so copy from the example:

```bash
cp tenants.yml.example tenants.yml
# Edit tenants.yml with your configuration
```

Example `tenants.yml`:

```yaml
tenants:
  - id: "main"
    name: "Main Production Instance"
    cms_base_url: "https://cms.strandnerd.com/api"
    access_token: "your-main-access-token-here"
    enabled: true
    
  - id: "dev"
    name: "Development Instance" 
    cms_base_url: "https://dev-cms.strandnerd.com/api"
    access_token: "your-dev-access-token-here"
    enabled: true
    
  - id: "partner1"
    name: "Partner Site 1"
    cms_base_url: "https://partner1.example.com/api"
    access_token: "partner1-access-token"
    enabled: false  # Can disable tenants temporarily

# Global settings (optional - override environment variables)
global:
  log_level: "info"
  feed_refresh_interval: 5
  request_timeout: 30
  max_concurrent_crawls: 3
  enable_content_analysis: true
  # openai_api_key: "your-openai-key"  # Can also be set via env var
```

### Environment Variables (Legacy/Single Tenant)

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `CMS_BASE_URL` | Base URL of the CMS API (single tenant) | - | ✅* |
| `ACCESS_TOKEN` | CMS access token (single tenant) | - | ✅* |
| `TENANT_1_CMS_BASE_URL` | First tenant CMS URL (multi-tenant env) | - | ❌ |
| `TENANT_1_ACCESS_TOKEN` | First tenant access token (multi-tenant env) | - | ❌ |
| `OPENAI_API_KEY` | OpenAI API key for content analysis | - | ❌ |
| `ENABLE_CONTENT_ANALYSIS` | Enable GPT content analysis | `true` | ❌ |
| `LOG_LEVEL` | Log level (debug, info, warn, error) | `info` | ❌ |
| `FEED_REFRESH_INTERVAL` | Feed cache refresh interval (minutes) | `5` | ❌ |
| `REQUEST_TIMEOUT` | HTTP request timeout (seconds) | `30` | ❌ |
| `MAX_CONCURRENT_CRAWLS` | Maximum concurrent feed crawls | `3` | ❌ |
| `USER_AGENT` | User agent for RSS requests | `StrandNerd-Crawler/1.0` | ❌ |
| `PROXY_HOST` | Proxy host for external requests | - | ✅ |
| `PROXY_AUTH` | Proxy authentication (username:password) | - | ✅ |

*Required only when not using YAML configuration

### Example .env file (Legacy)

```bash
# Single tenant setup
CMS_BASE_URL=https://cms.strandnerd.com
ACCESS_TOKEN=your_access_token_here

# Or multi-tenant environment setup
TENANT_1_CMS_BASE_URL=https://cms.strandnerd.com
TENANT_1_ACCESS_TOKEN=your_main_token_here
TENANT_2_CMS_BASE_URL=https://dev-cms.strandnerd.com  
TENANT_2_ACCESS_TOKEN=your_dev_token_here

# Common settings
OPENAI_API_KEY=sk-your_openai_api_key_here
ENABLE_CONTENT_ANALYSIS=true
LOG_LEVEL=info
FEED_REFRESH_INTERVAL=5
REQUEST_TIMEOUT=30
MAX_CONCURRENT_CRAWLS=3
USER_AGENT=StrandNerd-Crawler/1.0
PROXY_HOST=proxy.example.com:8080
PROXY_AUTH=username:password
```

## AI Content Analysis

The crawler includes an optional AI-powered content analysis feature that uses OpenAI's GPT-3.5-turbo model to analyze crawled articles and determine:

1. **Primary Reporting Detection**: Whether an article is original reporting or references other sources
2. **Original Source Extraction**: If the article references another source, the AI extracts the source name

### How it works

- When enabled, each crawled article is analyzed by GPT before being saved to the CMS
- The AI examines the title, description, and content to make determinations
- Results are saved as `is_primary_reporting` (boolean) and `original_source_name` (string) fields
- Analysis failures don't prevent articles from being saved - they just won't have the analysis fields populated

### Configuration

```bash
# Enable/disable content analysis (default: true)
ENABLE_CONTENT_ANALYSIS=true

# OpenAI API key (required if content analysis is enabled)
OPENAI_API_KEY=sk-your_openai_api_key_here
```

### Example Analysis Results

- **Primary Reporting**: Article written by the outlet's own journalists
  - `is_primary_reporting: true`
  - `original_source_name: null`

- **Referenced Reporting**: Article mainly referencing other sources
  - `is_primary_reporting: false`
  - `original_source_name: "BBC News"` (or whatever source was identified)

The AI uses a low temperature setting (0.3) for consistent results and includes confidence scoring and reasoning in its analysis logs.

## Development

### Prerequisites

- Go 1.21 or later
- Docker and Docker Compose
- Access to StrandNerd CMS with valid access token

### Local Development

1. **Clone and setup**:
   ```bash
   cd crawler/
   cp .env.example .env
   # Edit .env with your configuration
   ```

2. **Run locally** (without Docker):
   ```bash
   make go-run
   # or
   go run ./cmd/main.go -once
   ```

3. **Run with Docker**:
   ```bash
   # First, create your tenant configuration
   cp tenants.yml.example tenants.yml
   # Edit tenants.yml with your configuration
   
   # The Docker containers automatically bind ./tenants.yml:/app/tenants.yml
   make build
   make run      # Run once
   make dev      # Run continuously
   ```

### Available Commands

```bash
# Development
make build          # Build Docker image
make run            # Run crawler once
make dev            # Run crawler continuously
make clean          # Clean up Docker resources

# Go development (without Docker)
make go-run         # Run locally
make go-build       # Build binary
make go-clean       # Clean Go artifacts

# Production deployment
make deploy-build   # Build production image
make deploy-up      # Start production crawler
make deploy-down    # Stop production crawler
make logs           # View logs
```

### Command Line Options

```bash
./crawler [options]

Options:
  -once             Run crawl once and exit
  -feed <id>        Crawl specific feed ID only  
  -tenant <id>      Run only for specific tenant ID
  -interval <sec>   Crawl interval in seconds (default: 300)
  -help             Show help message

Examples:
  ./crawler -once                           # Run once for all tenants
  ./crawler -once -tenant main              # Run once for specific tenant
  ./crawler -once -feed abc123              # Crawl specific feed for all tenants
  ./crawler -once -feed abc123 -tenant dev  # Crawl specific feed for specific tenant
  ./crawler -interval 600                   # Run every 10 minutes for all tenants
  ./crawler -tenant main                    # Run continuously for specific tenant only
```

### Multi-tenant Operation

When using YAML configuration with multiple tenants:

- By default, the crawler processes all enabled tenants
- Use `-tenant <id>` to run for a specific tenant only
- Each tenant maintains separate feed lists and crawl schedules
- Queue processing runs for all tenants every 10 seconds
- Logs clearly identify which tenant each operation relates to

## Deployment

### Production Deployment

The crawler is deployed using Docker Compose with the following setup:

1. **Server Setup**:
   ```bash
   # Create deployment directory
   sudo mkdir -p /opt/strandnerd-crawler
   cd /opt/strandnerd-crawler
   
   # Create environment file
   sudo nano .env
   # Add your environment variables
   ```

2. **Manual Deployment**:
   ```bash
   # Build and deploy
   make deploy-build
   make deploy-up
   
   # Check logs
   make logs
   ```

3. **GitHub Actions Deployment**:
   
   The project includes automated deployment via GitHub Actions. Configure these secrets in your repository:

   | Secret | Description |
   |--------|-------------|
   | `DEPLOY_HOST` | Server hostname/IP |
   | `DEPLOY_USER` | SSH username |
   | `DEPLOY_SSH_KEY` | SSH private key |
   | `DEPLOY_PORT` | SSH port (optional, default: 22) |
   | `CMS_BASE_URL` | CMS API base URL |
   | `ACCESS_TOKEN` | CMS access token |

   Deployment triggers automatically on push to `main` branch.

### Resource Requirements

- **Memory**: 128MB-256MB
- **CPU**: 0.25-0.5 cores
- **Storage**: Minimal (logs only)
- **Network**: Outbound HTTPS for RSS feeds and CMS API

## API Integration

### CMS Endpoints Used

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/v1/crawler/inspiration_feeds` | GET | Fetch all feeds |
| `/api/v1/crawler/inspiration_feeds/{id}` | GET | Get specific feed |
| `/api/v1/inspiration-posts` | GET | Check existing posts |
| `/api/v1/crawler/inspiration_feed_posts` | POST | Create new posts |
| `/api/v1/crawler/inspiration_feeds/{id}/last-crawled` | PUT | Update crawl timestamp |
| `/api/v1/crawler/requests/poll` | GET | Poll for crawl requests |
| `/api/v1/crawler/requests/{id}` | DELETE | Acknowledge crawl completion |

### Access Token Requirements

The crawler requires an access token with the following permissions:
- Read access to inspiration feeds
- Read access to inspiration posts (for duplicate checking)
- Write access to create new inspiration posts
- Write access to update feed timestamps

## Monitoring

### Logs

The crawler provides structured logging with different levels:

```bash
# View real-time logs
make logs

# Filter logs by level
docker-compose -f docker-compose.prod.yml logs crawler | grep ERROR
```

### Health Checks

Monitor crawler health by checking:
- Container status: `docker-compose ps`
- Recent logs: `docker-compose logs --tail 50 crawler`
- Feed processing: Look for "Crawl summary" messages

### Metrics

Key metrics to monitor:
- **Feed processing frequency**: Should match configured intervals
- **Success/error rates**: Check for consistent failures
- **Processing time**: Monitor for performance degradation
- **Duplicate detection**: Verify posts aren't being duplicated

## Troubleshooting

### Common Issues

1. **Authentication errors**:
   - Verify `ACCESS_TOKEN` is valid and has required permissions
   - Check `CMS_BASE_URL` is correct and accessible

2. **RSS parsing failures**:
   - Check if feeds are returning valid RSS/XML
   - Verify feed URLs are accessible from crawler server
   - Check for rate limiting from feed providers

3. **Network connectivity**:
   - Ensure outbound HTTPS access to feed URLs and CMS
   - Check DNS resolution for external domains

4. **Memory issues**:
   - Monitor container memory usage
   - Adjust `MAX_CONCURRENT_CRAWLS` if needed

### Debug Mode

Enable debug logging:
```bash
# In .env file
LOG_LEVEL=debug

# Restart crawler
make deploy-down && make deploy-up
```

## Contributing

1. Follow Go coding standards
2. Add tests for new functionality
3. Update documentation for configuration changes
4. Test with Docker before submitting PRs

## License

This project is part of the StrandNerd ecosystem and follows the same licensing terms as the main CMS project.
