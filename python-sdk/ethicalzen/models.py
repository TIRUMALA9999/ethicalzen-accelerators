"""Data models for EthicalZen SDK."""

from enum import Enum
from typing import Any, Dict, List, Optional
from pydantic import BaseModel, ConfigDict, Field


class Decision(str, Enum):
    """Guardrail evaluation decision."""
    ALLOW = "ALLOW"
    BLOCK = "BLOCK"
    REVIEW = "REVIEW"


class EvaluationResult(BaseModel):
    """Result of a guardrail evaluation."""
    
    decision: Decision = Field(description="The guardrail decision: ALLOW, BLOCK, or REVIEW")
    score: float = Field(ge=0.0, le=1.0, description="Confidence score (0-1)")
    reason: Optional[str] = Field(default=None, description="Explanation for the decision")
    guardrail_id: str = Field(description="ID of the guardrail that was evaluated")
    latency_ms: Optional[float] = Field(default=None, description="Evaluation latency in milliseconds")
    metadata: Optional[Dict[str, Any]] = Field(default=None, description="Additional metadata")

    @property
    def is_allowed(self) -> bool:
        """Check if content is allowed."""
        return self.decision == Decision.ALLOW

    @property
    def is_blocked(self) -> bool:
        """Check if content is blocked."""
        return self.decision == Decision.BLOCK

    @property
    def needs_review(self) -> bool:
        """Check if content needs human review."""
        return self.decision == Decision.REVIEW


class GuardrailConfig(BaseModel):
    """Configuration for a guardrail."""
    
    id: str = Field(description="Unique guardrail identifier")
    name: str = Field(description="Human-readable name")
    description: str = Field(description="What this guardrail does")
    threshold_allow: float = Field(
        default=0.30, 
        ge=0.0, 
        le=1.0,
        alias="t_allow",
        description="Score below this = ALLOW"
    )
    threshold_block: float = Field(
        default=0.70, 
        ge=0.0, 
        le=1.0,
        alias="t_block", 
        description="Score above this = BLOCK"
    )
    safe_examples: List[str] = Field(default_factory=list, description="Examples that should be allowed")
    unsafe_examples: List[str] = Field(default_factory=list, description="Examples that should be blocked")
    lexical_patterns: Optional[List[str]] = Field(default=None, description="Regex patterns for lexical matching")
    metadata: Optional[Dict[str, Any]] = Field(default=None, description="Additional metadata")

    model_config = ConfigDict(populate_by_name=True)


class SimulationMetrics(BaseModel):
    """Metrics from guardrail simulation."""
    
    accuracy: float = Field(ge=0.0, le=1.0, description="Overall accuracy")
    precision: float = Field(ge=0.0, le=1.0, description="Precision score")
    recall: float = Field(ge=0.0, le=1.0, description="Recall score")
    f1_score: float = Field(ge=0.0, le=1.0, description="F1 score")
    total_tests: int = Field(description="Total test cases run")
    correct: int = Field(description="Number of correct predictions")
    false_positives: int = Field(default=0, description="False positive count")
    false_negatives: int = Field(default=0, description="False negative count")


class SimulationResult(BaseModel):
    """Result of a guardrail simulation."""
    
    success: bool = Field(description="Whether simulation completed successfully")
    metrics: Dict[str, Any] = Field(description="Simulation metrics")
    test_results: Optional[List[Dict[str, Any]]] = Field(
        default=None, 
        description="Detailed results for each test case"
    )


class DesignResult(BaseModel):
    """Result of guardrail design from natural language."""
    
    success: bool = Field(description="Whether design completed successfully")
    config: GuardrailConfig = Field(description="Generated guardrail configuration")
    simulation: Optional[SimulationResult] = Field(
        default=None, 
        description="Simulation results if auto-simulated"
    )
    message: Optional[str] = Field(default=None, description="Additional message")


class OptimizeResult(BaseModel):
    """Result of guardrail optimization."""
    
    success: bool = Field(description="Whether optimization completed")
    iterations: int = Field(description="Number of optimization iterations")
    before_accuracy: float = Field(ge=0.0, le=1.0, description="Accuracy before optimization")
    after_accuracy: float = Field(ge=0.0, le=1.0, description="Accuracy after optimization")
    improvements: List[str] = Field(default_factory=list, description="List of improvements made")
    config: Optional[GuardrailConfig] = Field(default=None, description="Optimized configuration")


class Template(BaseModel):
    """Pre-built guardrail template."""
    
    id: str = Field(description="Template identifier")
    name: str = Field(description="Template name")
    description: str = Field(description="What this template does")
    category: str = Field(default="general", description="Template category")
    accuracy: Optional[float] = Field(default=None, description="Reported accuracy")
    safe_examples: List[str] = Field(default_factory=list, description="Example inputs that should be allowed")
    unsafe_examples: List[str] = Field(default_factory=list, description="Example inputs that should be blocked")

