#!/usr/bin/env python3
"""Shared helpers for the eval scripts: tokenization that works offline.

Prefers tiktoken (o200k_base, a good proxy for modern GPT/Claude tokenizers).
Falls back to a chars/4 heuristic if tiktoken is unavailable or --approx is set.
The break-even analysis only needs *relative* deltas, so the proxy is fine as
long as the same tokenizer is used for both arms.
"""

from __future__ import annotations

import json
from typing import Any

_ENCODER = None
_MODE = None


def get_tokenizer(approx: bool = False):
    """Return (count_fn, mode_label). count_fn(text:str) -> int."""
    global _ENCODER, _MODE
    if approx:
        return (lambda s: max(1, len(s) // 4), "approx(chars/4)")
    if _ENCODER is None:
        try:
            import tiktoken  # type: ignore

            _ENCODER = tiktoken.get_encoding("o200k_base")
            _MODE = "tiktoken(o200k_base)"
        except Exception:  # noqa: BLE001 - any failure -> fall back
            _ENCODER = False
            _MODE = "approx(chars/4)"
    if _ENCODER is False:
        return (lambda s: max(1, len(s) // 4), _MODE)
    return (lambda s: len(_ENCODER.encode(s)), _MODE)


def to_wire(obj: Any) -> str:
    """Serialize like a compact JSON-RPC payload on the wire.

    Whitespace/key-order are normalized so both arms are compared identically.
    """
    return json.dumps(obj, separators=(",", ":"), ensure_ascii=False, sort_keys=True)


def count_obj_tokens(obj: Any, approx: bool = False) -> int:
    count, _ = get_tokenizer(approx)
    return count(to_wire(obj))
