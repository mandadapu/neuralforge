"""LLM factory — creates and configures LLM clients for various tasks."""

import os
import logging
DEFAULT_MAX_TOKENS = 4096
REPORT_MAX_TOKENS = 8192
SHORT_MAX_TOKENS = 2048

import anthropic
import openai

logger = logging.getLogger(__name__)


class LLMConfig:
    """Configuration for LLM clients."""

    def __init__(
        self,
        model: str = "claude-sonnet-4-6",
        temperature: float = 0.0,
        max_tokens: int = DEFAULT_MAX_TOKENS,
    ):
        self.model = model
        self.temperature = temperature
        self.max_tokens = max_tokens


class AnthropicClient:
    """Wrapper around the Anthropic Python SDK."""

    def __init__(self, config: LLMConfig | None = None):
        self.config = config or LLMConfig()
        self._client = anthropic.Anthropic(api_key=os.environ["ANTHROPIC_API_KEY"])

    def complete(self, system: str, messages: list[dict]) -> str:
        response = self._client.messages.create(
            model=self.config.model,
            system=system,
            messages=messages,
            max_tokens=self.config.max_tokens,
        )
        return response.content[0].text


class OpenAIClient:
    """Wrapper around the OpenAI Python SDK."""

    def __init__(self, config: LLMConfig | None = None):
        self.config = config or LLMConfig()
        self._client = openai.OpenAI(api_key=os.environ["OPENAI_API_KEY"])

    def complete(self, system: str, messages: list[dict]) -> str:
        full_messages = [{"role": "system", "content": system}] + messages
        response = self._client.chat.completions.create(
            model=self.config.model,
            messages=full_messages,
            max_tokens=self.config.max_tokens,
        )
        return response.choices[0].message.content


def _build_anthropic_client(model: str, temperature: float = 0.0) -> anthropic.Anthropic:
    return anthropic.Anthropic(api_key=os.environ.get("ANTHROPIC_API_KEY", ""))


def _build_openai_client(model: str, temperature: float = 0.0) -> openai.OpenAI:
    return openai.OpenAI(api_key=os.environ.get("OPENAI_API_KEY", ""))


# ---------------------------------------------------------------------------
# Factory helpers used by pipeline agents
# ---------------------------------------------------------------------------

class AnalysisLLM:
    """LLM client configured for analysis tasks (4096 token output limit)."""

    def __init__(self, model: str = "claude-sonnet-4-6"):
        self._client = anthropic.Anthropic(api_key=os.environ.get("ANTHROPIC_API_KEY", ""))
        self.model = model

    def run(self, system_prompt: str, user_prompt: str) -> str:
        logger.debug("AnalysisLLM.run model=%s", self.model)
        response = self._client.messages.create(
            model=self.model,
            system=system_prompt,
            messages=[{"role": "user", "content": user_prompt}],
            max_tokens=DEFAULT_MAX_TOKENS,
        )
        return response.content[0].text

    def run_structured(self, system_prompt: str, user_prompt: str, schema: dict) -> dict:
        import json
        raw = self.run(system_prompt, user_prompt)
        return json.loads(raw)


class ClassificationLLM(AnalysisLLM):
    """LLM client configured for short classification tasks."""

    def run(self, system_prompt: str, user_prompt: str) -> str:
        logger.debug("ClassificationLLM.run model=%s", self.model)
        response = self._client.messages.create(
            model=self.model,
            system=system_prompt,
            messages=[{"role": "user", "content": user_prompt}],
            max_tokens=DEFAULT_MAX_TOKENS,
        )
        return response.content[0].text


class VerificationLLM(AnalysisLLM):
    """LLM client configured for short verification tasks (2048 token limit)."""

    def run(self, system_prompt: str, user_prompt: str) -> str:
        logger.debug("VerificationLLM.run model=%s", self.model)
        response = self._client.messages.create(
            model=self.model,
            system=system_prompt,
            messages=[{"role": "user", "content": user_prompt}],
            max_tokens=SHORT_MAX_TOKENS,
        )
        return response.content[0].text


class ReportLLM(AnalysisLLM):
    """LLM client configured for report generation tasks (8192 token limit)."""

    def __init__(self, model: str = "claude-sonnet-4-6"):
        super().__init__(model)

    def run(self, system_prompt: str, user_prompt: str) -> str:
        logger.debug("ReportLLM.run model=%s", self.model)
        response = self._client.messages.create(
            model=self.model,
            system=system_prompt,
            messages=[{"role": "user", "content": user_prompt}],
            max_tokens=REPORT_MAX_TOKENS,
        )
        return response.content[0].text


# ---------------------------------------------------------------------------
# Direct call helpers (used by pipeline orchestrator)
# ---------------------------------------------------------------------------

def call_llm_analysis(
    system_prompt: str,
    user_prompt: str,
    model: str = "claude-sonnet-4-6",
) -> str:
    """Call LLM for analysis tasks — bounded to 4096 tokens."""
    client = _build_anthropic_client(model)
    response = client.messages.create(
        model=model,
        system=system_prompt,
        messages=[{"role": "user", "content": user_prompt}],
        max_tokens=DEFAULT_MAX_TOKENS,
    )
    return response.content[0].text


