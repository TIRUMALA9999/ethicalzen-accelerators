package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethicalzen/acvps-gateway/internal/cache"
	"github.com/ethicalzen/acvps-gateway/internal/config"
	log "github.com/sirupsen/logrus"
)

// Client represents a blockchain client for querying DCRegistry
type Client struct {
	client          *ethclient.Client
	contractAddress common.Address
	contractABI     abi.ABI
	cache           *cache.Client
	cacheTTL        time.Duration
	config          config.BlockchainConfig
}

// Contract represents a DC contract from the blockchain
type Contract struct {
	ID              string
	ServiceName     string
	Suite           string
	PolicyDigest    string
	IssuedAt        time.Time
	ExpiresAt       time.Time
	RevocationEpoch uint64
	Status          string
	Issuer          string
	Metadata        string
}

// DCRegistry ABI (minimal, just what we need)
const dcRegistryABI = `[
	{
		"inputs": [
			{"internalType": "bytes32", "name": "contractId", "type": "bytes32"}
		],
		"name": "getContract",
		"outputs": [
			{
				"components": [
					{"internalType": "bytes32", "name": "contractId", "type": "bytes32"},
					{"internalType": "string", "name": "serviceName", "type": "string"},
					{"internalType": "uint8", "name": "suite", "type": "uint8"},
					{"internalType": "bytes32", "name": "policyDigest", "type": "bytes32"},
					{"internalType": "uint256", "name": "issuedAt", "type": "uint256"},
					{"internalType": "uint256", "name": "expiresAt", "type": "uint256"},
					{"internalType": "uint256", "name": "revocationEpoch", "type": "uint256"},
					{"internalType": "uint8", "name": "status", "type": "uint8"},
					{"internalType": "address", "name": "issuer", "type": "address"},
					{"internalType": "string", "name": "metadata", "type": "string"}
				],
				"internalType": "struct DCRegistry.Contract",
				"name": "",
				"type": "tuple"
			}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{"internalType": "bytes32", "name": "contractId", "type": "bytes32"},
			{"internalType": "bytes32", "name": "policyDigest", "type": "bytes32"}
		],
		"name": "validateContract",
		"outputs": [
			{"internalType": "bool", "name": "valid", "type": "bool"},
			{"internalType": "string", "name": "reason", "type": "string"}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "version",
		"outputs": [
			{"internalType": "string", "name": "", "type": "string"}
		],
		"stateMutability": "pure",
		"type": "function"
	}
]`

// New creates a new blockchain client
func New(cfg config.BlockchainConfig, cacheClient *cache.Client) (*Client, error) {
	// Connect to Ethereum node
	client, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum node: %w", err)
	}

	// Parse contract address
	contractAddress := common.HexToAddress(cfg.ContractAddress)

	// Parse ABI
	contractABI, err := abi.JSON(strings.NewReader(dcRegistryABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse contract ABI: %w", err)
	}

	cacheTTL := time.Duration(cfg.CacheTTL) * time.Second

	log.WithFields(log.Fields{
		"rpc_url":          cfg.RPCURL,
		"contract_address": cfg.ContractAddress,
		"cache_ttl":        cacheTTL,
	}).Info("Blockchain client initialized")

	return &Client{
		client:          client,
		contractAddress: contractAddress,
		contractABI:     contractABI,
		cache:           cacheClient,
		cacheTTL:        cacheTTL,
		config:          cfg,
	}, nil
}

// Close closes the blockchain client connection
func (c *Client) Close() {
	if c.client != nil {
		c.client.Close()
	}
}

// HealthCheck verifies the blockchain connection is working
func (c *Client) HealthCheck(ctx context.Context) error {
	_, err := c.client.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("blockchain health check failed: %w", err)
	}
	return nil
}

// GetBlockNumber returns the current block number
func (c *Client) GetBlockNumber(ctx context.Context) (uint64, error) {
	return c.client.BlockNumber(ctx)
}

// ValidateContract validates a contract by checking if it exists, is active, and matches the policy digest
func (c *Client) ValidateContract(ctx context.Context, contractID, policyDigest string) (bool, string, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("contract:valid:%s:%s", contractID, policyDigest)
	if cached, err := c.cache.Get(ctx, cacheKey); err == nil && cached != "" {
		return cached == "true", "cached", nil
	}

	// Convert contract ID to bytes32 using keccak256
	// Convert contract ID: if it's a hex string, parse it directly
	// Otherwise, hash it
	var contractIDBytes [32]byte
	if strings.HasPrefix(contractID, "0x") && len(contractID) == 66 {
		// It's already a hex hash, parse it
		contractHash := common.HexToHash(contractID)
		copy(contractIDBytes[:], contractHash[:])
	} else {
		// It's a string, hash it
		contractIDBytes = stringToBytes32(contractID)
	}
	
	// Convert policy digest: if it's a hex string, parse it directly
	// Otherwise, hash it like the contract ID
	var policyDigestBytes [32]byte
	if strings.HasPrefix(policyDigest, "0x") && len(policyDigest) == 66 {
		// It's already a hex hash, parse it
		policyHash := common.HexToHash(policyDigest)
		copy(policyDigestBytes[:], policyHash[:])
	} else {
		// It's a string, hash it
		policyDigestBytes = stringToBytes32(policyDigest)
	}

	// Call smart contract
	data, err := c.contractABI.Pack("validateContract", contractIDBytes, policyDigestBytes)
	if err != nil {
		return false, "", fmt.Errorf("failed to pack contract call: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &c.contractAddress,
		Data: data,
	}

	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		return false, "", fmt.Errorf("contract call failed: %w", err)
	}

	// Unpack result
	var (
		valid  bool
		reason string
	)
	err = c.contractABI.UnpackIntoInterface(&[]interface{}{&valid, &reason}, "validateContract", result)
	if err != nil {
		return false, "", fmt.Errorf("failed to unpack result: %w", err)
	}

	// Cache the result
	cacheValue := "false"
	if valid {
		cacheValue = "true"
	}
	_ = c.cache.Set(ctx, cacheKey, cacheValue, c.cacheTTL)

	log.WithFields(log.Fields{
		"contract_id":    contractID,
		"policy_digest":  policyDigest,
		"valid":          valid,
		"reason":         reason,
		"cache_duration": c.cacheTTL,
	}).Debug("Contract validation result")

	return valid, reason, nil
}

