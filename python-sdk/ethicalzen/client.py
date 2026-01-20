"""EthicalZen API client implementations."""

import os
from typing import Any, Dict, List, Optional, Union

import httpx

from ethicalzen.models import (
    Decision,
    DesignResult,
    EvaluationResult,
    GuardrailConfig,
    OptimizeResult,
    SimulationResult,
    Template,
)
from ethicalzen.exceptions import (
    APIError,
    AuthenticationError,
    EthicalZenError,
    GuardrailNotFoundError,
    RateLimitError,
    ValidationError,
)


DEFAULT_BASE_URL = "https://ethicalzen-backend-400782183161.us-central1.run.app"
DEFAULT_TIMEOUT = 60.0


class EthicalZen:
    """
    Synchronous EthicalZen API client.
    
    Usage:
        client = EthicalZen(api_key="your-api-key")
        
        # Evaluate content against a guardrail
        result = client.evaluate(
            guardrail="medical_advice_smart",
            input="What medication should I take?"
        )
        
        if result.decision == "BLOCK":
            print("Blocked:", result.reason)
    """

    def __init__(
        self,
        api_key: Optional[str] = None,
        base_url: Optional[str] = None,
        timeout: float = DEFAULT_TIMEOUT,
    ):
        """
        Initialize the EthicalZen client.
        
        Args:
            api_key: Your EthicalZen API key. If not provided, reads from
                     ETHICALZEN_API_KEY environment variable.
            base_url: API base URL. Defaults to production API.
            timeout: Request timeout in seconds. Default is 60s.
        """
        self.api_key = api_key or os.environ.get("ETHICALZEN_API_KEY")
        if not self.api_key:
            raise AuthenticationError(
                "API key required. Pass api_key or set ETHICALZEN_API_KEY environment variable."
            )
        
        self.base_url = (base_url or os.environ.get("ETHICALZEN_BASE_URL") or DEFAULT_BASE_URL).rstrip("/")
        self.timeout = timeout
        
        self._client = httpx.Client(
            base_url=self.base_url,
            headers={
                "X-API-Key": self.api_key,
                "Content-Type": "application/json",
                "User-Agent": "ethicalzen-python/0.1.0",
            },
            timeout=timeout,
        )

    def _handle_response(self, response: httpx.Response) -> Dict[str, Any]:
        """Handle API response and raise appropriate exceptions."""
        if response.status_code == 401:
            raise AuthenticationError("Invalid API key")
        
        if response.status_code == 429:
            retry_after = response.headers.get("Retry-After")
            raise RateLimitError(
                "Rate limit exceeded. Please slow down or upgrade your plan.",
                retry_after=int(retry_after) if retry_after else None
            )
        
        if response.status_code == 404:
            data = response.json() if response.text else {}
            raise GuardrailNotFoundError(data.get("guardrail_id", "unknown"))
        
        if response.status_code >= 400:
            try:
                data = response.json()
                message = data.get("error") or data.get("message") or "API request failed"
            except Exception:
                message = response.text or "API request failed"
            raise APIError(message, status_code=response.status_code, response_body=response.text)
        
        return response.json()

    def evaluate(
        self,
        guardrail: str,
        input: str,  # noqa: A002 - using 'input' to match API
        context: Optional[Dict[str, Any]] = None,
    ) -> EvaluationResult:
        """
        Evaluate content against a guardrail.
        
        Args:
            guardrail: Guardrail ID (e.g., "medical_advice_smart")
            input: The text content to evaluate
            context: Optional context for evaluation
            
        Returns:
            EvaluationResult with decision, score, and reason
            
        Example:
            result = client.evaluate(
                guardrail="medical_advice_smart",
                input="What medication should I take for a headache?"
            )
            
            if result.is_blocked:
                print(f"Blocked: {result.reason}")
        """
        if not guardrail:
            raise ValidationError("guardrail is required", field="guardrail")
        if not input:
            raise ValidationError("input is required", field="input")

        response = self._client.post(
            "/api/sg/evaluate",
            json={
                "guardrail_id": guardrail,
                "input": input,
                "context": context,
            },
        )
        
        data = self._handle_response(response)
        
        # Normalize decision to uppercase (API returns lowercase)
        decision_str = data.get("decision", "ALLOW").upper()
        
        return EvaluationResult(
            decision=Decision(decision_str),
            score=data.get("score", 0.0),
            reason=data.get("reason"),
            guardrail_id=guardrail,
            latency_ms=data.get("latency_ms"),
            metadata=data.get("metadata"),
        )

    def design(
        self,
        description: str,
        safe_examples: Optional[List[str]] = None,
        unsafe_examples: Optional[List[str]] = None,
        auto_simulate: bool = True,
    ) -> DesignResult:
        """
        Design a new guardrail from natural language description.
        
        Args:
            description: Natural language description of what to block/allow
            safe_examples: Examples that should be allowed
            unsafe_examples: Examples that should be blocked
            auto_simulate: Whether to run simulation after design
            
        Returns:
            DesignResult with the generated guardrail configuration
            
        Example:
            result = client.design(
                description="Block requests for medical diagnoses. Allow general health tips.",
                safe_examples=["What foods are healthy?"],
                unsafe_examples=["Diagnose my symptoms"]
            )
            
            print(f"Created guardrail: {result.config.id}")
            print(f"Accuracy: {result.simulation.metrics.accuracy:.0%}")
        """
        if not description:
            raise ValidationError("description is required", field="description")

        response = self._client.post(
            "/api/sg/design",
            json={
                "naturalLanguage": description,
                "safeExamples": safe_examples or [],
                "unsafeExamples": unsafe_examples or [],
                "autoSimulate": auto_simulate,
            },
        )
        
        data = self._handle_response(response)
        
        config_data = data.get("config", data)
        config = GuardrailConfig(
            id=config_data.get("id", ""),
            name=config_data.get("name", ""),
            description=config_data.get("description", description),
            t_allow=config_data.get("thresholdLow", 0.30),
            t_block=config_data.get("thresholdHigh", 0.70),
            safe_examples=config_data.get("safeExamples", []),
            unsafe_examples=config_data.get("unsafeExamples", []),
        )
        
        simulation = None
        if data.get("simulation"):
            sim_data = data["simulation"]
            metrics = sim_data.get("metrics", {})
            simulation = SimulationResult(
                success=True,
                metrics={
                    "accuracy": metrics.get("accuracy", 0),
                    "precision": metrics.get("precision", 0),
                    "recall": metrics.get("recall", 0),
                    "f1_score": metrics.get("f1", 0),
                    "total_tests": metrics.get("total", 0),
                    "correct": metrics.get("correct", 0),
                    "false_positives": metrics.get("falsePositives", 0),
                    "false_negatives": metrics.get("falseNegatives", 0),
                },
                test_results=sim_data.get("results"),
            )
        
        return DesignResult(
            success=data.get("success", True),
            config=config,
            simulation=simulation,
            message=data.get("message"),
        )

    def simulate(
        self,
        guardrail: str,
        test_cases: Optional[List[Dict[str, Any]]] = None,
    ) -> SimulationResult:
        """
        Run simulation tests on a guardrail.
        
        Args:
            guardrail: Guardrail ID to simulate
            test_cases: Optional custom test cases
            
        Returns:
            SimulationResult with accuracy metrics
        """
        response = self._client.post(
            f"/api/sg/simulate/{guardrail}",
            json={"testCases": test_cases} if test_cases else {},
        )
        
        data = self._handle_response(response)
        metrics = data.get("metrics", {})
        
        return SimulationResult(
            success=data.get("success", True),
            metrics={
                "accuracy": metrics.get("accuracy", 0),
                "precision": metrics.get("precision", 0),
                "recall": metrics.get("recall", 0),
                "f1_score": metrics.get("f1", 0),
                "total_tests": metrics.get("total", 0),
                "correct": metrics.get("correct", 0),
                "false_positives": metrics.get("falsePositives", 0),
                "false_negatives": metrics.get("falseNegatives", 0),
            },
            test_results=data.get("results"),
        )

    def optimize(
        self,
        guardrail: str,
        target_accuracy: float = 0.80,
        max_iterations: int = 3,
    ) -> OptimizeResult:
        """
        Auto-tune a guardrail to improve accuracy.
        
        Args:
            guardrail: Guardrail ID to optimize
            target_accuracy: Target accuracy (0-1)
            max_iterations: Maximum optimization iterations
            
        Returns:
            OptimizeResult with before/after metrics
        """
        response = self._client.post(
            "/api/sg/optimize",
            json={
                "guardrailId": guardrail,
                "targetAccuracy": target_accuracy,
                "maxIterations": max_iterations,
            },
        )
        
        data = self._handle_response(response)
        
        return OptimizeResult(
            success=data.get("success", True),
            iterations=data.get("iterations", 0),
            before_accuracy=data.get("beforeAccuracy", 0),
            after_accuracy=data.get("afterAccuracy", 0),
            improvements=data.get("improvements", []),
        )

    def list_templates(self) -> List[Template]:
        """
        List available guardrail templates.
        
        Returns:
            List of Template objects
        """
        response = self._client.get("/api/sg/templates")
        data = self._handle_response(response)
        
        templates = []
        for t in data.get("templates", []):
            templates.append(Template(
                id=t.get("id", ""),
                name=t.get("name", ""),
                description=t.get("description", ""),
                category=t.get("category", "general"),
                accuracy=t.get("accuracy"),
            ))
        
        return templates

    def get_guardrail(self, guardrail: str) -> GuardrailConfig:
        """
        Get a guardrail configuration.
        
        Args:
            guardrail: Guardrail ID
            
        Returns:
            GuardrailConfig object
        """
        response = self._client.get(f"/api/sg/guardrails/{guardrail}")
        data = self._handle_response(response)
        
        return GuardrailConfig(
            id=data.get("id", guardrail),
            name=data.get("name", ""),
            description=data.get("description", ""),
            t_allow=data.get("thresholdLow", 0.30),
            t_block=data.get("thresholdHigh", 0.70),
            safe_examples=data.get("safeExamples", []),
            unsafe_examples=data.get("unsafeExamples", []),
        )

    def close(self) -> None:
        """Close the HTTP client."""
        self._client.close()

    def __enter__(self) -> "EthicalZen":
        return self

    def __exit__(self, *args: Any) -> None:
        self.close()


