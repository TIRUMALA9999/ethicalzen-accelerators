"""
EthicalZen Proxy Client - Transparent LLM API protection.

This module provides a proxy client that wraps LLM API calls (OpenAI, Anthropic, etc.)
and routes them through the EthicalZen gateway for input/output validation.

Usage:
    from ethicalzen.proxy import EthicalZenProxy
    
    # Create proxy client
    proxy = EthicalZenProxy(
        api_key="your-ethicalzen-key",
        certificate_id="dc_your_certificate",
        gateway_url="https://gateway.ethicalzen.ai"  # or your self-hosted gateway
    )
    
    # Use like normal OpenAI client - requests are automatically protected
    response = proxy.chat_completions(
        target_url="https://api.openai.com/v1/chat/completions",
        target_api_key="sk-openai-key",
        model="gpt-4",
        messages=[{"role": "user", "content": "Hello!"}]
    )
"""

import os
from typing import Any, Dict, List, Optional, Union
import httpx

from ethicalzen.exceptions import (
    APIError,
    AuthenticationError,
    EthicalZenError,
    ValidationError,
)


DEFAULT_GATEWAY_URL = "https://acvps-gateway-mqnusyobga-uc.a.run.app"


class ProxyResponse:
    """Response from a proxied LLM request."""
    
    def __init__(
        self,
        data: Dict[str, Any],
        blocked: bool = False,
        block_reason: Optional[str] = None,
        guardrail_id: Optional[str] = None,
        score: Optional[float] = None,
    ):
        self.data = data
        self.blocked = blocked
        self.block_reason = block_reason
        self.guardrail_id = guardrail_id
        self.score = score
        
        # For OpenAI-compatible responses
        self.choices = data.get("choices", [])
        self.usage = data.get("usage", {})
        self.model = data.get("model", "")
        self.id = data.get("id", "")
    
    @property
    def content(self) -> str:
        """Get the response content (first choice message)."""
        if self.blocked:
            return f"[BLOCKED] {self.block_reason}"
        if self.choices:
            return self.choices[0].get("message", {}).get("content", "")
        return ""
    
    def __repr__(self) -> str:
        if self.blocked:
            return f"<ProxyResponse blocked={self.blocked} reason='{self.block_reason}'>"
        return f"<ProxyResponse model='{self.model}' content='{self.content[:50]}...'>"


class EthicalZenProxy:
    """
    Proxy client that routes LLM API calls through EthicalZen gateway.
    
    The gateway validates both input (user messages) and output (LLM responses)
    against your configured guardrails/certificates.
    
    Usage:
        proxy = EthicalZenProxy(
            api_key="sk-ethicalzen-key",
            certificate_id="dc_healthcare_patient_portal"
        )
        
        # OpenAI-style request
        response = proxy.chat_completions(
            target_url="https://api.openai.com/v1/chat/completions",
            target_api_key=os.environ["OPENAI_API_KEY"],
            model="gpt-4",
            messages=[{"role": "user", "content": "What medication should I take?"}]
        )
        
        if response.blocked:
            print(f"Request blocked: {response.block_reason}")
        else:
            print(response.content)
    """
    
    def __init__(
        self,
        api_key: Optional[str] = None,
        certificate_id: Optional[str] = None,
        gateway_url: Optional[str] = None,
        tenant_id: Optional[str] = None,
        timeout: float = 60.0,
        fail_open: bool = False,
    ):
        """
        Initialize the proxy client.
        
        Args:
            api_key: EthicalZen API key
            certificate_id: Your deployment certificate ID (e.g., "dc_healthcare_portal")
            gateway_url: Gateway URL (defaults to EthicalZen cloud gateway)
            tenant_id: Your tenant ID (optional, derived from API key)
            timeout: Request timeout in seconds
            fail_open: If True, allow requests through on gateway error. Default False (fail-closed).
        """
        self.api_key = api_key or os.environ.get("ETHICALZEN_API_KEY")
        if not self.api_key:
            raise AuthenticationError(
                "API key required. Pass api_key or set ETHICALZEN_API_KEY environment variable."
            )
        
        self.certificate_id = certificate_id or os.environ.get("ETHICALZEN_CERTIFICATE_ID")
        self.gateway_url = (
            gateway_url or 
            os.environ.get("ETHICALZEN_GATEWAY_URL") or 
            DEFAULT_GATEWAY_URL
        ).rstrip("/")
        self.tenant_id = tenant_id or os.environ.get("ETHICALZEN_TENANT_ID")
        self.timeout = timeout
        self.fail_open = fail_open
        
        self._client = httpx.Client(timeout=timeout)
    
    def chat_completions(
        self,
        target_url: str,
        target_api_key: str,
        messages: List[Dict[str, str]],
        model: str = "gpt-4",
        **kwargs: Any,
    ) -> ProxyResponse:
        """
        Send a chat completion request through the EthicalZen gateway.
        
        Args:
            target_url: Target LLM API URL (e.g., "https://api.openai.com/v1/chat/completions")
            target_api_key: API key for the target LLM
            messages: Chat messages in OpenAI format
            model: Model name
            **kwargs: Additional parameters passed to the LLM API
            
        Returns:
            ProxyResponse with LLM response or block information
        """
        return self._proxy_request(
            target_url=target_url,
            target_api_key=target_api_key,
            body={
                "model": model,
                "messages": messages,
                **kwargs,
            },
        )
    
    def completions(
        self,
        target_url: str,
        target_api_key: str,
        prompt: str,
        model: str = "gpt-3.5-turbo-instruct",
        **kwargs: Any,
    ) -> ProxyResponse:
        """
        Send a completion request through the EthicalZen gateway.
        
        Args:
            target_url: Target LLM API URL
            target_api_key: API key for the target LLM
            prompt: The prompt text
            model: Model name
            **kwargs: Additional parameters
            
        Returns:
            ProxyResponse with LLM response or block information
        """
        return self._proxy_request(
            target_url=target_url,
            target_api_key=target_api_key,
            body={
                "model": model,
                "prompt": prompt,
                **kwargs,
            },
        )
    
    def _proxy_request(
        self,
        target_url: str,
        target_api_key: str,
        body: Dict[str, Any],
    ) -> ProxyResponse:
        """Send a request through the gateway proxy."""
        headers = {
            "Content-Type": "application/json",
            "X-API-Key": self.api_key,
            "X-Target-Endpoint": target_url,
            "Authorization": f"Bearer {target_api_key}",
        }
        
        if self.certificate_id:
            headers["X-Contract-ID"] = self.certificate_id
        if self.tenant_id:
            headers["X-Tenant-ID"] = self.tenant_id
        
        try:
            response = self._client.post(
                f"{self.gateway_url}/api/proxy",
                headers=headers,
                json=body,
            )
            
            # Check for blocked response
            if response.status_code == 403:
                data = response.json()
                return ProxyResponse(
                    data=data,
                    blocked=True,
                    block_reason=data.get("reason") or data.get("message") or "Request blocked by guardrail",
                    guardrail_id=data.get("guardrail_id"),
                    score=data.get("score"),
                )
            
            # Check for other errors
            if response.status_code >= 400:
                data = response.json() if response.text else {}
                
                if response.status_code == 401:
                    raise AuthenticationError("Invalid API key")
                
                raise APIError(
                    data.get("error") or data.get("message") or f"Gateway error: {response.status_code}",
                    status_code=response.status_code,
                )
            
            # Success
            data = response.json()
            
            # Check if response was blocked (output validation)
            if data.get("blocked"):
                return ProxyResponse(
                    data=data,
                    blocked=True,
                    block_reason=data.get("reason") or "Output blocked by guardrail",
                    guardrail_id=data.get("guardrail_id"),
                    score=data.get("score"),
                )
            
            return ProxyResponse(data=data, blocked=False)
            
        except httpx.TimeoutException:
            if self.fail_open:
                # Fail-open: make direct request (no protection)
                direct_response = self._client.post(
                    target_url,
                    headers={
                        "Content-Type": "application/json",
                        "Authorization": f"Bearer {target_api_key}",
                    },
                    json=body,
                )
                return ProxyResponse(data=direct_response.json(), blocked=False)
            raise EthicalZenError("Gateway request timed out", status_code=408)
            
        except httpx.RequestError as e:
            if self.fail_open:
                # Fail-open: make direct request
                direct_response = self._client.post(
                    target_url,
                    headers={
                        "Content-Type": "application/json",
                        "Authorization": f"Bearer {target_api_key}",
                    },
                    json=body,
                )
                return ProxyResponse(data=direct_response.json(), blocked=False)
            raise EthicalZenError(f"Gateway connection error: {e}")
    
    def close(self) -> None:
        """Close the HTTP client."""
        self._client.close()
    
    def __enter__(self) -> "EthicalZenProxy":
        return self
    
    def __exit__(self, *args: Any) -> None:
        self.close()


