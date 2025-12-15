#!/usr/bin/env python3
"""Fetch latest news from EODHD API.

Usage:
  python scripts/fetch_eodhd_news.py --limit 10
  # Fetches news for the default symbols defined in the script
  python scripts/fetch_eodhd_news.py --limit 10 --symbols AAPL.US,MSFT.US
"""
from __future__ import annotations
import os
import sys
import argparse
import requests
from datetime import datetime


def fetch_news(api_token: str, limit: int = 10, symbols: str = None, offset: int = 0):
    """Fetch news directly from EODHD API."""
    # Use EODHD API directly, not via MCP proxy for news
    base_url = "https://eodhd.com/api/news"

    params = {
        'api_token': api_token,
        'limit': limit,
        'offset': offset,
        'fmt': 'json'
    }

    if symbols:
        params['s'] = symbols

    print(f"Fetching {limit} latest news from EODHD for symbols: {symbols}...")
    try:
        resp = requests.get(base_url, params=params, timeout=30)
        resp.raise_for_status()
        data = resp.json()
        return data
    except Exception as e:
        print(f"Error fetching news: {e}")
        return None


def display_news(news_items):
    """Display news items in a readable format."""
    if not news_items:
        print("No news items found.")
        return

    print(f"\n{'=' * 100}")
    print(f"LATEST {len(news_items)} NEWS FROM EODHD")
    print(f"{'=' * 100}\n")

    for i, item in enumerate(news_items, 1):
        print(f"{i}. {item.get('title', 'No Title')}")
        print(f"   Date: {item.get('date', 'N/A')}")

        # Parse and format date if available
        if item.get('date'):
            try:
                dt = datetime.fromisoformat(item['date'].replace('Z', '+00:00'))
                print(f"   Published: {dt.strftime('%Y-%m-%d %H:%M:%S UTC')}")
            except:
                pass

        if item.get('symbols'):
            print(f"   Symbols: {', '.join(item.get('symbols', []))}")

        if item.get('tags'):
            print(f"   Tags: {', '.join(item.get('tags', []))}")

        if item.get('link'):
            print(f"   Link: {item.get('link')}")

        # Show preview of content if available
        if item.get('content'):
            content = item['content']
            preview = content[:200] + '...' if len(content) > 200 else content
            print(f"   Preview: {preview}")

        print()


def main():
    parser = argparse.ArgumentParser(description='Fetch latest news from EODHD API, aligned with the Enhanced Hybrid Trading Plan.')
    parser.add_argument('--limit', type=int, default=15, help='Number of news items to fetch (default: 15)')
    # Default symbols are set to align with the trading plan: AAPL for the stock, XAUUSD for the commodity, and SPY as a market proxy.
    parser.add_argument('--symbols', type=str, default='AAPL.US,XAUUSD.FOREX,SPY.US', help='Filter by symbols (comma-separated)')
    parser.add_argument('--offset', type=int, default=0, help='Offset for pagination (default: 0)')
    parser.add_argument('--apikey', default=None, help='EODHD API key')

    args = parser.parse_args()

    # Get API key from various sources. The user-provided key is used as the final fallback.
    api_token = (args.apikey or
                 os.environ.get('EODHD_APIKEY') or
                 os.environ.get('EODHD_API_KEY') or
                 os.environ.get('EODHD_API') or
                 os.environ.get('EODHD_MCP_APIKEY') or
                 '690d7cdc3013f4.57364117')

    # Fetch news
    news_items = fetch_news(api_token, args.limit, args.symbols, args.offset)

    if news_items:
        display_news(news_items)

        # --- INTEGRATION PLACEHOLDER ---
        # The following section is where the logic from the ENHANCED_HYBRID_TRADING_PLAN.md would be integrated.
        print("\n" + "="*50)
        print("NEXT STEPS: TRADING PLAN INTEGRATION")
        print("="*50)
        print("1. SENTIMENT ANALYSIS:")
        print("   - For each news item, process the 'title' and 'content' using a sentiment analysis library (e.g., NLTK, spaCy, or a pre-trained model).")
        print("   - Calculate an overall sentiment score (e.g., from -1 to +1) for each asset (AAPL, XAUUSD).")
        print("   - This score corresponds to 'Layer 3: News Sentiment Analysis' in the trading plan.\n")

        print("2. IBD SCORECARD (for AAPL.US):")
        print("   - This script provides the 'N' (New...) and 'M' (Market Direction via SPY news) components.")
        print("   - To complete the IBD score, you would need to integrate other data sources for:")
        print("     - C: Current Earnings (from EODHD Fundamentals API)")
        print("     - A: Annual Earnings (from EODHD Fundamentals API)")
        print("     - S: Supply & Demand (from EODHD Price/Volume API)")
        print("     - L: Leader/Laggard (Relative Strength vs. industry)")
        print("     - I: Institutional Sponsorship (from EODHD Fundamentals API)")
        print("   - The combined score corresponds to 'Layer 2: IBD CAN SLIM Technical Scorecard'.\n")

        print("3. MACROECONOMIC CHECK:")
        print("   - Integrate data from TradingEconomics or the EODHD Macroeconomics API for the indicators listed in 'Layer 1' of the plan.")
        print("   - Use these indicators to determine if the overall macro environment is 'Favorable' or 'Hostile'.\n")

    else:
        print("Failed to fetch news.")
        sys.exit(1)


if __name__ == '__main__':
    main()
