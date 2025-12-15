#!/usr/bin/env python3
"""Get latest prices and historical data for Gold and Apple with IBD analysis."""
import requests
import json
import os
from datetime import datetime, timedelta

def fetch_price_data(symbol, api_token, days=100):
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
        print(f"Error fetching {symbol}: {e}")
        return []

def calculate_ibd_metrics(price_data):
    """Calculate IBD methodology metrics."""
    if not price_data or len(price_data) < 50:
        return {}

    latest = price_data[0]
    current_price = float(latest['close'])

    # Get prices for different periods
    prices = [float(d['close']) for d in price_data]
    volumes = [float(d.get('volume', 0)) for d in price_data if d.get('volume')]

    # Calculate moving averages
    ma_10 = sum(prices[:10]) / 10 if len(prices) >= 10 else None
    ma_21 = sum(prices[:21]) / 21 if len(prices) >= 21 else None
    ma_50 = sum(prices[:50]) / 50 if len(prices) >= 50 else None

    # Calculate average volume
    avg_vol_50 = sum(volumes[:min(50, len(volumes))]) / min(50, len(volumes)) if volumes else 0
    latest_vol = float(latest.get('volume', 0))

    # Calculate % from highs
    high_52w = max(prices[:min(252, len(prices))])
    pct_from_high = ((current_price - high_52w) / high_52w) * 100

    # Calculate relative strength (simplified - last 50 days performance)
    if len(prices) >= 50:
        price_50d_ago = prices[49]
        rs_50d = ((current_price - price_50d_ago) / price_50d_ago) * 100
    else:
        rs_50d = None

    # Price action analysis
    is_above_ma10 = current_price > ma_10 if ma_10 else None
    is_above_ma21 = current_price > ma_21 if ma_21 else None
    is_above_ma50 = current_price > ma_50 if ma_50 else None

    # Volume analysis
    vol_ratio = (latest_vol / avg_vol_50) if avg_vol_50 > 0 else 0

    # Support and resistance (simple approach)
    recent_highs = [float(d['high']) for d in price_data[:20]]
    recent_lows = [float(d['low']) for d in price_data[:20]]
    resistance = max(recent_highs) if recent_highs else current_price
    support = min(recent_lows) if recent_lows else current_price

    return {
        'current_price': current_price,
        'ma_10': round(ma_10, 2) if ma_10 else None,
        'ma_21': round(ma_21, 2) if ma_21 else None,
        'ma_50': round(ma_50, 2) if ma_50 else None,
        'high_52w': round(high_52w, 2),
        'pct_from_high': round(pct_from_high, 2),
        'rs_50d': round(rs_50d, 2) if rs_50d else None,
        'is_above_ma10': is_above_ma10,
        'is_above_ma21': is_above_ma21,
        'is_above_ma50': is_above_ma50,
        'avg_volume_50d': int(avg_vol_50),
        'latest_volume': int(latest_vol),
        'volume_ratio': round(vol_ratio, 2),
        'resistance': round(resistance, 2),
        'support': round(support, 2),
        'latest_date': latest['date']
    }

def ibd_rating(metrics):
    """Provide IBD-style rating."""
    score = 0

    # Trend following (30 points)
    if metrics.get('is_above_ma10'): score += 10
    if metrics.get('is_above_ma21'): score += 10
    if metrics.get('is_above_ma50'): score += 10

    # Relative strength (30 points)
    rs = metrics.get('rs_50d', 0)
    if rs and rs > 20: score += 30
    elif rs and rs > 10: score += 20
    elif rs and rs > 0: score += 10

    # Position from high (20 points)
    pct_high = metrics.get('pct_from_high', -100)
    if pct_high > -5: score += 20
    elif pct_high > -10: score += 15
    elif pct_high > -15: score += 10
    elif pct_high > -25: score += 5

    # Volume (20 points)
    vol_ratio = metrics.get('volume_ratio', 0)
    if vol_ratio > 1.5: score += 20
    elif vol_ratio > 1.2: score += 15
    elif vol_ratio > 1.0: score += 10
    elif vol_ratio > 0.8: score += 5

    # Rating
    if score >= 80: return 'A+ (Strong Buy)'
    elif score >= 70: return 'A (Buy)'
    elif score >= 60: return 'B+ (Accumulate)'
    elif score >= 50: return 'B (Hold)'
    elif score >= 40: return 'C+ (Weak Hold)'
    elif score >= 30: return 'C (Caution)'
    else: return 'D (Avoid)'

def main():
    api_token = (os.environ.get('EODHD_APIKEY') or
                 os.environ.get('EODHD_API_KEY') or
                 '690d7cdc3013f4.57364117')

    symbols = {
        'GC.COMM': 'Gold Futures',
        'AAPL.US': 'Apple Inc.'
    }

    print("=" * 100)
    print("LATEST PRICES AND IBD ANALYSIS")
    print("=" * 100)

    for symbol, name in symbols.items():
        print(f"\n{'*' * 100}")
        print(f"{name} ({symbol})")
        print(f"{'*' * 100}")

        data = fetch_price_data(symbol, api_token, days=252)

        if not data:
            print(f"Failed to fetch data for {symbol}")
            continue

        latest = data[0]
        print(f"\nLatest Trading Data ({latest['date']}):")
        print(f"  Open:   ${float(latest['open']):.2f}")
        print(f"  High:   ${float(latest['high']):.2f}")
        print(f"  Low:    ${float(latest['low']):.2f}")
        print(f"  Close:  ${float(latest['close']):.2f}")
        print(f"  Volume: {int(float(latest.get('volume', 0))):,}")

        # Calculate IBD metrics
        metrics = calculate_ibd_metrics(data)

        print(f"\nIBD Methodology Analysis:")
        print(f"  10-Day MA:  ${metrics.get('ma_10', 'N/A')}")
        print(f"  21-Day MA:  ${metrics.get('ma_21', 'N/A')}")
        print(f"  50-Day MA:  ${metrics.get('ma_50', 'N/A')}")
        print(f"  52-Week High: ${metrics.get('high_52w', 'N/A')}")
        print(f"  % from High:  {metrics.get('pct_from_high', 'N/A')}%")
        print(f"  50-Day RS:    {metrics.get('rs_50d', 'N/A')}%")
        print(f"  Support:      ${metrics.get('support', 'N/A')}")
        print(f"  Resistance:   ${metrics.get('resistance', 'N/A')}")

        print(f"\nTrend Analysis:")
        print(f"  Above 10-MA: {'✓' if metrics.get('is_above_ma10') else '✗'}")
        print(f"  Above 21-MA: {'✓' if metrics.get('is_above_ma21') else '✗'}")
        print(f"  Above 50-MA: {'✓' if metrics.get('is_above_ma50') else '✗'}")

        print(f"\nVolume Analysis:")
        print(f"  50-Day Avg Volume: {metrics.get('avg_volume_50d', 0):,}")
        print(f"  Latest Volume:     {metrics.get('latest_volume', 0):,}")
        print(f"  Volume Ratio:      {metrics.get('volume_ratio', 'N/A')}x")

        rating = ibd_rating(metrics)
        print(f"\nIBD Rating: {rating}")

        # Save metrics for later use
        with open(f'c:\\Users\\micha\\github-mcp-server\\scripts\\{symbol.replace(".", "_")}_metrics.json', 'w') as f:
            json.dump({
                'symbol': symbol,
                'name': name,
                'latest_data': latest,
                'metrics': metrics,
                'rating': rating
            }, f, indent=2)

if __name__ == '__main__':
    main()
