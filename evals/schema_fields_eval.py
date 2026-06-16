#!/usr/bin/env python3
"""Phase 2 online eval: measure real context (prompt token) usage over multi-tool
agent sessions across THREE shippable tool-definition configurations (an A/B/C
test of the schema/fields knobs).

Uses GitHub Models (OpenAI-compatible) so it authenticates with your GitHub
token -- no third-party API key, no separate billing. Point `--base-url` at any
other OpenAI-compatible endpoint if you have an internal one.

IMPORTANT -- target repo: the tasks hit a live repo, so point them at a large,
public, SAML-free repo (default: cli/cli) that a plain PAT can read. If you aim
at a SAML-protected org repo, every call 403s, the model only ever sees tiny
error payloads, and the `fields` arms look like pure overhead because there is
nothing to filter. Override with --repo / --org.

All three arms run the SAME tasks against the SAME live MCP server. The only
difference is the tool definitions presented to the model:

  * S1 baseline      -- no output schema, no `fields` param. The model cannot
                        filter and always receives full responses. (today's behavior)
  * S2 schema+fields -- output schema folded into the description AND the `fields`
                        param exposed. The model can filter, and pays the full
                        documented fixed tax in context.
  * S3 fields-only   -- the `fields` param exposed but NO output schema. The
                        hypothesized sweet spot: the model still knows it can
                        filter (and which fields exist, via the param's enum) but
                        doesn't carry the heavy schema text.

The server attaches the `fields` param unconditionally and performs the filtering
regardless of any feature flag, so every arm gets correct server behavior; we only
vary what each arm shows the MODEL. The server is booted WITH output schemas
enabled so the S2 arm actually has a schema to embed (and pays its real tax).

Headline metric: cumulative `usage.prompt_tokens` across every turn of a session
(= total context the model had to read). We also track completion tokens, tool
calls, and how often the model actually used `fields`. Use `--repeat` to average
out model nondeterminism, and read the per-task-type breakdown to see WHERE each
configuration helps or hurts.

Requirements:
    pip install -r requirements.txt          # openai
    export GITHUB_PERSONAL_ACCESS_TOKEN=ghp_...   # real token; do NOT commit

Run (GitHub Models gpt-5; the free tier caps requests at 16k -- fine for a smoke
test, but big unfiltered responses will error):
    python3 schema_fields_eval.py --model openai/gpt-5 --toolsets issues,pull_requests --repeat 3

For a large-context model with no request cap, point --base-url at any
OpenAI-compatible endpoint you already have access to. The Copilot API
(https://api.githubcopilot.com) works out of the box for Copilot users -- the
short-lived Copilot token is minted automatically from your GitHub token, no
out-of-pocket third-party key:
    # discover the exact model id first:
    python3 schema_fields_eval.py --base-url https://api.githubcopilot.com --list-models
    # then run:
    python3 schema_fields_eval.py --base-url https://api.githubcopilot.com \
        --model claude-opus-4 --toolsets issues,pull_requests --repeat 3
"""

from __future__ import annotations

import argparse
import copy
import json
import os
import sys
from pathlib import Path

from _mcp_client import MCPServer, text_content

DEFAULT_BASE_URL = "https://models.github.ai/inference"
DEFAULT_MODEL = "openai/gpt-5"

# Default target: a large, public, SAML-free repo so a plain PAT gets real,
# sizeable responses to filter. Pick a repo busy enough that unfiltered payloads
# are genuinely big -- that's what reveals the filtering payoff. Override with
# --repo / --org. (Do NOT point at a SAML-protected org repo like
# github/github-mcp-server: a PAT 403s, every task returns a tiny error, and the
# `fields` arms then look like pure overhead because there's nothing to trim.)
DEFAULT_REPO = "cli/cli"

