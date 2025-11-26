package txrepo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// GitHubRepoConfig represents configuration for loading guardrails from GitHub
// This is TENANT-SCOPED - each tenant provides their own GitHub repo
type GitHubRepoConfig struct {
	TenantID     string   // Tenant ID (for multi-tenant isolation)
	RepoOwner    string   // GitHub repository owner (e.g., "acme-corp")
	RepoName     string   // GitHub repository name (e.g., "acme-guardrails")
	Branch       string   // Branch to load from (e.g., "main", "v1.0")
	Categories   []string // Categories to load (e.g., "default", "custom")
	CacheMinutes int      // Cache duration in minutes (default: 60)
	GithubToken  string   // Optional: GitHub Personal Access Token for private repos
}

// MultiTenantGitHubLoader manages loading guardrails from multiple tenant GitHub repos
type MultiTenantGitHubLoader struct {
	tenantConfigs map[string]GitHubRepoConfig // tenantID -> config
	cache         map[string]*cachedGuardrail // "tenantID/category/filename" -> cached guardrail
	mu            sync.RWMutex
}

type cachedGuardrail struct {
	config    *GuardrailConfig
	loadedAt  time.Time
	expiresAt time.Time
}

var (
	globalLoader     *MultiTenantGitHubLoader
	globalLoaderOnce sync.Once
)

// GetMultiTenantLoader returns the global multi-tenant GitHub loader
func GetMultiTenantLoader() *MultiTenantGitHubLoader {
	globalLoaderOnce.Do(func() {
		globalLoader = &MultiTenantGitHubLoader{
			tenantConfigs: make(map[string]GitHubRepoConfig),
			cache:         make(map[string]*cachedGuardrail),
		}
	})
	return globalLoader
}

