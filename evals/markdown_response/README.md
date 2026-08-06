# Markdown response eval

This harness compares the current JSON responses with the lossless Markdown representation enabled by `markdown_output` for `get_file_contents`, `pull_request_read`, `create_pull_request`, `list_pull_requests`, and `search_pull_requests`.

Each read-only scenario is captured once from the MCP server and then converted offline with the same Go renderer used by the feature flag. This ensures both arms contain identical source data. The directory, list, and search tools are measured both with their full output and after existing `fields` filtering. `create_pull_request` uses a representative fixture matching its exact `{id, url}` response shape and is never called against GitHub.

The headline measurement serializes the complete MCP `tools/call` result with compact JSON, approximating the model-facing tool-result message and accounting for the escaping paid when JSON is nested inside a text content block. The output also records inner-text bytes and tokens for structured text responses. Token counts use `tiktoken` with `o200k_base`; `--approx` forces a chars/4 fallback for smoke tests.

## Run

```bash
cd evals/markdown_response
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
export GITHUB_PERSONAL_ACCESS_TOKEN=...
python3 markdown_response_eval.py
```

By default the live scenarios use public data from `github/github-mcp-server` and pull request `#2658`. The script writes metrics only to `out/markdown-response-eval.json`; it does not persist response bodies.

```bash
python3 markdown_response_eval.py --owner cli --repo cli --pull-number 12345 --per-page 30
```

To measure the table break-even point, run the same dataset with multiple page sizes:

```bash
for page_size in 1 10 30; do
  python3 markdown_response_eval.py --pull-number 2797 --per-page "$page_size" --out "out/page-${page_size}.json"
done
```

The included `results/github-mcp-server-2026-07-31.json` file is a response-body-free summary of this scale run.

`get_file_contents.file` and `pull_request_read.get_diff` are included as controls. They already return resource/plain-text content and should show zero change.
