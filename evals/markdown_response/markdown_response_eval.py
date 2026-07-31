#!/usr/bin/env python3
"""Compare JSON and lossless Markdown MCP tool responses."""

from __future__ import annotations

import argparse
import copy
import json
import subprocess
import sys
import tempfile
from collections.abc import Callable
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from _mcp_client import MCPServer, REPO_ROOT
from _tokenize import get_tokenizer

EVAL_DIR = Path(__file__).resolve().parent


@dataclass(frozen=True)
class Scenario:
    name: str
    tool: str
    arguments: dict[str, Any]


def scenarios(
    owner: str,
    repo: str,
    pull_number: int,
    per_page: int,
) -> list[Scenario]:
    repository = f"{owner}/{repo}"
    pagination = {"page": 1, "perPage": per_page}
    return [
        Scenario(
            "get_file_contents.directory",
            "get_file_contents",
            {"owner": owner, "repo": repo, "path": ""},
        ),
        Scenario(
            "get_file_contents.directory.filtered",
            "get_file_contents",
            {
                "owner": owner,
                "repo": repo,
                "path": "",
                "fields": ["name", "type"],
            },
        ),
        Scenario(
            "get_file_contents.file",
            "get_file_contents",
            {"owner": owner, "repo": repo, "path": "README.md"},
        ),
        Scenario(
            "pull_request_read.get",
            "pull_request_read",
            {
                "method": "get",
                "owner": owner,
                "repo": repo,
                "pullNumber": pull_number,
            },
        ),
        Scenario(
            "pull_request_read.get_status",
            "pull_request_read",
            {
                "method": "get_status",
                "owner": owner,
                "repo": repo,
                "pullNumber": pull_number,
            },
        ),
        Scenario(
            "pull_request_read.get_files",
            "pull_request_read",
            {
                "method": "get_files",
                "owner": owner,
                "repo": repo,
                "pullNumber": pull_number,
                **pagination,
            },
        ),
        Scenario(
            "pull_request_read.get_commits",
            "pull_request_read",
            {
                "method": "get_commits",
                "owner": owner,
                "repo": repo,
                "pullNumber": pull_number,
                **pagination,
            },
        ),
        Scenario(
            "pull_request_read.get_review_comments",
            "pull_request_read",
            {
                "method": "get_review_comments",
                "owner": owner,
                "repo": repo,
                "pullNumber": pull_number,
                "perPage": per_page,
            },
        ),
        Scenario(
            "pull_request_read.get_reviews",
            "pull_request_read",
            {
                "method": "get_reviews",
                "owner": owner,
                "repo": repo,
                "pullNumber": pull_number,
                **pagination,
            },
        ),
        Scenario(
            "pull_request_read.get_comments",
            "pull_request_read",
            {
                "method": "get_comments",
                "owner": owner,
                "repo": repo,
                "pullNumber": pull_number,
                **pagination,
            },
        ),
        Scenario(
            "pull_request_read.get_check_runs",
            "pull_request_read",
            {
                "method": "get_check_runs",
                "owner": owner,
                "repo": repo,
                "pullNumber": pull_number,
                **pagination,
            },
        ),
        Scenario(
            "pull_request_read.get_diff",
            "pull_request_read",
            {
                "method": "get_diff",
                "owner": owner,
                "repo": repo,
                "pullNumber": pull_number,
            },
        ),
        Scenario(
            "list_pull_requests",
            "list_pull_requests",
            {
                "owner": owner,
                "repo": repo,
                "state": "all",
                **pagination,
            },
        ),
        Scenario(
            "list_pull_requests.filtered",
            "list_pull_requests",
            {
                "owner": owner,
                "repo": repo,
                "state": "all",
                "fields": ["number", "title"],
                **pagination,
            },
        ),
        Scenario(
            "search_pull_requests",
            "search_pull_requests",
            {
                "query": f"repo:{repository} is:pr",
                **pagination,
            },
        ),
        Scenario(
            "search_pull_requests.filtered",
            "search_pull_requests",
            {
                "query": f"repo:{repository} is:pr",
                "fields": ["number", "title"],
                **pagination,
            },
        ),
    ]


def create_pull_request_fixture(owner: str, repo: str) -> dict[str, Any]:
    response = {
        "id": "1234567890",
        "url": f"https://github.com/{owner}/{repo}/pull/9999",
    }
    return {
        "content": [
            {
                "type": "text",
                "text": json.dumps(response, separators=(",", ":")),
            }
        ]
    }


