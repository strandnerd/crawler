package main

import (
	"flag"
	"log"
	"os"
	"time"

	"strandnerd-crawler/internal/client"
	"strandnerd-crawler/internal/config"
	"strandnerd-crawler/internal/crawler"
	"strandnerd-crawler/internal/models"
)

func main() {
	var (
		runOnce   = flag.Bool("once", false, "Run crawl once and exit")
		feedID    = flag.String("feed", "", "Crawl specific feed ID only")
		tenantID  = flag.String("tenant", "", "Run only for specific tenant ID")
		interval  = flag.Int("interval", 300, "Crawl interval in seconds (default: 5 minutes)")
		help      = flag.Bool("help", false, "Show help message")
	)
	flag.Parse()

	if *help {
		printHelp()
		return
	}

	log.Println("Starting StrandNerd Inspiration Feeds Crawler...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Loaded configuration for %d tenant(s)", len(cfg.Tenants))
	for _, tenant := range cfg.Tenants {
		log.Printf("  - Tenant: %s (%s)", tenant.ID, tenant.Name)
	}

	// Initialize crawler services for each tenant
	crawlerServices := make(map[string]*crawler.Service)
	for _, tenant := range cfg.Tenants {
		if !tenant.Enabled {
			log.Printf("Skipping disabled tenant: %s", tenant.ID)
			continue
		}

		// Initialize CMS API client for this tenant
		cmsClient := client.NewCMSClient(tenant.CMSBaseURL, tenant.AccessToken)

		// Initialize crawler service for this tenant
		crawlerService := crawler.NewService(cmsClient, cfg)
		crawlerServices[tenant.ID] = crawlerService

		log.Printf("Initialized crawler service for tenant: %s", tenant.ID)
	}

	if len(crawlerServices) == 0 {
		log.Fatalf("No enabled tenants found")
	}

	if *runOnce {
		// Run once and exit
		runCrawlOnce(crawlerServices, *feedID, *tenantID)
		return
	}

	// Start a goroutine to process manual crawl requests more frequently (every 10 seconds)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			for tenantID, crawlerService := range crawlerServices {
				if err := crawlerService.ProcessQueueRequests(); err != nil {
					log.Printf("Failed to process queue requests for tenant %s: %v", tenantID, err)
				}
			}
		}
	}()

	// Run continuously
	runCrawlScheduler(crawlerServices, *feedID, *tenantID, *interval)
}

func printHelp() {
	log.Println("StrandNerd Inspiration Feeds Crawler")
	log.Println()
	log.Println("Usage:")
	log.Println("  crawler [options]")
	log.Println()
	log.Println("Options:")
	log.Println("  -once             Run crawl once and exit")
	log.Println("  -feed <id>        Crawl specific feed ID only")
	log.Println("  -tenant <id>      Run only for specific tenant ID")
	log.Println("  -interval <sec>   Crawl interval in seconds (default: 300)")
	log.Println("  -help             Show this help message")
	log.Println()
	log.Println("Configuration:")
	log.Println("  The crawler looks for tenants.yml in the current directory.")
	log.Println("  If not found, it falls back to environment variables:")
	log.Println()
	log.Println("  Legacy Environment Variables (single tenant):")
	log.Println("  CMS_BASE_URL              CMS API base URL (required)")
	log.Println("  ACCESS_TOKEN              CMS access token for API authentication (required)")
	log.Println()
	log.Println("  Multi-tenant Environment Variables:")
	log.Println("  TENANT_1_CMS_BASE_URL     First tenant CMS URL")
	log.Println("  TENANT_1_ACCESS_TOKEN     First tenant access token")
	log.Println("  TENANT_2_CMS_BASE_URL     Second tenant CMS URL")
	log.Println("  TENANT_2_ACCESS_TOKEN     Second tenant access token")
	log.Println("  ...")
	log.Println()
	log.Println("  Optional Environment Variables:")
	log.Println("  OPENAI_API_KEY            OpenAI API key for content analysis (optional)")
	log.Println("  ENABLE_CONTENT_ANALYSIS   Enable GPT content analysis (default: true)")
	log.Println("  LOG_LEVEL                 Log level (debug, info, warn, error) (default: info)")
	log.Println("  PROXY_HOST                Proxy host (required)")
	log.Println("  PROXY_AUTH                Proxy authentication (required)")
	log.Println()
	log.Println("Examples:")
	log.Println("  # Run once and exit for all tenants")
	log.Println("  crawler -once")
	log.Println()
	log.Println("  # Run specific feed once for specific tenant")
	log.Println("  crawler -once -feed abc123 -tenant main")
	log.Println()
	log.Println("  # Run continuously every 10 minutes for all tenants")
	log.Println("  crawler -interval 600")
	log.Println()
	log.Println("  # Run continuously for specific tenant only")
	log.Println("  crawler -tenant dev")
}

