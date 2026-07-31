#!/usr/bin/env python3
"""Token counting for the Markdown response eval."""

from __future__ import annotations

from collections.abc import Callable


def get_tokenizer(approx: bool = False) -> tuple[Callable[[str], int], str]:
    if approx:
        return (lambda text: max(1, len(text) // 4), "approx(chars/4)")

    try:
        import tiktoken
    except ImportError as exc:
        raise RuntimeError(
            "tiktoken is required; install requirements.txt or pass --approx"
        ) from exc

    encoder = tiktoken.get_encoding("o200k_base")
    return (lambda text: len(encoder.encode(text)), "tiktoken(o200k_base)")
