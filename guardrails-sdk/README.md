# EthicalZen Guardrails SDK

The Guardrails Development Kit (GDK) enables rapid creation, testing, and deployment of AI safety guardrails.

## 🚀 Quick Start

```bash
# Run the customer journey demo
cd examples
node gdk-customer-journey.js healthcare

# Try different personas
node gdk-customer-journey.js fintech
node gdk-customer-journey.js security
node gdk-customer-journey.js legal
node gdk-customer-journey.js research
```

## 📋 Customer Journey Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│  1. DISCOVER          2. CREATE           3. OPTIMIZE                   │
│  ───────────          ─────────           ──────────                    │
│  Browse templates  →  One-click setup  →  Auto-improve                  │
│  See expected          Get instant         Add suggested                │
│  metrics               results             examples                     │
│                                                                         │
│  4. VALIDATE          5. DEPLOY           6. MONITOR                    │
│  ──────────          ─────────           ────────                       │
│  P90/P95 check   →   One-click deploy →  Track metrics                 │
│  Export report        30s to live         Feedback loop                 │
└─────────────────────────────────────────────────────────────────────────┘
```

## 👤 Supported Personas

| Persona | Role | Template | Use Case |
|---------|------|----------|----------|
| **healthcare** | Healthcare AI PM | phi_protection | Prevent medical diagnoses in chatbots |
| **fintech** | Compliance Officer | financial_advice | Block SEC-violating investment advice |
| **security** | Security Engineer | prompt_injection | Block injection attacks |
| **legal** | Legal Tech Founder | legal_advice | Prevent specific legal recommendations |
| **research** | AI Ethics Researcher | hate_speech | Build nuanced hate speech detector |

## 🔧 API Endpoints

### Templates
```bash
# List all templates
GET /api/sg/templates

# Get specific template with examples
GET /api/sg/templates/:type
```

### Design & Create
```bash
# Design guardrail from natural language
POST /api/sg/design
{
  "naturalLanguage": "Block prompt injection attempts",
  "guardrailId": "my-guardrail",
  "safeExamples": ["..."],
  "unsafeExamples": ["..."]
}
```

### Optimize
```bash
# Get AI-suggested examples
POST /api/sg/suggest-examples
{
  "guardrailType": "prompt_injection",
  "currentMetrics": { "accuracy": 0.75, "recall": 0.85 }
}

# One-click optimize (async)
POST /api/sg/optimize
{
  "guardrailId": "my-guardrail",
  "targetAccuracy": 0.80
}

# Check optimization status
GET /api/sg/optimize/status/:jobId
```

### Validate & Evaluate
```bash
# Get P90/P95 reliability estimates
GET /api/sg/reliability/:guardrailId

# Evaluate input
POST /api/sg/evaluate
{
  "guardrail_id": "my-guardrail",
  "input": "user message to check"
}
```

### Recalibrate
```bash
# Add examples and recalibrate
POST /api/sg/recalibrate/:guardrailId
{
  "additionalSafe": ["new safe example"],
  "additionalUnsafe": ["new unsafe example"]
}
```

## 📊 Metrics Explained

| Metric | Description | Target |
|--------|-------------|--------|
| **Accuracy** | Overall correctness | ≥80% |
| **Recall** | % of unsafe inputs caught | ≥85% (critical for security) |
| **Precision** | % of blocks that were correct | ≥80% |
| **F1 Score** | Harmonic mean of precision/recall | ≥80% |
| **P90 Reliability** | 90% of decisions ≥ this reliability | ≥85% |
| **P95 Reliability** | 95% of decisions ≥ this reliability | ≥80% |

## 🎯 Best Practices

### 1. Start with Templates
Use pre-built templates as starting points - they come with validated examples.

### 2. Customize for Your Domain
Add industry-specific examples that reflect real threats you've seen.

### 3. Prioritize Recall for Security
For security guardrails (prompt injection, data exfiltration), optimize for recall first - missing an attack is worse than a false positive.

### 4. Iterative Improvement
Use the optimization loop to gradually improve accuracy:
1. Test with real traffic
2. Identify false positives/negatives
3. Add to examples
4. Recalibrate

### 5. Monitor P95 Reliability
Track P95 reliability in production - if it drops below 80%, trigger recalibration.

## 🔒 Environment Variables

```bash
# Required
GDK_API_KEY=your-api-key

# Optional
GDK_API_URL=https://ethicalzen-backend.your-domain.run.app
```

## 📁 Examples

| File | Description |
|------|-------------|
| `gdk-customer-journey.js` | Full customer journey demo with 5 personas |

## 📝 License

MIT License - See LICENSE file for details.

---

**EthicalZen AI** - Building Trust in AI Systems

