#!/usr/bin/env python3
"""
Ultimate Autopilot Portfolio System - EOD Edition with Option Pricing
=====================================================================
Uses END OF DAY prices from EODHD (primary) and MarketStack (backup)
Calculates proper option P&L using Black-Scholes approximation

Features:
- EOD prices from EODHD + MarketStack (primary)
- Other APIs for cross-validation
- Black-Scholes option valuation
- Real option P&L (not just intrinsic value)

Assets Managed:
- Shanghai Gold Options (CNY) - 2 positions (P1 & P2)
- GLD Call Options (USD) - 47 contracts @ $420 strike
- China Stock Portfolio (CNY) - 11 stocks with CNY 2.3M margin
"""

import sys
import os
os.environ['PYTHONIOENCODING'] = 'utf-8'

import requests
import json
import math
import numpy as np
from datetime import datetime, timedelta
from typing import Dict, List, Tuple, Optional, Any
from dataclasses import dataclass
import pandas as pd
from scipy.stats import norm

# Import unified data client for China stocks and Shanghai gold
try:
    from unified_data_client import UnifiedDataClient
    import config
    HAS_UNIFIED_CLIENT = True
except ImportError:
    print("[WARN] unified_data_client not available, will use cached data")
    HAS_UNIFIED_CLIENT = False

# Import NowAPI and JisuAPI as primary sources for China data
try:
    from nowapi_integration import NowAPIClient, JisuAPIClient
    HAS_NOWAPI = True
    print("[INFO] NowAPI and JisuAPI available")
except ImportError:
    print("[WARN] nowapi_integration not available")
    HAS_NOWAPI = False

# Initialize NowAPI and JisuAPI clients (primary sources)
NOWAPI_CLIENT = None
JISUAPI_CLIENT = None
if HAS_NOWAPI:
    try:
        NOWAPI_CLIENT = NowAPIClient()
        JISUAPI_CLIENT = JisuAPIClient()
        print("[INFO] NowAPI and JisuAPI clients initialized")
    except Exception as e:
        print(f"[WARN] Failed to initialize NowAPI/JisuAPI: {e}")

# ============================================================================
# CONFIGURATION
# ============================================================================

EODHD_API_KEY = "690d7cdc3013f4.57364117"
MARKETSTACK_KEY = "cd80ef2d290fcd803d3422a5c5e6897b"
FINNHUB_KEY = "d4v95q1r01qnm7pqc4o0d4v95q1r01qnm7pqc4og"
FRED_KEY = "499d03dfed99d1b963ba896f72a351fd"
TUSHARE_TOKEN = "11e74c430352e33744476bf8dece65bac16e21631337dd70f6259e97"

# Initialize unified client once (shared across all calls)
UNIFIED_CLIENT = None
if HAS_UNIFIED_CLIENT:
    try:
        UNIFIED_CLIENT = UnifiedDataClient(tushare_token=TUSHARE_TOKEN)
        print("[INFO] Unified Data Client initialized")
    except Exception as e:
        print(f"[WARN] Failed to initialize unified client: {e}")
        UNIFIED_CLIENT = None

# Option Data APIs
MASSIVE_API_KEY = "u4dMOUGpagbWpFbiTRsTDtz1XTM4izNT"  # Massive.com (Polygon.io)

# ============================================================================
# PORTFOLIO CONFIGURATION
# ============================================================================

def calculate_dte(expiry_str: str) -> int:
    """Calculate Days To Expiry from expiry date string"""
    expiry = datetime.strptime(expiry_str, '%Y-%m-%d')
    return max(0, (expiry - datetime.now()).days)

