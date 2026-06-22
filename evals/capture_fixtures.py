#!/usr/bin/env python3
"""Capture REAL tool-response payloads (full vs field-filtered) for the affected
tools, writing them as fixtures consumed by response_savings.py.

Requires a real token in the environment (read-only is enough):
    export GITHUB_PERSONAL_ACCESS_TOKEN=ghp_xxx   # do NOT commit this
    python3 capture_fixtures.py --owner github --repo github-mcp-server --org github

Each scenario is run twice: once with no `fields` (all fields) and once asking for
a small subset, mirroring how the model would call it. The server does the actual
filtering, so these are genuine payloads.
"""

from __future__ import annotations

import argparse
import os
import sys
from pathlib import Path

from _mcp_client import MCPServer, text_content

FIXTURES = Path(__file__).parent / "fixtures"


def scenarios(owner: str, repo: str, org: str, query_issue: str, query_pr: str):
    """Return list of (name, tool, full_args, filtered_args)."""
    issue_fields = ["number", "title"]
    return [
        (
            "list_issue_types",
            "list_issue_types",
            {"owner": org},
            {"owner": org, "fields": ["name"]},
        ),
        (
            "list_issues",
            "list_issues",
            {"owner": owner, "repo": repo},
            {"owner": owner, "repo": repo, "fields": issue_fields},
        ),
        (
            "search_issues",
            "search_issues",
            {"query": query_issue},
            {"query": query_issue, "fields": issue_fields},
        ),
        (
            "list_pull_requests",
            "list_pull_requests",
            {"owner": owner, "repo": repo},
            {"owner": owner, "repo": repo, "fields": issue_fields},
        ),
        (
            "search_pull_requests",
            "search_pull_requests",
            {"query": query_pr},
            {"query": query_pr, "fields": issue_fields},
        ),
    ]


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--owner", default="github")
    parser.add_argument("--repo", default="github-mcp-server")
    parser.add_argument("--org", default="github", help="Org for list_issue_types")
    parser.add_argument("--issue-query", default="repo:github/github-mcp-server lockdown")
    parser.add_argument("--pr-query", default="repo:github/github-mcp-server is:open")
    parser.add_argument("--server-cmd", default="go run ./cmd/github-mcp-server stdio")
    args = parser.parse_args()

    if not os.environ.get("GITHUB_PERSONAL_ACCESS_TOKEN"):
        print(
            "ERROR: set GITHUB_PERSONAL_ACCESS_TOKEN in the environment "
            "(real token needed for live tool calls).",
            file=sys.stderr,
        )
        return 2

    FIXTURES.mkdir(parents=True, exist_ok=True)
    # Read-only server; tools we use are all reads.
    with MCPServer(server_cmd=args.server_cmd, extra_args=["--toolsets", "all", "--read-only"]) as server:
        for name, tool, full_args, filt_args in scenarios(
            args.owner, args.repo, args.org, args.issue_query, args.pr_query
        ):
            full = text_content(server.call_tool(tool, full_args))
            filtered = text_content(server.call_tool(tool, filt_args))
            (FIXTURES / f"{name}.full.json").write_text(full)
            (FIXTURES / f"{name}.filtered.json").write_text(filtered)
            print(
                f"[fixtures] {name}: full={len(full)}B filtered={len(filtered)}B",
                file=sys.stderr,
            )

    print("[fixtures] done. Now run: python3 response_savings.py", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
