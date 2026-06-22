# Phase 1 evals: context cost vs. benefit of output schemas + field filtering

Small, deterministic scripts to get the numbers we care about, compared across
**three shippable configurations**:

| Scenario | output schema | `fields` param | what it represents |
|----------|:---:|:---:|--------------------|
| **S1 baseline** | ✗ | ✗ | today's behavior (no experiment) |
| **S2 schema+fields** | ✓ | ✓ | the full experiment |
| **S3 fields-only** | ✗ | ✓ | hypothesized sweet spot: the model can filter without carrying the heavy schema |

The intuition behind **S3**: the model doesn't need the full output schema to
filter — it just needs the `fields` param (whose enum already tells it which
fields exist). So S3 may capture almost all of the benefit at a fraction of the
fixed cost.

The two numbers we derive:

1. **Fixed tax** — extra tokens added to the `tools/list` payload (paid once at
   client init) by each scenario.
2. **Per-call savings** — tokens saved when the model filters a tool response to
   a subset of fields.

From those: **break-even calls = fixed_tax / avg_savings_per_call**, computed per
scenario.

No LLM required for the offline numbers (tokenization falls back to a chars/4
proxy if `tiktoken` isn't installed). The online A/B (step 4) runs the three
scenarios through a real model over realistic multi-tool sessions.

## Setup

```bash
cd evals
python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt          # tiktoken (+ anthropic for the online A/B)
```

### Token & secrets

Live tool calls and the online A/B need a real GitHub token. **Never hardcode or
commit it.** Provide it via the environment only; `.env*` is gitignored:

```bash
export GITHUB_PERSONAL_ACCESS_TOKEN=ghp_xxx     # read-only scope is enough
```

A dummy token is used automatically for `capture_tools.py` (it never calls the API).


## 1) Fixed tax

Capture the tool list WITH the experiment enabled, then let the analyzer derive
all three scenarios by stripping `outputSchema` and/or the `fields` property:

```bash
python3 capture_tools.py --features output_schemas --toolsets all \
    --out out/tools.treatment.json

python3 fixed_tax.py --tools out/tools.treatment.json --json-out out/fixed_tax.json
# add --approx if offline without tiktoken
```

`fixed_tax.py` prints the payload tokens for each scenario (S1/S2/S3), the fixed
tax of each vs the S1 baseline, a component breakdown (schema vs `fields`), and a
per-tool breakdown. `--json-out` writes the per-scenario taxes for step 3.

> Tip: measure with the `--toolsets` you'd actually ship. The tax is fixed in
> absolute tokens but its *relative* size shrinks the more tools you expose.

## 2) Per-call savings (real data)

Capture real full vs filtered responses for the affected tools, straight from
live GitHub (read-only), then token-diff them:

```bash
export GITHUB_PERSONAL_ACCESS_TOKEN=ghp_xxx
python3 capture_fixtures.py --owner github --repo github-mcp-server --org github
python3 response_savings.py --fixed-tax-json out/fixed_tax.json
```

`capture_fixtures.py` calls each tool twice (no `fields` vs a small subset) and
writes `fixtures/<tool>.full.json` / `.filtered.json`. Use a busy repo for a
realistic upper bound, **and** a small repo for the lower bound — report a range.
An example pair is included so the script also runs offline.

## 3) Break-even (per scenario)

`response_savings.py --fixed-tax-json out/fixed_tax.json` prints break-even for
both taxed scenarios:

```
break_even_calls = scenario_fixed_tax / avg_saved_per_call
```

Interpretation:
- A session with **more** filtered list/search calls than `break_even_calls` is
  net-positive on context for that scenario.
- **S3 (fields-only)** has a far smaller tax than S2, so its break-even is tiny —
  this is the configuration to scrutinize first.
- Short sessions (few tool calls) are where the fixed tax dominates — call this
  out in the writeup.

## 4) Online A/B (Phase 2 — real multi-tool sessions, all 3 scenarios)

Runs the same tasks through a real model across all three scenarios, measuring
cumulative prompt tokens. This is the only way to confirm the model actually
*uses* `fields` and to get the true net effect — including whether S3 really is
the sweet spot.

**Use a model with a real context window.** The harness talks to any
**OpenAI-compatible** endpoint, so you don't need a paid third-party key:

- **GitHub Models** (default) — authenticated with your GitHub token, no extra
  key. Convenient, but the free tier caps requests at **16,000 tokens**, so large
  unfiltered responses error out (`413`). Fine for a smoke test; **not** for the
  headline numbers.
- **A Copilot / internal proxy** — point `--base-url` at any OpenAI-compatible
  endpoint you already have access to and pass its token via `--api-key-env`. This
  is how to run a large-context model (e.g. `claude-opus-4-6`) with no request cap
  and no out-of-pocket billing.

```bash
export GITHUB_PERSONAL_ACCESS_TOKEN=ghp_xxx     # always: the MCP server uses this

# Smoke test: GitHub Models gpt-5 (16k cap — expect overflow failures on big repos)
python3 schema_fields_eval.py --model openai/gpt-5 --toolsets issues,pull_requests --repeat 3

# Recommended: a large-context model via your OpenAI-compatible endpoint
export COPILOT_TOKEN=...                          # whatever token that endpoint needs
python3 schema_fields_eval.py \
    --base-url https://your-openai-compatible-endpoint/v1 \
    --api-key-env COPILOT_TOKEN \
    --model claude-opus-4-6 --toolsets issues,pull_requests --repeat 3

# --base-url <url>          any OpenAI-compatible endpoint
# --api-key-env VAR         env var holding that endpoint's token
# --repo owner/repo         target repo for the tasks (default cli/cli; see below)
# --tasks-file mytasks.txt  one task per line, optionally 'tag<TAB>task'
```

> Target a readable repo: the tasks run against a **live** repo, so point `--repo`
> at a large, **public, SAML-free** repo a plain PAT can read (default `cli/cli`).
> If you aim at a SAML-protected org repo (e.g. `github/github-mcp-server`), every
> call 403s, the model only ever sees tiny error payloads, and the `fields` arms
> look like pure overhead because there's nothing to filter — the experiment then
> measures only the fixed schema/param tax, not the filtering payoff. Such runs
> are now flagged as failures (a tool returning `isError`) and excluded from the
> token comparison rather than silently counted as valid.

The server is booted with `--features output_schemas` so the **S2** arm has a real
schema to embed; the `fields` param and server-side filtering are present in every
arm regardless, so only what each arm shows the *model* differs.

It prints, per scenario: cumulative prompt/completion tokens, tool-call counts,
`fields` adoption, the net delta vs the S1 baseline, and a **per-task-type
breakdown** (narrow / full / neutral) so you can see *where* each config helps.
Only task-runs where all three arms succeeded count toward the token comparison
(so on the capped GitHub Models endpoint, the biggest filtering wins — tasks where
the unfiltered baseline overflowed — show up in the failure counts, not the token
table; another reason to use a large-context endpoint). Use `--repeat >= 3` to
average out model nondeterminism. Per-run detail is written to
`out/schema_fields_eval.jsonl`.

> Authoritative billed cost: when `--base-url` is the Copilot API
> (`https://api.githubcopilot.com`), every response carries a vendor
> `copilot_usage.token_details` block with the **real per-type prices** the
> billing system uses (input / cache_read / cache_write / output) plus a summed
> `total_nano_aiu`. The harness reads it straight off each response and reports an
> **authoritative billed cost in AIU** (AI credits — the native billing unit),
> including the `cache_write` bucket that OpenAI-style usage never exposes. This is
> the same source the Copilot agent runtime benchmarks use, so no hand-typed
> prices are involved. For other endpoints (GitHub Models, OpenAI) that don't
> return `copilot_usage`, it falls back to a flat per-1M estimate from
> `--price-prompt` / `--price-cached` / `--price-completion`. A credit→USD rate is
> account-specific and non-public, so the cost is reported in AIU by default; pass
> `--aiu-to-usd <rate>` only if you know yours and want the billed-cost tables in
> dollars.

> Task design matters: the default tasks are intentionally **neutral** (they do
> not tell the model to "return only X"). Biasing prompts toward terse answers
> would inflate the filtering arms. Keep a balanced mix of narrow/full/neutral.

> Cost control: the default toolsets are narrow on purpose. The relevant
> differences live in the affected tools, so you don't need all 79 tools loaded
> each turn. Use `fixed_tax.py` (all toolsets) for the init-tax number and the
> online run for the savings/net dynamic.



## Honesty notes

- Tokenizers differ across providers; report **deltas** and state the tokenizer.
- Step 2 assumes the model actually uses `fields`. That adoption rate can only be
  confirmed by the Phase 2 online A/B — Phase 1 is an upper bound on benefit.
- Real response sizes vary a lot by repo; capture fixtures from both a small and a
  large/busy repo and report a range, not a single number.
- The `fields` param and the server-side filtering are **not** gated by the
  `output_schemas` feature flag in the server — only the `outputSchema` and the
  response's `structuredContent` are. So S1 (baseline) here means "pre-experiment
  main", and "flag off" in production today would still ship the `fields` param.
  Reconcile the scenario you measure with the toggle you'd actually ship.
- With output schemas on, each tool result also carries a `structuredContent`
  duplicate of the payload. The online A/B forwards only the text content to the
  model (so all arms see identical response bytes); a client that also feeds
  `structuredContent` to the model would pay more in the S2 arm. State this
  assumption when you report.

## Files

| File | Purpose |
|------|---------|
| `capture_tools.py` | Boot server over stdio, dump `tools/list` result |
| `fixed_tax.py` | Per-scenario token-diff (S1/S2/S3); `--json-out` for break-even |
| `capture_fixtures.py` | Capture real full/filtered tool responses (live GitHub) |
| `response_savings.py` | Token-diff full vs filtered responses; per-scenario break-even |
| `schema_fields_eval.py` | 3-scenario (A/B/C) multi-tool agent eval, prompt-token accounting |
| `_mcp_client.py` | Shared MCP stdio client |
| `_tokenize.py` | Tokenizer helper (tiktoken or chars/4 fallback) |
| `fixtures/` | Response pairs (example + captured) |

