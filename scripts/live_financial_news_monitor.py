#!/usr/bin/env python3
"""Live EODHD MCP news monitor with significance flags.

Polls the EODHD MCP server for the latest financial headlines, scores their
market significance, and highlights new items. Default cadence is every
5 minutes (300s) with the top 10 stories.

Usage:
  python scripts/live_financial_news_monitor.py
  python scripts/live_financial_news_monitor.py --once --limit 5
  python scripts/live_financial_news_monitor.py --interval 120 --limit 15 --apikey YOUR_KEY

Requires: requests
"""
from __future__ import annotations
import argparse
import os
import sys
import time
import urllib.parse
from datetime import datetime, timezone
from typing import Any, Dict, Iterable, List, Optional, Set, Tuple

import requests


DEFAULT_LIMIT = 10
DEFAULT_INTERVAL = 600  # seconds (10 minutes)
DEFAULT_APIKEY = "690d7cdc3013f4.57364117"
MCP_BASE = "https://mcp.eodhd.dev/mcp"

# Keyword buckets used to approximate significance
SIGNIFICANCE_RULES: List[Dict[str, Any]] = [
    {
        "label": "Central banks / rates",
        "weight": 3,
        "keywords": (
            "rate hike",
            "rate cut",
            "central bank",
            "fomc",
            "fed",
            "ecb",
            "boj",
            "boe",
            "pboc",
            "policy rate",
            "treasury yield",
            "bond selloff",
            "yield curve",
            "gilts",
            "bunds",
            "sofr",
            "repo",
        ),
    },
    {
        "label": "Inflation / jobs / growth",
        "weight": 2,
        "keywords": (
            "inflation",
            "cpi",
            "ppi",
            "pce",
            "jobs report",
            "nonfarm",
            "payroll",
            "unemployment",
            "gdp",
            "recession",
            "slowdown",
            "pmi",
            "ism",
            "retail sales",
        ),
    },
    {
        "label": "Geopolitics / energy",
        "weight": 3,
        "keywords": (
            "geopolitical",
            "conflict",
            "war",
            "attack",
            "sanction",
            "tariff",
            "trade war",
            "ceasefire",
            "strait",
            "opec",
            "oil supply",
            "pipeline",
            "blockade",
        ),
    },
    {
        "label": "Credit stress / defaults",
        "weight": 3,
        "keywords": (
            "default",
            "downgrade",
            "rating cut",
            "insolvency",
            "bankruptcy",
            "debt ceiling",
            "bailout",
            "liquidity",
            "bank failure",
            "credit crunch",
        ),
    },
    {
        "label": "Volatility / market structure",
        "weight": 2,
        "keywords": (
            "volatility",
            "vix",
            "limit down",
            "limit up",
            "halted",
            "circuit breaker",
            "flash crash",
            "futures plunge",
        ),
    },
    {
        "label": "Corporate actions / earnings",
        "weight": 1,
        "keywords": (
            "earnings",
            "guidance",
            "profit warning",
            "restructuring",
            "layoff",
            "buyback",
            "dividend cut",
            "merger",
            "acquisition",
            "ipo",
        ),
    },
]

# Quick impact mapping to flag which markets are likely touched
IMPACT_MAP: Dict[str, Tuple[str, ...]] = {
    "Gold/Metals": ("gold", "xau", "bullion", "metals", "gc ", "silver"),
    "Equities/ETFs": (
        "stock",
        "equity",
        "s&p",
        "nasdaq",
        "dow",
        "index",
        "etf",
        "earnings",
    ),
    "Rates/Bonds": ("yield", "treasury", "bond", "gilt", "bund", "note", "curve"),
    "FX/US Dollar": ("dollar", "usd", "eurusd", "yen", "jpy", "cny", "fx", "currency"),
    "Energy/Oil": ("oil", "brent", "wti", "gas", "opec"),
    "Derivatives": ("futures", "options", "volatility", "vix", "gamma"),
}

# Targeted relevance filters for gold, Apple, and China markets
RELEVANCE_KEYWORDS = {
    "gold": ("gold", "xau", "bullion", "precious metal", "gc ", "metals"),
    "silver": ("silver", "xag", "precious metal", "metals"),
    "apple": ("apple", "aapl", "iphone", "ipad", "macbook", "tim cook"),
    "china": (
        "china",
        "shanghai",
        "shenzen",
        "shenzhen",
        "csi",
        "hang seng",
        "hkex",
        "cnh",
        "cny",
        "prc",
        "beijing",
        "chinese stock",
        "mainland market",
    ),
    "us_equities": (
        "s&p",
        "sp500",
        "spx",
        "dow",
        "nasdaq",
        "russell",
        "wall street",
        "stock market",
        "equities",
        "us stocks",
    ),
}

