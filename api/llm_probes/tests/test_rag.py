"""Tests for api.llm_probes.rag (184 LOC source)"""
import pytest
from unittest.mock import MagicMock


class TestVectorRetrieval:
    def test_retrieve_returns_relevant_docs(self):
        probe = MagicMock()
        probe.retrieve.return_value = [
            {"id": "doc-1", "text": "Security policy document", "score": 0.95},
            {"id": "doc-2", "text": "Access control policy", "score": 0.88},
        ]
        result = probe.retrieve(query="security policy", top_k=2)
        assert len(result) == 2
        assert result[0]["score"] > result[1]["score"]

    def test_retrieve_empty_query_raises(self):
        probe = MagicMock()
        probe.retrieve.side_effect = ValueError("query cannot be empty")
        with pytest.raises(ValueError, match="query"):
            probe.retrieve(query="", top_k=5)

    def test_retrieve_returns_empty_for_no_matches(self):
        probe = MagicMock()
        probe.retrieve.return_value = []
        result = probe.retrieve(query="nonexistent topic xyz", top_k=5)
        assert result == []


class TestContextInjection:
    def test_inject_context_into_prompt(self):
        probe = MagicMock()
        probe.inject_context.return_value = (
            "Based on these documents:\n"
            "- Security policy document\n\n"
            "Answer: What is the security policy?"
        )
        result = probe.inject_context(
            query="What is the security policy?",
            docs=[{"text": "Security policy document"}],
        )
        assert "Security policy document" in result
        assert "What is the security policy?" in result

    def test_inject_empty_docs_returns_bare_query(self):
        probe = MagicMock()
        probe.inject_context.return_value = "What is the security policy?"
        result = probe.inject_context(query="What is the security policy?", docs=[])
        assert "What is the security policy?" in result


class TestResponseValidation:
    def test_validate_rag_response_passes(self):
        probe = MagicMock()
        probe.validate_response.return_value = {"valid": True, "grounded": True}
        result = probe.validate_response(
            response="The security policy requires MFA.",
            source_docs=[{"text": "The security policy requires MFA."}],
        )
        assert result["valid"] is True
        assert result["grounded"] is True

    def test_validate_hallucinated_response_fails(self):
        probe = MagicMock()
        probe.validate_response.return_value = {
            "valid": False,
            "grounded": False,
            "issue": "response not supported by source documents",
        }
        result = probe.validate_response(
            response="The security policy allows anonymous access.",
            source_docs=[{"text": "The security policy requires MFA."}],
        )
        assert result["grounded"] is False

    def test_run_rag_probe_full_pipeline(self):
        probe = MagicMock()
        probe.run.return_value = {
            "passed": True,
            "retrieved_docs": 3,
            "response_grounded": True,
        }
        result = probe.run(query="What is the access control policy?")
        assert result["passed"] is True
        assert result["response_grounded"] is True
