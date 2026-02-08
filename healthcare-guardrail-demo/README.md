# Healthcare Guardrail Demo

End-to-end demo of EthicalZen AI Safety Guardrails for a HIPAA-compliant healthcare patient triage chatbot.

## What This Demonstrates

- **6 guardrail types**: regex, smart (SSG v3), hybrid, keyword, DLM kernel, composite DAG
- **Healthcare narrative**: MedFirst Health patient triage chatbot with 5 guardrails
- **Alex Agent workflow**: Conversational guardrail design → FMA analysis → contract creation
- **Deterministic Contracts**: Envelope constraints with safety boundaries
- **Compliance evidence**: Auto-generated audit reports
- **Live metrics dashboard**: Terminal-based monitoring

## Architecture

```
[Patient] → [MedFirst Chatbot] → [EthicalZen Gateway :8080] → [OpenAI GPT-4]
                                         │
                                    ┌────┴────┐
                                    │ Smart   │
                                    │Guardrail│
                                    │ Engine  │
                                    │  :3001  │
                                    └─────────┘
                                    Guardrails:
                                    • pii_blocker (regex)
                                    • hipaa_compliance (hybrid)
                                    • medical_advice_smart (SSG v3)
                                    • prompt_injection_blocker (regex)
                                    • toxicity_detector (regex)
```

## Quick Start

```bash
# 1. Prerequisites
#    - Docker running
#    - jq installed (brew install jq)
#    - EthicalZen gateway container running on :8080/:3001

# 2. Configure
cp .env.example .env
# Edit .env with your EthicalZen API key

# 3. Run the full demo
./e2e-full.sh --mode full

# 4. Or run individual pieces
./test-guardrails.sh                    # All guardrail types
./narrative/healthcare-triage.sh        # Healthcare story
./e2e/setup-via-alex.sh                 # Alex Agent contract setup
./metrics-dashboard.sh                  # Live metrics
```

## Demo Modes

| Mode | Command | What It Shows |
|------|---------|---------------|
| Offline | `./e2e-full.sh --mode offline` | Sidecar guardrail tests (no backend needed) |
| Full | `./e2e-full.sh --mode full` | Backend + tenant + contract creation |
| Narrated | `./e2e-full.sh --mode full --narrate` | Pauses between acts for live presentation |

## Test Results

| Suite | Tests | What It Proves |
|-------|-------|----------------|
| Guardrail Tests | 35 PASS, 1 SKIP | All 6 guardrail types work |
| Healthcare Narrative | 16 PASS | HIPAA use case end-to-end |
| Proxy Test | 3 PASS | Gateway enforcement |
| **Total** | **54 PASS, 0 FAIL** | |

## File Structure

```
healthcare-guardrail-demo/
├── e2e-full.sh                    # Complete E2E entry point
├── test-guardrails.sh             # All guardrail types
├── deploy-vpc.sh                  # One-click VPC deployment
├── metrics-dashboard.sh           # Live terminal dashboard
├── .env.example                   # Environment template
│
├── lib/                           # Shared bash libraries
│   ├── colors.sh                  # PASS/FAIL/INFO formatting
│   ├── assert.sh                  # Test assertions + score rounding
│   ├── report.sh                  # JSON + markdown report generator
│   └── health.sh                  # Health check + wait functions
│
├── guardrails/                    # Guardrail test scripts
│   ├── test-data.json             # All safe/unsafe test payloads
│   ├── test-regex.sh              # PII, injection, toxicity, leakage
│   ├── test-smart.sh              # Medical, legal, financial (SSG v3)
│   ├── test-hybrid.sh             # HIPAA, content mod, embedding attack
│   ├── test-keyword.sh            # Bias detection
│   ├── test-dlm-kernel.sh         # DLM kernel (Enterprise tier)
│   └── test-composite-dag.sh      # AND/OR/NOT logic trees
│
├── narrative/                     # Healthcare demo story
│   ├── healthcare-triage.sh       # 6-act narrated demo
│   └── scenario-data.json         # Scenario payloads
│
├── e2e/                           # End-to-end scripts
│   ├── setup-via-alex.sh          # Alex Agent contract flow
│   ├── setup-contract.sh          # Direct API contract setup
│   ├── proxy-openai.sh            # OpenAI proxy test
│   └── generate-report.sh         # Compliance evidence report
│
├── deploy/                        # Deployment scripts
│   ├── docker-compose.demo.yml    # Local Docker compose
│   ├── gcp-vm.sh                  # GCP VM provisioning
│   ├── existing-host.sh           # Deploy to existing VM
│   └── validate-deployment.sh     # Post-deploy smoke tests
│
└── reports/                       # Auto-generated (gitignored)
    └── .gitkeep
```

## Requirements

- Docker (for gateway container)
- jq (JSON processor)
- curl
- bash 4+
- Python 3 (for score rounding)

## License

Copyright (c) 2026 EthicalZen.ai. All rights reserved.
