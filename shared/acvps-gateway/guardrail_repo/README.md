# Guardrail Repository

This directory contains all custom guardrails for the EthicalZen platform. Guardrails are automatically loaded by the ACVPS Gateway on startup.

## Directory Structure

```
guardrail_repo/
├── README.md                    # This file
├── default/                     # Default/built-in guardrails
│   ├── financial_compliance_v1.json
│   ├── medical_advice_blocker_v1.json
│   ├── content_moderation_v1.json
│   ├── hipaa_compliance_v1.json
│   └── legal_advice_blocker_v1.json
└── custom/                      # Your custom guardrails (future)
    └── your_guardrail_v1.json
```

## How It Works

### 1. Guardrail Storage
- Each guardrail is stored as a JSON file
- Filename format: `{guardrail_id}.json`
- Organized by category (default, custom, etc.)

### 2. Auto-Loading
The ACVPS Gateway automatically:
- Scans `guardrail_repo/default/` on startup
- Registers all guardrails to the dynamic registry
- Makes them available immediately for contracts

### 3. GDK Integration
When you deploy a guardrail via the GDK:
1. Guardrail registers to Gateway API (runtime)
2. Guardrail saves to `guardrail_repo/default/{id}.json` (persistent)
3. File gets committed to Git (version control)

## Guardrail File Format

```json
{
  "id": "financial_compliance_v1",
  "name": "Financial Compliance",
  "description": "Validates financial advice against SEC regulations",
  "type": "dynamic",
  "prompt_template": "You are a compliance officer...",
  "keywords": ["guaranteed", "risk-free", "insider"],
  "metric_name": "sec_compliance_score",
  "threshold": 0.85,
  "invert_score": false,
  "registered_at": "2025-11-11T00:00:00Z"
}
```

## Default Guardrails

### 💰 Financial Compliance (`financial_compliance_v1`)
- **Purpose**: Blocks financial advice that violates SEC regulations
- **Detects**: Guaranteed returns, insider trading hints, missing risk disclosures
- **Threshold**: 85% compliance required
- **Type**: LLM-assisted with keyword fallback

### 🏥 Medical Advice Blocker (`medical_advice_blocker_v1`)
- **Purpose**: Prevents AI from providing medical diagnoses
- **Detects**: Diagnoses, treatment plans, prescription advice
- **Threshold**: 30% risk tolerance (inverted)
- **Type**: LLM-assisted with keyword fallback

### 🛡️ Content Moderation (`content_moderation_v1`)
- **Purpose**: Blocks inappropriate content
- **Detects**: Hate speech, violence, illegal activities, explicit content
- **Threshold**: 90% safety required
- **Type**: LLM-assisted with keyword fallback

### 🔒 HIPAA Compliance (`hipaa_compliance_v1`)
- **Purpose**: Prevents exposure of Protected Health Information
- **Detects**: Patient names, medical records, SSN, diagnoses with identifiers
- **Threshold**: 95% compliance required
- **Type**: LLM-assisted with keyword fallback

### ⚖️ Legal Advice Blocker (`legal_advice_blocker_v1`)
- **Purpose**: Prevents AI from providing legal advice
- **Detects**: Legal recommendations, "you should sue" statements
- **Threshold**: 25% risk tolerance (inverted)
- **Type**: LLM-assisted with keyword fallback

## Adding Custom Guardrails

### Via GDK (Recommended)
1. Open Dashboard → Development Kit
2. Use Builder or Templates tab
3. Click "Deploy Guardrail"
4. Guardrail auto-saves to `guardrail_repo/default/`

### Manually
1. Create a JSON file in `guardrail_repo/default/`
2. Follow the format above
3. Restart the Gateway to load
4. Commit to Git for version control

## Version Control

All guardrails are tracked in Git:
- **Add new guardrail**: Commit the JSON file
- **Update guardrail**: Modify JSON, commit changes
- **Remove guardrail**: Delete JSON file, commit removal
- **Audit changes**: Use `git log` to see guardrail history

## Best Practices

1. **Naming Convention**: Use `{purpose}_v{version}` format
2. **Semantic Versioning**: Increment version for breaking changes
3. **Test Before Deploy**: Always use GDK Tester tab
4. **Document Changes**: Add meaningful commit messages
5. **Review Guardrails**: Periodically audit performance

## API Integration

Guardrails in this repo are automatically available via:
- `GET /api/guardrails/list` - List all guardrails
- `GET /api/guardrails/configs/{id}` - Get specific config
- `POST /api/extract-features` - Use for validation
- `POST /api/validate` - Use in contracts

## Troubleshooting

### Guardrail not loading?
1. Check JSON syntax: `cat guardrail_repo/default/{id}.json | jq`
2. Verify filename matches `id` field
3. Restart Gateway: `go run cmd/gateway/main.go`
4. Check logs for errors

### Guardrail not working as expected?
1. Test in GDK Tester tab
2. Check threshold settings
3. Verify `invert_score` flag
4. Review LLM prompt quality

### Need to update a guardrail?
1. Edit JSON file directly
2. OR use GDK to re-deploy with same ID
3. Restart Gateway to reload
4. Commit changes to Git

---

**Note**: Built-in Go guardrails (`pii_detector_v1`, `grounding_analyzer_v1`, `hallucination_detector_v1`) are compiled into the gateway binary and don't need JSON files.

