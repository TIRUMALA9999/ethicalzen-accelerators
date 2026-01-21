"""Custom exceptions for EthicalZen SDK."""

from typing import Optional


class EthicalZenError(Exception):
    """Base exception for all EthicalZen errors."""

    def __init__(self, message: str, status_code: Optional[int] = None):
        self.message = message
        self.status_code = status_code
        super().__init__(self.message)


class AuthenticationError(EthicalZenError):
    """Raised when API key is invalid or missing."""

    def __init__(self, message: str = "Invalid or missing API key"):
        super().__init__(message, status_code=401)


class RateLimitError(EthicalZenError):
    """Raised when rate limit is exceeded."""

    def __init__(
        self, 
        message: str = "Rate limit exceeded", 
        retry_after: Optional[int] = None
    ):
        super().__init__(message, status_code=429)
        self.retry_after = retry_after


class APIError(EthicalZenError):
    """Raised when the API returns an error."""

    def __init__(
        self, 
        message: str, 
        status_code: Optional[int] = None,
        response_body: Optional[str] = None
    ):
        super().__init__(message, status_code)
        self.response_body = response_body


class ValidationError(EthicalZenError):
    """Raised when input validation fails."""

    def __init__(self, message: str, field: Optional[str] = None):
        super().__init__(message, status_code=400)
        self.field = field


class GuardrailNotFoundError(EthicalZenError):
    """Raised when a guardrail is not found."""

    def __init__(self, guardrail_id: str):
        super().__init__(f"Guardrail not found: {guardrail_id}", status_code=404)
        self.guardrail_id = guardrail_id


class TimeoutError(EthicalZenError):
    """Raised when a request times out."""

    def __init__(self, message: str = "Request timed out"):
        super().__init__(message, status_code=408)