// RegisterTenantRepo registers a tenant's GitHub repository configuration
// This is called when a tenant provides their GitHub repo URL in the portal
func (l *MultiTenantGitHubLoader) RegisterTenantRepo(config GitHubRepoConfig) error {
	if config.TenantID == "" {
		return fmt.Errorf("tenant ID is required")
	}
	if config.RepoOwner == "" || config.RepoName == "" {
		return fmt.Errorf("GitHub repo owner and name are required")
	}

	// Set defaults
	if config.CacheMinutes == 0 {
		config.CacheMinutes = 60 // Default 1 hour cache
	}
	if config.Branch == "" {
		config.Branch = "main" // Default branch
	}
	if len(config.Categories) == 0 {
		config.Categories = []string{"default"}
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.tenantConfigs[config.TenantID] = config

	log.WithFields(log.Fields{
		"tenant_id": config.TenantID,
		"repo":      fmt.Sprintf("%s/%s", config.RepoOwner, config.RepoName),
		"branch":    config.Branch,
	}).Info("✅ Tenant GitHub repository registered")

	return nil
}

// GetTenantConfig returns the GitHub configuration for a tenant
func (l *MultiTenantGitHubLoader) GetTenantConfig(tenantID string) (GitHubRepoConfig, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	config, exists := l.tenantConfigs[tenantID]
	return config, exists
}

// LoadGuardrailsFromGitHub loads guardrails for a specific tenant from their GitHub repo
// This enables:
// 1. Tenant-isolated guardrails (each tenant has their own repo)
// 2. Version-controlled guardrails (tenants control their own versions)
// 3. Bring Your Own Guardrails (BYOG) - like BYOK, but for feature extractors
// 4. Easy updates without platform redeployment
func LoadGuardrailsFromGitHub(config GitHubRepoConfig) error {
	if config.TenantID == "" {
		return fmt.Errorf("tenant ID is required for GitHub loading")
	}
	if config.RepoOwner == "" || config.RepoName == "" {
		log.Warn("GitHub repository not configured, skipping GitHub-based guardrail loading")
		return nil
	}

	loader := GetMultiTenantLoader()

	// Register tenant repo if not already registered
	if err := loader.RegisterTenantRepo(config); err != nil {
		return fmt.Errorf("failed to register tenant repo: %w", err)
	}

	// Default to loading from "default" category if none specified
	if len(config.Categories) == 0 {
		config.Categories = []string{"default"}
	}

	log.WithFields(log.Fields{
		"tenant_id":  config.TenantID,
		"repo":       fmt.Sprintf("%s/%s", config.RepoOwner, config.RepoName),
		"branch":     config.Branch,
		"categories": config.Categories,
	}).Info("Loading tenant guardrails from GitHub...")

	totalLoaded := 0
	totalErrors := 0

	// Load guardrails from each category
	for _, category := range config.Categories {
		files, err := loader.listGuardrailFiles(config, category)
		if err != nil {
			log.WithError(err).Errorf("Failed to list guardrails in category: %s", category)
			totalErrors++
			continue
		}

		for _, filename := range files {
			if err := loader.loadGuardrailFromGitHub(config, category, filename); err != nil {
				log.WithError(err).Errorf("Failed to load guardrail: %s/%s", category, filename)
				totalErrors++
			} else {
				totalLoaded++
			}
		}
	}

	log.WithFields(log.Fields{
		"tenant_id": config.TenantID,
		"loaded":    totalLoaded,
		"errors":    totalErrors,
	}).Info("Tenant GitHub guardrail loading complete")

	if totalLoaded == 0 && totalErrors == 0 {
		log.Warn("No guardrails found in tenant GitHub repository")
	}

	return nil
}

// LoadAllTenantGuardrails loads guardrails for all registered tenants
// This is called at gateway boot to load all tenant-specific guardrails
func LoadAllTenantGuardrails() error {
	loader := GetMultiTenantLoader()
	
	loader.mu.RLock()
	tenantConfigs := make([]GitHubRepoConfig, 0, len(loader.tenantConfigs))
	for _, config := range loader.tenantConfigs {
		tenantConfigs = append(tenantConfigs, config)
	}
	loader.mu.RUnlock()

	if len(tenantConfigs) == 0 {
		log.Info("No tenant GitHub repositories registered")
		return nil
	}

	log.WithField("tenant_count", len(tenantConfigs)).Info("Loading guardrails for all tenants...")

	totalLoaded := 0
	totalErrors := 0

	for _, config := range tenantConfigs {
		if err := LoadGuardrailsFromGitHub(config); err != nil {
			log.WithError(err).Errorf("Failed to load guardrails for tenant: %s", config.TenantID)
			totalErrors++
		} else {
			totalLoaded++
		}
	}

	log.WithFields(log.Fields{
		"tenants_loaded": totalLoaded,
		"tenants_failed": totalErrors,
	}).Info("✅ All tenant guardrails loaded")

	return nil
}

// listGuardrailFiles lists all JSON files in a category directory on GitHub
func (l *MultiTenantGitHubLoader) listGuardrailFiles(config GitHubRepoConfig, category string) ([]string, error) {
	// GitHub API endpoint for listing directory contents
	// https://api.github.com/repos/{owner}/{repo}/contents/{path}?ref={branch}
	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/contents/guardrail_repo/%s?ref=%s",
		config.RepoOwner,
		config.RepoName,
		category,
		config.Branch,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add GitHub token if provided (for private repos or higher rate limits)
	if config.GithubToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", config.GithubToken))
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch directory listing: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		log.Warnf("Category directory not found on GitHub: %s", category)
		return []string{}, nil
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse directory listing
	var items []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("failed to parse directory listing: %w", err)
	}

	// Filter for JSON files
	var jsonFiles []string
	for _, item := range items {
		if item.Type == "file" && strings.HasSuffix(item.Name, ".json") {
			jsonFiles = append(jsonFiles, item.Name)
		}
	}

	return jsonFiles, nil
}