# Balanced, intentionally NEUTRAL task set, templated on {repo}/{org} so the same
# tasks can target any repo. We deliberately do NOT instruct the model to "return
# only X" -- that would bias it toward using `fields` and inflate the treatment
# arms. Instead we mix tasks whose faithful answer needs just a field or two with
# tasks that need full objects, and let the model's own filtering decisions be
# what we measure. Each task is tagged with the kind of answer it implies (used
# only for the per-type breakdown; never sent to the model):
#   narrow  -> a faithful answer needs just a field or two (filtering should help)
#   full    -> a faithful answer needs rich fields/bodies (filtering shouldn't help)
#   neutral -> genuinely ambiguous; the model decides
TASK_TEMPLATES: list[tuple[str, str]] = [
    ("narrow", "How many open issues are there in {repo}, and what are their titles?"),
    ("narrow", "What are the numbers of the currently open pull requests in {repo}?"),
    ("narrow", "What issue types are configured for the {org} organization?"),
    ("narrow", "What are the names of the branches in {repo}?"),
    ("narrow", "List the tag names in {repo}."),
    ("narrow", "What are the tag names of the most recent releases in {repo}?"),
    ("full", "Summarize what the most recently updated open issue in {repo} is about."),
    ("full", "Look at the open pull requests in {repo} and tell me which one looks most substantial, and why."),
    ("full", "Find issues mentioning 'lockdown' in {repo} and explain what they're asking for."),
    ("full", "Summarize what changed in the most recent release of {repo}."),
    ("full", "Summarize the last few commits on the default branch of {repo} and what they changed."),
    ("neutral", "Give me an overview of recent activity in {repo}'s open issues."),
    ("neutral", "Search for open pull requests about tests in {repo} and tell me what they do."),
    ("neutral", "Give me an overview of recent commit activity in {repo}."),
]


def build_tasks(repo: str, org: str) -> list[tuple[str, str]]:
    """Render the templated task set for a concrete repo/org."""
    return [(tag, text.format(repo=repo, org=org)) for tag, text in TASK_TEMPLATES]

SYSTEM_PROMPT = (
    "You are an assistant with access to GitHub tools. Use the tools to answer the "
    "user's request, then stop when you can answer."
)

# The three shippable configurations under test, each defined by two independent
# knobs:
#   keep_fields  -- expose the `fields` param so the model CAN filter responses
#   embed_schema -- fold the tool's output schema into its description (the
#                   documented fixed tax). Only has an effect when the server was
#                   booted with output schemas enabled so the schema is present.
ARMS: dict[str, dict[str, bool]] = {
    "baseline": {"keep_fields": False, "embed_schema": False},
    "fields_only": {"keep_fields": True, "embed_schema": False},
    "schema_fields": {"keep_fields": True, "embed_schema": True},
}
# Presentation order: cheapest fixed tax -> most expensive, so deltas read naturally.
ARM_ORDER = ["baseline", "fields_only", "schema_fields"]
SCENARIO_LABEL = {
    "baseline": "S1 no-schema/no-fields",
    "fields_only": "S3 fields-only",
    "schema_fields": "S2 schema+fields",
}


def mcp_tool_to_openai(tool: dict, *, keep_fields: bool, embed_output_schema: bool) -> dict:
    schema = copy.deepcopy(tool.get("inputSchema", {"type": "object"}))
    if not keep_fields:
        props = schema.get("properties")
        if props and "fields" in props:
            del props["fields"]
        req = schema.get("required")
        if isinstance(req, list) and "fields" in req:
            req.remove("fields")
    description = tool.get("description", "")
    if embed_output_schema and "outputSchema" in tool:
        description += "\n\nReturns (output schema): " + json.dumps(
            tool["outputSchema"], separators=(",", ":")
        )
    return {
        "type": "function",
        "function": {
            "name": tool["name"],
            "description": description,
            "parameters": schema,
        },
    }


