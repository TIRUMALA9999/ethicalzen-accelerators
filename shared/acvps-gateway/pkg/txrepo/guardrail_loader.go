package txrepo

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// GuardrailRepoConfig represents the configuration for loading guardrails from filesystem
type GuardrailRepoConfig struct {
	RepositoryPath string   // Path to guardrail_repo directory
	Categories     []string // Categories to load (e.g., "default", "custom")
	AutoReload     bool     // Auto-reload on file changes (future)
}

// LoadGuardrailsFromRepository loads all guardrails from the filesystem repository
// This is called during gateway startup to register all persistent guardrails
func LoadGuardrailsFromRepository(config GuardrailRepoConfig) error {
	if config.RepositoryPath == "" {
		log.Warn("Guardrail repository path not configured, skipping file-based guardrail loading")
		return nil
	}

	// Check if repository exists
	if _, err := os.Stat(config.RepositoryPath); os.IsNotExist(err) {
		log.Warnf("Guardrail repository not found at: %s", config.RepositoryPath)
		return nil
	}

	// Default to loading from "default" category if none specified
	if len(config.Categories) == 0 {
		config.Categories = []string{"default"}
	}

	log.WithFields(log.Fields{
		"repository": config.RepositoryPath,
		"categories": config.Categories,
	}).Info("Loading guardrails from repository...")

	totalLoaded := 0
	totalErrors := 0

	// Load guardrails from each category
	for _, category := range config.Categories {
		categoryPath := filepath.Join(config.RepositoryPath, category)

		// Check if category directory exists
		if _, err := os.Stat(categoryPath); os.IsNotExist(err) {
			log.Warnf("Category directory not found: %s", categoryPath)
			continue
		}

		// Read all JSON files in the category
		files, err := ioutil.ReadDir(categoryPath)
		if err != nil {
			log.WithError(err).Errorf("Failed to read category directory: %s", categoryPath)
			totalErrors++
			continue
		}

		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
				continue
			}

			filePath := filepath.Join(categoryPath, file.Name())
			if err := loadGuardrailFile(filePath, category); err != nil {
				log.WithError(err).Errorf("Failed to load guardrail from: %s", filePath)
				totalErrors++
			} else {
				totalLoaded++
			}
		}
	}

	log.WithFields(log.Fields{
		"loaded": totalLoaded,
		"errors": totalErrors,
	}).Info("Guardrail repository loading complete")

	if totalLoaded == 0 && totalErrors == 0 {
		log.Warn("No guardrails found in repository")
	}

	return nil
}

// loadGuardrailFile loads a single guardrail configuration file
func loadGuardrailFile(filePath string, category string) error {
	// Read file
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Parse JSON
	var config GuardrailConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate required fields
	if config.ID == "" {
		return fmt.Errorf("guardrail ID is required")
	}
	if config.Name == "" {
		return fmt.Errorf("guardrail name is required")
	}
	
	// Handle both legacy format (metric_name) and new schema (metrics)
	if config.MetricName == "" && len(config.Metrics) == 0 {
		config.MetricName = "compliance_score" // Default metric name for legacy format
	}
	
	// If new schema (metrics object exists), extract metric names and keywords for backward compatibility
	if len(config.Metrics) > 0 {
		// Use first metric as the primary metric for legacy code paths
		for metricName, metricDef := range config.Metrics {
			config.MetricName = metricName
			config.InvertScore = metricDef.InvertScore
			if metricDef.Threshold != nil {
				// For legacy threshold field, use the min value (most restrictive)
				config.Threshold = metricDef.Threshold.Min
			}
			break // Only need first metric for legacy compatibility
		}
		
		// Extract keywords from feature extractors for pattern-based fallback
		if len(config.Keywords) == 0 && len(config.FeatureExtractors) > 0 {
			for _, extractor := range config.FeatureExtractors {
				// Handle both "pattern" type and "hybrid" type (which has pattern_based nested)
				if extractor.Type == "pattern" {
					// Direct pattern type: keywords are at extractor.Extractor["keywords"]
					if keywords, ok := extractor.Extractor["keywords"].([]interface{}); ok {
						for _, kw := range keywords {
							if kwStr, ok := kw.(string); ok {
								config.Keywords = append(config.Keywords, kwStr)
							}
						}
					}
				} else if extractor.Type == "hybrid" {
					// Hybrid type: keywords are at extractor.Extractor["pattern_based"]["keywords"]
					if patternBased, ok := extractor.Extractor["pattern_based"].(map[string]interface{}); ok {
						if keywords, ok := patternBased["keywords"].([]interface{}); ok {
							for _, kw := range keywords {
								if kwStr, ok := kw.(string); ok {
									config.Keywords = append(config.Keywords, kwStr)
								}
							}
						}
					}
				}
			}
		}
	}

	// Set type if not specified
	if config.Type == "" {
		config.Type = "dynamic"
	}

	// Set registered_at if not specified
	if config.RegisteredAt == "" {
		config.RegisteredAt = time.Now().UTC().Format(time.RFC3339)
	}

	// Register the guardrail
	logger := log.WithFields(log.Fields{
		"id":       config.ID,
		"name":     config.Name,
		"category": category,
		"file":     filepath.Base(filePath),
	})

	// Check if already registered (might be a duplicate or built-in override)
	if existingFn, _, _ := GetGuardrail(config.ID); existingFn != nil {
		logger.Warn("Guardrail already registered, skipping (file-based guardrails don't override built-in ones)")
		return nil
	}

	// Register to dynamic registry
	if err := RegisterConfig(&config); err != nil {
		return fmt.Errorf("failed to register guardrail: %w", err)
	}

	logger.WithFields(log.Fields{
		"metric":    config.MetricName,
		"threshold": config.Threshold,
		"type":      config.Type,
	}).Info("✅ Guardrail loaded from repository")

	return nil
}

