#!/usr/bin/env python3
"""Comprehensive Gold and Apple analysis with IBD methodology, citations, and predictions."""
import requests
import json
import os
import sys
from datetime import datetime, timedelta
from statistics import mean, stdev

def fetch_price_data(symbol, api_token, days=252):
    """Fetch EOD price data from EODHD API."""
    url = f"https://eodhd.com/api/eod/{symbol}"
    params = {
        'api_token': api_token,
        'fmt': 'json',
        'period': 'd',
        'order': 'd'
    }

    try:
        resp = requests.get(url, params=params, timeout=30)
        resp.raise_for_status()
        data = resp.json()
        return data[:days] if isinstance(data, list) else []
    except Exception as e:
        print(f"Error fetching {symbol}: {e}", file=sys.stderr)
        return []

def fetch_news_with_citations(api_token, limit=50):
    """Fetch news and return with full citation info."""
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
        print(f"Error fetching news: {e}", file=sys.stderr)
        return []

def calculate_ibd_metrics(price_data):
    """Calculate IBD methodology metrics."""
    if not price_data or len(price_data) < 50:
        return {}

    latest = price_data[0]
    current_price = float(latest['close'])

    # Get prices for different periods
    prices = [float(d['close']) for d in price_data]
    highs = [float(d['high']) for d in price_data]
    lows = [float(d['low']) for d in price_data]
    volumes = [float(d.get('volume', 0)) for d in price_data if d.get('volume')]

    # Calculate moving averages
    ma_10 = mean(prices[:10]) if len(prices) >= 10 else None
    ma_21 = mean(prices[:21]) if len(prices) >= 21 else None
    ma_50 = mean(prices[:50]) if len(prices) >= 50 else None
    ma_200 = mean(prices[:200]) if len(prices) >= 200 else None

    # Calculate average volume
    avg_vol_50 = mean(volumes[:min(50, len(volumes))]) if volumes else 0
    latest_vol = float(latest.get('volume', 0))

    # Calculate % from highs
    high_52w = max(prices[:min(252, len(prices))])
    low_52w = min(prices[:min(252, len(prices))])
    pct_from_high = ((current_price - high_52w) / high_52w) * 100
    pct_from_low = ((current_price - low_52w) / low_52w) * 100

    # Calculate relative strength (performance)
    rs_10d = ((current_price - prices[9]) / prices[9]) * 100 if len(prices) >= 10 else None
    rs_21d = ((current_price - prices[20]) / prices[20]) * 100 if len(prices) >= 21 else None
    rs_50d = ((current_price - prices[49]) / prices[49]) * 100 if len(prices) >= 50 else None

    # Price action analysis
    is_above_ma10 = current_price > ma_10 if ma_10 else None
    is_above_ma21 = current_price > ma_21 if ma_21 else None
    is_above_ma50 = current_price > ma_50 if ma_50 else None
    is_above_ma200 = current_price > ma_200 if ma_200 else None

    # Volume analysis
    vol_ratio = (latest_vol / avg_vol_50) if avg_vol_50 > 0 else 0

    # Support and resistance (pivots)
    recent_20d_high = max(highs[:20]) if len(highs) >= 20 else current_price
    recent_20d_low = min(lows[:20]) if len(lows) >= 20 else current_price

    # Volatility (ATR-like calculation)
    ranges = [float(d['high']) - float(d['low']) for d in price_data[:14]]
    avg_true_range = mean(ranges) if ranges else 0
    atr_pct = (avg_true_range / current_price) * 100 if current_price > 0 else 0

    # Trend strength (ADX-like)
    up_days = sum(1 for i in range(1, min(20, len(prices))) if prices[i-1] > prices[i])
    trend_consistency = (up_days / 19) * 100 if len(prices) >= 20 else 50

    return {
        'current_price': round(current_price, 2),
        'previous_close': round(float(price_data[1]['close']), 2) if len(price_data) > 1 else current_price,
        'ma_10': round(ma_10, 2) if ma_10 else None,
        'ma_21': round(ma_21, 2) if ma_21 else None,
        'ma_50': round(ma_50, 2) if ma_50 else None,
        'ma_200': round(ma_200, 2) if ma_200 else None,
        'high_52w': round(high_52w, 2),
        'low_52w': round(low_52w, 2),
        'pct_from_high': round(pct_from_high, 2),
        'pct_from_low': round(pct_from_low, 2),
        'rs_10d': round(rs_10d, 2) if rs_10d else None,
        'rs_21d': round(rs_21d, 2) if rs_21d else None,
        'rs_50d': round(rs_50d, 2) if rs_50d else None,
        'is_above_ma10': is_above_ma10,
        'is_above_ma21': is_above_ma21,
        'is_above_ma50': is_above_ma50,
        'is_above_ma200': is_above_ma200,
        'avg_volume_50d': int(avg_vol_50),
        'latest_volume': int(latest_vol),
        'volume_ratio': round(vol_ratio, 2),
        'resistance': round(recent_20d_high, 2),
        'support': round(recent_20d_low, 2),
        'atr_pct': round(atr_pct, 2),
        'trend_consistency': round(trend_consistency, 2),
        'latest_date': latest['date']
    }

