#!/bin/bash
export GATEWAY_MODE=local
export PORT=8443
export ETHICALZEN_API_KEY=sk-eth-zen-dev-2024-11-12-srini
export CONTROL_PLANE_URL=http://localhost:3001
export GATEWAY_TENANT_ID=default
export TENANT_ID=default
export LOG_LEVEL=info
export VALIDATION_MODE=enforce
export REDIS_DISABLED=true
export BLOCKCHAIN_DISABLED=true

cd "$(dirname "$0")"
./gateway-local

