package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the entire gateway configuration
type Config struct {
	Gateway    GatewayConfig    `yaml:"gateway"`
	Blockchain BlockchainConfig `yaml:"blockchain"`
	Backend    BackendConfig    `yaml:"backend"`
	Validation ValidationConfig `yaml:"validation"`
	Mitigation MitigationConfig `yaml:"mitigation"`
	Cache      CacheConfig      `yaml:"cache"`
	Evidence   EvidenceConfig   `yaml:"evidence"`
	Logging    LoggingConfig    `yaml:"logging"`
	Metrics    MetricsConfig    `yaml:"metrics"`
	RateLimit  RateLimitConfig  `yaml:"rate_limit"`
	CORS       CORSConfig       `yaml:"cors"`
	DevMode    bool             `yaml:"dev_mode"`
}

type GatewayConfig struct {
	Name            string    `yaml:"name"`
	Port            int       `yaml:"port"`
	MetricsPort     int       `yaml:"metrics_port"`
	TLS             TLSConfig `yaml:"tls"`
	Mode            string    `yaml:"mode"`              // "cloud" or "local"
	APIKey          string    `yaml:"api_key"`           // For local mode: authenticate to control plane
	TenantID        string    `yaml:"tenant_id"`         // For local mode: tenant scope
	ControlPlaneURL string    `yaml:"control_plane_url"` // For local mode: backend API URL
}

type TLSConfig struct {
	Enabled            bool   `yaml:"enabled"`
	Cert               string `yaml:"cert"`
	Key                string `yaml:"key"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

type BlockchainConfig struct {
	Enabled         bool   `yaml:"enabled"`
	RPCURL          string `yaml:"rpc_url"`
	ContractAddress string `yaml:"contract_address"`
	CacheTTL        int    `yaml:"cache_ttl"`
	Timeout         int    `yaml:"timeout"`
	MaxRetries      int    `yaml:"max_retries"`
	RetryDelay      int    `yaml:"retry_delay"`
}

type BackendConfig struct {
	URL             string `yaml:"url"`
	Timeout         int    `yaml:"timeout"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	MaxConnsPerHost int    `yaml:"max_conns_per_host"`
	IdleConnTimeout int    `yaml:"idle_conn_timeout"`
}

type ValidationConfig struct {
	Mode                   string   `yaml:"mode"`
	RequireDCHeaders       bool     `yaml:"require_dc_headers"`
	EnforceFailureModes    bool     `yaml:"enforce_failure_modes"`
	BlockchainVerification bool     `yaml:"blockchain_verification"`
	SkipHashVerification   bool     `yaml:"skip_hash_verification"`
	AllowedSuites          []string `yaml:"allowed_suites"`
}

type MitigationConfig struct {
	PIIRedaction  PIIRedactionConfig  `yaml:"pii_redaction"`
	Grounding     GroundingConfig     `yaml:"grounding"`
	Hallucination HallucinationConfig `yaml:"hallucination"`
}

type PIIRedactionConfig struct {
	Enabled      bool     `yaml:"enabled"`
	AlwaysRedact []string `yaml:"always_redact"`
}

type GroundingConfig struct {
	Enabled       bool    `yaml:"enabled"`
	MinConfidence float64 `yaml:"min_confidence"`
	Action        string  `yaml:"action"`
}

type HallucinationConfig struct {
	Enabled bool    `yaml:"enabled"`
	MaxRisk float64 `yaml:"max_risk"`
	Action  string  `yaml:"action"`
}

type CacheConfig struct {
	Enabled       bool   `yaml:"enabled"`
	RedisURL      string `yaml:"redis_url"`
	RedisPassword string `yaml:"redis_password"`
	RedisDB       int    `yaml:"redis_db"`
	PoolSize      int    `yaml:"pool_size"`
	MaxRetries    int    `yaml:"max_retries"`
}

type EvidenceConfig struct {
	ControlPlaneURL string `yaml:"control_plane_url"`
	APIKey          string `yaml:"api_key"`
	BufferSize      int    `yaml:"buffer_size"`
	FlushInterval   int    `yaml:"flush_interval"`
	Async           bool   `yaml:"async"`
}