class AsyncEthicalZen:
    """
    Asynchronous EthicalZen API client.
    
    Usage:
        async with AsyncEthicalZen(api_key="your-api-key") as client:
            result = await client.evaluate(
                guardrail="medical_advice_smart",
                input="What medication should I take?"
            )
    """

    def __init__(
        self,
        api_key: Optional[str] = None,
        base_url: Optional[str] = None,
        timeout: float = DEFAULT_TIMEOUT,
    ):
        """Initialize the async EthicalZen client."""
        self.api_key = api_key or os.environ.get("ETHICALZEN_API_KEY")
        if not self.api_key:
            raise AuthenticationError(
                "API key required. Pass api_key or set ETHICALZEN_API_KEY environment variable."
            )
        
        self.base_url = (base_url or os.environ.get("ETHICALZEN_BASE_URL") or DEFAULT_BASE_URL).rstrip("/")
        self.timeout = timeout
        
        self._client = httpx.AsyncClient(
            base_url=self.base_url,
            headers={
                "X-API-Key": self.api_key,
                "Content-Type": "application/json",
                "User-Agent": "ethicalzen-python/0.1.0",
            },
            timeout=timeout,
        )

    def _handle_response(self, response: httpx.Response) -> Dict[str, Any]:
        """Handle API response and raise appropriate exceptions."""
        if response.status_code == 401:
            raise AuthenticationError("Invalid API key")
        
        if response.status_code == 429:
            retry_after = response.headers.get("Retry-After")
            raise RateLimitError(
                "Rate limit exceeded. Please slow down or upgrade your plan.",
                retry_after=int(retry_after) if retry_after else None
            )
        
        if response.status_code == 404:
            data = response.json() if response.text else {}
            raise GuardrailNotFoundError(data.get("guardrail_id", "unknown"))
        
        if response.status_code >= 400:
            try:
                data = response.json()
                message = data.get("error") or data.get("message") or "API request failed"
            except Exception:
                message = response.text or "API request failed"
            raise APIError(message, status_code=response.status_code, response_body=response.text)
        
        return response.json()

    async def evaluate(
        self,
        guardrail: str,
        input: str,  # noqa: A002
        context: Optional[Dict[str, Any]] = None,
    ) -> EvaluationResult:
        """Evaluate content against a guardrail (async)."""
        if not guardrail:
            raise ValidationError("guardrail is required", field="guardrail")
        if not input:
            raise ValidationError("input is required", field="input")

        response = await self._client.post(
            "/api/sg/evaluate",
            json={
                "guardrail_id": guardrail,
                "input": input,
                "context": context,
            },
        )
        
        data = self._handle_response(response)
        
        # Normalize decision to uppercase (API returns lowercase)
        decision_str = data.get("decision", "ALLOW").upper()
        
        return EvaluationResult(
            decision=Decision(decision_str),
            score=data.get("score", 0.0),
            reason=data.get("reason"),
            guardrail_id=guardrail,
            latency_ms=data.get("latency_ms"),
            metadata=data.get("metadata"),
        )

    async def design(
        self,
        description: str,
        safe_examples: Optional[List[str]] = None,
        unsafe_examples: Optional[List[str]] = None,
        auto_simulate: bool = True,
    ) -> DesignResult:
        """Design a new guardrail from natural language (async)."""
        if not description:
            raise ValidationError("description is required", field="description")

        response = await self._client.post(
            "/api/sg/design",
            json={
                "naturalLanguage": description,
                "safeExamples": safe_examples or [],
                "unsafeExamples": unsafe_examples or [],
                "autoSimulate": auto_simulate,
            },
        )
        
        data = self._handle_response(response)
        
        config_data = data.get("config", data)
        config = GuardrailConfig(
            id=config_data.get("id", ""),
            name=config_data.get("name", ""),
            description=config_data.get("description", description),
            t_allow=config_data.get("thresholdLow", 0.30),
            t_block=config_data.get("thresholdHigh", 0.70),
            safe_examples=config_data.get("safeExamples", []),
            unsafe_examples=config_data.get("unsafeExamples", []),
        )
        
        simulation = None
        if data.get("simulation"):
            sim_data = data["simulation"]
            metrics = sim_data.get("metrics", {})
            simulation = SimulationResult(
                success=True,
                metrics={
                    "accuracy": metrics.get("accuracy", 0),
                    "precision": metrics.get("precision", 0),
                    "recall": metrics.get("recall", 0),
                    "f1_score": metrics.get("f1", 0),
                    "total_tests": metrics.get("total", 0),
                    "correct": metrics.get("correct", 0),
                    "false_positives": metrics.get("falsePositives", 0),
                    "false_negatives": metrics.get("falseNegatives", 0),
                },
                test_results=sim_data.get("results"),
            )
        
        return DesignResult(
            success=data.get("success", True),
            config=config,
            simulation=simulation,
            message=data.get("message"),
        )

    async def close(self) -> None:
        """Close the HTTP client."""
        await self._client.aclose()

    async def __aenter__(self) -> "AsyncEthicalZen":
        return self

    async def __aexit__(self, *args: Any) -> None:
        await self.close()