def capture_results(
    server_cmd: str,
    capture_scenarios: list[Scenario],
) -> list[tuple[Scenario, dict[str, Any]]]:
    captured: list[tuple[Scenario, dict[str, Any]]] = []
    with MCPServer(
        server_cmd=server_cmd,
        extra_args=["--toolsets", "repos,pull_requests", "--read-only"],
    ) as server:
        for scenario in capture_scenarios:
            print(f"[capture] {scenario.name}", file=sys.stderr)
            result = server.call_tool(scenario.tool, scenario.arguments)
            if result.get("isError"):
                raise RuntimeError(
                    f"{scenario.name} returned an error: {first_text(result)}"
                )
            captured.append((scenario, result))
    return captured


def build_renderer(output_path: Path) -> None:
    subprocess.run(
        ["go", "build", "-o", str(output_path), "./evals/markdown_response/render"],
        cwd=REPO_ROOT,
        check=True,
    )


def render_markdown(renderer: Path, json_text: str) -> str:
    completed = subprocess.run(
        [str(renderer)],
        cwd=REPO_ROOT,
        input=json_text,
        text=True,
        capture_output=True,
        check=True,
    )
    return completed.stdout


def markdownify_result(
    result: dict[str, Any],
    renderer: Path,
) -> tuple[dict[str, Any], bool]:
    converted = copy.deepcopy(result)
    if converted.get("isError"):
        return converted, False

    content = converted.get("content")
    if not isinstance(content, list) or len(content) != 1:
        return converted, False
    text_content = content[0]
    if not isinstance(text_content, dict) or text_content.get("type") != "text":
        return converted, False
    text = text_content.get("text")
    if not isinstance(text, str):
        return converted, False
    try:
        json.loads(text)
    except json.JSONDecodeError:
        return converted, False

    text_content["text"] = render_markdown(renderer, text)
    converted.pop("structuredContent", None)
    return converted, True


def compact_json(value: Any) -> str:
    return json.dumps(
        value,
        separators=(",", ":"),
        ensure_ascii=False,
        sort_keys=True,
    )


def first_text(result: dict[str, Any]) -> str:
    content = result.get("content")
    if not isinstance(content, list):
        return ""
    for item in content:
        if isinstance(item, dict) and item.get("type") == "text":
            text = item.get("text")
            if isinstance(text, str):
                return text
    return ""


def measure(
    scenario: Scenario,
    baseline: dict[str, Any],
    markdown: dict[str, Any],
    changed: bool,
    count_tokens: Callable[[str], int],
) -> dict[str, Any]:
    baseline_wire = compact_json(baseline)
    markdown_wire = compact_json(markdown)
    baseline_text = first_text(baseline)
    markdown_text = first_text(markdown)

    baseline_tokens = count_tokens(baseline_wire)
    markdown_tokens = count_tokens(markdown_wire)
    baseline_bytes = len(baseline_wire.encode())
    markdown_bytes = len(markdown_wire.encode())
    token_saved = baseline_tokens - markdown_tokens
    byte_saved = baseline_bytes - markdown_bytes

    return {
        "scenario": scenario.name,
        "tool": scenario.tool,
        "converted": changed,
        "response": {
            "json_bytes": baseline_bytes,
            "markdown_bytes": markdown_bytes,
            "bytes_saved": byte_saved,
            "bytes_saved_percent": percent(byte_saved, baseline_bytes),
            "json_tokens": baseline_tokens,
            "markdown_tokens": markdown_tokens,
            "tokens_saved": token_saved,
            "tokens_saved_percent": percent(token_saved, baseline_tokens),
        },
        "text": {
            "json_bytes": len(baseline_text.encode()),
            "markdown_bytes": len(markdown_text.encode()),
            "json_tokens": count_tokens(baseline_text),
            "markdown_tokens": count_tokens(markdown_text),
        },
    }


def percent(saved: int, baseline: int) -> float:
    if baseline == 0:
        return 0.0
    return round(100 * saved / baseline, 1)


def aggregate(rows: list[dict[str, Any]]) -> dict[str, Any]:
    baseline_bytes = sum(row["response"]["json_bytes"] for row in rows)
    markdown_bytes = sum(row["response"]["markdown_bytes"] for row in rows)
    baseline_tokens = sum(row["response"]["json_tokens"] for row in rows)
    markdown_tokens = sum(row["response"]["markdown_tokens"] for row in rows)
    return {
        "scenarios": len(rows),
        "json_bytes": baseline_bytes,
        "markdown_bytes": markdown_bytes,
        "bytes_saved": baseline_bytes - markdown_bytes,
        "bytes_saved_percent": percent(
            baseline_bytes - markdown_bytes,
            baseline_bytes,
        ),
        "json_tokens": baseline_tokens,
        "markdown_tokens": markdown_tokens,
        "tokens_saved": baseline_tokens - markdown_tokens,
        "tokens_saved_percent": percent(
            baseline_tokens - markdown_tokens,
            baseline_tokens,
        ),
    }