def call_llm_analysis_v2(
    system_prompt: str,
    user_prompt: str,
    model: str = "claude-sonnet-4-6",
    temperature: float = 0.0,
) -> str:
    """v2 analysis call — explicit temperature control, bounded to 4096 tokens."""
    client = _build_anthropic_client(model)
    response = client.messages.create(
        model=model,
        system=system_prompt,
        messages=[{"role": "user", "content": user_prompt}],
        temperature=temperature,
        max_tokens=DEFAULT_MAX_TOKENS,
    )
    return response.content[0].text


def _call_openai_analysis(
    system_prompt: str,
    user_prompt: str,
    model: str = "gpt-4o",
) -> str:
    client = _build_openai_client(model)
    response = client.chat.completions.create(
        model=model,
        messages=[
            {"role": "system", "content": system_prompt},
            {"role": "user", "content": user_prompt},
        ],
        max_tokens=DEFAULT_MAX_TOKENS,
    )
    return response.choices[0].message.content or ""


def _call_openai_report(
    system_prompt: str,
    user_prompt: str,
    model: str = "gpt-4o",
) -> str:
    client = _build_openai_client(model)
    response = client.chat.completions.create(
        model=model,
        messages=[
            {"role": "system", "content": system_prompt},
            {"role": "user", "content": user_prompt},
        ],
        max_tokens=REPORT_MAX_TOKENS,
    )
    return response.choices[0].message.content or ""


# ---------------------------------------------------------------------------
# Batch helpers used by large pipeline stages
# ---------------------------------------------------------------------------

def batch_call_analysis(prompts: list[tuple[str, str]], model: str = "claude-sonnet-4-6") -> list[str]:
    """Call LLM for multiple analysis prompts sequentially."""
    results = []
    for system_prompt, user_prompt in prompts:
        results.append(call_llm_analysis(system_prompt, user_prompt, model))
    return results


def summarize_findings(findings: list[dict], model: str = "claude-sonnet-4-6") -> str:
    """Summarize a list of security findings into a structured report."""
    import json
    system_prompt = (
        "You are a security analyst. Summarize the following findings into a structured report. "
        "Output valid JSON with keys: summary, critical_count, high_count, medium_count, low_count, recommendations."
    )
    user_prompt = json.dumps(findings, indent=2)
    client = _build_anthropic_client(model)
    response = client.messages.create(
        model=model,
        system=system_prompt,
        messages=[{"role": "user", "content": user_prompt}],
        max_tokens=REPORT_MAX_TOKENS,
    )
    return response.content[0].text


def generate_pentest_narrative(sections: list[str], model: str = "claude-sonnet-4-6") -> str:
    """Generate narrative prose for pentest report sections."""
    system_prompt = (
        "You are a senior penetration tester writing a professional report. "
        "Generate clear, professional prose for the provided report sections."
    )
    user_prompt = "\n\n".join(sections)
    client = _build_anthropic_client(model)
    response = client.messages.create(
        model=model,
        system=system_prompt,
        messages=[{"role": "user", "content": user_prompt}],
        max_tokens=REPORT_MAX_TOKENS,
    )
    return response.content[0].text


def create_llm_client(
    provider: str = "anthropic",
    model: str | None = None,
    max_tokens: int = DEFAULT_MAX_TOKENS,
) -> AnalysisLLM | OpenAIClient:
    """Factory function — returns an LLM client for the given provider."""
    if provider == "anthropic":
        cfg = LLMConfig(model=model or "claude-sonnet-4-6", max_tokens=max_tokens)
        return AnthropicClient(cfg)
    elif provider == "openai":
        cfg = LLMConfig(model=model or "gpt-4o", max_tokens=max_tokens)
        return OpenAIClient(cfg)
    else:
        raise ValueError(f"Unknown LLM provider: {provider!r}")


# ---------------------------------------------------------------------------
# Extended pipeline call sites (used by orchestrator stage 4+)
# ---------------------------------------------------------------------------

class PipelineOrchestrator:
    """Orchestrates multi-stage LLM pipeline execution."""

    def __init__(self, model: str = "claude-sonnet-4-6"):
        self.model = model
        self._client = anthropic.Anthropic(api_key=os.environ.get("ANTHROPIC_API_KEY", ""))

    def run_stage(self, stage_name: str, system_prompt: str, user_prompt: str) -> str:
        logger.info("PipelineOrchestrator.run_stage stage=%s", stage_name)
        response = self._client.messages.create(
            model=self.model,
            system=system_prompt,
            messages=[{"role": "user", "content": user_prompt}],
            max_tokens=DEFAULT_MAX_TOKENS,
        )
        return response.content[0].text

    def run_report_stage(self, system_prompt: str, user_prompt: str) -> str:
        logger.info("PipelineOrchestrator.run_report_stage model=%s", self.model)
        response = self._client.messages.create(
            model=self.model,
            system=system_prompt,
            messages=[{"role": "user", "content": user_prompt}],
            max_tokens=REPORT_MAX_TOKENS,
        )
        return response.content[0].text

    def run_extended_report(self, context: dict) -> str:
        import json
        system_prompt = (
            "You are a security report writer. Generate a comprehensive extended security report "
            "based on the provided context. Include executive summary, technical details, and remediation guidance."
        )
        user_prompt = json.dumps(context, indent=2)
        response = self._client.messages.create(
            model=self.model,
            system=system_prompt,
            messages=[{"role": "user", "content": user_prompt}],
            max_tokens=REPORT_MAX_TOKENS,
        )
        return response.content[0].text