PORTFOLIO = {
    'shanghai_gold': {
        'current_price': 953.42,
        'currency': 'CNY',
        'allocated_capital': 886330,
        'positions': [
            {'name': 'P1', 'strike': 960, 'expiry': '2026-04-25', 'units': 5, 'premium_paid': 248050, 'note': 'C960 call option'},
            {'name': 'P2', 'strike': 1000, 'expiry': '2026-04-25', 'units': 18, 'premium_paid': 638280, 'note': 'C1000 call option'}
        ]
    },
    'us_stocks': {
        'currency': 'USD',
        'positions': [
            {
                'ticker': 'GLD',
                'type': 'call_option',
                'contracts': 47,
                'strike': 420,
                'expiry': '2026-03-20',
                'total_cost': 54990.00,
                'multiplier': 100,
                'underlying': 'GLD'
            }
        ]
    },
    'china_stocks': {
        'currency': 'CNY',
        'margin_borrowed': 2300000,
        'positions': [
            {'name': '创业ETF', 'code': '159952', 'shares': 483100, 'cost': 903397},
            {'name': '创业板HX', 'code': '159957', 'shares': 429200, 'cost': 882435},
            {'name': '山金国际', 'code': '000975', 'shares': 25400, 'cost': 615798},
            {'name': '赤峰黄金', 'code': '600988', 'shares': 16300, 'cost': 514396},
            {'name': '海量数据', 'code': '603138', 'shares': 29700, 'cost': 498752},
            {'name': '恒生科技', 'code': '513130', 'shares': 577700, 'cost': 499133},
            {'name': '恒生科技', 'code': '513380', 'shares': 576400, 'cost': 499162},
            {'name': '科技恒生', 'code': '159740', 'shares': 625500, 'cost': 499149},
            {'name': '绿的谐波', 'code': '688017', 'shares': 2776, 'cost': 499219},
            {'name': '信创ETF', 'code': '562570', 'shares': 216000, 'cost': 335880},
            {'name': '数据产业', 'code': '516700', 'shares': 69000, 'cost': 78729}
        ]
    }
}

# ============================================================================
# BLACK-SCHOLES OPTION PRICING
# ============================================================================

def black_scholes_call(S: float, K: float, T: float, r: float, sigma: float) -> float:
    """
    Calculate call option price using Black-Scholes model

    Args:
        S: Current stock price
        K: Strike price
        T: Time to expiration (years)
        r: Risk-free rate
        sigma: Volatility (annualized)

    Returns:
        Call option price
    """
    if T <= 0:
        return max(0, S - K)

    d1 = (np.log(S / K) + (r + 0.5 * sigma ** 2) * T) / (sigma * np.sqrt(T))
    d2 = d1 - sigma * np.sqrt(T)

    call_price = S * norm.cdf(d1) - K * np.exp(-r * T) * norm.cdf(d2)
    return max(call_price, 0)

# ============================================================================
# EOD PRICE FETCHING
# ============================================================================

def get_eod_price_eodhd(symbol: str) -> Optional[Dict]:
    """Get EOD price from EODHD (primary source)"""
    try:
        url = f"https://eodhd.com/api/eod/{symbol}.US"
        params = {
            'api_token': EODHD_API_KEY,
            'fmt': 'json',
            'order': 'd',
            'limit': 1
        }

        response = requests.get(url, params=params, timeout=10)
        response.raise_for_status()

        data = response.json()
        if data and len(data) > 0:
            latest = data[0]
            return {
                'symbol': symbol,
                'close': float(latest['close']),
                'date': latest['date'],
                'volume': int(latest['volume']),
                'source': 'EODHD',
                'timestamp': datetime.now().isoformat()
            }
    except Exception as e:
        print(f"[EODHD Error] {e}")
        return None

def get_eod_price_marketstack(symbol: str) -> Optional[Dict]:
    """Get EOD price from MarketStack (backup)"""
    try:
        url = "http://api.marketstack.com/v1/eod/latest"
        params = {
            'access_key': MARKETSTACK_KEY,
            'symbols': symbol
        }

        response = requests.get(url, params=params, timeout=10)
        response.raise_for_status()

        data = response.json()
        if data.get('data') and len(data['data']) > 0:
            quote = data['data'][0]
            return {
                'symbol': symbol,
                'close': float(quote['close']),
                'date': quote['date'].split('T')[0],
                'volume': int(quote['volume']),
                'source': 'MarketStack',
                'timestamp': datetime.now().isoformat()
            }
    except Exception as e:
        print(f"[MarketStack Error] {e}")
        return None

def get_eod_price_finnhub(symbol: str) -> Optional[Dict]:
    """Get current price from Finnhub (cross-check)"""
    try:
        url = f"https://finnhub.io/api/v1/quote"
        params = {
            'symbol': symbol,
            'token': FINNHUB_KEY
        }

        response = requests.get(url, params=params, timeout=10)
        response.raise_for_status()

        data = response.json()
        if data and data.get('c'):
            return {
                'symbol': symbol,
                'close': float(data['c']),
                'date': datetime.now().strftime('%Y-%m-%d'),
                'volume': 0,
                'source': 'Finnhub',
                'timestamp': datetime.now().isoformat()
            }
    except Exception as e:
        print(f"[Finnhub Error] {e}")
        return None

