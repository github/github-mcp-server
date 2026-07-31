#!/usr/bin/env python3
"""Minimal MCP stdio client for response capture."""

from __future__ import annotations

import json
import os
import select
import shlex
import subprocess
import sys
from pathlib import Path
from types import TracebackType
from typing import Any

REPO_ROOT = Path(__file__).resolve().parents[2]
PROTOCOL_VERSION = "2025-06-18"


class MCPServer:
    def __init__(
        self,
        server_cmd: str = "go run ./cmd/github-mcp-server stdio",
        extra_args: list[str] | None = None,
        timeout: float = 180.0,
    ) -> None:
        self.cmd = shlex.split(server_cmd) + list(extra_args or [])
        self.timeout = timeout
        self.proc: subprocess.Popen[str] | None = None
        self._id = 0

    def __enter__(self) -> "MCPServer":
        self.start()
        return self

    def __exit__(
        self,
        _exc_type: type[BaseException] | None,
        _exc_value: BaseException | None,
        _traceback: TracebackType | None,
    ) -> None:
        self.close()

    def start(self) -> None:
        if not os.environ.get("GITHUB_PERSONAL_ACCESS_TOKEN"):
            raise RuntimeError("GITHUB_PERSONAL_ACCESS_TOKEN is required for live captures")

        print(f"[mcp] starting: {' '.join(self.cmd)}", file=sys.stderr)
        self.proc = subprocess.Popen(
            self.cmd,
            cwd=REPO_ROOT,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=sys.stderr,
            text=True,
            env=os.environ.copy(),
        )
        self._request(
            "initialize",
            {
                "protocolVersion": PROTOCOL_VERSION,
                "capabilities": {},
                "clientInfo": {"name": "markdown-response-eval", "version": "0"},
            },
        )
        self._notify("notifications/initialized")

    def call_tool(self, name: str, arguments: dict[str, Any]) -> dict[str, Any]:
        return self._request("tools/call", {"name": name, "arguments": arguments})

    def close(self) -> None:
        if self.proc is None:
            return
        if self.proc.stdin is not None:
            try:
                self.proc.stdin.close()
            except BrokenPipeError:
                pass
        self.proc.terminate()
        try:
            self.proc.wait(timeout=10)
        except subprocess.TimeoutExpired:
            self.proc.kill()
            self.proc.wait(timeout=10)
        self.proc = None

    def _send(self, payload: dict[str, Any]) -> None:
        if self.proc is None or self.proc.stdin is None:
            raise RuntimeError("MCP server is not running")
        self.proc.stdin.write(json.dumps(payload) + "\n")
        self.proc.stdin.flush()

    def _notify(self, method: str, params: dict[str, Any] | None = None) -> None:
        self._send({"jsonrpc": "2.0", "method": method, "params": params or {}})

    def _read(self) -> dict[str, Any]:
        if self.proc is None or self.proc.stdout is None:
            raise RuntimeError("MCP server is not running")
        while True:
            ready, _, _ = select.select([self.proc.stdout], [], [], self.timeout)
            if not ready:
                raise TimeoutError("timed out waiting for the MCP server")
            line = self.proc.stdout.readline()
            if line == "":
                raise EOFError("MCP server closed stdout unexpectedly")
            line = line.strip()
            if not line:
                continue
            try:
                return json.loads(line)
            except json.JSONDecodeError:
                continue

    def _request(self, method: str, params: dict[str, Any]) -> dict[str, Any]:
        self._id += 1
        request_id = self._id
        self._send(
            {
                "jsonrpc": "2.0",
                "id": request_id,
                "method": method,
                "params": params,
            }
        )
        while True:
            message = self._read()
            if message.get("id") != request_id:
                continue
            if "error" in message:
                raise RuntimeError(f"{method} error: {message['error']}")
            result = message.get("result")
            if not isinstance(result, dict):
                raise RuntimeError(f"{method} returned a non-object result")
            return result