// loadGuardrailFromGitHub loads a single guardrail file from GitHub
func (l *MultiTenantGitHubLoader) loadGuardrailFromGitHub(config GitHubRepoConfig, category, filename string) error {
	cacheKey := fmt.Sprintf("%s/%s/%s", config.TenantID, category, filename)

	// Check cache first
	l.mu.RLock()
	cached, exists := l.cache[cacheKey]
	l.mu.RUnlock()

	if exists && time.Now().Before(cached.expiresAt) {
		log.WithFields(log.Fields{
			"tenant_id": config.TenantID,
			"file":      cacheKey,
			"cached":    true,
			"age":       time.Since(cached.loadedAt).Round(time.Second),
		}).Debug("Using cached guardrail from GitHub")
		
		// Re-register from cache (in case registry was cleared)
		return RegisterConfig(cached.config)
	}

	// Fetch from GitHub raw content
	// https://raw.githubusercontent.com/{owner}/{repo}/{branch}/guardrail_repo/{category}/{filename}
	url := fmt.Sprintf(
		"https://raw.githubusercontent.com/%s/%s/%s/guardrail_repo/%s/%s",
		config.RepoOwner,
		config.RepoName,
		config.Branch,
		category,
		filename,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add GitHub token if provided (for private repos)
	if config.GithubToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", config.GithubToken))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch guardrail: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub fetch error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse guardrail config
	var guardrailCfg GuardrailConfig
	if err := json.NewDecoder(resp.Body).Decode(&guardrailCfg); err != nil {
		return fmt.Errorf("failed to parse guardrail JSON: %w", err)
	}

	// Validate required fields
	if guardrailCfg.ID == "" {
		return fmt.Errorf("guardrail ID is required")
	}
	if guardrailCfg.Name == "" {
		return fmt.Errorf("guardrail name is required")
	}
	if guardrailCfg.MetricName == "" {
		guardrailCfg.MetricName = "compliance_score" // Default metric name
	}

	// Set type if not specified
	if guardrailCfg.Type == "" {
		guardrailCfg.Type = "dynamic"
	}

	// Set registered_at if not specified
	if guardrailCfg.RegisteredAt == "" {
		guardrailCfg.RegisteredAt = time.Now().UTC().Format(time.RFC3339)
	}

	// Register the guardrail
	logger := log.WithFields(log.Fields{
		"tenant_id": config.TenantID,
		"id":        guardrailCfg.ID,
		"name":      guardrailCfg.Name,
		"category":  category,
		"source":    "github",
		"url":       url,
	})

	// Check if already registered (might be a duplicate or platform extractor)
	// Platform extractors (tenant="platform") take precedence
	// Tenant extractors can extend but not override platform ones
	if existingFn, _, _ := GetGuardrail(guardrailCfg.ID); existingFn != nil {
		if config.TenantID == "platform" {
			logger.Debug("Platform guardrail already registered (built-in Go extractor)")
			return nil
		}
		logger.Warn("Guardrail already registered, skipping (tenant guardrails don't override platform ones)")
		return nil
	}

	// Register to dynamic registry
	if err := RegisterConfig(&guardrailCfg); err != nil {
		return fmt.Errorf("failed to register guardrail: %w", err)
	}

	// Update cache
	l.mu.Lock()
	l.cache[cacheKey] = &cachedGuardrail{
		config:    &guardrailCfg,
		loadedAt:  time.Now(),
		expiresAt: time.Now().Add(time.Duration(config.CacheMinutes) * time.Minute),
	}
	l.mu.Unlock()

	logger.WithFields(log.Fields{
		"metric":    guardrailCfg.MetricName,
		"threshold": guardrailCfg.Threshold,
		"type":      guardrailCfg.Type,
	}).Info("✅ Guardrail loaded from GitHub")

	return nil
}

// RefreshCache clears the cache and forces a reload from GitHub
func (l *MultiTenantGitHubLoader) RefreshCache() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cache = make(map[string]*cachedGuardrail)
	log.Info("GitHub guardrail cache cleared for all tenants")
}

// RefreshTenantCache clears the cache for a specific tenant
func (l *MultiTenantGitHubLoader) RefreshTenantCache(tenantID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	// Remove cache entries for this tenant
	for key := range l.cache {
		if strings.HasPrefix(key, tenantID+"/") {
			delete(l.cache, key)
		}
	}
	log.WithField("tenant_id", tenantID).Info("GitHub guardrail cache cleared for tenant")
}

// GetCacheStats returns cache statistics
func (l *MultiTenantGitHubLoader) GetCacheStats() map[string]interface{} {
	l.mu.RLock()
	defer l.mu.RUnlock()

	cached := 0
	expired := 0
	now := time.Now()
	
	tenantCounts := make(map[string]int)

	for key, item := range l.cache {
		// Extract tenant ID from cache key (format: "tenantID/category/filename")
		parts := strings.SplitN(key, "/", 2)
		if len(parts) > 0 {
			tenantCounts[parts[0]]++
		}
		
		if now.Before(item.expiresAt) {
			cached++
		} else {
			expired++
		}
	}

	return map[string]interface{}{
		"total_cached":    len(l.cache),
		"active":          cached,
		"expired":         expired,
		"tenants":         len(l.tenantConfigs),
		"per_tenant":      tenantCounts,
	}
}

