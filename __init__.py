"""Hermes Agent adapter for procoder.

Thin by design: injects the canonical AGENTS.md contract before each LLM
call. All content lives in AGENTS.md, which the portability checks pin.
"""

import os

_AGENTS = os.path.join(os.path.dirname(os.path.abspath(__file__)), "AGENTS.md")


def _contract():
    try:
        with open(_AGENTS, encoding="utf-8") as f:
            return f.read()
    except OSError:
        return ""


def register(ctx):
    """Hermes entry point: hook the contract into every LLM call."""
    body = _contract()
    if not body:
        return

    def pre_llm_call(*_args, **_kwargs):
        return {"context": body}

    ctx.register_hook("pre_llm_call", pre_llm_call)
