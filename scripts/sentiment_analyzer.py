#!/usr/bin/env python3
# -*- coding: utf-8 -*-

"""
Sentiment Analyzer for Financial News (v2 - Corrected and Improved)

This script fetches the latest financial news, performs a keyword-based sentiment
analysis, and calculates an aggregate sentiment score for each symbol.

This implements 'Layer 3: News Sentiment Analysis' from the
ENHANCED_HYBRID_TRADING_PLAN.md.

Corrections in v2:
- Handles potential ImportError by modifying sys.path to make the script more robust.

Improvements in v2:
- Expanded sentiment keyword dictionary for better accuracy.
- Keywords in the title are given more weight (3x) than in the content.

Usage:
  python scripts/sentiment_analyzer.py
"""

from __future__ import annotations
import os
import sys
from collections import defaultdict

# --- Correction for Robust Importing ---
# This ensures that the script can find other modules in the same directory,
# regardless of where the script is executed from.
try:
    # Add the script's directory to the Python path
    script_dir = os.path.dirname(os.path.abspath(__file__))
    if script_dir not in sys.path:
        sys.path.append(script_dir)
    
    from fetch_eodhd_news import fetch_news

except ImportError:
    print("FATAL: Could not import from 'fetch_eodhd_news.py'.")
    print("Please ensure 'fetch_eodhd_news.py' is in the same directory.")
    sys.exit(1)


# --- Expanded Sentiment Keyword Sets ---
POSITIVE_WORDS = {
    'advances', 'upbeat', 'bullish', 'buy', 'strong', 'gains', 'profit', 'record',
    'surpass', 'outperform', 'rally', 'boost', 'growth', 'upgrade', 'optimistic',
    'recover', 'surge', 'beat', 'exceeds', 'positive', 'stabilizes', 'stronger',
    'approval', 'breakthrough', 'expansion', 'highest', 'launches', 'momentum'
}

NEGATIVE_WORDS = {
    'declines', 'bearish', 'sell', 'weak', 'losses', 'slump', 'downgrade',
    'pessimistic', 'fears', 'recession', 'correction', 'drop', 'plunge',
    'misses', 'negative', 'volatile', 'crisis', 'risk', 'warns', 'cuts', 'down',
    'disappoints', 'fraud', 'investigation', 'lawsuit', 'slashes', 'tumbles'
}


def get_sentiment_score(title: str, content: str) -> int:
    """
    Calculates a weighted sentiment score based on keyword matching.
    Keywords in the title have a 3x weight.

    Args:
        title: The news title.
        content: The news content/preview.

    Returns:
        An integer score. Positive is bullish, negative is bearish.
    """
    score = 0
    title_words = set(title.lower().split())
    content_words = set(content.lower().split())

    # Score title (higher weight)
    score += len(title_words.intersection(POSITIVE_WORDS)) * 3
    score -= len(title_words.intersection(NEGATIVE_WORDS)) * 3

    # Score content
    score += len(content_words.intersection(POSITIVE_WORDS))
    score -= len(content_words.intersection(NEGATIVE_WORDS))

    return score


def analyze_sentiment_for_news(news_items: list) -> dict:
    """
    Analyzes a list of news items and aggregates sentiment by symbol.

    Args:
        news_items: A list of news dictionaries from fetch_news.

    Returns:
        A dictionary mapping each symbol to its aggregated sentiment score.
    """
    sentiment_scores = defaultdict(int)

    if not news_items:
        return sentiment_scores

    print(f"\nAnalyzing sentiment for {len(news_items)} articles...")
    for item in news_items:
        title = item.get('title', '')
        content = item.get('content', '')

        # Use the improved scoring function
        score = get_sentiment_score(title, content)

        if score == 0:
            continue # Skip articles with no sentiment keywords

        # Attribute the score to all symbols associated with the news
        symbols_in_item = item.get('symbols', [])
        if not symbols_in_item:
            # If no symbols are tagged, attribute to a general 'market' category
            sentiment_scores['market'] += score
        else:
            for symbol in symbols_in_item:
                sentiment_scores[symbol] += score
    
    return sentiment_scores


def main():
    """
    Main function to fetch, analyze, and display news sentiment.
    """
    print("--- Running Gemini 3 Enhanced Sentiment Analyzer (v2) ---")

    # 1. Fetch News using the existing module
    api_token = (os.environ.get('EODHD_APIKEY') or
                 os.environ.get('EODHD_API_KEY') or
                 '690d7cdc3013f4.57364117')
    
    # Symbols aligned with our trading plan
    symbols_to_fetch = 'AAPL.US,XAUUSD.FOREX,SPY.US'
    
    news_items = fetch_news(api_token=api_token, limit=50, symbols=symbols_to_fetch)

    if not news_items:
        print("Could not fetch news. Aborting analysis.")
        return

    # 2. Analyze Sentiment
    sentiment_by_symbol = analyze_sentiment_for_news(news_items)

    # 3. Display Results
    print("\n" + "="*50)
    print("Layer 3: News Sentiment Analysis Results")
    print("="*50)
    print(f"Analysis based on the last {len(news_items)} news articles for {symbols_to_fetch}.")
    print("Method: Weighted keyword matching (Title weight: 3x).\n")

    if not sentiment_by_symbol:
        print("No meaningful sentiment could be derived from the news articles found.")
    else:
        # Sort for consistent output
        sorted_symbols = sorted(sentiment_by_symbol.keys())
        for symbol in sorted_symbols:
            score = sentiment_by_symbol[symbol]
            sentiment = "Neutral"
            if score > 0:
                sentiment = "Positive"
            elif score < 0:
                sentiment = "Negative"
            
            print(f"  - {symbol:<15} | Aggregate Score: {score:<4} | Sentiment: {sentiment}")
    
    print("\n" + "="*50)
    print("This output can now be used as the 'News Sentiment' filter in the trading plan.")


if __name__ == "__main__":
    main()
