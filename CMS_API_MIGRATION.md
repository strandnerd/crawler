# CMS API Migration Summary

## Overview
This document summarizes the changes made to the StrandNerd Crawler to adapt to the CMS API restructuring that separated public/crawler endpoints from tenant-scoped admin endpoints.

## Migration Date
August 4, 2025

## Changes Made

### 1. API Endpoint Updates in `internal/client/cms_client.go`

#### Updated Endpoints
- **GetInspirationFeeds()**: 
  - OLD: `/api/v1/crawler/feeds`
  - NEW: `/api/v1/crawler/inspiration_feeds`

- **GetInspirationFeedByID()**: 
  - OLD: `/api/v1/crawler/feeds/{id}`
  - NEW: `/api/v1/crawler/inspiration_feeds/{id}`

- **CreateInspirationFeedPost()**: 
  - OLD: `/api/v1/inspiration-posts`
  - NEW: `/api/v1/crawler/inspiration_feed_posts`

- **UpdateFeedLastCrawledAt()**: 
  - OLD: `/api/v1/crawler/feeds/{id}/last-crawled`
  - NEW: `/api/v1/crawler/inspiration_feeds/{id}/last-crawled`

#### Unchanged Endpoints
- **PollCrawlRequest()**: `/api/v1/crawler/requests/poll` (already correct)
- **AcknowledgeRequest()**: `/api/v1/crawler/requests/{id}` (already correct)
- **GetInspirationPosts()**: `/api/v1/inspiration-posts` (kept for duplicate checking)

### 2. Documentation Updates

#### README.md
- Updated "CMS Endpoints Used" table with new endpoint URLs
- Added queue management endpoints documentation

#### CLAUDE.md
- Updated "Endpoints Used" section with new crawler API paths
- Updated debugging curl command example

### 3. Authentication
- No changes required to authentication mechanism
- Continues using access token authentication: `Authorization: Bearer {token}`
- No X-Tenant header needed for crawler endpoints (automatic tenant derivation)

## Testing
- ✅ Build verification: `go build ./...` successful
- ✅ Test suite: `go test ./...` all tests pass
- ✅ No compilation errors introduced

## Compatibility
- **Breaking Change**: This update requires the CMS to have the new API structure deployed
- **Backward Compatibility**: None - old endpoints will return 404
- **Action Required**: Deploy this updated crawler only after CMS API restructuring is complete

## Benefits of New API Structure
- **Cleaner URLs**: Logical grouping with `/crawler/` prefix for all crawler operations
- **Better Error Handling**: Consistent error responses across all crawler endpoints  
- **Improved Performance**: Optimized middleware chain for crawler-specific needs
- **Enhanced Security**: Dedicated authentication flow for crawler operations

## Rollback Plan
If rollback is needed:
1. Revert the endpoint URLs in `cms_client.go` to old paths
2. Revert documentation changes
3. Rebuild and redeploy

## Support
For issues related to this migration:
1. Verify CMS has new API structure deployed
2. Check access token permissions for new endpoints
3. Review API response headers for any deprecation warnings
4. Test connectivity with new endpoint URLs manually

---

**Migration completed successfully on August 4, 2025**
