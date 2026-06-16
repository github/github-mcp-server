#!/usr/bin/env python3
"""Boot the GitHub MCP server over stdio, do the MCP handshake, and dump the
`tools/list` result to a JSON file.

This captures exactly what an MCP client receives at initialization, so it is the
ground truth for the "fixed tax" measurement (see fixed_tax.py).

Example (WITH output schemas + the new `fields` params):
    python3 capture_tools.py --features output_schemas --toolsets all \
        --out out/tools.treatment.json
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

from _mcp_client import MCPServer


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True, help="Output JSON file path")
    parser.add_argument("--features", default="", help="Comma-separated feature flags")
    parser.add_argument("--toolsets", default="all", help="Comma-separated toolsets / 'all'")
    parser.add_argument("--read-only", action="store_true")
    parser.add_argument("--server-cmd", default="go run ./cmd/github-mcp-server stdio")
    parser.add_argument("--timeout", type=float, default=180.0)
    args = parser.parse_args()

    extra = ["--toolsets", args.toolsets]
    if args.features:
        extra += ["--features", args.features]
    if args.read_only:
        extra += ["--read-only"]

    with MCPServer(server_cmd=args.server_cmd, extra_args=extra, timeout=args.timeout) as server:
        tools = server.list_tools()

    out_path = Path(args.out)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(
        json.dumps(
            {
                "config": {
                    "features": args.features,
                    "toolsets": args.toolsets,
                    "read_only": args.read_only,
                },
                "tool_count": len(tools),
                "tools": tools,
            },
            indent=2,
        )
    )
    have_schema = sum(1 for t in tools if "outputSchema" in t)
    have_fields = sum(
        1 for t in tools if "fields" in (t.get("inputSchema", {}).get("properties", {}) or {})
    )
    print(
        f"[capture] wrote {len(tools)} tools -> {out_path} "
        f"({have_schema} with outputSchema, {have_fields} with a `fields` param)",
        file=sys.stderr,
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