def build_toolset(tools: list[dict], *, arm: str) -> list[dict]:
    cfg = ARMS[arm]
    return [
        mcp_tool_to_openai(
            t, keep_fields=cfg["keep_fields"], embed_output_schema=cfg["embed_schema"]
        )
        for t in tools
    ]


def run_task(client, server: MCPServer, openai_tools: list[dict], task: str, *, model: str, max_turns: int, allow_fields: bool) -> dict:
    messages = [
        {"role": "system", "content": SYSTEM_PROMPT},
        {"role": "user", "content": task},
    ]
    prompt_tokens = completion_tokens = turns = tool_calls = fields_calls = 0
    tool_errors = 0
    final_text = ""
    error = ""

    for _ in range(max_turns):
        try:
            resp = client.chat.completions.create(
                model=model,
                messages=messages,
                tools=openai_tools,
                tool_choice="auto",
            )
        except Exception as exc:  # noqa: BLE001 - record (e.g. context/rate limit) and stop
            error = f"{type(exc).__name__}: {str(exc)[:200]}"
            break
        turns += 1
        if resp.usage:
            prompt_tokens += resp.usage.prompt_tokens
            completion_tokens += resp.usage.completion_tokens

        msg = resp.choices[0].message
        if not msg.tool_calls:
            final_text = msg.content or ""
            break

        messages.append(
            {
                "role": "assistant",
                "content": msg.content,
                "tool_calls": [
                    {
                        "id": tc.id,
                        "type": "function",
                        "function": {"name": tc.function.name, "arguments": tc.function.arguments},
                    }
                    for tc in msg.tool_calls
                ],
            }
        )
        for tc in msg.tool_calls:
            tool_calls += 1
            try:
                args = json.loads(tc.function.arguments or "{}")
            except json.JSONDecodeError:
                args = {}
            if not isinstance(args, dict):
                args = {}
            # In the baseline arm the model has no `fields` param; defensively drop
            # it in case the model invents one, so baseline always gets full responses.
            if not allow_fields:
                args.pop("fields", None)
            elif "fields" in args:
                fields_calls += 1
            try:
                result = server.call_tool(tc.function.name, args)
                content = text_content(result) or json.dumps(result)
                # An `isError` result (e.g. 403/SAML, not-found, rate limit) means
                # the model never saw a real payload to filter. Record it so this
                # task-run is excluded from the apples-to-apples token comparison
                # instead of silently counting as a success.
                if result.get("isError"):
                    tool_errors += 1
                    if not error:
                        error = "tool_error: " + (content or "isError")[:200]
            except Exception as exc:  # noqa: BLE001
                tool_errors += 1
                if not error:
                    error = f"{type(exc).__name__}: {str(exc)[:200]}"
                content = f"ERROR: {exc}"
            messages.append({"role": "tool", "tool_call_id": tc.id, "content": content})

    return {
        "prompt_tokens": prompt_tokens,
        "completion_tokens": completion_tokens,
        "turns": turns,
        "tool_calls": tool_calls,
        "fields_calls": fields_calls,
        "tool_errors": tool_errors,
        "final_text": final_text,
        "error": error,
    }