// SaveGuardrailToRepository saves a guardrail configuration to the filesystem repository
// This is called when a guardrail is deployed via the GDK
func SaveGuardrailToRepository(config *GuardrailConfig, repositoryPath string) error {
	if repositoryPath == "" {
		return fmt.Errorf("repository path not configured")
	}

	// Determine category (default for now, could be customizable)
	category := "default"
	if config.Type == "custom" {
		category = "custom"
	}

	categoryPath := filepath.Join(repositoryPath, category)

	// Create category directory if it doesn't exist
	if err := os.MkdirAll(categoryPath, 0755); err != nil {
		return fmt.Errorf("failed to create category directory: %w", err)
	}

	// Set metadata
	if config.RegisteredAt == "" {
		config.RegisteredAt = time.Now().UTC().Format(time.RFC3339)
	}
	if config.Type == "" {
		config.Type = "dynamic"
	}

	// Serialize to JSON (pretty-printed)
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize guardrail: %w", err)
	}

	// Write to file
	filePath := filepath.Join(categoryPath, config.ID+".json")
	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	log.WithFields(log.Fields{
		"id":       config.ID,
		"category": category,
		"file":     filePath,
	}).Info("💾 Guardrail saved to repository")

	return nil
}

// DeleteGuardrailFromRepository removes a guardrail configuration file
func DeleteGuardrailFromRepository(guardrailID string, repositoryPath string) error {
	if repositoryPath == "" {
		return fmt.Errorf("repository path not configured")
	}

	// Check both default and custom categories
	categories := []string{"default", "custom"}
	deleted := false

	for _, category := range categories {
		filePath := filepath.Join(repositoryPath, category, guardrailID+".json")
		if _, err := os.Stat(filePath); err == nil {
			if err := os.Remove(filePath); err != nil {
				return fmt.Errorf("failed to delete file: %w", err)
			}
			log.WithFields(log.Fields{
				"id":       guardrailID,
				"category": category,
				"file":     filePath,
			}).Info("🗑️ Guardrail deleted from repository")
			deleted = true
		}
	}

	if !deleted {
		return fmt.Errorf("guardrail file not found: %s", guardrailID)
	}

	return nil
}

// ListGuardrailFiles returns a list of all guardrail files in the repository
func ListGuardrailFiles(repositoryPath string) ([]string, error) {
	if repositoryPath == "" {
		return nil, fmt.Errorf("repository path not configured")
	}

	var files []string
	categories := []string{"default", "custom"}

	for _, category := range categories {
		categoryPath := filepath.Join(repositoryPath, category)
		if _, err := os.Stat(categoryPath); os.IsNotExist(err) {
			continue
		}

		dirFiles, err := ioutil.ReadDir(categoryPath)
		if err != nil {
			log.WithError(err).Warnf("Failed to read category: %s", category)
			continue
		}

		for _, file := range dirFiles {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
				files = append(files, filepath.Join(category, file.Name()))
			}
		}
	}

	return files, nil
}