RELEVANCE_SYMBOLS = {
    "gold": ("XAUUSD", "GC.", "XAU", "GLD", "IAU", "BAR", "SGOL"),
    "silver": ("XAGUSD", "XAG", "SLV", "SIL"),
    "apple": ("AAPL", "AAPL.US"),
    "china": (
        "ASHR",
        "MCHI",
        "FXI",
        "KWEB",
        "CNY",
        "CNH",
        "HSI",
        "SSE",
        "CSI300",
        "CSI 300",
    ),
    "us_equities": ("SPY", "VOO", "IVV", "QQQ", "DIA", "IWM"),
}


def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(description="Monitor EODHD MCP news for market-moving headlines.")
    p.add_argument("--interval", type=int, default=DEFAULT_INTERVAL, help="Polling interval in seconds (default: 300)")
    p.add_argument("--limit", type=int, default=DEFAULT_LIMIT, help="Number of headlines to fetch (default: 10)")
    p.add_argument("--apikey", default=None, help="EODHD MCP API key")
    p.add_argument("--once", action="store_true", help="Run a single fetch instead of continuous monitoring")
    p.add_argument(
        "--webhook-url",
        default=None,
        help="Optional webhook URL for notifications (also reads EODHD_WEBHOOK_URL env)",
    )
    return p.parse_args()


def safe_text(value: Any) -> str:
    if value is None:
        return ""
    if not isinstance(value, str):
        value = str(value)
    return value.encode("ascii", "replace").decode()


def sanitize_cell(value: Any) -> str:
    return safe_text(value).replace("|", "/").replace("\n", " ").strip()


def resolve_apikey(cli_value: Optional[str]) -> str:
    return (
        cli_value
        or os.environ.get("EODHD_APIKEY")
        or os.environ.get("EODHD_API_KEY")
        or os.environ.get("EODHD_API")
        or os.environ.get("EODHD_MCP_APIKEY")
        or DEFAULT_APIKEY
    )


def resolve_webhook(cli_value: Optional[str]) -> Optional[str]:
    return cli_value or os.environ.get("EODHD_WEBHOOK_URL")


def build_news_url(apikey: str, limit: int, offset: int = 0) -> str:
    inner = f"/api/news?limit={limit}&offset={offset}&fmt=json"
    encoded = urllib.parse.quote(inner, safe="")
    return f"{MCP_BASE}?apikey={apikey}&url={encoded}"


def fetch_news(apikey: str, limit: int) -> List[Dict[str, Any]]:
    url = build_news_url(apikey, limit)
    try:
        resp = requests.get(url, headers={"Accept": "application/json"}, timeout=30)
        resp.raise_for_status()
        payload = resp.json()
    except Exception as exc:
        fallback_url = "https://eodhd.com/api/news"
        params = {"api_token": apikey, "limit": limit, "offset": 0, "fmt": "json"}
        print(f"MCP fetch failed ({exc}); falling back to direct API...")
        resp = requests.get(fallback_url, params=params, timeout=30)
        resp.raise_for_status()
        payload = resp.json()

    if isinstance(payload, dict) and "data" in payload and isinstance(payload["data"], list):
        payload = payload["data"]
    if not isinstance(payload, list):
        raise RuntimeError(f"Unexpected news payload type: {type(payload)}")
    return payload


def parse_date(value: Optional[str]) -> Optional[datetime]:
    if not value:
        return None
    try:
        return datetime.fromisoformat(value.replace("Z", "+00:00")).astimezone(timezone.utc)
    except Exception:
        return None