def summarize(records: list[dict], *, model: str, base_url: str, out_path: Path) -> None:
    """Print a 3-scenario comparison plus a per-task-type breakdown.

    A task-run only counts toward the token comparison if ALL three arms
    succeeded for it, so every comparison is apples-to-apples.
    """
    from collections import defaultdict

    by_key: dict[tuple, dict] = defaultdict(dict)
    for r in records:
        by_key[(r["run"], r["task"])][r["arm"]] = r

    keys = list(by_key)
    valid = [
        k for k in keys
        if all(a in by_key[k] and not by_key[k][a]["error"] for a in ARM_ORDER)
    ]

    print("\n=== 3-SCENARIO ONLINE A/B (cumulative prompt tokens) ===")
    print(f"model:      {model} @ {base_url}")
    print(f"task-runs:  {len(keys)}   valid (all 3 arms ok): {len(valid)}")
    for arm in ARM_ORDER:
        errs = sum(1 for k in keys if arm not in by_key[k] or by_key[k][arm]["error"])
        print(f"  failures[{SCENARIO_LABEL[arm]:<22}]: {errs}")
    print(
        "  NOTE: a failure means an arm errored on a task-run (a tool call returned "
        "isError -- e.g. 403/SAML/not-found/rate-limit -- or an unfiltered response "
        "overflowed the model's input limit). Such runs are excluded from the token "
        "comparison below so it stays apples-to-apples."
    )

    if not valid:
        print("\nNo task-runs where all three arms succeeded -- nothing to compare.")
        print(f"per-run JSONL: {out_path}")
        return

    def arm_prompt(arm: str) -> int:
        return sum(by_key[k][arm]["prompt_tokens"] for k in valid)

    base_tot = arm_prompt("baseline")
    print(f"\nVALID COMPARISON over {len(valid)} task-runs (lower prompt tokens = better):")
    print(f"  {'scenario':<24}{'prompt_tok':>11}{'Δ vs S1':>10}{'Δ%':>8}{'cheaper':>10}{'fields_use':>12}")
    for arm in ARM_ORDER:
        tot = arm_prompt(arm)
        delta = tot - base_tot
        pct = (100.0 * delta / base_tot) if base_tot else 0.0
        cheaper = sum(
            1 for k in valid
            if by_key[k][arm]["prompt_tokens"] < by_key[k]["baseline"]["prompt_tokens"]
        )
        fc = sum(by_key[k][arm]["fields_calls"] for k in valid)
        tc = sum(by_key[k][arm]["tool_calls"] for k in valid)
        sign = "+" if delta >= 0 else ""
        print(
            f"  {SCENARIO_LABEL[arm]:<24}{tot:>11}{sign + str(delta):>10}"
            f"{sign + f'{pct:.1f}':>8}{f'{cheaper}/{len(valid)}':>10}{f'{fc}/{tc}':>12}"
        )

    # Where does the benefit live? Mean prompt tokens per task-run, by task type.
    tags = sorted({by_key[k]["baseline"]["tag"] for k in valid})
    print("\nBY TASK TYPE (mean prompt tokens per task-run):")
    header = f"  {'tag':<10}" + "".join(f"{SCENARIO_LABEL[a]:>24}" for a in ARM_ORDER)
    print(header)
    for tag in tags:
        tag_keys = [k for k in valid if by_key[k]["baseline"]["tag"] == tag]
        cells = "".join(
            f"{sum(by_key[k][arm]['prompt_tokens'] for k in tag_keys) / len(tag_keys):>24.0f}"
            for arm in ARM_ORDER
        )
        print(f"  {tag:<10}{cells}")

    # ---- Per-task analytics --------------------------------------------------
    # The cumulative table above is dominated by a few heavy tasks (e.g. listing
    # hundreds of issues), which can hide the experience on small tasks a user may
    # run far more often. Here we average every task across its runs and also give
    # each task equal weight, so one huge high-savings task can't overshadow the
    # rest.
    from statistics import mean, median

    per_task: dict[str, list] = defaultdict(list)
    task_tag: dict[str, str] = {}
    for k in valid:
        task = by_key[k]["baseline"]["task"]
        per_task[task].append(k)
        task_tag[task] = by_key[k]["baseline"]["tag"]

    def task_arm_mean(task: str, arm: str) -> float:
        ks = per_task[task]
        return sum(by_key[k][arm]["prompt_tokens"] for k in ks) / len(ks)

    print("\nPER-TASK (mean prompt tokens per run; Δ% vs S1, negative = cheaper):")
    print(f"  {'tag':<8}{'S1':>9}{'S3':>9}{'S2':>9}{'S3 Δ%':>8}{'S2 Δ%':>8}  task")
    # Heaviest S1 tasks first, so big and small tasks are both visible.
    for task in sorted(per_task, key=lambda t: task_arm_mean(t, "baseline"), reverse=True):
        s1 = task_arm_mean(task, "baseline")
        s3 = task_arm_mean(task, "fields_only")
        s2 = task_arm_mean(task, "schema_fields")
        s3p = 100.0 * (s3 - s1) / s1 if s1 else 0.0
        s2p = 100.0 * (s2 - s1) / s1 if s1 else 0.0
        print(
            f"  {task_tag[task]:<8}{s1:>9.0f}{s3:>9.0f}{s2:>9.0f}"
            f"{s3p:>+8.1f}{s2p:>+8.1f}  {task[:60]}"
        )

    # Equal-weight view: each task counts once, so a single huge task can't dominate.
    print("\nEQUAL-WEIGHT ACROSS TASKS (each task counts once, regardless of size):")
    print(f"  {'scenario':<24}{'mean/task':>10}{'median/task':>13}{'mean Δ% vs S1':>15}")
    for arm in ARM_ORDER:
        per_task_means = [task_arm_mean(t, arm) for t in per_task]
        eq_mean = mean(per_task_means)
        eq_med = median(per_task_means)
        pct = [
            100.0 * (task_arm_mean(t, arm) - task_arm_mean(t, "baseline")) / task_arm_mean(t, "baseline")
            for t in per_task
            if task_arm_mean(t, "baseline")
        ]
        eq_pct = mean(pct) if pct else 0.0
        print(f"  {SCENARIO_LABEL[arm]:<24}{eq_mean:>10.0f}{eq_med:>13.0f}{eq_pct:>+15.1f}")
    print(
        "  (Unlike the cumulative table, here every task -- including cheap,\n"
        "   frequently-run ones -- counts equally, better reflecting a typical mix.)"
    )

    print(f"\nper-run JSONL: {out_path}")


