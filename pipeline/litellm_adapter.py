"""litellm adapter — thin wrapper that enforces max_tokens on every completion call."""
import litellm

DEFAULT_MAX_TOKENS = 4096


def completion(model: str, messages: list[dict], **kwargs) -> litellm.ModelResponse:
    """Call litellm.completion with a guaranteed max_tokens bound."""
    response = litellm.completion(
        model=model,
        messages=messages,
        max_tokens=kwargs.pop("max_tokens", DEFAULT_MAX_TOKENS),
        **kwargs,
    )
    return response


def acompletion(model: str, messages: list[dict], **kwargs):
    """Async variant — enforces max_tokens bound."""
    return litellm.acompletion(
        model=model,
        messages=messages,
        max_tokens=kwargs.pop("max_tokens", DEFAULT_MAX_TOKENS),
        **kwargs,
    )