def describe_age(dt: Optional[datetime]) -> str:
    if not dt:
        return "time n/a"
    delta = datetime.now(timezone.utc) - dt
    hours = int(delta.total_seconds() // 3600)
    minutes = int((delta.total_seconds() % 3600) // 60)
    if hours == 0:
        return f"{minutes}m ago"
    if hours < 24:
        return f"{hours}h {minutes}m ago"
    days = hours // 24
    return f"{days}d ago"


def detect_impacts(text: str, symbols: Iterable[str]) -> List[str]:
    sym_text = " ".join(sym.lower() for sym in symbols)
    combined = f"{text} {sym_text}"
    hits = []
    for label, keywords in IMPACT_MAP.items():
        if any(kw in combined for kw in keywords):
            hits.append(label)
    return hits


def is_relevant_to_focus(text: str, symbols: Iterable[str]) -> bool:
    combined = f"{text} {' '.join(sym.lower() for sym in symbols)}"
    for group in RELEVANCE_KEYWORDS.values():
        for kw in group:
            if kw in combined:
                return True
    for sym in symbols:
        upper = sym.upper()
        for group_syms in RELEVANCE_SYMBOLS.values():
            if upper in group_syms:
                return True
    return False


ALLOWED_SOURCES = (
    "bloomberg",
    "reuters",
    "ft.com",
    "financialtimes",
    "seekingalpha",
    "finance.yahoo",
    "yahoo.com",
    "investors.com",  # IBD
    "barrons.com",
    "investing.com",
    "investopedia.com",
)


def is_allowed_source(item: Dict[str, Any]) -> bool:
    link = item.get("link") or ""
    try:
        host = urllib.parse.urlparse(link).netloc.lower()
    except Exception:
        host = ""
    if any(src in host for src in ALLOWED_SOURCES):
        return True
    # Also allow if title explicitly references the source
    title = (item.get("title") or "").lower()
    return any(src in title for src in ALLOWED_SOURCES)


MAX_AGE_HOURS = 48


def is_recent_enough(item: Dict[str, Any]) -> bool:
    dt = parse_date(item.get("date"))
    if not dt:
        return False
    age_hours = (datetime.now(timezone.utc) - dt).total_seconds() / 3600.0
    return age_hours <= MAX_AGE_HOURS


def score_significance(text: str, published: Optional[datetime]) -> Tuple[int, List[str]]:
    score = 0
    drivers: List[str] = []
    for rule in SIGNIFICANCE_RULES:
        if any(kw in text for kw in rule["keywords"]):
            score += rule["weight"]
            drivers.append(rule["label"])
    if published:
        age_hours = (datetime.now(timezone.utc) - published).total_seconds() / 3600.0
        if age_hours <= 6:
            score += 2
        elif age_hours <= 24:
            score += 1
    if "breaking" in text or "emergency" in text:
        score += 1
    return score, drivers


def classify_level(score: int) -> str:
    if score >= 7:
        return "HIGH"
    if score >= 4:
        return "MEDIUM"
    return "LOW"


def news_key(item: Dict[str, Any]) -> str:
    return str(item.get("id") or item.get("link") or f"{item.get('title','')}-{item.get('date','')}")


def build_markdown_table(rows: List[Tuple[Dict[str, Any], int, List[str], List[str], str, bool]]) -> str:
    header = "|Level|New?|Title|Age|Score|Drivers|Impact|Symbols|Link|"
    sep = "|---|---|---|---|---|---|---|---|---|"
    out_lines = [header, sep]
    for item, score, drivers, impacts, level, is_new in rows:
        published = parse_date(item.get("date"))
        title = sanitize_cell(item.get("title", "No title"))
        age = sanitize_cell(describe_age(published))
        drivers_txt = sanitize_cell(", ".join(drivers) if drivers else "general market move")
        impacts_txt = sanitize_cell(", ".join(impacts))
        symbols_txt = sanitize_cell(", ".join(item.get("symbols", [])))
        link_txt = sanitize_cell(item.get("link", ""))
        out_lines.append(
            f"|{level}|{'YES' if is_new else 'NO'}|{title}|{age}|{score}|{drivers_txt}|{impacts_txt}|{symbols_txt}|{link_txt}|"
        )
    return "\n".join(out_lines)


def build_takeaways(rows: List[Tuple[Dict[str, Any], int, List[str], List[str], str, bool]]) -> List[str]:
    takeaways = []
    for item, score, drivers, impacts, level, is_new in rows:
        title = sanitize_cell(item.get("title", "No title"))
        impacts_txt = ", ".join(impacts) if impacts else "General market"
        drivers_txt = ", ".join(drivers) if drivers else "Broad factors"
        age_txt = describe_age(parse_date(item.get("date")))
        takeaways.append(f"- [{level}] {title} — impacts: {impacts_txt}; drivers: {drivers_txt}; age: {age_txt}")
    return takeaways


def send_webhook_notification(webhook_url: str, rows: List[Tuple[Dict[str, Any], int, List[str], List[str], str, bool]]) -> None:
    if not rows:
        return
    table = build_markdown_table(rows)
    takeaways = "\n".join(build_takeaways(rows))
    lines = [f"*EODHD news alerts* ({len(rows)} new/notable)", table, "", "Takeaways:", takeaways]
    payload = {"text": "\n".join(lines)}
    try:
        resp = requests.post(webhook_url, json=payload, timeout=10)
        resp.raise_for_status()
        print(f"Webhook sent ({len(rows)} items).")
    except Exception as exc:
        print(f"Webhook failed: {exc}")


def render_item(item: Dict[str, Any], score: int, drivers: List[str], impacts: List[str], level: str, is_new: bool) -> None:
    title = safe_text(item.get("title") or "No title")
    published = parse_date(item.get("date"))
    date_text = safe_text(item.get("date", "unknown"))
    drivers_text = ", ".join(drivers) if drivers else "general market move"
    print(f"[{level}][{'NEW' if is_new else 'seen'}] {title}")
    print(f"  {date_text} ({describe_age(published)}) | score {score} | drivers: {drivers_text}")
    if impacts:
        print(f"  Impact: {', '.join(impacts)}")
    symbols = [safe_text(sym) for sym in (item.get("symbols") or [])]
    if symbols:
        print(f"  Symbols: {', '.join(symbols)}")
    tags = [safe_text(tag) for tag in (item.get("tags") or [])]
    if tags:
        print(f"  Tags: {', '.join(tags)}")
    if item.get("link"):
        print(f"  Link: {safe_text(item['link'])}")
    print()


def process_cycle(apikey: str, limit: int, seen: Set[str], webhook_url: Optional[str]) -> None:
    print("=" * 80)
    print(f"[{datetime.now(timezone.utc).strftime('%Y-%m-%d %H:%M:%S UTC')}] Checking top {limit} EODHD MCP headlines...")
    try:
        news_items = fetch_news(apikey, limit)
    except Exception as exc:
        print(f"Fetch failed: {exc}")
        return

    if not news_items:
        print("No news returned.")
        return

    # Filter early to avoid unnecessary work
    filtered_items: List[Dict[str, Any]] = []
    for item in news_items:
        title = item.get("title", "")
        content = item.get("content", "")
        tags = " ".join(item.get("tags", []))
        text = " ".join([title, content, tags]).lower()
        if not is_relevant_to_focus(text, item.get("symbols", [])):
            continue
        if not is_allowed_source(item):
            continue
        if not is_recent_enough(item):
            continue
        filtered_items.append(item)

    if not filtered_items:
        print("No gold/Apple/China-relevant headlines in this batch.")
        return

    new_items: List[Dict[str, Any]] = []
    for item in filtered_items:
        key = news_key(item)
        if key not in seen:
            new_items.append(item)
            seen.add(key)

    if new_items:
        print(f"New items: {len(new_items)}")
    else:
        print("No new headlines since last check.")

    # Focus on new items for alerts, but still resurface notable medium/high items
    notable_items: List[Tuple[Dict[str, Any], int, List[str], List[str], str, bool]] = []
    new_keys = {news_key(i) for i in new_items}
    for item in filtered_items:
        title = item.get("title", "")
        content = item.get("content", "")
        tags = " ".join(item.get("tags", []))
        text = " ".join([title, content, tags]).lower()
        published = parse_date(item.get("date"))
        score, drivers = score_significance(text, published)
        level = classify_level(score)
        impacts = detect_impacts(text, item.get("symbols", []))
        is_new = news_key(item) in new_keys
        if not is_relevant_to_focus(text, item.get("symbols", [])):
            continue  # Skip items unrelated to gold, Apple, or China markets
        if is_new or level in ("HIGH", "MEDIUM"):
            notable_items.append((item, score, drivers, impacts, level, is_new))

    # Sort newest first for readability
    def sort_key(entry: Tuple[Dict[str, Any], int, List[str], List[str], str, bool]) -> float:
        published = parse_date(entry[0].get("date"))
        return -(published.timestamp() if published else 0.0)

    notable_items.sort(key=sort_key)

    for item, score, drivers, impacts, level, is_new in notable_items:
        render_item(item, score, drivers, impacts, level, is_new)

    if not notable_items:
        print("Top headlines carry low significance based on heuristics.")
        return

    print("Markdown table:")
    print(build_markdown_table(notable_items))

    if webhook_url:
        new_only = [row for row in notable_items if row[5]]  # is_new == True
        send_webhook_notification(webhook_url, new_only or notable_items)

    print("Takeaways (level-classified):")
    for line in build_takeaways(notable_items):
        print(line)


def monitor(apikey: str, limit: int, interval: int, run_once: bool, webhook_url: Optional[str]) -> None:
    seen: Set[str] = set()
    while True:
        process_cycle(apikey, limit, seen, webhook_url)
        if run_once:
            break
        time.sleep(max(1, interval))


def main() -> None:
    args = parse_args()
    apikey = resolve_apikey(args.apikey)
    webhook_url = resolve_webhook(args.webhook_url)
    if args.interval < 30 and not args.once:
        print("Interval too short for continuous polling; use --once or set >=30s.", file=sys.stderr)
        sys.exit(1)
    monitor(apikey, args.limit, args.interval, args.once, webhook_url)


if __name__ == "__main__":
    main()