def parse_tasks_file(path: str, *, repo: str, org: str) -> list[tuple[str, str]]:
    """Read a tasks file: one task per line, optionally 'tag<TAB>task text'.

    Task text may use {repo}/{org} placeholders, rendered with the run's target.
    """
    tasks: list[tuple[str, str]] = []
    for line in Path(path).read_text().splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        if "\t" in line:
            tag, text = line.split("\t", 1)
            tasks.append((tag.strip() or "neutral", text.strip().format(repo=repo, org=org)))
        else:
            tasks.append(("neutral", line.format(repo=repo, org=org)))
    return tasks


def get_copilot_token(github_token: str) -> str:
    """Return a short-lived Copilot API bearer token (no out-of-pocket key).

    The Copilot API needs a token minted from a Copilot-entitled GitHub OAuth
    token -- a plain PAT can't mint one (the mint endpoint 404s). We obtain the
    OAuth token, in priority order, from:
      1. $COPILOT_OAUTH_TOKEN (a gho_... token you already have), or
      2. the editor's stored token (~/.config/github-copilot/apps.json|hosts.json), or
      3. a cached token from a previous device login, or
      4. an interactive GitHub device login (one-time; cached afterwards).
    Then it exchanges that for the short-lived bearer the Copilot API accepts.
    """
    import urllib.error

    oauth = _copilot_oauth_token()
    try:
        return _mint_copilot_bearer(oauth)
    except urllib.error.HTTPError as exc:
        if exc.code in (401, 404) and _OAUTH_CACHE.exists():
            # Cached OAuth token likely expired/revoked -- re-login once.
            _OAUTH_CACHE.unlink(missing_ok=True)
            oauth = _device_login()
            _save_cached_oauth(oauth)
            return _mint_copilot_bearer(oauth)
        if exc.code == 404:
            raise RuntimeError(
                "Copilot token mint returned 404. The OAuth token used isn't "
                "Copilot-entitled. Sign in via the device flow (rerun and follow the "
                "prompt) or set COPILOT_OAUTH_TOKEN to a gho_ token from your editor."
            ) from exc
        raise RuntimeError(f"Copilot token mint failed: HTTP {exc.code}") from exc


