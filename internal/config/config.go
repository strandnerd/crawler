package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the crawler
type Config struct {
	CMSBaseURL            string
	AccessToken           string
	LogLevel              string
	FeedRefreshInterval   int // in minutes
	RequestTimeout        int // in seconds
	MaxConcurrentCrawls   int
	UserAgent             string
	OpenAIAPIKey          string
	EnableContentAnalysis bool
	ProxyHost             string
	ProxyAuth             string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		CMSBaseURL:            getEnv("CMS_BASE_URL", ""),
		AccessToken:           getEnv("ACCESS_TOKEN", ""),
		LogLevel:              getEnv("LOG_LEVEL", "info"),
		FeedRefreshInterval:   getEnvInt("FEED_REFRESH_INTERVAL", 5), // 5 minutes
		RequestTimeout:        getEnvInt("REQUEST_TIMEOUT", 30),      // 30 seconds
		MaxConcurrentCrawls:   getEnvInt("MAX_CONCURRENT_CRAWLS", 3),
		UserAgent:             getEnv("USER_AGENT", "StrandNerd-Crawler/1.0"),
		OpenAIAPIKey:          getEnv("OPENAI_API_KEY", ""),
		EnableContentAnalysis: getEnv("ENABLE_CONTENT_ANALYSIS", "true") == "true",
		ProxyHost:             getEnv("PROXY_HOST", ""),
		ProxyAuth:             getEnv("PROXY_AUTH", ""),
	}

	// Validate required fields
	if cfg.CMSBaseURL == "" {
		return nil, fmt.Errorf("CMS_BASE_URL environment variable is required")
	}
	if cfg.AccessToken == "" {
		return nil, fmt.Errorf("ACCESS_TOKEN environment variable is required")
	}

	// OpenAI API key is optional - if not provided, content analysis will be skipped
	if cfg.EnableContentAnalysis && cfg.OpenAIAPIKey == "" {
		fmt.Println("Warning: OPENAI_API_KEY not provided, content analysis will be disabled")
		cfg.EnableContentAnalysis = false
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
