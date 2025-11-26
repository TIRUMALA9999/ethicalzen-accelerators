package gateway

import "errors"

var (
	// ErrContractNotLoaded is returned when a contract is not in the runtime table
	ErrContractNotLoaded = errors.New("contract not loaded in runtime table")

	// ErrHashMismatch is returned when extractor hash doesn't match contract
	ErrHashMismatch = errors.New("extractor hash mismatch - possible tampering")

	// ErrExtractorNotFound is returned when an extractor is not registered
	ErrExtractorNotFound = errors.New("feature extractor not found")
)
