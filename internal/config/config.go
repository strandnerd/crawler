package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// TenantConfig holds configuration for a single tenant
type TenantConfig struct {
	ID                string `yaml:"id"`
	Name              string `yaml:"name"`
	CMSBaseURL        string `yaml:"cms_base_url"`
	AccessToken       string `yaml:"access_token"`
	Enabled           bool   `yaml:"enabled"`
	CrawlInterval     *int   `yaml:"crawl_interval,omitempty"`     // Optional: tenant-specific crawl interval
	MaxPostsPerCrawl  *int   `yaml:"max_posts_per_crawl,omitempty"` // Optional: tenant-specific limit
}

// GlobalConfig holds global configuration settings
type GlobalConfig struct {
	LogLevel              string `yaml:"log_level,omitempty"`
	FeedRefreshInterval   *int   `yaml:"feed_refresh_interval,omitempty"`
	RequestTimeout        *int   `yaml:"request_timeout,omitempty"`
	MaxConcurrentCrawls   *int   `yaml:"max_concurrent_crawls,omitempty"`
	UserAgent             string `yaml:"user_agent,omitempty"`
	OpenAIAPIKey          string `yaml:"openai_api_key,omitempty"`
	EnableContentAnalysis *bool  `yaml:"enable_content_analysis,omitempty"`
	ProxyHost             string `yaml:"proxy_host,omitempty"`
	ProxyAuth             string `yaml:"proxy_auth,omitempty"`
}

// YAMLConfig represents the YAML configuration file structure
type YAMLConfig struct {
	Tenants []TenantConfig `yaml:"tenants"`
	Global  GlobalConfig   `yaml:"global,omitempty"`
}

// Config holds all configuration for the crawler
type Config struct {
	Tenants               []TenantConfig
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

// Load loads configuration from YAML file or environment variables
func Load() (*Config, error) {
	// Try to load from YAML file first
	yamlConfig, err := loadYAMLConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load YAML configuration: %w", err)
	}

	// Initialize config with defaults, then override with YAML global settings and environment variables
	cfg := &Config{
		LogLevel:              getConfigValue(yamlConfig.Global.LogLevel, "LOG_LEVEL", "info"),
		FeedRefreshInterval:   getConfigIntValue(yamlConfig.Global.FeedRefreshInterval, "FEED_REFRESH_INTERVAL", 5),
		RequestTimeout:        getConfigIntValue(yamlConfig.Global.RequestTimeout, "REQUEST_TIMEOUT", 30),
		MaxConcurrentCrawls:   getConfigIntValue(yamlConfig.Global.MaxConcurrentCrawls, "MAX_CONCURRENT_CRAWLS", 3),
		UserAgent:             getConfigValue(yamlConfig.Global.UserAgent, "USER_AGENT", "StrandNerd-Crawler/1.0"),
		OpenAIAPIKey:          getConfigValue(yamlConfig.Global.OpenAIAPIKey, "OPENAI_API_KEY", ""),
		EnableContentAnalysis: getConfigBoolValue(yamlConfig.Global.EnableContentAnalysis, "ENABLE_CONTENT_ANALYSIS", true),
		ProxyHost:             getConfigValue(yamlConfig.Global.ProxyHost, "PROXY_HOST", ""),
		ProxyAuth:             getConfigValue(yamlConfig.Global.ProxyAuth, "PROXY_AUTH", ""),
	}

	// Load tenant configurations
	tenants, err := loadTenants(yamlConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load tenant configurations: %w", err)
	}
	cfg.Tenants = tenants

	if len(cfg.Tenants) == 0 {
		return nil, fmt.Errorf("no tenants configured - at least one tenant is required")
	}

	// OpenAI API key is optional - if not provided, content analysis will be skipped
	if cfg.EnableContentAnalysis && cfg.OpenAIAPIKey == "" {
		fmt.Println("Warning: OPENAI_API_KEY not provided, content analysis will be disabled")
		cfg.EnableContentAnalysis = false
	}

	return cfg, nil
}