func runCrawlOnce(crawlerServices map[string]*crawler.Service, feedID, tenantID string) {
	log.Println("Running crawl once...")

	var servicesToRun map[string]*crawler.Service

	if tenantID != "" {
		// Run for specific tenant only
		if service, exists := crawlerServices[tenantID]; exists {
			servicesToRun = map[string]*crawler.Service{tenantID: service}
			log.Printf("Running crawl for tenant: %s", tenantID)
		} else {
			log.Fatalf("Tenant %s not found or not enabled", tenantID)
		}
	} else {
		// Run for all tenants
		servicesToRun = crawlerServices
		log.Printf("Running crawl for all %d tenant(s)", len(servicesToRun))
	}

	allSuccessful := true
	totalResults := make(map[string][]models.CrawlResult)

	for currentTenantID, crawlerService := range servicesToRun {
		log.Printf("\n--- Processing Tenant: %s ---", currentTenantID)

		// First, check for any queue requests (higher priority)
		if err := crawlerService.ProcessQueueRequests(); err != nil {
			log.Printf("Failed to process queue requests for tenant %s: %v", currentTenantID, err)
		}

		var results []models.CrawlResult
		var err error

		if feedID != "" {
			// Crawl specific feed
			result, crawlErr := crawlerService.CrawlFeed(feedID)
			if crawlErr != nil {
				log.Printf("Failed to crawl feed %s for tenant %s: %v", feedID, currentTenantID, crawlErr)
				allSuccessful = false
				continue
			}
			results = []models.CrawlResult{*result}
		} else {
			// Crawl all due feeds
			results, err = crawlerService.CrawlAllDueFeeds()
			if err != nil {
				log.Printf("Failed to crawl feeds for tenant %s: %v", currentTenantID, err)
				allSuccessful = false
				continue
			}
		}

		totalResults[currentTenantID] = results

		// Process results for this tenant
		successCount := 0
		errorCount := 0
		totalPosts := 0

		for _, result := range results {
			printCrawlResult(&result, currentTenantID)
			if result.Success {
				successCount++
				totalPosts += result.PostsAdded
			} else {
				errorCount++
				allSuccessful = false
			}
		}

		log.Printf("Tenant %s summary: %d feeds processed, %d successful, %d errors, %d total posts added",
			currentTenantID, len(results), successCount, errorCount, totalPosts)
	}

	// Overall summary
	log.Printf("\n=== Overall Crawl Summary ===")
	grandTotalFeeds := 0
	grandTotalPosts := 0
	for tenantID, results := range totalResults {
		totalPosts := 0
		for _, result := range results {
			if result.Success {
				totalPosts += result.PostsAdded
			}
		}
		log.Printf("Tenant %s: %d feeds, %d posts added", tenantID, len(results), totalPosts)
		grandTotalFeeds += len(results)
		grandTotalPosts += totalPosts
	}
	log.Printf("Grand Total: %d feeds processed, %d posts added", grandTotalFeeds, grandTotalPosts)

	if !allSuccessful {
		os.Exit(1)
	}

	log.Println("Crawl completed successfully")
}

func runCrawlScheduler(crawlerServices map[string]*crawler.Service, feedID, tenantID string, intervalSec int) {
	log.Printf("Starting crawler scheduler (interval: %d seconds)", intervalSec)

	ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
	defer ticker.Stop()

	// Run initial crawl
	runScheduledCrawl(crawlerServices, feedID, tenantID)

	// Run on schedule
	for range ticker.C {
		runScheduledCrawl(crawlerServices, feedID, tenantID)
	}
}

func runScheduledCrawl(crawlerServices map[string]*crawler.Service, feedID, tenantID string) {
	log.Println("Running scheduled crawl...")

	var servicesToRun map[string]*crawler.Service

	if tenantID != "" {
		// Run for specific tenant only
		if service, exists := crawlerServices[tenantID]; exists {
			servicesToRun = map[string]*crawler.Service{tenantID: service}
		} else {
			log.Printf("Tenant %s not found or not enabled", tenantID)
			return
		}
	} else {
		// Run for all tenants
		servicesToRun = crawlerServices
	}

	for currentTenantID, crawlerService := range servicesToRun {
		log.Printf("\n--- Processing Tenant: %s ---", currentTenantID)

		// First, check for any queue requests (higher priority)
		if err := crawlerService.ProcessQueueRequests(); err != nil {
			log.Printf("Failed to process queue requests for tenant %s: %v", currentTenantID, err)
		}

		if feedID != "" {
			// Crawl specific feed
			result, err := crawlerService.CrawlFeed(feedID)
			if err != nil {
				log.Printf("Failed to crawl feed %s for tenant %s: %v", feedID, currentTenantID, err)
				continue
			}
			printCrawlResult(result, currentTenantID)
		} else {
			// Crawl all due feeds
			results, err := crawlerService.CrawlAllDueFeeds()
			if err != nil {
				log.Printf("Failed to crawl feeds for tenant %s: %v", currentTenantID, err)
				continue
			}

			if len(results) == 0 {
				log.Printf("No feeds due for crawling for tenant %s", currentTenantID)
				continue
			}

			successCount := 0
			errorCount := 0
			totalPosts := 0

			for _, result := range results {
				printCrawlResult(&result, currentTenantID)
				if result.Success {
					successCount++
					totalPosts += result.PostsAdded
				} else {
					errorCount++
				}
			}

			log.Printf("Tenant %s crawl summary: %d feeds processed, %d successful, %d errors, %d total posts added",
				currentTenantID, len(results), successCount, errorCount, totalPosts)
		}
	}
}

func printCrawlResult(result *models.CrawlResult, tenantID string) {
	if result.Success {
		log.Printf("✓ [%s] Feed %s: %d found, %d added, %d skipped",
			tenantID, result.FeedID, result.PostsFound, result.PostsAdded, result.PostsSkipped)
	} else {
		log.Printf("✗ [%s] Feed %s failed: %v", tenantID, result.FeedID, result.Error)
	}
}
