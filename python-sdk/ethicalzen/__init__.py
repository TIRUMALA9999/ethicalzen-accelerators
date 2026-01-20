"""
EthicalZen Python SDK - AI Guardrails for Runtime Enforcement

Usage:
    from ethicalzen import EthicalZen

    client = EthicalZen(api_key="your-api-key")
    
    result = client.evaluate(
        guardrail="medical_advice_smart",
        input="What medication should I take for headaches?"
    )
    
    if result.decision == "BLOCK":
        print("Content blocked:", result.reason)
"""

from ethicalzen.client import EthicalZen, AsyncEthicalZen
from ethicalzen.proxy import EthicalZenProxy, ProxyResponse, wrap_openai
from ethicalzen.models import (
    EvaluationResult,
    GuardrailConfig,
    DesignResult,
    SimulationResult,
    Decision,
)
from ethicalzen.exceptions import (
    EthicalZenError,
    AuthenticationError,
    RateLimitError,
    APIError,
    ValidationError,
)

__version__ = "0.1.0"
__all__ = [
    # Clients
    "EthicalZen",
    "AsyncEthicalZen",
    # Proxy
    "EthicalZenProxy",
    "ProxyResponse",
    "wrap_openai",
    # Models
    "EvaluationResult",
    "GuardrailConfig", 
    "DesignResult",
    "SimulationResult",
    "Decision",
    # Exceptions
    "EthicalZenError",
    "AuthenticationError",
    "RateLimitError",
    "APIError",
    "ValidationError",
]