// loadYAMLConfig loads configuration from tenants.yml file
func loadYAMLConfig() (*YAMLConfig, error) {
	// Try to read tenants.yml file
	filename := "tenants.yml"
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, return empty config (will fall back to environment variables)
			return &YAMLConfig{}, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", filename, err)
	}

	var config YAMLConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filename, err)
	}

	return &config, nil
}

// loadTenants loads tenant configurations from YAML file or environment variables
// Priority: YAML file > Environment variables (for backward compatibility)
func loadTenants(yamlConfig *YAMLConfig) ([]TenantConfig, error) {
	var tenants []TenantConfig

	// If YAML config has tenants, use them
	if len(yamlConfig.Tenants) > 0 {
		for _, tenant := range yamlConfig.Tenants {
			if tenant.CMSBaseURL == "" {
				return nil, fmt.Errorf("tenant %s: cms_base_url is required", tenant.ID)
			}
			if tenant.AccessToken == "" {
				return nil, fmt.Errorf("tenant %s: access_token is required", tenant.ID)
			}
			
			// Set default enabled state if not specified
			if !tenant.Enabled {
				// Skip disabled tenants
				continue
			}
			
			tenants = append(tenants, tenant)
		}
		return tenants, nil
	}

	// Fall back to environment variables for backward compatibility
	return loadTenantsFromEnv()
}

// loadTenantsFromEnv loads tenant configurations from environment variables (legacy support)
func loadTenantsFromEnv() ([]TenantConfig, error) {
	var tenants []TenantConfig

	// Check for legacy single tenant configuration first
	legacyCMSURL := getEnv("CMS_BASE_URL", "")
	legacyAccessToken := getEnv("ACCESS_TOKEN", "")

	if legacyCMSURL != "" && legacyAccessToken != "" {
		tenants = append(tenants, TenantConfig{
			ID:          "default",
			Name:        "Default Tenant",
			CMSBaseURL:  legacyCMSURL,
			AccessToken: legacyAccessToken,
			Enabled:     true,
		})
		return tenants, nil
	}

	// Look for multi-tenant configuration
	for i := 1; i <= 50; i++ { // Support up to 50 tenants
		prefix := fmt.Sprintf("TENANT_%d_", i)
		cmsURL := getEnv(fmt.Sprintf("%sCMS_BASE_URL", prefix), "")
		accessToken := getEnv(fmt.Sprintf("%sACCESS_TOKEN", prefix), "")

		if cmsURL == "" && accessToken == "" {
			continue // Skip empty tenant slots
		}

		if cmsURL == "" {
			return nil, fmt.Errorf("tenant %d: CMS_BASE_URL is required when ACCESS_TOKEN is provided", i)
		}
		if accessToken == "" {
			return nil, fmt.Errorf("tenant %d: ACCESS_TOKEN is required when CMS_BASE_URL is provided", i)
		}

		tenants = append(tenants, TenantConfig{
			ID:          fmt.Sprintf("tenant_%d", i),
			Name:        fmt.Sprintf("Tenant %d", i),
			CMSBaseURL:  cmsURL,
			AccessToken: accessToken,
			Enabled:     true,
		})
	}

	return tenants, nil
}

// getConfigValue returns YAML value if not empty, otherwise environment variable, otherwise default
func getConfigValue(yamlValue, envKey, defaultValue string) string {
	if yamlValue != "" {
		return yamlValue
	}
	return getEnv(envKey, defaultValue)
}

// getConfigIntValue returns YAML value if not nil, otherwise environment variable, otherwise default
func getConfigIntValue(yamlValue *int, envKey string, defaultValue int) int {
	if yamlValue != nil {
		return *yamlValue
	}
	return getEnvInt(envKey, defaultValue)
}

// getConfigBoolValue returns YAML value if not nil, otherwise environment variable, otherwise default
func getConfigBoolValue(yamlValue *bool, envKey string, defaultValue bool) bool {
	if yamlValue != nil {
		return *yamlValue
	}
	envValue := getEnv(envKey, "")
	if envValue != "" {
		return envValue == "true"
	}
	return defaultValue
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
