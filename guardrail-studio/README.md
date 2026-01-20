# 🛡️ Guardrail Studio

**The open-source toolkit for designing, testing, and contributing AI safety guardrails.**

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![GitHub Stars](https://img.shields.io/github/stars/ethicalzen/guardrail-sdk?style=social)](https://github.com/ethicalzen/guardrail-sdk)
[![Discord](https://img.shields.io/discord/1234567890?color=7389D8&label=Discord&logo=discord&logoColor=white)](https://discord.gg/ethicalzen)
[![Made with EthicalZen](https://img.shields.io/badge/Powered%20by-EthicalZen-6366f1)](https://ethicalzen.ai)

<p align="center">
  <img src="docs/assets/guardrail-studio-hero.png" alt="Guardrail Studio" width="800">
</p>

---

## 🎯 What is Guardrail Studio?

Guardrail Studio is an **open-source UI and template library** for creating AI safety guardrails. It's designed for:

- **Developers** building AI applications who need safety controls
- **AI Safety Researchers** contributing guardrails to the community
- **Organizations** needing compliance guardrails (HIPAA, FINRA, etc.)

### How It Compares

| Feature | Guardrail Studio | NeMo Guardrails | AWS Bedrock | Azure AI Safety |
|---------|------------------|-----------------|-------------|-----------------|
| **Open Source** | ✅ Full SDK | ✅ Full | ❌ Closed | ❌ Closed |
| **No-Code UI** | ✅ Browser-based | ❌ Code only | ✅ Console | ✅ Portal |
| **Template Library** | ✅ Community | ✅ Community | ❌ Fixed | ❌ Fixed |
| **Model Agnostic** | ✅ Any LLM | ✅ Any LLM | ⚠️ Bedrock focus | ⚠️ Azure focus |
| **Self-Hostable** | ✅ Static files | ✅ Python | ❌ AWS only | ❌ Azure only |
| **Smart Semantic Detection** | ✅ Embeddings | ⚠️ Colang rules | ✅ Unknown | ⚠️ Limited |

---

## 🚀 Quick Start

### Option 1: Use Hosted Version (Fastest)

Visit **[studio.ethicalzen.ai](https://studio.ethicalzen.ai)** — no installation required.

### Option 2: Run Locally (2 commands)

```bash
# Clone the repository
git clone https://github.com/ethicalzen/guardrail-studio.git
cd guardrail-studio

# Start local server (Python 3 required)
python3 -m http.server 8090
```

Then open **http://localhost:8090** in your browser.

### Option 3: Docker

```bash
docker run -p 8090:80 ethicalzen/guardrail-studio:latest
```

---

## ✨ Features

### 🎨 Natural Language Design
Describe what you want to block in plain English. No ML expertise needed.

```
Block AI from providing specific medical diagnoses or prescribing medications.
Allow general health education and encouragement to see a doctor.
```

### 🔍 Smart Template Recommendation
Before creating a new guardrail, the system checks if existing templates already cover your use case:

- **Single Match**: "This looks like `financial_advice_smart` (99% accuracy)"
- **Composite Match**: "Combine `pii_blocker` + `prompt_injection` for full coverage"
- **New Creation**: Only when genuinely novel

### 📚 Template Library
Pre-built guardrails with tested accuracy:

| Template | Use Case | Accuracy | Status |
|----------|----------|----------|--------|
| `financial_advice_smart` | Block investment advice | **99%** | ✅ Production |
| `medical_advice_smart` | Block diagnoses/prescriptions | 77% | ✅ Production |
| `legal_advice_smart` | Block legal recommendations | 75% | ✅ Production |
| `pii_blocker` | Block SSN, email, credit cards | **99%** | ✅ Production |
| `prompt_injection_blocker` | Block jailbreaks | **95%** | ✅ Production |
| `academic_integrity` | Block homework completion | 77% | ✅ Production |
| `toxicity_detector` | Block hate speech | 85% | ✅ Production |
| `threat_detector` | Block violence/threats | 70% | 🔄 Beta |

### 🧪 Built-in Testing
Test your guardrail before deploying with instant feedback.

### 📤 Export Formats
- **YAML** — Version control friendly
- **JSON** — API integration ready

---

## 📖 Usage Guide

### Step 1: Describe Your Guardrail

Enter a natural language description of what to block:

```
Block AI from providing specific diagnoses like "you have diabetes" or 
prescribing medications like "take 500mg ibuprofen". 

Allow general health information like explaining what diabetes is or 
encouraging users to consult a doctor.
```

### Step 2: Add Training Examples

**Safe Examples (Should ALLOW):**
```
- "What is diabetes?"
- "What are the symptoms of a cold?"
- "Should I see a doctor for this?"
- "How does blood pressure medication work in general?"
```

**Unsafe Examples (Should BLOCK):**
```
- "You have Type 2 diabetes"
- "Take 500mg of ibuprofen twice daily"
- "Your symptoms indicate you have COVID-19"
- "Stop taking your blood pressure medication"
```

### Step 3: Generate & Test

Click **Generate Guardrail** and test with sample inputs:

```
Input: "Based on your symptoms, you likely have strep throat"
Result: 🚫 BLOCKED (score: 0.82)

Input: "Strep throat is a bacterial infection of the throat"
Result: ✅ ALLOWED (score: 0.15)
```

### Step 4: Export & Deploy

Export your guardrail as YAML:

```yaml
id: medical_advice_custom
name: Medical Advice Blocker
version: 1.0.0
type: smart_guardrail

description: |
  Blocks diagnoses and prescriptions.
  Allows general health education.

calibration:
  t_allow: 0.25
  t_block: 0.65
  embedding_weight: 0.6
  lexical_weight: 0.4

safe_examples:
  - "What is diabetes?"
  - "What are the symptoms of a cold?"
  
unsafe_examples:
  - "You have Type 2 diabetes"
  - "Take 500mg of ibuprofen twice daily"
```

---

## 🔌 Integration

### JavaScript/TypeScript

```javascript
import { EthicalZen } from '@ethicalzen/sdk';

const client = new EthicalZen({ 
  apiKey: process.env.ETHICALZEN_API_KEY 
});

// Single guardrail
const result = await client.evaluate({
  guardrail: 'medical_advice_smart',
  input: userMessage
});

if (result.decision === 'block') {
  return "I can't provide medical advice. Please consult a healthcare provider.";
}

// Multiple guardrails (composite)
const multiResult = await client.evaluateComposite({
  guardrails: ['pii_blocker', 'prompt_injection_blocker', 'medical_advice_smart'],
  input: userMessage,
  mode: 'all_must_pass'  // or 'any_can_block'
});
```

### Python

```python
from ethicalzen import EthicalZen

client = EthicalZen(api_key=os.environ["ETHICALZEN_API_KEY"])

result = client.evaluate(
    guardrail="medical_advice_smart",
    input=user_message
)

if result.decision == "block":
    return "I can't provide medical advice. Please consult a healthcare provider."
```

### cURL / REST API

```bash
curl -X POST https://api.ethicalzen.ai/v1/evaluate \
  -H "Authorization: Bearer $ETHICALZEN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "guardrail": "medical_advice_smart",
    "input": "What medication should I take for my headache?"
  }'
```

**Response:**
```json
{
  "decision": "block",
  "score": 0.78,
  "threshold": 0.65,
  "reason": "Medical prescription request detected",
  "latency_ms": 45
}
```

### Streaming (SSE) Support

For real-time LLM output validation:

```javascript
const stream = await client.evaluateStream({
  guardrails: ['prompt_injection_blocker'],
  llmStream: openaiStream,
  onViolation: (chunk, violation) => {
    console.log(`Blocked at chunk: ${chunk}`);
    stream.abort();
  }
});
```

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    GUARDRAIL STUDIO (Open Source)                       │
│                     github.com/ethicalzen/guardrail-studio              │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐    │
│   │   UI (HTML/JS)  │    │ Template Library│    │  Client SDK     │    │
│   │   index.html    │    │ templates/*.yaml│    │ @ethicalzen/sdk │    │
│   └────────┬────────┘    └────────┬────────┘    └────────┬────────┘    │
│            │                      │                      │              │
└────────────┼──────────────────────┼──────────────────────┼──────────────┘
             │                      │                      │
             └──────────────────────┼──────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    ETHICALZEN API (Closed Source)                       │
│                         api.ethicalzen.ai                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐    │
│   │ Smart Guardrail │    │ Auto-Calibration│    │ Runtime Gateway │    │
│   │    Algorithm    │    │     Engine      │    │  (Low Latency)  │    │
│   └─────────────────┘    └─────────────────┘    └─────────────────┘    │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### What's Open Source vs Closed

| Component | Open Source | Rationale |
|-----------|-------------|-----------|
| UI (index.html) | ✅ | Adoption & transparency |
| Template Library | ✅ | Community contributions |
| Client SDK | ✅ | Developer experience |
| Smart Guardrail Algorithm | ❌ | Competitive advantage |
| Calibration Engine | ❌ | Trade secret |
| Production Gateway | ❌ | SLA & security |

---

## 🤝 Contributing

We welcome contributions! The most impactful way to help is **adding new guardrail templates**.

### Adding a Template

1. **Fork** this repository
2. **Create** `templates/your-template.yaml`
3. **Test** locally with `python3 -m http.server 8090`
4. **Submit** a Pull Request

### Template Requirements

- **Minimum 10 safe examples** and **10 unsafe examples**
- **Clear description** of what to block vs allow
- **Tested accuracy** > 70%
- **Appropriate metadata** (category, compliance, tags)

### Template Format

```yaml
id: your_template_id
name: Human Readable Name
version: 1.0.0
type: smart_guardrail

description: |
  Clear description of what this guardrail blocks and allows.

calibration:
  t_allow: 0.25
  t_block: 0.65

safe_examples:
  - "Example that should be ALLOWED"
  # ... minimum 10

unsafe_examples:
  - "Example that should be BLOCKED"
  # ... minimum 10

metadata:
  category: finance | healthcare | legal | safety | content
  risk_level: low | medium | high | critical
  compliance: [SEC, HIPAA, GDPR, etc.]
  tags: [relevant, keywords]
  author: Your Name
  created: YYYY-MM-DD
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for full guidelines.

### Wanted Templates

- [ ] EU AI Act compliance
- [ ] GDPR data subject rights
- [ ] Age-appropriate content (COPPA)
- [ ] Political neutrality
- [ ] Self-harm prevention
- [ ] Misinformation detection
- [ ] Copyright infringement
- [ ] Real estate fair housing

---

## 📁 Repository Structure

```
guardrail-studio/
├── index.html              # Main UI (standalone, no build required)
├── README.md               # This file
├── CONTRIBUTING.md         # Contribution guidelines
├── LICENSE                 # Apache 2.0
├── package.json            # NPM metadata (for SDK publishing)
│
├── templates/              # Community guardrail templates
│   ├── financial-advice.yaml
│   ├── medical-advice.yaml
│   ├── legal-advice.yaml
│   ├── pii-blocker.yaml
│   ├── prompt-injection.yaml
│   └── ...
│
├── src/
│   └── lib/
│       └── client.js       # Lightweight API client
│
├── examples/               # Integration examples
│   ├── nextjs/
│   ├── express/
│   ├── python-flask/
│   └── langchain/
│
├── docs/                   # Documentation
│   ├── architecture.md
│   ├── api-reference.md
│   └── assets/
│
└── tests/                  # Template validation tests
    └── validate-templates.js
```

---

## 🔒 Security

### Reporting Vulnerabilities

Please report security vulnerabilities to **security@ethicalzen.ai**. Do not open public issues for security concerns.

### Security Considerations

- The UI runs entirely in your browser
- API calls are made directly to EthicalZen servers
- No data is stored locally beyond browser session
- Templates are static YAML files (no execution)

---

## 📊 Benchmarks

Tested on EthicalZen internal benchmark (1000 examples per category):

| Guardrail | Precision | Recall | F1 Score | Latency (p95) |
|-----------|-----------|--------|----------|---------------|
| financial_advice_smart | 99.2% | 98.8% | 99.0% | 48ms |
| pii_blocker | 99.5% | 99.1% | 99.3% | 12ms |
| prompt_injection_blocker | 96.2% | 94.8% | 95.5% | 15ms |
| medical_advice_smart | 78.5% | 76.2% | 77.3% | 52ms |
| toxicity_detector | 87.3% | 83.1% | 85.1% | 45ms |

---

## 📜 License

- **This repository (SDK, UI, Templates):** Apache 2.0
- **EthicalZen API:** Commercial (free tier: 1000 evaluations/month)

---

## 🔗 Links

| Resource | Link |
|----------|------|
| **Website** | [ethicalzen.ai](https://ethicalzen.ai) |
| **Documentation** | [docs.ethicalzen.ai](https://docs.ethicalzen.ai) |
| **API Reference** | [api.ethicalzen.ai/docs](https://api.ethicalzen.ai/docs) |
| **Discord** | [discord.gg/ethicalzen](https://discord.gg/ethicalzen) |
| **Twitter** | [@ethicalzen](https://twitter.com/ethicalzen) |
| **Status** | [status.ethicalzen.ai](https://status.ethicalzen.ai) |

---

## 🙏 Acknowledgments

- [NVIDIA NeMo Guardrails](https://github.com/NVIDIA/NeMo-Guardrails) for pioneering open-source AI guardrails
- [Guardrails AI](https://github.com/guardrails-ai/guardrails) for the validation framework inspiration
- [Sentence Transformers](https://www.sbert.net/) for the embedding models
- All our community contributors! 💜

---

<p align="center">
  <strong>Made with 💜 by <a href="https://ethicalzen.ai">EthicalZen</a></strong>
  <br>
  <sub>Building AI safety, one guardrail at a time.</sub>
</p>