def calculate_ibd_composite_rating(metrics):
    """Calculate IBD Composite Rating (0-100 scale)."""
    score = 0

    # Relative Strength (40 points max)
    rs_50d = metrics.get('rs_50d', 0) or 0
    if rs_50d >= 30: score += 40
    elif rs_50d >= 20: score += 32
    elif rs_50d >= 10: score += 24
    elif rs_50d >= 5: score += 16
    elif rs_50d >= 0: score += 8

    # Price trend vs MAs (30 points max)
    ma_score = 0
    if metrics.get('is_above_ma10'): ma_score += 8
    if metrics.get('is_above_ma21'): ma_score += 8
    if metrics.get('is_above_ma50'): ma_score += 7
    if metrics.get('is_above_ma200'): ma_score += 7
    score += ma_score

    # Position from high (15 points max)
    pct_high = metrics.get('pct_from_high', -100)
    if pct_high >= -2: score += 15
    elif pct_high >= -5: score += 12
    elif pct_high >= -10: score += 9
    elif pct_high >= -20: score += 6
    elif pct_high >= -30: score += 3

    # Volume (15 points max)
    vol_ratio = metrics.get('volume_ratio', 0)
    if vol_ratio >= 2.0: score += 15
    elif vol_ratio >= 1.5: score += 12
    elif vol_ratio >= 1.2: score += 9
    elif vol_ratio >= 1.0: score += 6
    elif vol_ratio >= 0.8: score += 3

    return min(score, 100)

def get_ibd_rating_label(score):
    """Convert score to IBD rating."""
    if score >= 90: return "A+"
    elif score >= 80: return "A"
    elif score >= 70: return "A-"
    elif score >= 60: return "B+"
    elif score >= 50: return "B"
    elif score >= 40: return "B-"
    elif score >= 30: return "C+"
    elif score >= 20: return "C"
    else: return "D"

def predict_next_week(price_data, metrics, news_sentiment):
    """Predict next week price range using technical and fundamental analysis."""
    if not price_data:
        return {}

    current_price = metrics['current_price']
    atr_pct = metrics.get('atr_pct', 2)

    # Base volatility prediction
    expected_weekly_move = atr_pct * 2.5  # Weekly volatility estimate

    # Adjust based on trend
    trend_bias = 0
    if metrics.get('is_above_ma10') and metrics.get('is_above_ma21'):
        trend_bias = 0.02  # 2% bullish bias
    elif not metrics.get('is_above_ma10') and not metrics.get('is_above_ma21'):
        trend_bias = -0.02  # 2% bearish bias

    # Adjust based on news sentiment
    sentiment_bias = news_sentiment * 0.01  # Convert sentiment to % bias

    # Calculate predictions
    total_bias = trend_bias + sentiment_bias

    # Conservative, base, and aggressive scenarios
    base_prediction = current_price * (1 + total_bias)
    upside_target = current_price * (1 + total_bias + expected_weekly_move/100)
    downside_target = current_price * (1 + total_bias - expected_weekly_move/100)

    # Support and resistance levels
    support = metrics.get('support', current_price * 0.97)
    resistance = metrics.get('resistance', current_price * 1.03)

    return {
        'base_prediction': round(base_prediction, 2),
        'upside_target': round(upside_target, 2),
        'downside_target': round(downside_target, 2),
        'support_level': support,
        'resistance_level': resistance,
        'expected_range_pct': round(expected_weekly_move, 2),
        'trend_bias_pct': round(trend_bias * 100, 2),
        'sentiment_bias_pct': round(sentiment_bias * 100, 2)
    }

def analyze_news_sentiment(news_items, keywords_positive, keywords_negative):
    """Analyze news sentiment for specific asset."""
    positive_count = 0
    negative_count = 0
    citations = {'positive': [], 'negative': []}

    for item in news_items:
        title = item.get('title', '').lower()
        content = item.get('content', '').lower()
        tags = ' '.join(item.get('tags', [])).lower()
        text = f"{title} {content} {tags}"

        is_positive = any(kw in text for kw in keywords_positive)
        is_negative = any(kw in text for kw in keywords_negative)

        citation = {
            'title': item.get('title', 'N/A'),
            'date': item.get('date', 'N/A'),
            'link': item.get('link', 'N/A'),
            'symbols': item.get('symbols', [])
        }

        if is_positive and len(citations['positive']) < 10:
            positive_count += 1
            citations['positive'].append(citation)
        elif is_negative and len(citations['negative']) < 10:
            negative_count += 1
            citations['negative'].append(citation)

    net_sentiment = positive_count - negative_count
    return net_sentiment, citations

