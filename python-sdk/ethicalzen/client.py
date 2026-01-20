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
MAX_INPUT_LENGTH = 100000  # 100KB max input to prevent DoS


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
        if len(input) > MAX_INPUT_LENGTH:
            raise ValidationError(
                f"input exceeds maximum length of {MAX_INPUT_LENGTH} characters",
                field="input"
            )

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
        config: Optional[Dict[str, Any]] = None,
    ) -> SimulationResult:
        """
        Run simulation tests on a guardrail.
        
        Args:
            guardrail: Guardrail ID to simulate
            test_cases: Optional custom test cases
            config: Optional guardrail config (if not saved yet)
            
        Returns:
            SimulationResult with accuracy metrics
        """
        body: Dict[str, Any] = {"guardrailId": guardrail}
        if test_cases:
            body["testCases"] = test_cases
        if config:
            body["config"] = config
            
        response = self._client.post(
            "/api/sg/simulate",
            json=body,
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
            # Backend returns: type, displayName, description, exampleCount, expectedMetrics
            templates.append(Template(
                id=t.get("type", ""),  # 'type' is the template ID
                name=t.get("displayName", ""),  # 'displayName' is the name
                description=t.get("description", ""),
                category=t.get("category", "general"),
                accuracy=t.get("expectedMetrics", {}).get("accuracy"),
            ))
        
        return templates
    
    def get_template(self, template_id: str) -> Template:
        """
        Get a specific guardrail template with examples.
        
        Args:
            template_id: Template type ID (e.g., "medical_advice", "pii_blocker")
            
        Returns:
            Template object with examples
        """
        response = self._client.get(f"/api/sg/templates/{template_id}")
        data = self._handle_response(response)
        
        t = data.get("template", {})
        return Template(
            id=data.get("type", template_id),
            name=t.get("displayName", ""),
            description=t.get("description", ""),
            category="general",
            accuracy=t.get("expectedAccuracy"),
            safe_examples=t.get("safeExamples", []),
            unsafe_examples=t.get("unsafeExamples", []),
        )

    def list_guardrails(self) -> List[GuardrailConfig]:
        """
        List all guardrails for the tenant.
        
        Returns:
            List of GuardrailConfig objects
        """
        response = self._client.get("/api/sg/list")
        data = self._handle_response(response)
        
        guardrails = []
        for g in data.get("guardrails", []):
            guardrails.append(GuardrailConfig(
                id=g.get("id", ""),
                name=g.get("name", ""),
                description=g.get("description", ""),
                t_allow=g.get("thresholdLow", g.get("t_allow", 0.30)),
                t_block=g.get("thresholdHigh", g.get("t_block", 0.70)),
                safe_examples=g.get("safeExamples", g.get("safe_examples", [])),
                unsafe_examples=g.get("unsafeExamples", g.get("unsafe_examples", [])),
            ))
        
        return guardrails

    def get_guardrail(self, guardrail: str) -> GuardrailConfig:
        """
        Get a specific guardrail configuration.
        
        Args:
            guardrail: Guardrail ID
            
        Returns:
            GuardrailConfig object
            
        Raises:
            GuardrailNotFoundError: If guardrail not found
        """
        # List all and filter by ID (backend doesn't have single-get endpoint)
        guardrails = self.list_guardrails()
        for g in guardrails:
            if g.id == guardrail:
                return g
        
        raise GuardrailNotFoundError(guardrail)

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
        if len(input) > MAX_INPUT_LENGTH:
            raise ValidationError(
                f"input exceeds maximum length of {MAX_INPUT_LENGTH} characters",
                field="input"
            )

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

