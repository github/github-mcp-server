#!/usr/bin/env python3
"""Measure tokens saved per tool call when the model requests a subset of fields.

Pairs of fixture files represent the *text* payload a tool returns:
  fixtures/<name>.full.json       # response with no `fields` filter (all fields)
  fixtures/<name>.filtered.json   # response when the model asked for a few fields

For each pair it prints tokens(full), tokens(filtered) and the savings, plus an
overall average used for the break-even calculation.

Usage:
    python3 response_savings.py                 # uses ./fixtures
    python3 response_savings.py --dir fixtures --approx

Capture real fixtures with the bundled mcpcurl, e.g.:
    ./mcpcurl --stdio-server-cmd "go run ./cmd/github-mcp-server stdio" \
        tools list_issue_types --owner github > fixtures/list_issue_types.full.json
    ... and again with --fields name for the filtered version.
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path

from _tokenize import get_tokenizer


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dir", default=str(Path(__file__).parent / "fixtures"))
    parser.add_argument("--approx", action="store_true", help="Force chars/4 proxy")
    parser.add_argument(
        "--fixed-tax-json",
        help="Optional fixed_tax.py --json-out file; prints per-scenario break-even.",
    )
    args = parser.parse_args()

    count, mode = get_tokenizer(args.approx)
    fixtures_dir = Path(args.dir)
    fulls = sorted(fixtures_dir.glob("*.full.json"))
    if not fulls:
        print(f"No *.full.json fixtures found in {fixtures_dir}")
        return 1

    print(f"tokenizer: {mode}\n")
    print(f"  {'scenario':<28}{'full':>8}{'filtered':>10}{'saved':>8}{'saved%':>8}")
    total_saved = 0
    pairs = 0
    for full_path in fulls:
        name = full_path.name[: -len(".full.json")]
        filtered_path = fixtures_dir / f"{name}.filtered.json"
        if not filtered_path.exists():
            print(f"  {name:<28}  (missing {filtered_path.name}, skipped)")
            continue
        full_tokens = count(full_path.read_text())
        filtered_tokens = count(filtered_path.read_text())
        saved = full_tokens - filtered_tokens
        savedpct = f"{(100.0 * saved / full_tokens):.1f}%" if full_tokens else "n/a"
        print(f"  {name:<28}{full_tokens:>8}{filtered_tokens:>10}{saved:>8}{savedpct:>8}")
        total_saved += saved
        pairs += 1

    if not pairs:
        return 0

    avg = total_saved / pairs
    print()
    print(f"avg saved per filtered call: {avg:.0f} tokens (over {pairs} scenarios)")

    if args.fixed_tax_json:
        data = json.loads(Path(args.fixed_tax_json).read_text())
        scenarios = data.get("scenarios", {})
        print()
        print("BREAK-EVEN (filtered list/search calls needed to repay the init tax)")
        print(f"  {'scenario':<20}{'fixed tax':>11}{'break-even calls':>18}")
        # Show the two configurations that actually carry a tax.
        for key, label in (("fields_only", "S3 fields-only"), ("schema_fields", "S2 schema+fields")):
            tax = scenarios.get(key, {}).get("fixed_tax")
            if tax is None:
                continue
            be = (tax / avg) if avg else float("inf")
            print(f"  {label:<20}{tax:>11}{be:>18.2f}")
        print(
            "  (A session with more filtered calls than the break-even is net-positive "
            "on context for that scenario.)"
        )
    else:
        print("break-even calls = fixed_tax / avg_saved_per_call")
        print("  (pass --fixed-tax-json out/fixed_tax.json to compute it per scenario)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
