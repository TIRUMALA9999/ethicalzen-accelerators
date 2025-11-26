package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

// ApiKeyValidator validates API keys against Redis
type ApiKeyValidator struct {
	redis *redis.Client
}

// ApiKeyData represents the stored API key data
type ApiKeyData struct {
	TenantID      string `json:"tenant_id"`
	LiveKeyHash   string `json:"live_key_hash"`
	LiveKeyPrefix string `json:"live_key_prefix"`
	TestKeyHash   string `json:"test_key_hash"`
	TestKeyPrefix string `json:"test_key_prefix"`
	CreatedAt     string `json:"created_at"`
	LastUsed      string `json:"last_used"`
	Status        string `json:"status"`
}

// NewApiKeyValidator creates a new API key validator
func NewApiKeyValidator(redisClient *redis.Client) *ApiKeyValidator {
	return &ApiKeyValidator{
		redis: redisClient,
	}
}

// ValidateApiKey validates an API key and returns the tenant ID
func (v *ApiKeyValidator) ValidateApiKey(apiKey string) (string, error) {
	// Support both formats:
	// 1. Legacy: acvps_{live|test}_sk_{64 hex chars}
	// 2. New: sk-{hex chars} (from backend)
	
	if strings.HasPrefix(apiKey, "sk-") {
		// Check for demo/playground key first (hardcoded for local testing)
		if apiKey == "sk-demo-public-playground-ethicalzen" {
			log.Debug("Demo API key validated (hardcoded mapping)")
			return "demo", nil
		}
		
		// Check if Redis is available
		if v.redis == nil {
			log.Debug("Redis not available, rejecting API key (no in-memory mapping)")
			return "", fmt.Errorf("invalid API key: storage not available")
		}
		
		// New format from backend: sk-{hex}
		// Look up tenant directly in Redis using backend's key format
		ctx := context.Background()
		tenantID, err := v.redis.Get(ctx, fmt.Sprintf("tenant:%s", apiKey)).Result()
		if err != nil {
			log.WithField("api_key", apiKey[:10]+"...").WithError(err).Debug("API key not found in Redis (backend format)")
			return "", fmt.Errorf("invalid API key")
		}
		log.WithFields(log.Fields{
			"tenant_id": tenantID,
			"key_type": "backend",
		}).Debug("API key validated (backend format)")
		return tenantID, nil
	}
	
	if !strings.HasPrefix(apiKey, "acvps_") {
		return "", fmt.Errorf("invalid API key format")
	}

	parts := strings.Split(apiKey, "_")
	if len(parts) != 4 || parts[2] != "sk" || len(parts[3]) != 64 {
		return "", fmt.Errorf("invalid API key format")
	}

	environment := parts[1]
	if environment != "live" && environment != "test" {
		return "", fmt.Errorf("invalid API key environment")
	}

	// Hash the API key
	keyHash := hashApiKey(apiKey)

	// Lookup tenant by key hash in Redis
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tenantID, err := v.redis.Get(ctx, fmt.Sprintf("apikey:hash:%s", keyHash)).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("API key not found")
	} else if err != nil {
		log.WithError(err).Error("Redis lookup failed")
		return "", fmt.Errorf("validation failed")
	}
	
	// Note: tenantID is now set from Redis lookup

	// Get tenant key data to check status
	keyDataStr, err := v.redis.Get(ctx, fmt.Sprintf("apikey:tenant:%s", tenantID)).Result()
	if err != nil {
		log.WithError(err).Error("Failed to get tenant key data")
		return "", fmt.Errorf("tenant data not found")
	}

	var keyData ApiKeyData
	if err := json.Unmarshal([]byte(keyDataStr), &keyData); err != nil {
		log.WithError(err).Error("Failed to parse key data")
		return "", fmt.Errorf("invalid key data")
	}

	// Check if key is active
	if keyData.Status != "active" {
		return "", fmt.Errorf("API key revoked")
	}

	// Update last used timestamp (fire and forget)
	go func() {
		ctx := context.Background()
		keyData.LastUsed = time.Now().Format(time.RFC3339)
		updatedData, _ := json.Marshal(keyData)
		v.redis.Set(ctx, fmt.Sprintf("apikey:tenant:%s", tenantID), updatedData, 0)
	}()

	log.WithFields(log.Fields{
		"tenant_id":   tenantID,
		"environment": environment,
	}).Debug("API key validated")

	return tenantID, nil
}

// hashApiKey hashes an API key using SHA256
func hashApiKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}

