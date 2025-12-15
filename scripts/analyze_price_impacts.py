#!/usr/bin/env python3
"""Analyze positive and negative impact factors for Gold and Apple prices.

Usage:
  python scripts/analyze_price_impacts.py
"""
from __future__ import annotations
import os
import sys
import requests
from datetime import datetime


def fetch_news(api_token: str, limit: int = 50):
    """Fetch general market news from EODHD API."""
    base_url = "https://eodhd.com/api/news"
    params = {
        'api_token': api_token,
        'limit': limit,
        'offset': 0,
        'fmt': 'json'
    }

    try:
        resp = requests.get(base_url, params=params, timeout=30)
        resp.raise_for_status()
        return resp.json()
    except Exception as e:
        print(f"Error fetching news: {e}")
        return []


def analyze_gold_impacts(news_items):
    """Analyze factors impacting gold prices."""
    print("\n" + "=" * 100)
    print("GOLD PRICE IMPACT ANALYSIS")
    print("=" * 100)

    positive_factors = []
    negative_factors = []

    # Keywords that indicate positive impact on gold
    gold_positive_keywords = [
        'inflation', 'rate cut', 'uncertainty', 'geopolitical', 'risk',
        'dollar weakness', 'fed dovish', 'economic slowdown', 'recession',
        'safe haven', 'central bank buying', 'debt crisis', 'volatility'
    ]

    # Keywords that indicate negative impact on gold
    gold_negative_keywords = [
        'rate hike', 'strong dollar', 'fed hawkish', 'economic growth',
        'risk appetite', 'rate increase', 'dollar strength', 'optimism',
        'equity rally', 'stock surge'
    ]

    # Analyze news items
    for item in news_items:
        title = item.get('title', '').lower()
        content = item.get('content', '').lower()
        tags = [tag.lower() for tag in item.get('tags', [])]

        text = f"{title} {content} {' '.join(tags)}"

        # Check for gold-positive factors
        for keyword in gold_positive_keywords:
            if keyword in text:
                positive_factors.append({
                    'factor': keyword.title(),
                    'title': item.get('title', 'N/A'),
                    'date': item.get('date', 'N/A'),
                    'link': item.get('link', 'N/A')
                })
                break

        # Check for gold-negative factors
        for keyword in gold_negative_keywords:
            if keyword in text:
                negative_factors.append({
                    'factor': keyword.title(),
                    'title': item.get('title', 'N/A'),
                    'date': item.get('date', 'N/A'),
                    'link': item.get('link', 'N/A')
                })
                break

    return positive_factors, negative_factors


def analyze_apple_impacts(news_items):
    """Analyze factors impacting Apple stock prices."""
    print("\n" + "=" * 100)
    print("APPLE (AAPL) PRICE IMPACT ANALYSIS")
    print("=" * 100)

    positive_factors = []
    negative_factors = []

    # Keywords that indicate positive impact on Apple
    apple_positive_keywords = [
        'apple', 'aapl', 'iphone sales', 'services growth', 'innovation',
        'earnings beat', 'revenue growth', 'ai integration', 'product launch',
        'market share', 'tech rally', 'chip', 'semiconductor strength',
        'consumer demand', 'holiday sales'
    ]

    # Keywords that indicate negative impact on Apple
    apple_negative_keywords = [
        'supply chain', 'china risk', 'regulatory', 'antitrust',
        'tech selloff', 'valuation concerns', 'competition', 'weak demand',
        'earnings miss', 'guidance cut', 'trade war', 'tariff',
        'manufacturing issue', 'recall'
    ]

    # Analyze news items
    for item in news_items:
        title = item.get('title', '').lower()
        content = item.get('content', '').lower()
        tags = [tag.lower() for tag in item.get('tags', [])]
        symbols = [sym.upper() for sym in item.get('symbols', [])]

        text = f"{title} {content} {' '.join(tags)}"

        # Direct Apple mention
        is_apple_related = 'AAPL.US' in symbols or 'apple' in title.lower()

        # Check for Apple-positive factors
        for keyword in apple_positive_keywords:
            if keyword in text:
                positive_factors.append({
                    'factor': keyword.title(),
                    'title': item.get('title', 'N/A'),
                    'date': item.get('date', 'N/A'),
                    'direct': is_apple_related,
                    'link': item.get('link', 'N/A')
                })
                break

        # Check for Apple-negative factors
        for keyword in apple_negative_keywords:
            if keyword in text:
                negative_factors.append({
                    'factor': keyword.title(),
                    'title': item.get('title', 'N/A'),
                    'date': item.get('date', 'N/A'),
                    'direct': is_apple_related,
                    'link': item.get('link', 'N/A')
                })
                break

    return positive_factors, negative_factors


