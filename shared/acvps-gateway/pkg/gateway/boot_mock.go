package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethicalzen/acvps-gateway/pkg/contracts"
	log "github.com/sirupsen/logrus"
)

// LoadMockContracts loads pre-defined contracts from the filesystem for local testing
// This enables full validation testing in local mode without requiring Redis/control plane
func LoadMockContracts(ctx context.Context, mockContractsPath string) error {
	// Check if directory exists
	if _, err := os.Stat(mockContractsPath); os.IsNotExist(err) {
		return fmt.Errorf("mock contracts directory not found: %s", mockContractsPath)
	}

	log.WithField("path", mockContractsPath).Info("📁 Scanning mock contracts directory...")

	// Read all JSON files in the directory
	files, err := ioutil.ReadDir(mockContractsPath)
	if err != nil {
		return fmt.Errorf("failed to read mock contracts directory: %w", err)
	}

	loadedCount := 0
	errorCount := 0

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(mockContractsPath, file.Name())
		
		// Read file
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			log.WithError(err).Errorf("Failed to read mock contract file: %s", file.Name())
			errorCount++
			continue
		}

		// Parse JSON
		var contract contracts.Contract
		if err := json.Unmarshal(data, &contract); err != nil {
			log.WithError(err).Errorf("Failed to parse mock contract JSON: %s", file.Name())
			errorCount++
			continue
		}

		// Validate contract
		if contract.ContractID == "" {
			log.Errorf("Mock contract missing contract_id: %s", file.Name())
			errorCount++
			continue
		}

		// Load into runtime table
		if err := LoadContract(ctx, contract.ContractID, &contract); err != nil {
			log.WithError(err).Errorf("Failed to load mock contract: %s", contract.ContractID)
			errorCount++
			continue
		}

		log.WithFields(log.Fields{
			"contract_id": contract.ContractID,
			"tenant_id":   contract.TenantID,
			"file":        file.Name(),
		}).Info("✅ Mock contract loaded")

		loadedCount++
	}

	if loadedCount == 0 && errorCount == 0 {
		return fmt.Errorf("no mock contract files found in %s", mockContractsPath)
	}

	log.WithFields(log.Fields{
		"loaded": loadedCount,
		"errors": errorCount,
	}).Info("📊 Mock contract loading complete")

	return nil
}

