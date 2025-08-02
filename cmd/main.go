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
		runOnce  = flag.Bool("once", false, "Run crawl once and exit")
		feedID   = flag.String("feed", "", "Crawl specific feed ID only")
		interval = flag.Int("interval", 300, "Crawl interval in seconds (default: 5 minutes)")
		help     = flag.Bool("help", false, "Show help message")
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

	// Initialize CMS API client
	cmsClient := client.NewCMSClient(cfg.CMSBaseURL, cfg.AccessToken)

	// Initialize crawler service
	crawlerService := crawler.NewService(cmsClient, cfg)

	   if *runOnce {
			   // Run once and exit
			   runCrawlOnce(crawlerService, *feedID)
			   return
	   }

	   // Start a goroutine to process manual crawl requests more frequently (every 10 seconds)
	   go func() {
			   ticker := time.NewTicker(10 * time.Second)
			   defer ticker.Stop()
			   for range ticker.C {
					   if err := crawlerService.ProcessQueueRequests(); err != nil {
							   log.Printf("Failed to process queue requests: %v", err)
					   }
			   }
	   }()

	   // Run continuously
	   runCrawlScheduler(crawlerService, *feedID, *interval)
}

func printHelp() {
	log.Println("StrandNerd Inspiration Feeds Crawler")
	log.Println()
	log.Println("Usage:")
	log.Println("  crawler [options]")
	log.Println()
	log.Println("Options:")
	log.Println("  -once           Run crawl once and exit")
	log.Println("  -feed <id>      Crawl specific feed ID only")
	log.Println("  -interval <sec> Crawl interval in seconds (default: 300)")
	log.Println("  -help           Show this help message")
	log.Println()
	log.Println("Environment Variables:")
	log.Println("  CMS_BASE_URL              CMS API base URL (required)")
	log.Println("  ACCESS_TOKEN              CMS access token for API authentication (required)")
	log.Println("  OPENAI_API_KEY            OpenAI API key for content analysis (optional)")
	log.Println("  ENABLE_CONTENT_ANALYSIS   Enable GPT content analysis (default: true)")
	log.Println("  LOG_LEVEL                 Log level (debug, info, warn, error) (default: info)")
	log.Println()
	log.Println("Examples:")
	log.Println("  # Run once and exit")
	log.Println("  crawler -once")
	log.Println()
	log.Println("  # Run specific feed once")
	log.Println("  crawler -once -feed abc123")
	log.Println()
	log.Println("  # Run continuously every 10 minutes")
	log.Println("  crawler -interval 600")
}

func runCrawlOnce(crawlerService *crawler.Service, feedID string) {
	// First, check for any queue requests (higher priority)
	if err := crawlerService.ProcessQueueRequests(); err != nil {
		log.Printf("Failed to process queue requests: %v", err)
	}

	log.Println("Running crawl once...")

	if feedID != "" {
		// Crawl specific feed
		result, err := crawlerService.CrawlFeed(feedID)
		if err != nil {
			log.Fatalf("Failed to crawl feed %s: %v", feedID, err)
		}
		printCrawlResult(result)

		if !result.Success {
			os.Exit(1)
		}
	} else {
		// Crawl all due feeds
		results, err := crawlerService.CrawlAllDueFeeds()
		if err != nil {
			log.Fatalf("Failed to crawl feeds: %v", err)
		}

		successCount := 0
		errorCount := 0
		totalPosts := 0

		for _, result := range results {
			printCrawlResult(&result)
			if result.Success {
				successCount++
				totalPosts += result.PostsAdded
			} else {
				errorCount++
			}
		}

		log.Printf("Crawl summary: %d feeds processed, %d successful, %d errors, %d total posts added",
			len(results), successCount, errorCount, totalPosts)

		if errorCount > 0 {
			os.Exit(1)
		}
	}

	log.Println("Crawl completed successfully")
}

func runCrawlScheduler(crawlerService *crawler.Service, feedID string, intervalSec int) {
	log.Printf("Starting crawler scheduler (interval: %d seconds)", intervalSec)

	ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
	defer ticker.Stop()

	// Run initial crawl
	runScheduledCrawl(crawlerService, feedID)

	// Run on schedule
	for range ticker.C {
		runScheduledCrawl(crawlerService, feedID)
	}
}

func runScheduledCrawl(crawlerService *crawler.Service, feedID string) {
	// First, check for any queue requests (higher priority)
	if err := crawlerService.ProcessQueueRequests(); err != nil {
		log.Printf("Failed to process queue requests: %v", err)
	}

	log.Println("Running scheduled crawl...")

	if feedID != "" {
		// Crawl specific feed
		result, err := crawlerService.CrawlFeed(feedID)
		if err != nil {
			log.Printf("Failed to crawl feed %s: %v", feedID, err)
			return
		}
		printCrawlResult(result)
	} else {
		// Crawl all due feeds
		results, err := crawlerService.CrawlAllDueFeeds()
		if err != nil {
			log.Printf("Failed to crawl feeds: %v", err)
			return
		}

		if len(results) == 0 {
			log.Println("No feeds due for crawling")
			return
		}

		successCount := 0
		errorCount := 0
		totalPosts := 0

		for _, result := range results {
			printCrawlResult(&result)
			if result.Success {
				successCount++
				totalPosts += result.PostsAdded
			} else {
				errorCount++
			}
		}

		log.Printf("Crawl summary: %d feeds processed, %d successful, %d errors, %d total posts added",
			len(results), successCount, errorCount, totalPosts)
	}
}

func printCrawlResult(result *models.CrawlResult) {
	if result.Success {
		log.Printf("✓ Feed %s: %d found, %d added, %d skipped",
			result.FeedID, result.PostsFound, result.PostsAdded, result.PostsSkipped)
	} else {
		log.Printf("✗ Feed %s failed: %v", result.FeedID, result.Error)
	}
}
