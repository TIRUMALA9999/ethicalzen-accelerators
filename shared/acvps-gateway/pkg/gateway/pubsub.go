package gateway

import (
	"context"
	"encoding/json"

	"github.com/ethicalzen/acvps-gateway/internal/cache"
	log "github.com/sirupsen/logrus"
)

// ContractApprovalNotification represents a contract approval event
type ContractApprovalNotification struct {
	TenantID   string `json:"tenant_id"`
	ContractID string `json:"contract_id"`
	ApprovedAt string `json:"approved_at"`
	ApprovedBy string `json:"approved_by"`
}

// StartPubSubListener listens for contract approval notifications via Redis Pub/Sub
func StartPubSubListener(ctx context.Context, cacheClient *cache.Client) {
	redisClient := cacheClient.GetRedisClient()
	if redisClient == nil {
		log.Warn("⚠️  Redis not available, Pub/Sub disabled")
		return
	}

	pubsub := redisClient.Subscribe(ctx, "contract:approved")
	defer pubsub.Close()

	log.Info("📡 Subscribed to contract:approved channel for instant updates")

	// Wait for confirmation
	_, err := pubsub.Receive(ctx)
	if err != nil {
		log.WithError(err).Error("Failed to subscribe to Redis channel")
		return
	}

	// Listen for messages
	ch := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			log.Info("Pub/Sub listener stopped")
			return
		case msg := <-ch:
			handleContractApproval(ctx, cacheClient, msg.Payload)
		}
	}
}

// handleContractApproval processes a contract approval notification
func handleContractApproval(ctx context.Context, cacheClient *cache.Client, payload string) {
	var notification ContractApprovalNotification
	if err := json.Unmarshal([]byte(payload), &notification); err != nil {
		log.WithError(err).Warn("Failed to parse contract approval notification")
		return
	}

	log.WithFields(log.Fields{
		"contract_id": notification.ContractID,
		"tenant_id":   notification.TenantID,
		"approved_by": notification.ApprovedBy,
	}).Info("🔔 Received contract approval notification")

	// Load the contract from Redis
	key := "tenant:" + notification.TenantID + ":contract:" + notification.ContractID
	contractJSON, err := cacheClient.Get(ctx, key)
	if err != nil {
		log.WithError(err).Warnf("Failed to load approved contract %s", notification.ContractID)
		return
	}

	// Parse contract (simplified - just check status)
	var contract struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	
	if err := json.Unmarshal([]byte(contractJSON), &contract); err != nil {
		log.WithError(err).Warnf("Failed to parse approved contract %s", notification.ContractID)
		return
	}

	// Only load if active/approved
	if contract.Status != "active" && contract.Status != "approved" {
		log.Warnf("Received notification for non-active contract: %s (status: %s)", notification.ContractID, contract.Status)
		return
	}

	// Check if already loaded
	if _, err := GetBinding(notification.ContractID); err == nil {
		log.Infof("✅ Contract %s already loaded, skipping", notification.ContractID)
		return
	}

	// For now, just trigger a full reload (safer and simpler)
	// In production, you'd want to load just this one contract
	log.Infof("🔄 Reloading contracts after approval of: %s", notification.ContractID)
	if err := LoadAllContractsAtBoot(ctx, cacheClient); err != nil {
		log.WithError(err).Warn("Failed to reload contracts after approval")
	} else {
		log.Infof("✅ Contract %s is now active and will be enforced immediately", notification.ContractID)
	}
}