def get_us_stock_eod(symbol: str, validate: bool = True) -> Dict:
    """
    Get US stock EOD price with multi-source validation
    Priority: EODHD (primary) → MarketStack (backup) → Finnhub (cross-check)
    """
    print(f"\n{'='*80}")
    print(f"FETCHING EOD PRICE: {symbol}")
    print(f"{'='*80}")

    result = {
        'symbol': symbol,
        'close': None,
        'source': None,
        'date': None,
        'sources_checked': [],
        'validation': {},
        'timestamp': datetime.now().isoformat()
    }

    # Try EODHD first (primary)
    eodhd_data = get_eod_price_eodhd(symbol)
    if eodhd_data:
        result['close'] = eodhd_data['close']
        result['source'] = 'EODHD'
        result['date'] = eodhd_data['date']
        result['sources_checked'].append('eodhd')
        result['validation']['eodhd'] = eodhd_data['close']
        print(f"[OK] EODHD: ${eodhd_data['close']:.2f} ({eodhd_data['date']})")
    else:
        result['sources_checked'].append('eodhd_failed')
        print(f"[FAIL] EODHD: Failed")

    # Try MarketStack (backup)
    marketstack_data = get_eod_price_marketstack(symbol)
    if marketstack_data:
        if not result['close']:
            result['close'] = marketstack_data['close']
            result['source'] = 'MarketStack'
            result['date'] = marketstack_data['date']
        result['sources_checked'].append('marketstack')
        result['validation']['marketstack'] = marketstack_data['close']
        print(f"[OK] MarketStack: ${marketstack_data['close']:.2f} ({marketstack_data['date']})")
    else:
        result['sources_checked'].append('marketstack_failed')
        print(f"[FAIL] MarketStack: Failed")

    # Try Finnhub for cross-check
    if validate:
        finnhub_data = get_eod_price_finnhub(symbol)
        if finnhub_data:
            result['sources_checked'].append('finnhub')
            result['validation']['finnhub'] = finnhub_data['close']
            print(f"[OK] Finnhub (cross-check): ${finnhub_data['close']:.2f}")
        else:
            result['sources_checked'].append('finnhub_failed')

    # Calculate deviations
    if len(result['validation']) > 1:
        prices = list(result['validation'].values())
        avg_price = np.mean(prices)
        max_dev = max([abs(p - avg_price) / avg_price * 100 for p in prices])
        result['max_deviation'] = max_dev
        print(f"\n[INFO] Price Validation: Max deviation {max_dev:.2f}%")
        if max_dev > 2.0:
            print(f"[WARN] HIGH DEVIATION WARNING")

    if not result['close']:
        print(f"\n[FAIL] Failed to get EOD price for {symbol}")

    return result

# ============================================================================
# OPTION PRICE FETCHING
# ============================================================================

def get_option_price_massive(symbol: str, strike: float, expiry: str, option_type: str = 'call') -> Optional[float]:
    """
    Get option price from Massive.com API (formerly Polygon.io) - FREE TIER
    Uses EOD aggregates endpoint which works with free Options Basic plan

    Args:
        symbol: Underlying symbol (e.g., 'GLD')
        strike: Strike price (e.g., 420)
        expiry: Expiry date in YYYY-MM-DD format
        option_type: 'call' or 'put'

    Returns:
        Option price (last close price) or None
    """
    try:
        # Format the option ticker symbol (OCC format)
        # Format: O:{underlying}YYMMDDP/C{strike*1000}
        # Example: O:GLD260320C00420000 for GLD 420 Call 03/20/26
        expiry_date = datetime.strptime(expiry, '%Y-%m-%d')
        expiry_formatted = expiry_date.strftime('%y%m%d')

        # Strike price with 8 digits (multiply by 1000 and pad)
        strike_formatted = f"{int(strike * 1000):08d}"

        # Option type: C for call, P for put
        option_type_char = 'C' if option_type.lower() == 'call' else 'P'

        # Build the option ticker
        option_ticker = f"O:{symbol}{expiry_formatted}{option_type_char}{strike_formatted}"

        # Use the aggregates endpoint (works with free tier)
        url = f"https://api.polygon.io/v2/aggs/ticker/{option_ticker}/prev"
        params = {'apiKey': MASSIVE_API_KEY}

        response = requests.get(url, params=params, timeout=10)
        response.raise_for_status()

        data = response.json()

        # Parse the response
        if data.get('status') == 'OK' and 'results' in data:
            results = data['results']

            if len(results) > 0:
                result = results[0]
                # Get close price
                close_price = result.get('c')

                if close_price and close_price > 0:
                    print(f"[OK] Massive.com: ${float(close_price):.2f} (EOD)")
                    return float(close_price)

        print(f"[FAIL] Massive.com: No price data for {option_ticker}")
        return None

    except Exception as e:
        print(f"[FAIL] Massive.com: {str(e)[:80]}")
        return None