# Convenience function to wrap OpenAI client
def wrap_openai(
    openai_client: Any,
    api_key: Optional[str] = None,
    certificate_id: Optional[str] = None,
    gateway_url: Optional[str] = None,
) -> "WrappedOpenAI":
    """
    Wrap an OpenAI client to route requests through EthicalZen.
    
    Usage:
        from openai import OpenAI
        from ethicalzen.proxy import wrap_openai
        
        client = OpenAI()
        protected_client = wrap_openai(
            client,
            api_key="sk-ethicalzen-key",
            certificate_id="dc_my_app"
        )
        
        # Use like normal - requests are automatically protected
        response = protected_client.chat.completions.create(
            model="gpt-4",
            messages=[{"role": "user", "content": "Hello!"}]
        )
    """
    return WrappedOpenAI(
        openai_client=openai_client,
        ethicalzen_api_key=api_key,
        certificate_id=certificate_id,
        gateway_url=gateway_url,
    )


class WrappedOpenAI:
    """
    Wrapped OpenAI client that routes requests through EthicalZen gateway.
    """
    
    def __init__(
        self,
        openai_client: Any,
        ethicalzen_api_key: Optional[str] = None,
        certificate_id: Optional[str] = None,
        gateway_url: Optional[str] = None,
    ):
        self._openai = openai_client
        self._proxy = EthicalZenProxy(
            api_key=ethicalzen_api_key,
            certificate_id=certificate_id,
            gateway_url=gateway_url,
        )
        self.chat = WrappedChat(self._openai, self._proxy)
    
    def close(self) -> None:
        self._proxy.close()


class WrappedChat:
    """Wrapped chat namespace."""
    
    def __init__(self, openai_client: Any, proxy: EthicalZenProxy):
        self._openai = openai_client
        self._proxy = proxy
        self.completions = WrappedCompletions(openai_client, proxy)


class WrappedCompletions:
    """Wrapped completions."""
    
    def __init__(self, openai_client: Any, proxy: EthicalZenProxy):
        self._openai = openai_client
        self._proxy = proxy
    
    def create(self, **kwargs: Any) -> ProxyResponse:
        """Create a chat completion through EthicalZen gateway."""
        # Extract OpenAI API key from client
        openai_api_key = getattr(self._openai, "api_key", None) or os.environ.get("OPENAI_API_KEY")
        if not openai_api_key:
            raise AuthenticationError("OpenAI API key not found")
        
        return self._proxy.chat_completions(
            target_url="https://api.openai.com/v1/chat/completions",
            target_api_key=openai_api_key,
            **kwargs,
        )