# GitHub Copilot (editor) OAuth app client id -- the same public id editors use to
# authenticate Copilot. Used only for the device-login flow below.
_COPILOT_OAUTH_CLIENT_ID = "Iv1.b507a08c87ecfe98"
_OAUTH_CACHE = Path.home() / ".config" / "github-mcp-evals" / "copilot_oauth.json"


def _mint_copilot_bearer(oauth_token: str) -> str:
    import urllib.request

    req = urllib.request.Request(
        "https://api.github.com/copilot_internal/v2/token",
        headers={
            "Authorization": f"token {oauth_token}",
            "Accept": "application/json",
            "Editor-Version": "GitHubMCPServerEvals/1.0",
            "User-Agent": "GitHubMCPServerEvals/1.0",
        },
    )
    with urllib.request.urlopen(req) as resp:  # noqa: S310 - fixed, trusted URL
        data = json.loads(resp.read().decode())
    token = data.get("token")
    if not token:
        raise RuntimeError(f"unexpected Copilot token response keys: {sorted(data)}")
    return token


def _copilot_oauth_token() -> str:
    """Resolve a Copilot-entitled GitHub OAuth (gho_) token; device-login if needed."""
    env = os.environ.get("COPILOT_OAUTH_TOKEN")
    if env:
        return env
    editor = _find_editor_oauth_token()
    if editor:
        return editor
    cached = _load_cached_oauth()
    if cached:
        return cached
    token = _device_login()
    _save_cached_oauth(token)
    return token


def _find_editor_oauth_token() -> str | None:
    base = Path(os.environ.get("XDG_CONFIG_HOME") or (Path.home() / ".config")) / "github-copilot"
    for name in ("apps.json", "hosts.json"):
        path = base / name
        if not path.exists():
            continue
        try:
            data = json.loads(path.read_text())
        except Exception:  # noqa: BLE001
            continue
        for key, val in data.items():
            if "github.com" in key and isinstance(val, dict) and val.get("oauth_token"):
                return val["oauth_token"]
    return None


def _load_cached_oauth() -> str | None:
    if not _OAUTH_CACHE.exists():
        return None
    try:
        return json.loads(_OAUTH_CACHE.read_text()).get("oauth_token")
    except Exception:  # noqa: BLE001
        return None


def _save_cached_oauth(token: str) -> None:
    _OAUTH_CACHE.parent.mkdir(parents=True, exist_ok=True)
    _OAUTH_CACHE.write_text(json.dumps({"oauth_token": token}))
    try:
        os.chmod(_OAUTH_CACHE, 0o600)
    except OSError:
        pass