type LoggingConfig struct {
	Level      string `yaml:"level"`
	Format     string `yaml:"format"`
	Output     string `yaml:"output"`
	File       string `yaml:"file"`
	MaxSize    int    `yaml:"max_size"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`
}

type MetricsConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Path      string `yaml:"path"`
	Namespace string `yaml:"namespace"`
}

type RateLimitConfig struct {
	Enabled bool `yaml:"enabled"`
	RPS     int  `yaml:"rps"`
	Burst   int  `yaml:"burst"`
}

type CORSConfig struct {
	Enabled        bool     `yaml:"enabled"`
	AllowedOrigins []string `yaml:"allowed_origins"`
	AllowedMethods []string `yaml:"allowed_methods"`
	AllowedHeaders []string `yaml:"allowed_headers"`
	MaxAge         int      `yaml:"maxage"`
}

// Load reads configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables
	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	setDefaults(&cfg)

	// Validate
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

func setDefaults(cfg *Config) {
	if cfg.Gateway.Port == 0 {
		cfg.Gateway.Port = 443
	}
	if cfg.Gateway.MetricsPort == 0 {
		cfg.Gateway.MetricsPort = 9090
	}
	if cfg.Gateway.Mode == "" {
		cfg.Gateway.Mode = "cloud" // Default to cloud mode
	}
	// Override from environment variables
	if mode := os.Getenv("GATEWAY_MODE"); mode != "" {
		cfg.Gateway.Mode = mode
	}
	if apiKey := os.Getenv("GATEWAY_API_KEY"); apiKey != "" {
		cfg.Gateway.APIKey = apiKey
	}
	if tenantID := os.Getenv("GATEWAY_TENANT_ID"); tenantID != "" {
		cfg.Gateway.TenantID = tenantID
	}
	if controlPlaneURL := os.Getenv("CONTROL_PLANE_URL"); controlPlaneURL != "" {
		cfg.Gateway.ControlPlaneURL = controlPlaneURL
	}
	if cfg.Blockchain.CacheTTL == 0 {
		cfg.Blockchain.CacheTTL = 300 // 5 minutes
	}
	if cfg.Blockchain.Timeout == 0 {
		cfg.Blockchain.Timeout = 10
	}
	if cfg.Cache.PoolSize == 0 {
		cfg.Cache.PoolSize = 100
	}

	// Override Redis URL from environment variables if set
	if redisHost := os.Getenv("REDIS_HOST"); redisHost != "" {
		redisPort := os.Getenv("REDIS_PORT")
		if redisPort == "" {
			redisPort = "6379"
		}
		cfg.Cache.RedisURL = fmt.Sprintf("redis://%s:%s", redisHost, redisPort)
	}
	if redisPassword := os.Getenv("REDIS_PASSWORD"); redisPassword != "" {
		cfg.Cache.RedisPassword = redisPassword
	}

	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "json"
	}
	if cfg.Logging.Output == "" {
		cfg.Logging.Output = "stdout"
	}
	if cfg.Metrics.Path == "" {
		cfg.Metrics.Path = "/metrics"
	}
	if cfg.Metrics.Namespace == "" {
		cfg.Metrics.Namespace = "acvps"
	}
	if cfg.Validation.Mode == "" {
		cfg.Validation.Mode = "strict"
	}
}

func validate(cfg *Config) error {
	// Only validate blockchain config if it's enabled
	if cfg.Blockchain.Enabled {
		if cfg.Blockchain.RPCURL == "" {
			return fmt.Errorf("blockchain.rpc_url is required when blockchain is enabled")
		}
		if cfg.Blockchain.ContractAddress == "" {
			return fmt.Errorf("blockchain.contract_address is required when blockchain is enabled")
		}
	}

	if cfg.Backend.URL == "" {
		return fmt.Errorf("backend.url is required")
	}

	// Only validate cache config if it's enabled
	if cfg.Cache.Enabled {
		if cfg.Cache.RedisURL == "" {
			return fmt.Errorf("cache.redis_url is required when cache is enabled")
		}
	}

	return nil
}