def get_option_price_multi_source(symbol: str, strike: float, expiry: str,
                                  option_type: str = 'call', user_price: float = None) -> Dict:
    """
    Get option price from multiple sources with validation

    Priority:
    1. Massive.com API (Polygon.io) - Primary automated source
    2. User-provided price - Cross-validation and fallback

    Args:
        user_price: User's actual market price (for validation)

    Returns:
        Dict with price, source, and validation data
    """
    print(f"\n{'='*80}")
    print(f"FETCHING OPTION PRICE: {symbol} {strike} {option_type.upper()} {expiry}")
    print(f"{'='*80}")

    result = {
        'symbol': symbol,
        'strike': strike,
        'expiry': expiry,
        'option_type': option_type,
        'price': None,
        'source': None,
        'sources_checked': [],
        'validation': {},
        'timestamp': datetime.now().isoformat()
    }

    # 1. Try Massive.com API (Polygon.io) - PRIMARY
    massive_price = get_option_price_massive(symbol, strike, expiry, option_type)
    if massive_price:
        result['price'] = massive_price
        result['source'] = 'MASSIVE'
        result['validation']['massive'] = massive_price
        result['sources_checked'].append('massive')
    else:
        result['sources_checked'].append('massive_failed')

    # 2. Use user price for validation or as fallback
    if user_price is not None:
        result['validation']['user'] = user_price
        result['sources_checked'].append('user')
        print(f"[OK] USER ACCOUNT: ${user_price:.2f} (CROSS-CHECK)")

        # If no price from API, use user price as fallback
        if not result['price']:
            result['price'] = user_price
            result['source'] = 'USER_ACCOUNT'
            print(f"[INFO] Using user price as fallback")

    # Calculate deviations if we have multiple sources
    if len(result['validation']) > 1:
        prices = list(result['validation'].values())
        avg_price = np.mean(prices)
        max_dev = max([abs(p - avg_price) / avg_price * 100 for p in prices])
        result['max_deviation'] = max_dev

        print(f"\n[INFO] Option Price Validation:")
        for source, price in result['validation'].items():
            deviation = abs(price - avg_price) / avg_price * 100 if avg_price > 0 else 0
            status = "[WARN]" if deviation > 5 else "[OK]"
            print(f"  {status} {source.upper()}: ${price:.2f} (deviation: {deviation:.2f}%)")

        if max_dev > 5.0:
            print(f"[WARN] HIGH DEVIATION: {max_dev:.2f}%")

    if not result['price']:
        print(f"\n[FAIL] Failed to get option price from any source")
    else:
        print(f"\n[FINAL] Using price: ${result['price']:.2f} from {result['source']}")

    return result

# ============================================================================
# OPTION VALUATION
# ============================================================================

def calculate_option_value(underlying_price: float, strike: float, expiry_date: str,
                          contracts: int, multiplier: int = 100, market_price: float = None) -> Dict:
    """
    Calculate option value using actual market price or Black-Scholes model

    Args:
        market_price: Actual market price per share (if available)
    """
    dte = calculate_dte(expiry_date)
    T = dte / 365.0  # Time to expiration in years

    # Use actual market price if provided, otherwise use Black-Scholes
    if market_price is not None:
        option_price_per_share = market_price
        print(f"[INFO] Using actual market price: ${market_price:.2f}")
    else:
        # Use realistic parameters for Black-Scholes
        r = 0.045  # 4.5% risk-free rate (current Treasury)
        sigma = 0.18  # 18% volatility for GLD (historically stable)

        # Calculate option price per share
        if T > 0:
            option_price_per_share = black_scholes_call(underlying_price, strike, T, r, sigma)
        else:
            option_price_per_share = max(0, underlying_price - strike)
        print(f"[INFO] Using Black-Scholes estimate: ${option_price_per_share:.2f}")

    # Total value
    total_value = option_price_per_share * contracts * multiplier

    # Intrinsic and time value
    intrinsic = max(0, underlying_price - strike) * contracts * multiplier
    time_value = total_value - intrinsic

    return {
        'underlying_price': underlying_price,
        'option_price_per_share': option_price_per_share,
        'total_value': total_value,
        'intrinsic_value': intrinsic,
        'time_value': time_value,
        'dte': dte,
        'strike': strike,
        'itm': underlying_price > strike,
        'market_price_used': market_price is not None
    }

