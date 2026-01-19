# Architecture

This document explains the architecture of Guardrail Studio and how it integrates with the EthicalZen platform.

## Overview

Guardrail Studio follows a **hybrid open-source model**:

- **Open Source (This Repo)**: UI, templates, client SDK
- **Closed Source (EthicalZen API)**: Smart Guardrail algorithm, calibration engine

## System Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              YOUR APPLICATION                                │
│                                                                             │
│   ┌─────────────┐    ┌─────────────┐    ┌─────────────┐                    │
│   │   User      │───>│  Your LLM   │───>│   User      │                    │
│   │   Input     │    │   (OpenAI,  │    │   Output    │                    │
│   └──────┬──────┘    │  Anthropic) │    └──────┬──────┘                    │
│          │           └─────────────┘           │                            │
│          │                                     │                            │
│          ▼                                     ▼                            │
│   ┌─────────────────────────────────────────────────────┐                  │
│   │              ETHICALZEN SDK (Open Source)            │                  │
│   │                                                      │                  │
│   │   client.evaluate({                                  │                  │
│   │     guardrail: 'medical_advice_smart',               │                  │
│   │     input: userMessage                               │                  │
│   │   })                                                 │                  │
│   └──────────────────────────┬──────────────────────────┘                  │
│                              │                                              │
└──────────────────────────────┼──────────────────────────────────────────────┘
                               │
                               │ HTTPS API Call
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                        ETHICALZEN API (Closed Source)                        │
│                           api.ethicalzen.ai                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌────────────────┐   ┌────────────────┐   ┌────────────────┐            │
│   │   API Gateway  │──>│ Smart Guardrail│──>│   Response     │            │
│   │   (Auth, Rate) │   │    Engine      │   │   Formatter    │            │
│   └────────────────┘   └───────┬────────┘   └────────────────┘            │
│                               │                                            │
│                               ▼                                            │
│   ┌─────────────────────────────────────────────────────────────────┐     │
│   │                    EVALUATION PIPELINE                           │     │
│   │                                                                  │     │
│   │   1. Embedding Computation (MiniLM-L6-v2)                       │     │
│   │      └─> 384-dimensional vector                                  │     │
│   │                                                                  │     │
│   │   2. Semantic Scoring                                           │     │
│   │      └─> Cosine similarity to safe/unsafe centroids             │     │
│   │                                                                  │     │
│   │   3. Lexical Feature Extraction                                 │     │
│   │      └─> Regex pattern matching                                  │     │
│   │                                                                  │     │
│   │   4. Score Fusion                                               │     │
│   │      └─> score = (embed_weight * embed) + (lex_weight * lex)    │     │
│   │                                                                  │     │
│   │   5. Decision Logic (3-Zone)                                    │     │
│   │      └─> ALLOW (< t_allow) | REVIEW | BLOCK (> t_block)         │     │
│   │                                                                  │     │
│   └─────────────────────────────────────────────────────────────────┘     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Smart Guardrail Algorithm

The Smart Guardrail algorithm is a hybrid approach combining:

### 1. Semantic Embeddings

- Uses **Sentence Transformers** (all-MiniLM-L6-v2)
- Generates 384-dimensional embeddings
- Compares to pre-computed **safe** and **unsafe** centroids
- Centroid = average of training example embeddings

### 2. Lexical Pattern Matching

- Regex-based feature extraction
- Domain-specific patterns (dosage, imperatives, etc.)
- Weighted scoring per pattern

### 3. Score Fusion

```
final_score = (embedding_weight × embedding_score) + (lexical_weight × lexical_score)
```

Default weights: 60% embedding, 40% lexical

### 4. 3-Zone Decision

```
if score < t_allow:    return ALLOW
if score > t_block:    return BLOCK
else:                  return REVIEW
```

## Calibration Process

Calibration tunes the guardrail for optimal accuracy:

1. **Collect Examples**: Safe and unsafe training data
2. **Compute Centroids**: Average embeddings for each class
3. **Optimize Thresholds**: Find t_allow and t_block that maximize F1
4. **Extract Patterns**: Identify high-signal lexical patterns
5. **Validate**: Test on held-out data

## Data Flow

### At Design Time (Guardrail Studio)

```
User Input (Description + Examples)
         │
         ▼
┌─────────────────────────┐
│  Guardrail Studio UI    │  ◀── Open Source
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  POST /api/sdk/generate │
│  (EthicalZen API)       │  ◀── API Call
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  Calibration Engine     │  ◀── Closed Source
│  - Compute embeddings   │
│  - Calculate centroids  │
│  - Optimize thresholds  │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  YAML/JSON Export       │  ◀── User Downloads
└─────────────────────────┘
```

### At Runtime (Production)

```
User Message
    │
    ▼
┌─────────────────────────┐
│  Your Application       │
│  - SDK.evaluate()       │
└───────────┬─────────────┘
            │ ~50ms
            ▼
┌─────────────────────────┐
│  EthicalZen Gateway     │
│  - Load calibration     │
│  - Compute embedding    │
│  - Score fusion         │
│  - Decision             │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  Result                 │
│  {                      │
│    decision: "block",   │
│    score: 0.82,         │
│    reason: "..."        │
│  }                      │
└─────────────────────────┘
```

## Security Considerations

### What Data is Sent to EthicalZen

| Data Type | Sent? | Purpose |
|-----------|-------|---------|
| User messages (evaluate) | ✅ | Evaluation |
| Training examples (generate) | ✅ | Calibration |
| API keys | ✅ | Authentication |
| User identity | ❌ | Not required |
| Session data | ❌ | Not collected |

### Data Retention

- **Evaluation requests**: Not stored (stateless)
- **Training examples**: Stored only if you save the guardrail
- **Calibration data**: Stored for your guardrails

### Self-Hosting Option

For enterprises requiring full data control:

- Contact sales@ethicalzen.ai
- On-premise deployment available
- Air-gapped environments supported

## Performance

### Latency Targets

| Operation | p50 | p95 | p99 |
|-----------|-----|-----|-----|
| Evaluate (single) | 30ms | 50ms | 100ms |
| Evaluate (batch) | 50ms | 100ms | 200ms |
| Generate | 2s | 5s | 10s |

### Throughput

- 1000 evaluations/second per tenant (default)
- Higher limits available (contact sales)

## Related Documents

- [API Reference](api-reference.md)
- [Template Format](../CONTRIBUTING.md)

