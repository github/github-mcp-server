#!/usr/bin/env python3
"""Compute the "fixed tax": how many extra tokens the output schemas and the new
`fields` params add to the `tools/list` payload a client receives at init.

Reports the three shippable configurations measured by the online A/B, so the
offline init-cost numbers line up with the online session numbers:

  * S1 baseline      -- no output schema, no `fields` param
  * S2 schema+fields -- output schema AND `fields` param (full experiment)
  * S3 fields-only   -- `fields` param but NO output schema (hypothesized sweet spot)

Takes a single capture produced WITH the experiment on (output schemas enabled),
then derives the other configurations by stripping `outputSchema` and/or the
`fields` property from each tool's `inputSchema`.

Usage:
    python3 fixed_tax.py --tools out/tools.treatment.json
    python3 fixed_tax.py --tools out/tools.treatment.json --approx        # offline
    python3 fixed_tax.py --tools out/tools.treatment.json --json-out out/fixed_tax.json
"""

from __future__ import annotations

import argparse
import copy
import json
from pathlib import Path

from _tokenize import count_obj_tokens, get_tokenizer


def strip_output_schema(tool: dict) -> dict:
    t = copy.deepcopy(tool)
    t.pop("outputSchema", None)
    return t


def strip_fields_param(tool: dict) -> dict:
    t = copy.deepcopy(tool)
    props = t.get("inputSchema", {}).get("properties")
    if props and "fields" in props:
        del props["fields"]
    req = t.get("inputSchema", {}).get("required")
    if isinstance(req, list) and "fields" in req:
        req.remove("fields")
    return t


def strip_both(tool: dict) -> dict:
    return strip_fields_param(strip_output_schema(tool))


def pct(part: int, whole: int) -> str:
    return f"{(100.0 * part / whole):.2f}%" if whole else "n/a"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--tools", required=True, help="Capture JSON (experiment ON)")
    parser.add_argument("--approx", action="store_true", help="Force chars/4 proxy")
    parser.add_argument(
        "--json-out",
        help="Optional path to write the three scenario taxes as JSON "
        "(consumed by response_savings.py --fixed-tax-json for break-even).",
    )
    args = parser.parse_args()

    data = json.loads(Path(args.tools).read_text())
    tools = data["tools"]
    _, mode = get_tokenizer(args.approx)

    full = [count_obj_tokens(t, args.approx) for t in tools]
    no_schema = [count_obj_tokens(strip_output_schema(t), args.approx) for t in tools]
    no_fields = [count_obj_tokens(strip_fields_param(t), args.approx) for t in tools]
    baseline = [count_obj_tokens(strip_both(t), args.approx) for t in tools]

    sum_full = sum(full)
    sum_no_schema = sum(no_schema)
    sum_no_fields = sum(no_fields)
    sum_baseline = sum(baseline)

    schema_cost = sum_full - sum_no_schema
    fields_cost = sum_full - sum_no_fields
    total_tax = sum_full - sum_baseline

    # Per-scenario payload sizes and their fixed tax vs the S1 baseline.
    #   S1 baseline      = strip both          (sum_baseline)
    #   S2 schema+fields = full                (sum_full)
    #   S3 fields-only   = strip output schema (sum_no_schema)
    s2_tax = sum_full - sum_baseline
    s3_tax = sum_no_schema - sum_baseline

    print(f"tokenizer:            {mode}")
    print(f"tools in payload:     {len(tools)}")
    print(f"  with outputSchema:  {sum(1 for t in tools if 'outputSchema' in t)}")
    print(
        f"  with `fields` param: "
        f"{sum(1 for t in tools if 'fields' in (t.get('inputSchema', {}).get('properties', {}) or {}))}"
    )
    print()
    print("PAYLOAD TOKENS PER SCENARIO (whole tools/list)")
    print(f"  S1 baseline (no schema/no fields): {sum_baseline}")
    print(f"  S2 schema+fields (full):           {sum_full}")
    print(f"  S3 fields-only (no schema):        {sum_no_schema}")
    print()
    print("FIXED TAX vs S1 baseline (extra tokens paid once at init)")
    print(f"  S2 schema+fields: +{s2_tax:>6}  ({pct(s2_tax, sum_baseline)} of baseline)")
    print(f"  S3 fields-only:   +{s3_tax:>6}  ({pct(s3_tax, sum_baseline)} of baseline)")
    print()
    print("FIXED TAX BY COMPONENT")
    print(f"  output schemas:  +{schema_cost:>6}  ({pct(schema_cost, sum_baseline)} of baseline)")
    print(f"  `fields` params: +{fields_cost:>6}  ({pct(fields_cost, sum_baseline)} of baseline)")
    print(f"  combined:        +{total_tax:>6}  ({pct(total_tax, sum_baseline)} of baseline)")
    print()

    # Per-tool breakdown, only for tools that changed.
    rows = []
    for i, t in enumerate(tools):
        delta = full[i] - baseline[i]
        if delta:
            rows.append((t.get("name", "?"), full[i] - no_schema[i], full[i] - no_fields[i], delta))
    rows.sort(key=lambda r: r[3], reverse=True)
    if rows:
        print("PER-TOOL ADDED TOKENS")
        print(f"  {'tool':<24}{'schema':>8}{'fields':>8}{'total':>8}")
        for name, sc, fl, tot in rows:
            print(f"  {name:<24}{sc:>8}{fl:>8}{tot:>8}")

    if args.json_out:
        out = {
            "tokenizer": mode,
            "tool_count": len(tools),
            "scenarios": {
                "baseline": {"payload_tokens": sum_baseline, "fixed_tax": 0},
                "schema_fields": {"payload_tokens": sum_full, "fixed_tax": s2_tax},
                "fields_only": {"payload_tokens": sum_no_schema, "fixed_tax": s3_tax},
            },
        }
        out_path = Path(args.json_out)
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_text(json.dumps(out, indent=2))
        print(f"\nwrote scenario taxes -> {out_path}")

    print()
    print(
        "Next: run response_savings.py (optionally --fixed-tax-json out/fixed_tax.json)\n"
        "to get avg tokens saved per filtered call and the per-scenario break-even."
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
