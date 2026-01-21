"""
EthicalZen Proxy Client - Transparent API protection for ANY web service.

This module provides a proxy client that wraps ANY HTTP API calls
and routes them through the EthicalZen gateway for input/output validation.

Works with: LLMs, REST APIs, GraphQL, internal microservices, third-party APIs, etc.

Usage:
    from ethicalzen import EthicalZenProxy
    
    # Create proxy client
    proxy = EthicalZenProxy(
        api_key="your-ethicalzen-key",
        certificate_id="dc_your_certificate",
    )
    
    # POST request to any endpoint
    response = proxy.post(
        "https://api.openai.com/v1/chat/completions",
        json={"model": "gpt-4", "messages": [...]},
        headers={"Authorization": "Bearer sk-..."}
    )
    
    # GET request
    response = proxy.get(
        "https://api.example.com/users/123",
        headers={"Authorization": "Bearer token"}
    )
    
    # Check if blocked
    if response.blocked:
        print(f"Request blocked: {response.block_reason}")
    else:
        print(response.json())
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


DEFAULT_GATEWAY_URL = "https://gateway.ethicalzen.ai"


class ProxyResponse:
    """
    Response from a proxied request.
    
    Provides access to the response data and block status.
    """
    
    def __init__(
        self,
        status_code: int,
        data: Any,
        headers: Dict[str, str],
        blocked: bool = False,
        block_reason: Optional[str] = None,
        guardrail_id: Optional[str] = None,
        score: Optional[float] = None,
        raw_response: Optional[httpx.Response] = None,
    ):
        self.status_code = status_code
        self._data = data
        self.headers = headers
        self.blocked = blocked
        self.block_reason = block_reason
        self.guardrail_id = guardrail_id
        self.score = score
        self._raw = raw_response
    
    def json(self) -> Any:
        """Get response as JSON (dict or list)."""
        return self._data
    
    @property
    def data(self) -> Any:
        """Alias for json() - get response data."""
        return self._data
    
    @property
    def text(self) -> str:
        """Get response as text."""
        if isinstance(self._data, (dict, list)):
            import json
            return json.dumps(self._data)
        return str(self._data)
    
    @property
    def ok(self) -> bool:
        """Check if response is successful (2xx) and not blocked."""
        return 200 <= self.status_code < 300 and not self.blocked
    
    # OpenAI-compatible convenience properties
    @property
    def choices(self) -> List[Dict]:
        """Get choices from OpenAI-style response."""
        if isinstance(self._data, dict):
            return self._data.get("choices", [])
        return []
    
    @property
    def content(self) -> str:
        """Get content from first choice (OpenAI-style)."""
        if self.blocked:
            return f"[BLOCKED] {self.block_reason}"
        if self.choices:
            msg = self.choices[0].get("message", {})
            return msg.get("content", "") if isinstance(msg, dict) else ""
        return ""
    
    @property
    def violation(self) -> Optional[Dict[str, Any]]:
        """Get violation details if blocked."""
        if not self.blocked:
            return None
        return {
            "guardrail": self.guardrail_id,
            "reason": self.block_reason,
            "score": self.score,
            "message": self.block_reason,
        }
    
    def __repr__(self) -> str:
        if self.blocked:
            return f"<ProxyResponse blocked=True reason='{self.block_reason}'>"
        return f"<ProxyResponse status={self.status_code} ok={self.ok}>"


class EthicalZenProxy:
    """
    Proxy client that routes ANY HTTP API calls through EthicalZen gateway.
    
    The gateway validates both request (input) and response (output)
    against your configured guardrails/certificates.
    
    Works with ANY HTTP endpoint: REST APIs, LLMs, GraphQL, microservices, etc.
    
    Usage:
        proxy = EthicalZenProxy(
            api_key="sk-ethicalzen-key",
            certificate_id="dc_my_app"
        )
        
        # POST to any endpoint
        response = proxy.post(
            "https://api.openai.com/v1/chat/completions",
            json={"model": "gpt-4", "messages": [{"role": "user", "content": "Hi"}]},
            headers={"Authorization": "Bearer sk-openai-key"}
        )
        
        # GET request
        response = proxy.get(
            "https://api.stripe.com/v1/customers/cus_123",
            headers={"Authorization": "Bearer sk-stripe-key"}
        )
        
        # Check result
        if response.blocked:
            print(f"Blocked: {response.block_reason}")
        else:
            print(response.json())
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
    
    def request(
        self,
        method: str,
        url: str,
        *,
        json: Optional[Any] = None,
        data: Optional[Any] = None,
        headers: Optional[Dict[str, str]] = None,
        params: Optional[Dict[str, Any]] = None,
        **kwargs: Any,
    ) -> ProxyResponse:
        """
        Send an HTTP request through the EthicalZen gateway.
        
        Args:
            method: HTTP method (GET, POST, PUT, DELETE, PATCH, etc.)
            url: Target URL (the actual API you want to call)
            json: JSON body (for POST/PUT/PATCH)
            data: Form data or raw body
            headers: Headers to pass to the target API
            params: Query parameters
            **kwargs: Additional arguments
            
        Returns:
            ProxyResponse with response data or block information
        """
        # Build gateway headers
        gateway_headers = {
            "Content-Type": "application/json",
            "X-API-Key": self.api_key,
            "X-Target-Endpoint": url,
            "X-Target-Method": method.upper(),
        }
        
        if self.certificate_id:
            gateway_headers["X-Contract-ID"] = self.certificate_id
        if self.tenant_id:
            gateway_headers["X-Tenant-ID"] = self.tenant_id
        
        # Pass through target headers (like Authorization)
        if headers:
            for key, value in headers.items():
                # Pass auth headers directly
                if key.lower() in ("authorization", "x-api-key", "api-key"):
                    gateway_headers[key] = value
                else:
                    # Prefix other headers to avoid conflicts
                    gateway_headers[f"X-Target-Header-{key}"] = value
        
        # Build request body for gateway
        gateway_body: Dict[str, Any] = {}
        if json is not None:
            gateway_body = json if isinstance(json, dict) else {"_body": json}
        elif data is not None:
            gateway_body = {"_raw_data": data}
        
        if params:
            gateway_body["_query_params"] = params
        
        try:
            response = self._client.post(
                f"{self.gateway_url}/api/proxy",
                headers=gateway_headers,
                json=gateway_body if gateway_body else None,
            )
            
            # Check for blocked response
            if response.status_code == 403:
                try:
                    resp_data = response.json()
                except Exception:
                    resp_data = {"message": response.text}
                    
                return ProxyResponse(
                    status_code=403,
                    data=resp_data,
                    headers=dict(response.headers),
                    blocked=True,
                    block_reason=resp_data.get("reason") or resp_data.get("message") or "Request blocked by guardrail",
                    guardrail_id=resp_data.get("guardrail_id"),
                    score=resp_data.get("score"),
                    raw_response=response,
                )
            
            # Check for auth errors
            if response.status_code == 401:
                raise AuthenticationError("Invalid API key")
            
            # Parse response
            try:
                resp_data = response.json()
            except Exception:
                resp_data = response.text
            
            # Check if response was blocked (output validation)
            if isinstance(resp_data, dict) and resp_data.get("blocked"):
                return ProxyResponse(
                    status_code=response.status_code,
                    data=resp_data,
                    headers=dict(response.headers),
                    blocked=True,
                    block_reason=resp_data.get("reason") or "Output blocked by guardrail",
                    guardrail_id=resp_data.get("guardrail_id"),
                    score=resp_data.get("score"),
                    raw_response=response,
                )
            
            return ProxyResponse(
                status_code=response.status_code,
                data=resp_data,
                headers=dict(response.headers),
                blocked=False,
                raw_response=response,
            )
            
        except httpx.TimeoutException:
            if self.fail_open:
                return self._direct_request(method, url, json=json, data=data, headers=headers, params=params)
            raise EthicalZenError("Gateway request timed out", status_code=408)
            
        except httpx.RequestError as e:
            if self.fail_open:
                return self._direct_request(method, url, json=json, data=data, headers=headers, params=params)
            raise EthicalZenError(f"Gateway connection error: {e}")
    
    def _direct_request(
        self,
        method: str,
        url: str,
        **kwargs: Any,
    ) -> ProxyResponse:
        """Make a direct request (bypass gateway) for fail-open mode."""
        response = self._client.request(method, url, **kwargs)
        try:
            data = response.json()
        except Exception:
            data = response.text
        return ProxyResponse(
            status_code=response.status_code,
            data=data,
            headers=dict(response.headers),
            blocked=False,
            raw_response=response,
        )
    
    # Convenience methods for common HTTP methods
    def get(self, url: str, **kwargs: Any) -> ProxyResponse:
        """Send a GET request through the gateway."""
        return self.request("GET", url, **kwargs)
    
    def post(self, url: str, **kwargs: Any) -> ProxyResponse:
        """Send a POST request through the gateway."""
        return self.request("POST", url, **kwargs)
    
    def put(self, url: str, **kwargs: Any) -> ProxyResponse:
        """Send a PUT request through the gateway."""
        return self.request("PUT", url, **kwargs)
    
    def patch(self, url: str, **kwargs: Any) -> ProxyResponse:
        """Send a PATCH request through the gateway."""
        return self.request("PATCH", url, **kwargs)
    
    def delete(self, url: str, **kwargs: Any) -> ProxyResponse:
        """Send a DELETE request through the gateway."""
        return self.request("DELETE", url, **kwargs)
    
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
        from ethicalzen import wrap_openai
        
        client = OpenAI()
        protected = wrap_openai(
            client,
            api_key="sk-ethicalzen-key",
            certificate_id="dc_my_app"
        )
        
        # Use like normal - requests are automatically protected
        response = protected.chat.completions.create(
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
    """Wrapped OpenAI client that routes requests through EthicalZen gateway."""
    
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
        openai_api_key = getattr(self._openai, "api_key", None) or os.environ.get("OPENAI_API_KEY")
        if not openai_api_key:
            raise AuthenticationError("OpenAI API key not found")
        
        return self._proxy.post(
            "https://api.openai.com/v1/chat/completions",
            json=kwargs,
            headers={"Authorization": f"Bearer {openai_api_key}"},
        )
