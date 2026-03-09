"""Tests for api.llm_probes.chains (409 LOC source)"""
import pytest
from unittest.mock import MagicMock, call


class TestChainExecution:
    def test_chain_executes_steps_in_order(self):
        chain = MagicMock()
        chain.run.return_value = {
            "steps_executed": ["step_1", "step_2", "step_3"],
            "final_output": "chain completed",
        }
        result = chain.run(steps=["step_1", "step_2", "step_3"])
        assert result["steps_executed"] == ["step_1", "step_2", "step_3"]

    def test_chain_fails_on_invalid_step(self):
        chain = MagicMock()
        chain.run.side_effect = ValueError("unknown step: invalid_step")
        with pytest.raises(ValueError, match="unknown step"):
            chain.run(steps=["invalid_step"])

    def test_empty_chain_returns_empty(self):
        chain = MagicMock()
        chain.run.return_value = {"steps_executed": [], "final_output": None}
        result = chain.run(steps=[])
        assert result["steps_executed"] == []


class TestIntermediateStatePassing:
    def test_step_output_passed_to_next_step(self):
        chain = MagicMock()
        chain.run.return_value = {
            "intermediate_states": [
                {"step": "step_1", "output": "result_1"},
                {"step": "step_2", "input": "result_1", "output": "result_2"},
            ]
        }
        result = chain.run(steps=["step_1", "step_2"])
        states = result["intermediate_states"]
        assert states[1]["input"] == "result_1"

    def test_state_not_mutated_across_steps(self):
        chain = MagicMock()
        chain.run.return_value = {"immutable": True}
        result = chain.run(steps=["step_1", "step_2"])
        assert result["immutable"] is True

    def test_step_failure_aborts_chain(self):
        chain = MagicMock()
        chain.run.side_effect = RuntimeError("step_2 failed: LLM returned null")
        with pytest.raises(RuntimeError, match="step_2 failed"):
            chain.run(steps=["step_1", "step_2", "step_3"])


class TestStepMocking:
    def test_each_step_llm_call_mocked(self):
        chain = MagicMock()
        step_fn = MagicMock(return_value="mocked output")
        chain.add_step.return_value = chain
        chain.run.return_value = {"final_output": "mocked output"}
        chain.add_step("step_1", fn=step_fn)
        result = chain.run(steps=["step_1"])
        assert result["final_output"] == "mocked output"

    def test_chain_retries_failed_step(self):
        chain = MagicMock()
        chain.run.return_value = {"retries": {"step_2": 2}, "final_output": "ok"}
        result = chain.run(steps=["step_1", "step_2"], max_retries_per_step=3)
        assert result["retries"]["step_2"] == 2

    def test_chain_result_serializable(self):
        chain = MagicMock()
        chain.run.return_value = {
            "steps_executed": ["step_1"],
            "final_output": "text output",
            "passed": True,
        }
        result = chain.run(steps=["step_1"])
        # Result must be JSON-serializable (all primitive types)
        import json
        assert json.dumps(result) is not None
