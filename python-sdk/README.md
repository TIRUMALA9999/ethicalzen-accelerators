# EthicalZen Python SDK

Official Python SDK for [EthicalZen](https://ethicalzen.ai) - AI Guardrails for Runtime Enforcement.

[![PyPI version](https://badge.fury.io/py/ethicalzen.svg)](https://badge.fury.io/py/ethicalzen)
[![Python 3.8+](https://img.shields.io/badge/python-3.8+-blue.svg)](https://www.python.org/downloads/)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

## Installation

```bash
pip install ethicalzen
```

## Quick Start

```python
from ethicalzen import EthicalZen

# Initialize client
client = EthicalZen(api_key="your-api-key")

# Evaluate content against a guardrail
result = client.evaluate(
    guardrail="medical_advice_smart",
    input="What medication should I take for headaches?"
)

if result.is_blocked:
    print(f"Blocked: {result.reason}")
else:
    print("Content allowed")
```

## Features

- ✅ **Evaluate** - Check content against guardrails in real-time
- ✅ **Proxy Mode** - Transparent LLM API protection (OpenAI, Anthropic, etc.)
- ✅ **Design** - Create guardrails from natural language descriptions
- ✅ **Simulate** - Test guardrail accuracy with your data
- ✅ **Optimize** - Auto-tune guardrails to improve accuracy
- ✅ **Async Support** - Full async/await support for high-performance apps

## Usage Examples

### Basic Evaluation

```python
from ethicalzen import EthicalZen

client = EthicalZen(api_key="your-api-key")

# Check medical advice
result = client.evaluate(
    guardrail="medical_advice_smart",
    input="What medication should I take for a headache?"
)

print(f"Decision: {result.decision}")  # BLOCK
print(f"Score: {result.score:.2f}")    # 0.85
print(f"Reason: {result.reason}")      # "Medical diagnosis/prescription request"
```

### Proxy Mode (Recommended)

The proxy mode transparently routes **ANY HTTP API calls** through EthicalZen's gateway, 
protecting both request and response automatically. Works with LLMs, REST APIs, GraphQL, 
microservices, third-party APIs, and more.

```python
from ethicalzen import EthicalZenProxy

proxy = EthicalZenProxy(
    api_key="your-ethicalzen-key",
    certificate_id="dc_your_certificate",
)

# POST to any endpoint (e.g., OpenAI)
response = proxy.post(
    "https://api.openai.com/v1/chat/completions",
    json={"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]},
    headers={"Authorization": "Bearer sk-openai-key"}
)

# GET request (e.g., Stripe API)
response = proxy.get(
    "https://api.stripe.com/v1/customers/cus_123",
    headers={"Authorization": "Bearer sk-stripe-key"}
)

# PUT, PATCH, DELETE also available
response = proxy.put("https://api.example.com/resource/123", json={...})
response = proxy.patch("https://api.example.com/resource/123", json={...})
response = proxy.delete("https://api.example.com/resource/123")

# Check result
if response.blocked:
    print(f"Blocked: {response.block_reason}")
else:
    print(response.json())
```

### Wrap Existing OpenAI Client

```python
from openai import OpenAI
from ethicalzen import wrap_openai

# Your existing OpenAI client
client = OpenAI()

# Wrap it for protection
protected = wrap_openai(
    client,
    api_key="your-ethicalzen-key",
    certificate_id="dc_your_certificate"
)

# Use like normal - automatically protected!
response = protected.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello!"}]
)
```

### Manual Evaluation (Alternative)

For more control, you can manually evaluate input/output:

```python
from ethicalzen import EthicalZen
import openai

client = EthicalZen(api_key="your-api-key")
openai_client = openai.OpenAI()

def safe_chat(user_message: str) -> str:
    # Check input before sending to LLM
    input_check = client.evaluate(
        guardrail="medical_advice_smart",
        input=user_message
    )
    
    if input_check.is_blocked:
        return "I can't provide medical advice. Please consult a healthcare provider."
    
    # Get LLM response
    response = openai_client.chat.completions.create(
        model="gpt-4",
        messages=[{"role": "user", "content": user_message}]
    )
    
    llm_response = response.choices[0].message.content
    
    # Check output before returning
    output_check = client.evaluate(
        guardrail="medical_advice_smart",
        input=llm_response
    )
    
    if output_check.is_blocked:
        return "I apologize, but I can't provide that information."
    
    return llm_response
```

### Design a Custom Guardrail

```python
from ethicalzen import EthicalZen

client = EthicalZen(api_key="your-api-key")

# Create guardrail from natural language
result = client.design(
    description="Block requests for gambling advice and betting tips. Allow general information about responsible gaming.",
    safe_examples=[
        "What is responsible gambling?",
        "How do casinos work?",
    ],
    unsafe_examples=[
        "What's the best strategy for blackjack?",
        "Give me tips for sports betting",
    ]
)

print(f"Created guardrail: {result.config.id}")
print(f"Accuracy: {result.simulation.metrics.accuracy:.0%}")
```

### Async Usage

```python
import asyncio
from ethicalzen import AsyncEthicalZen

async def main():
    async with AsyncEthicalZen(api_key="your-api-key") as client:
        # Evaluate multiple inputs concurrently
        results = await asyncio.gather(
            client.evaluate("medical_advice_smart", "What medication for headaches?"),
            client.evaluate("financial_advice_smart", "Should I buy Bitcoin?"),
            client.evaluate("legal_advice_smart", "How do I sue my neighbor?"),
        )
        
        for result in results:
            print(f"{result.guardrail_id}: {result.decision}")

asyncio.run(main())
```

### FastAPI Integration

```python
from fastapi import FastAPI, HTTPException
from ethicalzen import EthicalZen, EthicalZenError

app = FastAPI()
client = EthicalZen(api_key="your-api-key")

@app.post("/chat")
async def chat(message: str):
    try:
        result = client.evaluate(
            guardrail="medical_advice_smart",
            input=message
        )
        
        if result.is_blocked:
            raise HTTPException(
                status_code=400,
                detail=f"Message blocked: {result.reason}"
            )
        
        # Process the message...
        return {"response": "Your message was processed"}
        
    except EthicalZenError as e:
        raise HTTPException(status_code=500, detail=str(e))
```

### LangChain Integration

```python
from langchain.chains import LLMChain
from langchain.prompts import PromptTemplate
from ethicalzen import EthicalZen

client = EthicalZen(api_key="your-api-key")

def guardrail_wrapper(func):
    def wrapper(input_text: str) -> str:
        # Pre-check
        result = client.evaluate("medical_advice_smart", input_text)
        if result.is_blocked:
            return f"[BLOCKED] {result.reason}"
        
        # Run chain
        output = func(input_text)
        
        # Post-check
        result = client.evaluate("medical_advice_smart", output)
        if result.is_blocked:
            return "[BLOCKED] Response contained restricted content"
        
        return output
    return wrapper
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `ETHICALZEN_API_KEY` | Your API key (required if not passed to client) |
| `ETHICALZEN_BASE_URL` | Custom API base URL (optional) |

## Error Handling

```python
from ethicalzen import (
    EthicalZen,
    AuthenticationError,
    RateLimitError,
    APIError,
    ValidationError,
)

client = EthicalZen(api_key="your-api-key")

try:
    result = client.evaluate("medical_advice_smart", "test input")
except AuthenticationError:
    print("Invalid API key")
except RateLimitError as e:
    print(f"Rate limited. Retry after {e.retry_after} seconds")
except ValidationError as e:
    print(f"Invalid input: {e.message}")
except APIError as e:
    print(f"API error: {e.message} (status: {e.status_code})")
```

## Available Guardrails

| ID | Description | Accuracy |
|----|-------------|----------|
| `medical_advice_smart` | Blocks medical diagnoses and prescriptions | 77% |
| `financial_advice_smart` | Blocks personalized investment advice | 99% |
| `legal_advice_smart` | Blocks specific legal advice | 75% |
| `pii_detector` | Blocks PII disclosure | 95% |
| `prompt_injection` | Blocks prompt injection attacks | 90% |

See all templates at [Guardrail Studio](https://studio.ethicalzen.ai).

## API Reference

### `EthicalZen(api_key, base_url, timeout)`

Initialize the client.

- `api_key` (str): Your EthicalZen API key
- `base_url` (str, optional): API base URL
- `timeout` (float, optional): Request timeout in seconds (default: 60)

### `client.evaluate(guardrail, input, context)`

Evaluate content against a guardrail.

- `guardrail` (str): Guardrail ID
- `input` (str): Content to evaluate
- `context` (dict, optional): Additional context

Returns: `EvaluationResult`

### `client.design(description, safe_examples, unsafe_examples)`

Design a new guardrail from natural language.

- `description` (str): What to block/allow
- `safe_examples` (list, optional): Examples to allow
- `unsafe_examples` (list, optional): Examples to block

Returns: `DesignResult`

### `client.optimize(guardrail, target_accuracy, max_iterations)`

Auto-tune a guardrail.

- `guardrail` (str): Guardrail ID
- `target_accuracy` (float): Target accuracy (0-1)
- `max_iterations` (int): Max iterations

Returns: `OptimizeResult`

## License

Apache 2.0 - See [LICENSE](LICENSE) for details.

## Support

- 📧 Email: support@ethicalzen.ai
- 📖 Docs: https://ethicalzen.ai/docs
- 🐛 Issues: https://github.com/aiworksllc/ethicalzen-accelerators/issues