# ============================================================================
# CHINA DATA FETCHING
# ============================================================================

def get_shanghai_gold_price() -> Dict:
    """
    Get Shanghai Gold futures price
    Priority: AKShare (primary) -> CACHED (fallback)
    Note: JisuAPI futures endpoint has SSL issues, skip for now
    """
    print("\n" + "="*80)
    print("FETCHING SHANGHAI GOLD PRICE")
    print("="*80)

    # Try Unified Client (AKShare) as primary for gold futures
    if UNIFIED_CLIENT:
        try:
            result = UNIFIED_CLIENT.get_china_futures_quote('AU0')
            if result and result.get('price'):
                print(f"[OK] Shanghai Gold: CNY{result['price']:.2f}/g")
                print(f"  Source: {result['source'].upper()}")
                return result
        except Exception as e:
            print(f"[WARN] AKShare gold failed: {e}")

    # Fallback to cached price
    print("[WARN] Gold futures fetch failed, using cached price")
    return {'price': 953.42, 'source': 'CACHED', 'timestamp': 'N/A'}


def get_yahoo_china_stock(code: str) -> Optional[Dict]:
    """
    Get China stock price from Yahoo Finance
    Format: CODE.SZ (Shenzhen) or CODE.SS (Shanghai)
    """
    try:
        import yfinance as yf
        
        # Determine exchange suffix
        # Shenzhen: starts with 0, 3, or 15x, 16x
        if code.startswith(('0', '3')) or code.startswith('15') or code.startswith('16'):
            ticker = f"{code}.SZ"
        else:
            ticker = f"{code}.SS"
        
        stock = yf.Ticker(ticker)
        hist = stock.history(period='5d')
        
        if not hist.empty:
            price = hist['Close'].iloc[-1]
            return {
                'price': float(price),
                'source': 'YAHOO',
                'timestamp': str(hist.index[-1])
            }
        return None
    except Exception as e:
        return None


def get_china_stock_price(code: str, name: str) -> Dict:
    """
    Get China stock price with multi-source fallback
    Priority: NowAPI (primary) -> JisuAPI (secondary) -> TuShare/AKShare (backup) -> Yahoo Finance (last resort)
    """

    # 1. Try NowAPI first (PRIMARY - fastest for most stocks)
    if NOWAPI_CLIENT:
        try:
            nowapi_code = NOWAPI_CLIENT.convert_our_code_to_nowapi(code)
            quotes = NOWAPI_CLIENT.get_realtime_quote([nowapi_code])

            if nowapi_code in quotes:
                quote = quotes[nowapi_code]
                price = quote.get('price', 0)
                if price > 0:
                    print(f"[OK] {name} ({code}): CNY{price:.3f} (Source: NOWAPI)")
                    return {
                        'price': price,
                        'source': 'NOWAPI',
                        'timestamp': quote.get('timestamp', datetime.now().isoformat()),
                        'change_pct': quote.get('change_pct', 0)
                    }
        except Exception as e:
            print(f"[NowAPI Error] {e}")

    # 2. Try JisuAPI second (SECONDARY - good for ETFs that NowAPI misses)
    if JISUAPI_CLIENT:
        try:
            quote = JISUAPI_CLIENT.get_stock_quote(code)
            if quote and quote.get('price', 0) > 0:
                price = quote['price']
                print(f"[OK] {name} ({code}): CNY{price:.3f} (Source: JISUAPI)")
                return {
                    'price': price,
                    'source': 'JISUAPI',
                    'timestamp': quote.get('timestamp', datetime.now().isoformat()),
                    'change_pct': quote.get('change_pct', 0)
                }
        except Exception as e:
            print(f"[JisuAPI Error] {e}")

    # 3. Try Yahoo Finance (good for ETFs) - BEFORE AKShare to avoid slow fetching
    yahoo_result = get_yahoo_china_stock(code)
    if yahoo_result and yahoo_result.get('price', 0) > 0:
        price = yahoo_result['price']
        print(f"[OK] {name} ({code}): CNY{price:.3f} (Source: YAHOO)")
        return yahoo_result

    # 4. Try Unified Client (TuShare/AKShare) as backup
    if UNIFIED_CLIENT:
        try:
            result = UNIFIED_CLIENT.get_china_stock_quote(code, validate=False)
            if result and result.get('price'):
                price = result['price']
                source = result['source'].upper()
                print(f"[OK] {name} ({code}): CNY{price:.3f} (Source: {source})")
                return {
                    'price': price,
                    'source': source,
                    'timestamp': result.get('timestamp', 'N/A')
                }
        except Exception as e:
            print(f"[UnifiedClient Error] {e}")

    # All sources failed
    print(f"[FAIL] {name} ({code}): All sources failed")
    return {'price': None, 'source': 'FAILED', 'timestamp': 'N/A'}

