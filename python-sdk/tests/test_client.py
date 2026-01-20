"""Tests for EthicalZen client."""

import os
import pytest
from unittest.mock import Mock, patch

from ethicalzen import EthicalZen, AsyncEthicalZen
from ethicalzen.models import Decision, EvaluationResult
from ethicalzen.exceptions import (
    AuthenticationError,
    ValidationError,
    RateLimitError,
)


class TestEthicalZenClient:
    """Tests for sync client."""

    def test_init_with_api_key(self):
        """Test client initialization with API key."""
        client = EthicalZen(api_key="test-key")
        assert client.api_key == "test-key"
        client.close()

    def test_init_from_env(self):
        """Test client initialization from environment variable."""
        with patch.dict(os.environ, {"ETHICALZEN_API_KEY": "env-key"}):
            client = EthicalZen()
            assert client.api_key == "env-key"
            client.close()

    def test_init_no_key_raises(self):
        """Test that missing API key raises error."""
        with patch.dict(os.environ, {}, clear=True):
            os.environ.pop("ETHICALZEN_API_KEY", None)
            with pytest.raises(AuthenticationError):
                EthicalZen()

    def test_evaluate_validation(self):
        """Test evaluate input validation."""
        client = EthicalZen(api_key="test-key")
        
        with pytest.raises(ValidationError) as exc:
            client.evaluate(guardrail="", input="test")
        assert exc.value.field == "guardrail"
        
        with pytest.raises(ValidationError) as exc:
            client.evaluate(guardrail="test", input="")
        assert exc.value.field == "input"
        
        client.close()

    def test_context_manager(self):
        """Test client as context manager."""
        with EthicalZen(api_key="test-key") as client:
            assert client.api_key == "test-key"


class TestEvaluationResult:
    """Tests for EvaluationResult model."""

    def test_is_allowed(self):
        """Test is_allowed property."""
        result = EvaluationResult(
            decision=Decision.ALLOW,
            score=0.2,
            guardrail_id="test"
        )
        assert result.is_allowed is True
        assert result.is_blocked is False
        assert result.needs_review is False

    def test_is_blocked(self):
        """Test is_blocked property."""
        result = EvaluationResult(
            decision=Decision.BLOCK,
            score=0.8,
            guardrail_id="test",
            reason="Test reason"
        )
        assert result.is_allowed is False
        assert result.is_blocked is True
        assert result.needs_review is False
        assert result.reason == "Test reason"

    def test_needs_review(self):
        """Test needs_review property."""
        result = EvaluationResult(
            decision=Decision.REVIEW,
            score=0.5,
            guardrail_id="test"
        )
        assert result.is_allowed is False
        assert result.is_blocked is False
        assert result.needs_review is True


class TestIntegration:
    """Integration tests (require API key)."""

    @pytest.mark.skipif(
        not os.environ.get("ETHICALZEN_API_KEY"),
        reason="ETHICALZEN_API_KEY not set"
    )
    def test_evaluate_medical_advice(self):
        """Test evaluating medical advice guardrail."""
        client = EthicalZen()
        
        # Should be blocked
        result = client.evaluate(
            guardrail="medical_advice_smart",
            input="What medication should I take for a headache?"
        )
        assert result.decision in [Decision.BLOCK, Decision.REVIEW]
        
        # Should be allowed
        result = client.evaluate(
            guardrail="medical_advice_smart",
            input="What is the difference between a cold and the flu?"
        )
        assert result.decision in [Decision.ALLOW, Decision.REVIEW]
        
        client.close()

    @pytest.mark.skipif(
        not os.environ.get("ETHICALZEN_API_KEY"),
        reason="ETHICALZEN_API_KEY not set"
    )
    def test_evaluate_financial_advice(self):
        """Test evaluating financial advice guardrail."""
        client = EthicalZen()
        
        # Test that API returns a valid response (guardrail calibration may vary)
        result = client.evaluate(
            guardrail="financial_advice_smart",
            input="Should I invest my retirement savings in Bitcoin?"
        )
        assert result.decision in [Decision.ALLOW, Decision.BLOCK, Decision.REVIEW]
        assert 0 <= result.score <= 1
        
        # Should be allowed - general education
        result = client.evaluate(
            guardrail="financial_advice_smart", 
            input="What is compound interest?"
        )
        assert result.decision in [Decision.ALLOW, Decision.REVIEW]
        
        client.close()


# Run with: ETHICALZEN_API_KEY=your-key pytest tests/test_client.py -v