def display_impacts(asset_name, positive_factors, negative_factors):
    """Display impact analysis in a structured format."""
    print(f"\n{'*' * 100}")
    print(f"POSITIVE IMPACT FACTORS FOR {asset_name} (+{len(positive_factors)} factors)")
    print(f"{'*' * 100}\n")

    if positive_factors:
        for i, factor in enumerate(positive_factors[:10], 1):  # Top 10
            print(f"{i}. [{factor['factor']}]")
            print(f"   Title: {factor['title']}")
            print(f"   Date: {factor.get('date', 'N/A')}")
            if factor.get('direct'):
                print(f"   Direct Impact: YES")
            print()
    else:
        print("No significant positive factors identified.\n")

    print(f"\n{'*' * 100}")
    print(f"NEGATIVE IMPACT FACTORS FOR {asset_name} (-{len(negative_factors)} factors)")
    print(f"{'*' * 100}\n")

    if negative_factors:
        for i, factor in enumerate(negative_factors[:10], 1):  # Top 10
            print(f"{i}. [{factor['factor']}]")
            print(f"   Title: {factor['title']}")
            print(f"   Date: {factor.get('date', 'N/A')}")
            if factor.get('direct'):
                print(f"   Direct Impact: YES")
            print()
    else:
        print("No significant negative factors identified.\n")

    # Summary
    print(f"\n{'=' * 100}")
    print(f"SUMMARY FOR {asset_name}")
    print(f"{'=' * 100}")
    print(f"Positive Factors: {len(positive_factors)}")
    print(f"Negative Factors: {len(negative_factors)}")

    sentiment_score = len(positive_factors) - len(negative_factors)
    if sentiment_score > 0:
        sentiment = f"BULLISH (+{sentiment_score})"
    elif sentiment_score < 0:
        sentiment = f"BEARISH ({sentiment_score})"
    else:
        sentiment = "NEUTRAL (0)"

    print(f"Net Sentiment: {sentiment}")
    print(f"{'=' * 100}\n")


def main():
    # Get API key
    api_token = (os.environ.get('EODHD_APIKEY') or
                 os.environ.get('EODHD_API_KEY') or
                 os.environ.get('EODHD_API') or
                 os.environ.get('EODHD_MCP_APIKEY') or
                 '690d7cdc3013f4.57364117')

    print("=" * 100)
    print("FETCHING LATEST MARKET NEWS FOR IMPACT ANALYSIS")
    print("=" * 100)

    # Fetch news
    news_items = fetch_news(api_token, limit=50)

    if not news_items:
        print("Failed to fetch news.")
        sys.exit(1)

    print(f"Analyzing {len(news_items)} news items...\n")

    # Analyze Gold
    gold_positive, gold_negative = analyze_gold_impacts(news_items)
    display_impacts("GOLD", gold_positive, gold_negative)

    # Analyze Apple
    apple_positive, apple_negative = analyze_apple_impacts(news_items)
    display_impacts("APPLE", apple_positive, apple_negative)

    # Overall market context
    print("\n" + "=" * 100)
    print("KEY MARKET FACTORS (TODAY)")
    print("=" * 100)

    # Extract key themes
    themes = {}
    for item in news_items[:20]:
        tags = item.get('tags', [])
        for tag in tags:
            themes[tag] = themes.get(tag, 0) + 1

    # Sort and display top themes
    sorted_themes = sorted(themes.items(), key=lambda x: x[1], reverse=True)
    print("\nTop Market Themes:")
    for theme, count in sorted_themes[:10]:
        print(f"  - {theme}: {count} mentions")


if __name__ == '__main__':
    main()