# ============================================================================
# PORTFOLIO VALUATION
# ============================================================================

def calculate_portfolio_value() -> Dict:
    """Calculate total portfolio value using EOD prices"""
    print("\n" + "="*80)
    print("PORTFOLIO VALUATION (EOD Edition)")
    print("="*80)
    print(f"Report Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("="*80)

    total_value_usd = 0
    total_cost_usd = 0
    components = {}

    # Exchange rate
    print("\n" + "-"*80)
    print("EXCHANGE RATE (USD/CNY)")
    print("-"*80)
    try:
        url = "https://api.stlouisfed.org/fred/series/observations"
        params = {
            'series_id': 'DEXCHUS',
            'api_key': FRED_KEY,
            'file_type': 'json',
            'limit': 1,
            'sort_order': 'desc'
        }
        response = requests.get(url, params=params, timeout=10)
        data = response.json()
        usdcny = float(data['observations'][0]['value'])
        print(f"[OK] USD/CNY: {usdcny:.4f} (Source: FRED)")
    except:
        usdcny = 7.0696
        print(f"[CACHED] USD/CNY: {usdcny:.4f}")

    # 1. Shanghai Gold Options
    print("\n" + "="*80)
    print("1. SHANGHAI GOLD OPTIONS")
    print("="*80)

    # Fetch live Shanghai Gold price
    shanghai_gold_data = get_shanghai_gold_price()
    shanghai_gold_price = shanghai_gold_data.get('price', 953.42)

    shanghai_gold_value_cny = 0
    for pos in PORTFOLIO['shanghai_gold']['positions']:
        intrinsic = max(0, shanghai_gold_price - pos['strike'])
        value = intrinsic * 1000 * pos['units']
        shanghai_gold_value_cny += value
        print(f"{pos['name']}: CNY{value:,.0f} (intrinsic)")

    shanghai_gold_value_usd = shanghai_gold_value_cny / usdcny
    shanghai_gold_cost_usd = PORTFOLIO['shanghai_gold']['allocated_capital'] / usdcny

    print(f"\nShanghai Gold Total:")
    print(f"  Value: ${shanghai_gold_value_usd:,.0f}")
    print(f"  Cost: ${shanghai_gold_cost_usd:,.0f}")
    print(f"  P&L: ${shanghai_gold_value_usd - shanghai_gold_cost_usd:,.0f}")

    total_value_usd += shanghai_gold_value_usd
    total_cost_usd += shanghai_gold_cost_usd

    components['shanghai_gold'] = {
        'value_usd': shanghai_gold_value_usd,
        'cost_usd': shanghai_gold_cost_usd,
        'pnl_usd': shanghai_gold_value_usd - shanghai_gold_cost_usd,
        'gold_price_cny': shanghai_gold_price,
        'gold_data': shanghai_gold_data
    }

    # 2. GLD Call Options (MULTI-SOURCE OPTION PRICING)
    print("\n" + "="*80)
    print("2. GLD CALL OPTIONS (Multi-Source Option Pricing)")
    print("="*80)

    gld_eod = get_us_stock_eod('GLD', validate=True)
    gld_position = PORTFOLIO['us_stocks']['positions'][0]

    if gld_eod and gld_eod['close']:
        gld_price = gld_eod['close']

        # Get option price from multiple sources with user's verified price
        # User confirmed: GLD Call 03/20/26 420 is trading at $9.57
        user_verified_price = 9.57

        option_price_data = get_option_price_multi_source(
            symbol='GLD',
            strike=gld_position['strike'],
            expiry=gld_position['expiry'],
            option_type='call',
            user_price=user_verified_price
        )

        # Use the validated option price
        actual_option_price = option_price_data.get('price', user_verified_price)

        option_val = calculate_option_value(
            underlying_price=gld_price,
            strike=gld_position['strike'],
            expiry_date=gld_position['expiry'],
            contracts=gld_position['contracts'],
            multiplier=gld_position['multiplier'],
            market_price=actual_option_price
        )

        gld_value = option_val['total_value']
        gld_pnl = gld_value - gld_position['total_cost']

        print(f"\nGLD EOD Price: ${gld_price:.2f}")
        print(f"  Source: {gld_eod['source']}")
        print(f"  Date: {gld_eod['date']}")
        print(f"\nOption Price Validation:")
        print(f"  Final Price: ${actual_option_price:.2f}")
        print(f"  Source: {option_price_data['source']}")
        if 'validation' in option_price_data and len(option_price_data['validation']) > 1:
            print(f"  Sources Validated: {len(option_price_data['validation'])}")
            if 'max_deviation' in option_price_data:
                print(f"  Max Deviation: {option_price_data['max_deviation']:.2f}%")
        print(f"\nOption Valuation:")
        print(f"  Strike: ${option_val['strike']:.2f}")
        print(f"  ITM: {'Yes' if option_val['itm'] else 'No'}")
        print(f"  Option Price per Share: ${option_val['option_price_per_share']:.2f}")
        print(f"  Intrinsic Value: ${option_val['intrinsic_value']:,.0f}")
        print(f"  Time Value: ${option_val['time_value']:,.0f}")
        print(f"  Total Value ({gld_position['contracts']} contracts): ${gld_value:,.0f}")
        print(f"  Cost Basis: ${gld_position['total_cost']:,.0f}")
        print(f"  P&L: ${gld_pnl:,.0f} ({gld_pnl/gld_position['total_cost']*100:.1f}%)")
        print(f"  DTE: {option_val['dte']} days")

        total_value_usd += gld_value
        total_cost_usd += gld_position['total_cost']

        components['gld_options'] = {
            'value_usd': gld_value,
            'cost_usd': gld_position['total_cost'],
            'pnl_usd': gld_pnl,
            'option_details': option_val,
            'eod_data': gld_eod
        }
    else:
        print("[FAIL] Failed to get GLD EOD price")
        components['gld_options'] = {
            'value_usd': 0,
            'cost_usd': gld_position['total_cost'],
            'pnl_usd': -gld_position['total_cost']
        }

    # 3. China Stocks (UPDATED WITH LIVE PRICES)
    print("\n" + "="*80)
    print("3. CHINA STOCK PORTFOLIO (Live EOD Data)")
    print("="*80)

    # Fetch live prices for all positions
    china_value_cny = 0
    china_cost_cny = 0
    china_positions_data = []

    for position in PORTFOLIO['china_stocks']['positions']:
        code = position['code']
        name = position['name']
        shares = position['shares']
        cost = position['cost']

        # Fetch live price
        price_data = get_china_stock_price(code, name)
        price = price_data.get('price')

        if price:
            market_value = price * shares
            position_pnl = market_value - cost
            position_return = (position_pnl / cost * 100) if cost > 0 else 0

            print(f"  Value: CNY{market_value:,.0f}, P&L: CNY{position_pnl:,.0f} ({position_return:+.1f}%)")

            china_value_cny += market_value
            china_cost_cny += cost

            china_positions_data.append({
                'code': code,
                'name': name,
                'shares': shares,
                'price': price,
                'market_value': market_value,
                'cost': cost,
                'pnl': position_pnl,
                'return_pct': position_return,
                'source': price_data.get('source'),
                'timestamp': price_data.get('timestamp')
            })
        else:
            print(f"  [WARN] Skipping due to price fetch failure")
            # Use cached value if fetch fails
            china_cost_cny += cost

    # If no prices were fetched, fall back to cached values
    if china_value_cny == 0:
        print("\n[WARN] No prices fetched, using cached portfolio value")
        china_value_cny = 3047987  # From previous run in CNY
        china_cost_cny = 3531295   # Cost basis in CNY

    # CORRECTED: Margin borrowed is in CNY, not USD
    china_margin_borrowed_cny = PORTFOLIO['china_stocks']['margin_borrowed']  # CNY 2,300,000

    # Convert to USD
    china_value_usd = china_value_cny / usdcny
    china_cost_usd = china_cost_cny / usdcny
    china_margin_borrowed_usd = china_margin_borrowed_cny / usdcny

    # Calculate equity correctly
    china_equity_usd = china_value_usd - china_margin_borrowed_usd
    china_equity_ratio = (china_equity_usd / china_value_usd * 100) if china_value_usd > 0 else 0

    china_pnl_usd = china_value_usd - china_cost_usd

    print(f"\nChina Stocks Summary:")
    print(f"  Market Value: CNY{china_value_cny:,.0f} (${china_value_usd:,.0f})")
    print(f"  Cost Basis: CNY{china_cost_cny:,.0f} (${china_cost_usd:,.0f})")
    print(f"  Margin Borrowed: CNY{china_margin_borrowed_cny:,.0f} (${china_margin_borrowed_usd:,.0f})")
    print(f"  Net Equity: CNY{china_value_cny - china_margin_borrowed_cny:,.0f} (${china_equity_usd:,.0f})")
    print(f"  Equity Ratio: {china_equity_ratio:.1f}%")
    print(f"  P&L: CNY{china_value_cny - china_cost_cny:,.0f} (${china_pnl_usd:,.0f})")

    total_value_usd += china_value_usd
    total_cost_usd += china_cost_usd

    components['china_stocks'] = {
        'value_usd': china_value_usd,
        'value_cny': china_value_cny,
        'cost_usd': china_cost_usd,
        'cost_cny': china_cost_cny,
        'pnl_usd': china_pnl_usd,
        'pnl_cny': china_value_cny - china_cost_cny,
        'margin_borrowed_cny': china_margin_borrowed_cny,
        'margin_borrowed_usd': china_margin_borrowed_usd,
        'equity_usd': china_equity_usd,
        'equity_ratio': china_equity_ratio,
        'positions': china_positions_data
    }

    # Total
    print("\n" + "="*80)
    print("TOTAL PORTFOLIO")
    print("="*80)

    total_pnl = total_value_usd - total_cost_usd
    total_return = (total_pnl / total_cost_usd * 100) if total_cost_usd > 0 else 0

    print(f"\nTotal Value: ${total_value_usd:,.0f}")
    print(f"Total Cost: ${total_cost_usd:,.0f}")
    print(f"Total P&L: ${total_pnl:,.0f} ({total_return:+.2f}%)")

    return {
        'total_value_usd': total_value_usd,
        'total_cost_usd': total_cost_usd,
        'total_pnl_usd': total_pnl,
        'total_return_pct': total_return,
        'components': components,
        'usdcny': usdcny,
        'timestamp': datetime.now().isoformat()
    }

# ============================================================================
# MAIN
# ============================================================================

def main():
    print("\n" + "="*80)
    print("ULTIMATE AUTOPILOT - EOD EDITION with BLACK-SCHOLES")
    print("="*80)
    print(f"Execution Time: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("="*80)

    portfolio_data = calculate_portfolio_value()

    # Save report
    timestamp = datetime.now().strftime('%Y%m%d_%H%M')
    filename = f"autopilot_eod_{timestamp}.json"

    with open(filename, 'w', encoding='utf-8') as f:
        json.dump(portfolio_data, f, indent=2, ensure_ascii=False)

    print(f"\n{'='*80}")
    print(f"Report saved to: {filename}")
    print(f"{'='*80}\n")

    # --- FSD Integration: run meta-factor analysis on portfolio components ---
    try:
        from fsd_modeling_engine import FSDPredictor

        predictor = FSDPredictor()
        fsd_summary = []

        for comp_key, comp in portfolio_data.get('components', {}).items():
            # Build minimal input for analysis
            positions = comp.get('positions', []) if isinstance(comp.get('positions', None), list) else []
            price = None
            if positions:
                price = positions[0].get('price') or positions[0].get('market_value')
            price = price or comp.get('value_usd') or 0.0

            data = {
                'price': price,
                'sma_20': price,
                'sma_50': price,
                'sma_200': price,
                'rsi': 50,
                'kalshi_sentiment': 0.5,
                'polymarket_sentiment': 0.5,
                'news_sentiment': 0.0,
                'reddit_sentiment': 0.0,
            }

            analysis = predictor.analyze_asset(data)
            fsd_summary.append((comp_key, analysis))

        # Write aggregated FSD portfolio summary
        summary_file = f"fsd_portfolio_summary_{timestamp}.md"
        with open(summary_file, 'w', encoding='utf-8') as sf:
            sf.write('# FSD Portfolio Summary\n')
            sf.write(f'Generated: {datetime.now().isoformat()}\n\n')

            for key, a in fsd_summary:
                sf.write(f'## {key}\n')
                sf.write(f"- Composite Score: {a['composite_score']:.3f}\n")
                sf.write(f"- Predicted Return: {a['predicted_return']*100:+.2f}%\n")
                sf.write(f"- Signal: {a['signal']}\n\n")

        print(f"FSD portfolio summary written to {summary_file}")

    except Exception as e:
        print(f"[WARN] FSD integration failed: {e}")

if __name__ == "__main__":
    main()
