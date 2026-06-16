#!/usr/bin/env python3
"""Minimal MCP stdio client used by the eval scripts.

Spawns the GitHub MCP server, performs the MCP handshake, and exposes
`list_tools()` and `call_tool()`. stdout is newline-delimited JSON-RPC; server
logs go to stderr.

Security: never hardcode a token. The token is read from the process environment
(GITHUB_PERSONAL_ACCESS_TOKEN). A dummy non-`ghp_` token is used only as a
fallback so `tools/list` works offline without hitting GitHub.
"""

from __future__ import annotations

import json
import os
import select
import shlex
import subprocess
import sys
from pathlib import Path
from typing import Any

REPO_ROOT = Path(__file__).resolve().parent.parent
PROTOCOL_VERSION = "2025-06-18"


def text_content(result: dict) -> str:
    """Concatenate the text parts of a tools/call result."""
    parts = [c.get("text", "") for c in result.get("content", []) if c.get("type") == "text"]
    return "".join(parts)


class MCPServer:
    def __init__(
        self,
        server_cmd: str = "go run ./cmd/github-mcp-server stdio",
        extra_args: list[str] | None = None,
        env: dict[str, str] | None = None,
        cwd: Path = REPO_ROOT,
        timeout: float = 180.0,
    ) -> None:
        self.cmd = shlex.split(server_cmd) + list(extra_args or [])
        self.env = {**os.environ, **(env or {})}
        self.env.setdefault("GITHUB_PERSONAL_ACCESS_TOKEN", "dummy_token_no_network")
        self.cwd = str(cwd)
        self.timeout = timeout
        self.proc: subprocess.Popen | None = None
        self._id = 0

    def __enter__(self) -> "MCPServer":
        self.start()
        return self

    def __exit__(self, *_: Any) -> None:
        self.close()

    def start(self) -> None:
        print(f"[mcp] starting: {' '.join(self.cmd)}", file=sys.stderr)
        self.proc = subprocess.Popen(
            self.cmd,
            cwd=self.cwd,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=sys.stderr,
            text=True,
            env=self.env,
        )
        self._request(
            "initialize",
            {
                "protocolVersion": PROTOCOL_VERSION,
                "capabilities": {},
                "clientInfo": {"name": "evals", "version": "0"},
            },
        )
        self._notify("notifications/initialized")

    # -- JSON-RPC plumbing -------------------------------------------------
    def _send(self, payload: dict) -> None:
        assert self.proc and self.proc.stdin
        self.proc.stdin.write(json.dumps(payload) + "\n")
        self.proc.stdin.flush()

    def _notify(self, method: str, params: dict | None = None) -> None:
        self._send({"jsonrpc": "2.0", "method": method, "params": params or {}})

    def _read(self) -> dict:
        assert self.proc and self.proc.stdout
        while True:
            ready, _, _ = select.select([self.proc.stdout], [], [], self.timeout)
            if not ready:
                raise TimeoutError("timed out waiting for server (see stderr above)")
            line = self.proc.stdout.readline()
            if line == "":
                raise EOFError("server closed stdout unexpectedly")
            line = line.strip()
            if not line:
                continue
            try:
                return json.loads(line)
            except json.JSONDecodeError:
                continue  # ignore stray non-JSON output

    def _request(self, method: str, params: dict) -> dict:
        self._id += 1
        req_id = self._id
        self._send({"jsonrpc": "2.0", "id": req_id, "method": method, "params": params})
        while True:
            msg = self._read()
            if msg.get("id") == req_id:
                if "error" in msg:
                    raise RuntimeError(f"{method} error: {msg['error']}")
                return msg["result"]

    # -- High-level API ----------------------------------------------------
    def list_tools(self) -> list[dict]:
        tools: list[dict] = []
        cursor = None
        while True:
            params = {} if cursor is None else {"cursor": cursor}
            result = self._request("tools/list", params)
            tools.extend(result.get("tools", []))
            cursor = result.get("nextCursor")
            if not cursor:
                return tools

    def call_tool(self, name: str, arguments: dict) -> dict:
        return self._request("tools/call", {"name": name, "arguments": arguments})

    def close(self) -> None:
        if not self.proc:
            return
        try:
            if self.proc.stdin:
                self.proc.stdin.close()
        except Exception:  # noqa: BLE001
            pass
        self.proc.terminate()
        try:
            self.proc.wait(timeout=10)
        except Exception:  # noqa: BLE001
            self.proc.kill()
        self.proc = None