def aggregate_by_tool(rows: list[dict[str, Any]]) -> dict[str, dict[str, Any]]:
    tools = sorted({row["tool"] for row in rows})
    return {
        tool: aggregate([row for row in rows if row["tool"] == tool])
        for tool in tools
    }


def print_results(
    rows: list[dict[str, Any]],
    totals: dict[str, Any],
    tokenizer: str,
) -> None:
    print(f"tokenizer: {tokenizer}\n")
    print(
        f"{'scenario':<40}"
        f"{'JSON tok':>10}"
        f"{'MD tok':>10}"
        f"{'saved':>10}"
        f"{'saved%':>9}"
        f"{'JSON B':>11}"
        f"{'MD B':>11}"
        f"{'saved%':>9}"
    )
    for row in rows:
        response = row["response"]
        print(
            f"{row['scenario']:<40}"
            f"{response['json_tokens']:>10}"
            f"{response['markdown_tokens']:>10}"
            f"{response['tokens_saved']:>10}"
            f"{response['tokens_saved_percent']:>8.1f}%"
            f"{response['json_bytes']:>11}"
            f"{response['markdown_bytes']:>11}"
            f"{response['bytes_saved_percent']:>8.1f}%"
        )

    print("\nBY TOOL")
    for tool, values in totals["by_tool"].items():
        print(
            f"  {tool:<24}"
            f"{values['json_tokens']:>8} -> {values['markdown_tokens']:<8}"
            f"{values['tokens_saved_percent']:>6.1f}% tokens, "
            f"{values['bytes_saved_percent']:>6.1f}% bytes"
        )

    for label, key in (
        ("converted responses", "converted"),
        ("all scenarios", "all"),
    ):
        values = totals[key]
        print(
            f"  {label:<24}"
            f"{values['json_tokens']:>8} -> {values['markdown_tokens']:<8}"
            f"{values['tokens_saved_percent']:>6.1f}% tokens, "
            f"{values['bytes_saved_percent']:>6.1f}% bytes"
        )


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--owner", default="github")
    parser.add_argument("--repo", default="github-mcp-server")
    parser.add_argument("--pull-number", type=int, default=2658)
    parser.add_argument("--per-page", type=int, default=30)
    parser.add_argument(
        "--server-cmd",
        default="go run ./cmd/github-mcp-server stdio",
    )
    parser.add_argument("--approx", action="store_true")
    parser.add_argument(
        "--out",
        type=Path,
        default=EVAL_DIR / "out" / "markdown-response-eval.json",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    count_tokens, tokenizer = get_tokenizer(args.approx)
    capture_scenarios = scenarios(
        args.owner,
        args.repo,
        args.pull_number,
        args.per_page,
    )

    with tempfile.TemporaryDirectory(prefix="markdown-response-eval-") as temp_dir:
        renderer = Path(temp_dir) / "render-markdown"
        build_renderer(renderer)
        captured = capture_results(args.server_cmd, capture_scenarios)
        captured.append(
            (
                Scenario("create_pull_request", "create_pull_request", {}),
                create_pull_request_fixture(args.owner, args.repo),
            )
        )

        rows: list[dict[str, Any]] = []
        for scenario, baseline in captured:
            markdown, changed = markdownify_result(baseline, renderer)
            rows.append(
                measure(
                    scenario,
                    baseline,
                    markdown,
                    changed,
                    count_tokens,
                )
            )

    converted_rows = [row for row in rows if row["converted"]]
    totals = {
        "all": aggregate(rows),
        "converted": aggregate(converted_rows),
        "by_tool": aggregate_by_tool(rows),
    }
    output = {
        "captured_at": datetime.now(timezone.utc).isoformat(),
        "repository": f"{args.owner}/{args.repo}",
        "pull_number": args.pull_number,
        "per_page": args.per_page,
        "tokenizer": tokenizer,
        "measurement": "compact JSON serialization of the MCP tools/call result",
        "scenarios": rows,
        "aggregates": totals,
    }

    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(output, indent=2) + "\n")
    print_results(rows, totals, tokenizer)
    print(f"\nmetrics: {args.out}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