// GetContract retrieves full contract details from the blockchain
func (c *Client) GetContract(ctx context.Context, contractID string) (*Contract, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("contract:details:%s", contractID)
	if cached, err := c.cache.GetStruct(ctx, cacheKey); err == nil && cached != nil {
		if contract, ok := cached.(*Contract); ok {
			return contract, nil
		}
	}

	// Convert to bytes32
	// Convert contract ID: if it's a hex string, parse it directly
	// Otherwise, hash it
	var contractIDBytes [32]byte
	if strings.HasPrefix(contractID, "0x") && len(contractID) == 66 {
		// It's already a hex hash, parse it
		contractHash := common.HexToHash(contractID)
		copy(contractIDBytes[:], contractHash[:])
	} else {
		// It's a string, hash it
		contractIDBytes = stringToBytes32(contractID)
	}

	// Call smart contract
	data, err := c.contractABI.Pack("getContract", contractIDBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to pack contract call: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &c.contractAddress,
		Data: data,
	}

	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("contract call failed: %w", err)
	}

	// Unpack result (struct with 10 fields)
	unpacked, err := c.contractABI.Unpack("getContract", result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack result: %w", err)
	}

	if len(unpacked) == 0 {
		return nil, fmt.Errorf("contract not found")
	}

	// Parse the struct
	contractData, ok := unpacked[0].(struct {
		ContractId      [32]byte
		ServiceName     string
		Suite           uint8
		PolicyDigest    [32]byte
		IssuedAt        *big.Int
		ExpiresAt       *big.Int
		RevocationEpoch *big.Int
		Status          uint8
		Issuer          common.Address
		Metadata        string
	})
	if !ok {
		return nil, fmt.Errorf("failed to parse contract data")
	}

	// Convert to our Contract struct
	contract := &Contract{
		ID:              contractID,
		ServiceName:     contractData.ServiceName,
		Suite:           suiteToString(contractData.Suite),
		PolicyDigest:    bytes32ToString(contractData.PolicyDigest),
		IssuedAt:        time.Unix(contractData.IssuedAt.Int64(), 0),
		ExpiresAt:       time.Unix(contractData.ExpiresAt.Int64(), 0),
		RevocationEpoch: contractData.RevocationEpoch.Uint64(),
		Status:          statusToString(contractData.Status),
		Issuer:          contractData.Issuer.Hex(),
		Metadata:        contractData.Metadata,
	}

	// Cache the result
	_ = c.cache.SetStruct(ctx, cacheKey, contract, c.cacheTTL)

	log.WithFields(log.Fields{
		"contract_id":   contractID,
		"service_name":  contract.ServiceName,
		"suite":         contract.Suite,
		"status":        contract.Status,
		"expires_at":    contract.ExpiresAt,
	}).Debug("Retrieved contract details")

	return contract, nil
}

// GetVersion returns the DCRegistry contract version
func (c *Client) GetVersion(ctx context.Context) (string, error) {
	data, err := c.contractABI.Pack("version")
	if err != nil {
		return "", fmt.Errorf("failed to pack version call: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &c.contractAddress,
		Data: data,
	}

	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		return "", fmt.Errorf("version call failed: %w", err)
	}

	var version string
	err = c.contractABI.UnpackIntoInterface(&version, "version", result)
	if err != nil {
		return "", fmt.Errorf("failed to unpack version: %w", err)
	}

	return version, nil
}

// Helper functions

func stringToBytes32(s string) [32]byte {
	// Use Keccak256 hash (same as ethers.keccak256)
	// This is critical for long strings like contract IDs
	hash := crypto.Keccak256Hash([]byte(s))
	var b [32]byte
	copy(b[:], hash[:])
	return b
}

func bytes32ToString(b [32]byte) string {
	return common.BytesToHash(b[:]).Hex()
}

func suiteToString(suite uint8) string {
	suites := []string{"S0", "S1", "S2"}
	if int(suite) < len(suites) {
		return suites[suite]
	}
	return "Unknown"
}

func statusToString(status uint8) string {
	statuses := []string{"Draft", "Active", "Revoked", "Expired"}
	if int(status) < len(statuses) {
		return statuses[status]
	}
	return "Unknown"
}