def main():
    api_token = (os.environ.get('EODHD_APIKEY') or
                 os.environ.get('EODHD_API_KEY') or
                 '690d7cdc3013f4.57364117')

    print("="*120)
    print("COMPREHENSIVE GOLD & APPLE ANALYSIS WITH IBD METHODOLOGY")
    print(f"Analysis Date: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("="*120)

    # Fetch news for sentiment analysis
    print("\nFetching latest market news for sentiment analysis...")
    news_items = fetch_news_with_citations(api_token, limit=50)
    print(f"Fetched {len(news_items)} news items")

    # Define assets
    assets = {
        'GOLD': {
            'symbols': ['XAUUSD.FOREX', 'GLD.US'],
            'name': 'Gold',
            'positive_keywords': ['gold rally', 'gold surge', 'rate cut', 'inflation', 'safe haven',
                                'uncertainty', 'risk', 'dollar weakness', 'fed dovish', 'geopolitical'],
            'negative_keywords': ['gold decline', 'rate hike', 'strong dollar', 'fed hawkish',
                                'risk appetite', 'dollar strength', 'equity rally']
        },
        'APPLE': {
            'symbols': ['AAPL.US'],
            'name': 'Apple Inc.',
            'positive_keywords': ['apple', 'aapl', 'iphone sales', 'services growth', 'innovation',
                                'earnings beat', 'revenue growth', 'ai integration', 'tech rally'],
            'negative_keywords': ['apple', 'aapl', 'supply chain', 'china risk', 'regulatory',
                                'antitrust', 'tech selloff', 'competition', 'weak demand']
        }
    }

    results = {}

    for asset_key, asset_info in assets.items():
        print(f"\n{'*'*120}")
        print(f"{asset_info['name'].upper()} ANALYSIS")
        print(f"{'*'*120}")

        # Try each symbol until we get data
        price_data = None
        used_symbol = None
        for symbol in asset_info['symbols']:
            data = fetch_price_data(symbol, api_token, days=252)
            if data:
                price_data = data
                used_symbol = symbol
                print(f"\nUsing symbol: {symbol}")
                break

        if not price_data:
            print(f"Failed to fetch data for {asset_info['name']}")
            continue

        # Latest price info
        latest = price_data[0]
        prev = price_data[1] if len(price_data) > 1 else latest

        daily_change = float(latest['close']) - float(prev['close'])
        daily_change_pct = (daily_change / float(prev['close'])) * 100

        print(f"\nLATEST PRICE DATA (as of {latest['date']}):")
        print(f"  Close:        ${float(latest['close']):.2f}")
        print(f"  Open:         ${float(latest['open']):.2f}")
        print(f"  High:         ${float(latest['high']):.2f}")
        print(f"  Low:          ${float(latest['low']):.2f}")
        print(f"  Volume:       {int(float(latest.get('volume', 0))):,}")
        print(f"  Daily Change: ${daily_change:+.2f} ({daily_change_pct:+.2f}%)")

        # Calculate IBD metrics
        metrics = calculate_ibd_metrics(price_data)
        ibd_score = calculate_ibd_composite_rating(metrics)
        ibd_rating = get_ibd_rating_label(ibd_score)

        print(f"\nIBD METHODOLOGY TECHNICAL ANALYSIS:")
        print(f"  IBD Composite Rating: {ibd_score}/100 ({ibd_rating})")
        print(f"\n  Moving Averages:")
        print(f"    10-Day MA:  ${metrics.get('ma_10', 'N/A'):<10} [{'ABOVE' if metrics.get('is_above_ma10') else 'BELOW'}]")
        print(f"    21-Day MA:  ${metrics.get('ma_21', 'N/A'):<10} [{'ABOVE' if metrics.get('is_above_ma21') else 'BELOW'}]")
        print(f"    50-Day MA:  ${metrics.get('ma_50', 'N/A'):<10} [{'ABOVE' if metrics.get('is_above_ma50') else 'BELOW'}]")
        print(f"    200-Day MA: ${metrics.get('ma_200', 'N/A'):<10} [{'ABOVE' if metrics.get('is_above_ma200') else 'BELOW'}]")

        print(f"\n  Price Position:")
        print(f"    52-Week High: ${metrics.get('high_52w', 'N/A')}")
        print(f"    52-Week Low:  ${metrics.get('low_52w', 'N/A')}")
        print(f"    % from High:  {metrics.get('pct_from_high', 'N/A')}%")
        print(f"    % from Low:   +{metrics.get('pct_from_low', 'N/A')}%")

        print(f"\n  Relative Strength:")
        print(f"    10-Day RS:    {metrics.get('rs_10d', 'N/A')}%")
        print(f"    21-Day RS:    {metrics.get('rs_21d', 'N/A')}%")
        print(f"    50-Day RS:    {metrics.get('rs_50d', 'N/A')}%")

        print(f"\n  Volume Analysis:")
        print(f"    50-Day Avg:   {metrics.get('avg_volume_50d', 0):,}")
        print(f"    Latest:       {metrics.get('latest_volume', 0):,}")
        print(f"    Volume Ratio: {metrics.get('volume_ratio', 'N/A')}x")

        print(f"\n  Key Levels:")
        print(f"    Resistance:   ${metrics.get('resistance', 'N/A')}")
        print(f"    Support:      ${metrics.get('support', 'N/A')}")
        print(f"    ATR %:        {metrics.get('atr_pct', 'N/A')}%")

        # News sentiment analysis
        print(f"\nNEWS SENTIMENT ANALYSIS:")
        net_sentiment, citations = analyze_news_sentiment(
            news_items,
            asset_info['positive_keywords'],
            asset_info['negative_keywords']
        )

        print(f"  Positive Factors: {len(citations['positive'])}")
        print(f"  Negative Factors: {len(citations['negative'])}")
        print(f"  Net Sentiment:    {net_sentiment:+d}")

        if citations['positive']:
            print(f"\n  TOP POSITIVE FACTORS (with citations):")
            for i, cite in enumerate(citations['positive'][:5], 1):
                print(f"    {i}. {cite['title'][:80]}")
                print(f"       Date: {cite['date']} | Link: {cite['link'][:60]}...")

        if citations['negative']:
            print(f"\n  TOP NEGATIVE FACTORS (with citations):")
            for i, cite in enumerate(citations['negative'][:5], 1):
                print(f"    {i}. {cite['title'][:80]}")
                print(f"       Date: {cite['date']} | Link: {cite['link'][:60]}...")

        # Price prediction
        prediction = predict_next_week(price_data, metrics, net_sentiment)

        print(f"\nNEXT WEEK PRICE PREDICTION (Nov 25 - Nov 29, 2025):")
        print(f"  Base Prediction:  ${prediction['base_prediction']}")
        print(f"  Upside Target:    ${prediction['upside_target']} (+{((prediction['upside_target']/metrics['current_price']-1)*100):.2f}%)")
        print(f"  Downside Target:  ${prediction['downside_target']} ({((prediction['downside_target']/metrics['current_price']-1)*100):.2f}%)")
        print(f"  Expected Range:   ±{prediction['expected_range_pct']}%")
        print(f"  Trend Bias:       {prediction['trend_bias_pct']:+.2f}%")
        print(f"  Sentiment Bias:   {prediction['sentiment_bias_pct']:+.2f}%")

        print(f"\n  Key Levels to Watch:")
        print(f"    Resistance:   ${prediction['resistance_level']:.2f}")
        print(f"    Support:      ${prediction['support_level']:.2f}")

        # Trading recommendation
        print(f"\nTRADING RECOMMENDATION:")
        if ibd_score >= 70 and net_sentiment > 5:
            recommendation = "STRONG BUY"
        elif ibd_score >= 60 and net_sentiment > 0:
            recommendation = "BUY"
        elif ibd_score >= 50:
            recommendation = "HOLD"
        elif ibd_score >= 40:
            recommendation = "WEAK HOLD"
        else:
            recommendation = "AVOID"

        print(f"  Rating: {recommendation}")
        print(f"  Rationale:")
        print(f"    - IBD Rating: {ibd_rating} ({ibd_score}/100)")
        print(f"    - News Sentiment: {net_sentiment:+d}")
        print(f"    - Trend: {'Bullish' if metrics.get('is_above_ma50') else 'Bearish'}")
        print(f"    - Position: {metrics.get('pct_from_high', 0):.1f}% from 52-week high")

        # Save results
        results[asset_key] = {
            'symbol': used_symbol,
            'latest_data': latest,
            'metrics': metrics,
            'ibd_score': ibd_score,
            'ibd_rating': ibd_rating,
            'sentiment': net_sentiment,
            'citations': citations,
            'prediction': prediction,
            'recommendation': recommendation
        }

    # Save to file
    output_file = 'c:\\Users\\micha\\github-mcp-server\\scripts\\analysis_results.json'
    with open(output_file, 'w', encoding='utf-8') as f:
        json.dump(results, f, indent=2, ensure_ascii=False)

    print(f"\n{'='*120}")
    print(f"Analysis complete. Results saved to: {output_file}")
    print(f"{'='*120}")

if __name__ == '__main__':
    main()