def _device_login() -> str:
    """Interactive GitHub device-login for the Copilot OAuth app; returns gho_ token."""
    import time
    import urllib.parse
    import urllib.request

    def post(url: str, data: dict) -> dict:
        body = urllib.parse.urlencode(data).encode()
        req = urllib.request.Request(
            url,
            data=body,
            headers={"Accept": "application/json", "User-Agent": "GitHubMCPServerEvals/1.0"},
        )
        with urllib.request.urlopen(req) as resp:  # noqa: S310 - fixed, trusted URLs
            return json.loads(resp.read().decode())

    dc = post(
        "https://github.com/login/device/code",
        {"client_id": _COPILOT_OAUTH_CLIENT_ID, "scope": "read:user"},
    )
    print(
        f"\n[copilot auth] Open {dc['verification_uri']} and enter code: "
        f"{dc['user_code']}\n[copilot auth] Waiting for authorization...",
        file=sys.stderr,
    )
    interval = int(dc.get("interval", 5))
    deadline = time.time() + int(dc.get("expires_in", 900))
    while time.time() < deadline:
        time.sleep(interval)
        res = post(
            "https://github.com/login/oauth/access_token",
            {
                "client_id": _COPILOT_OAUTH_CLIENT_ID,
                "device_code": dc["device_code"],
                "grant_type": "urn:ietf:params:oauth:grant-type:device_code",
            },
        )
        if res.get("access_token"):
            print("[copilot auth] Authorized.", file=sys.stderr)
            return res["access_token"]
        err = res.get("error")
        if err == "authorization_pending":
            continue
        if err == "slow_down":
            interval += 5
            continue
        raise RuntimeError(f"device login failed: {res}")
    raise RuntimeError("device login timed out before authorization")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--model",
        default=DEFAULT_MODEL,
        help="Any model the endpoint serves, e.g. openai/gpt-5 or claude-opus-4-6.",
    )
    parser.add_argument(
        "--base-url",
        default=DEFAULT_BASE_URL,
        help="OpenAI-compatible endpoint. Defaults to GitHub Models; point it at a "
        "Copilot / internal proxy for a large-context model with no request cap.",
    )
    parser.add_argument(
        "--api-key-env",
        default=None,
        help="Env var holding the API key/token for the endpoint. If unset, tries "
        "GITHUB_MODELS_TOKEN, GITHUB_COPILOT_TOKEN, OPENAI_API_KEY, GITHUB_TOKEN, "
        "then GITHUB_PERSONAL_ACCESS_TOKEN.",
    )
    parser.add_argument("--toolsets", default="issues,pull_requests,repos")
    parser.add_argument("--max-turns", type=int, default=8)
    parser.add_argument(
        "--repo",
        default=DEFAULT_REPO,
        help="Target owner/repo for the default tasks and {repo} placeholders. Use a "
        "large, public, SAML-free repo so a plain PAT gets real, sizeable responses "
        f"to filter. Default: {DEFAULT_REPO}.",
    )
    parser.add_argument(
        "--org",
        default=None,
        help="Organization for the issue-types task and {org} placeholders. Defaults "
        "to the owner of --repo.",
    )
    parser.add_argument(
        "--repeat",
        type=int,
        default=1,
        help="Run every task in every arm this many times to average out model "
        "nondeterminism. Use >=3 for a result you'd present.",
    )
    parser.add_argument(
        "--tasks-file",
        help="Optional tasks file: one task per line, optionally prefixed with "
        "'tag<TAB>' where tag is narrow|full|neutral.",
    )
    parser.add_argument("--out", default="out/schema_fields_eval.jsonl")
    parser.add_argument("--server-cmd", default="go run ./cmd/github-mcp-server stdio")
    parser.add_argument(
        "--list-models",
        action="store_true",
        help="List the models the endpoint exposes (handy to find the exact "
        "Copilot model id) and exit.",
    )
    parser.add_argument(
        "--copilot-integration-id",
        default="vscode-chat",
        help="Copilot-Integration-Id header; only sent to the Copilot endpoint.",
    )
    parser.add_argument(
        "--editor-version",
        default="GitHubMCPServerEvals/1.0",
        help="Editor-Version header; only sent to the Copilot endpoint.",
    )
    args = parser.parse_args()

    gh_token = os.environ.get("GITHUB_PERSONAL_ACCESS_TOKEN")
    if not gh_token and not args.list_models:
        print("ERROR: set GITHUB_PERSONAL_ACCESS_TOKEN (real token for the MCP server).", file=sys.stderr)
        return 2
    try:
        from openai import OpenAI  # type: ignore
    except ImportError:
        print("ERROR: pip install openai", file=sys.stderr)
        return 2

    # The Copilot API is OpenAI-compatible but needs a short-lived token (minted
    # from your GitHub token) plus integration headers -- no out-of-pocket key.
    is_copilot = "githubcopilot.com" in args.base_url

    def build_client():
        headers: dict[str, str] = {}
        if args.api_key_env:
            key = os.environ.get(args.api_key_env)
            if not key:
                raise RuntimeError(f"{args.api_key_env} is not set")
        elif is_copilot:
            key = get_copilot_token(gh_token)
        else:
            key = (
                os.environ.get("GITHUB_MODELS_TOKEN")
                or os.environ.get("GITHUB_COPILOT_TOKEN")
                or os.environ.get("OPENAI_API_KEY")
                or os.environ.get("GITHUB_TOKEN")
                or gh_token
            )
        if is_copilot:
            headers = {
                "Copilot-Integration-Id": args.copilot_integration_id,
                "Editor-Version": args.editor_version,
            }
        return OpenAI(base_url=args.base_url, api_key=key, default_headers=headers or None)

    try:
        client = build_client()
    except Exception as exc:  # noqa: BLE001
        print(f"ERROR: failed to build model client: {exc}", file=sys.stderr)
        return 2

    if args.list_models:
        try:
            models = client.models.list()
            ids = sorted(getattr(m, "id", str(m)) for m in models.data)
        except Exception as exc:  # noqa: BLE001
            print(f"ERROR: failed to list models: {exc}", file=sys.stderr)
            return 2
        print(f"Models available at {args.base_url}:")
        for mid in ids:
            print(f"  {mid}")
        return 0

    org = args.org or args.repo.split("/", 1)[0]
    tasks = (
        parse_tasks_file(args.tasks_file, repo=args.repo, org=org)
        if args.tasks_file
        else build_tasks(args.repo, org)
    )

    out_path = Path(args.out)
    out_path.parent.mkdir(parents=True, exist_ok=True)

    records: list[dict] = []
    # Boot the server WITH output schemas enabled so the `schema_fields` arm has a
    # real outputSchema to embed (and thus pays its real fixed tax). The server
    # exposes the `fields` param and filters regardless of the flag, so all three
    # arms see correct server behavior; only the tool defs shown to the model vary.
    with MCPServer(
        server_cmd=args.server_cmd,
        extra_args=["--toolsets", args.toolsets, "--read-only", "--features", "output_schemas"],
    ) as server:
        mcp_tools = server.list_tools()
        have_schema = sum(1 for t in mcp_tools if "outputSchema" in t)
        if have_schema == 0:
            print(
                "WARNING: no tool reported an outputSchema, so the schema_fields arm "
                "is identical to fields_only. Is `--features output_schemas` wired up "
                "in the server build?",
                file=sys.stderr,
            )
        arms = {arm: build_toolset(mcp_tools, arm=arm) for arm in ARM_ORDER}

        with out_path.open("w") as fh:
            for run_idx in range(args.repeat):
                for tag, task in tasks:
                    # Copilot tokens are short-lived; mint a fresh one per task so a
                    # long --repeat run doesn't fail midway with an expired token.
                    if is_copilot:
                        try:
                            client = build_client()
                        except Exception as exc:  # noqa: BLE001
                            print(f"WARNING: Copilot token refresh failed: {exc}", file=sys.stderr)
                    for arm in ARM_ORDER:
                        m = run_task(
                            client,
                            server,
                            arms[arm],
                            task,
                            model=args.model,
                            max_turns=args.max_turns,
                            allow_fields=ARMS[arm]["keep_fields"],
                        )
                        m.update({"arm": arm, "tag": tag, "task": task, "run": run_idx})
                        records.append(m)
                        fh.write(json.dumps(m) + "\n")
                        print(
                            f"[{SCENARIO_LABEL[arm]:>22}] run{run_idx} "
                            f"prompt={m['prompt_tokens']:>7} calls={m['tool_calls']} "
                            f"fields={m['fields_calls']} "
                            f"{'ERR:' + m['error'] if m['error'] else ''} :: {task[:46]}",
                            file=sys.stderr,
                        )

    summarize(records, model=args.model, base_url=args.base_url, out_path=out_path)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
