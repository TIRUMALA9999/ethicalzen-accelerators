package gateway

import (
	"context"
	"time"

	"github.com/ethicalzen/acvps-gateway/internal/cache"
	log "github.com/sirupsen/logrus"
)

// StartAutoReload periodically reloads contracts from Redis
// This ensures new/updated contracts are picked up without gateway restart
func StartAutoReload(ctx context.Context, cacheClient *cache.Client, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Infof("🔄 Auto-reload enabled: checking for contract updates every %v", interval)

	for {
		select {
		case <-ctx.Done():
			log.Info("Auto-reload stopped")
			return
		case <-ticker.C:
			log.Debug("🔍 Checking for contract updates...")
			if err := ReloadContracts(ctx, cacheClient); err != nil {
				log.WithError(err).Warn("Failed to reload contracts")
			}
		}
	}
}

// ReloadContracts reloads all contracts from Redis, adding new ones and updating existing
func ReloadContracts(ctx context.Context, cacheClient *cache.Client) error {
	// Load contracts (same logic as boot, but doesn't clear existing)
	// This is idempotent - LoadContract checks if already loaded
	return LoadAllContractsAtBoot(ctx, cacheClient)
}

// ReloadContractsForTenant reloads contracts for a specific tenant
// Currently just calls the full reload (simpler and safer)
func ReloadContractsForTenant(ctx context.Context, cacheClient *cache.Client, tenantID string) error {
	log.Infof("🔄 Reloading contracts for tenant: %s", tenantID)
	
	// For now, just reload all contracts (tenant filtering happens in boot.go)
	// In production, you could optimize this to only reload specific tenant
	return LoadAllContractsAtBoot(ctx, cacheClient)
}

