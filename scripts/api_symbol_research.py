#!/usr/bin/env python3
"""
API SYMBOL RESEARCH - Find correct EODHD symbols for all critical factors

CRITICAL MISSING FACTORS:
1. Dollar Index (DXY) - #1 for gold
2. 10Y Treasury Yield - Real rates for gold
3. NASDAQ Composite - #1 for Apple
4. S&P 500 - Market sentiment
5. Apple Fundamentals - P/E, revenue, margins

This script tests all possible symbol variations to find working endpoints.
"""

import requests
import os
import json
from datetime import datetime, timedelta

API_KEY = os.environ.get('EODHD_APIKEY', '690d7cdc3013f4.57364117')
BASE_URL = "https://eodhd.com/api"


def test_symbol(symbol, endpoint_type='eod', description=''):
    """Test if a symbol works"""
    try:
        if endpoint_type == 'eod':
            url = f"{BASE_URL}/eod/{symbol}"
            params = {'api_token': API_KEY, 'fmt': 'json', 'period': 'd', 'order': 'd'}
        elif endpoint_type == 'real-time':
            url = f"{BASE_URL}/real-time/{symbol}"
            params = {'api_token': API_KEY, 'fmt': 'json'}
        elif endpoint_type == 'fundamentals':
            url = f"{BASE_URL}/fundamentals/{symbol}"
            params = {'api_token': API_KEY}
        else:
            return None

        resp = requests.get(url, params=params, timeout=10)

        if resp.status_code == 200:
            data = resp.json()

            # Check if data is valid
            if endpoint_type in ['eod', 'real-time']:
                if isinstance(data, list) and len(data) > 0:
                    return {
                        'symbol': symbol,
                        'status': 'SUCCESS',
                        'description': description,
                        'sample_value': data[0].get('close', 'N/A'),
                        'data_points': len(data) if isinstance(data, list) else 1
                    }
                elif isinstance(data, dict) and 'close' in data:
                    return {
                        'symbol': symbol,
                        'status': 'SUCCESS',
                        'description': description,
                        'sample_value': data.get('close', 'N/A'),
                        'data_points': 1
                    }
            elif endpoint_type == 'fundamentals':
                if isinstance(data, dict) and len(data) > 0:
                    return {
                        'symbol': symbol,
                        'status': 'SUCCESS',
                        'description': description,
                        'has_highlights': 'Highlights' in data,
                        'has_financials': 'Financials' in data,
                        'has_ratings': 'AnalystRatings' in data
                    }

            return {
                'symbol': symbol,
                'status': 'EMPTY',
                'description': description,
                'response': str(data)[:100]
            }
        else:
            return {
                'symbol': symbol,
                'status': f'HTTP_{resp.status_code}',
                'description': description
            }

    except Exception as e:
        return {
            'symbol': symbol,
            'status': 'ERROR',
            'description': description,
            'error': str(e)
        }


def research_dollar_index():
    """Research Dollar Index (DXY) symbols"""
    print("\n" + "="*80)
    print("RESEARCHING: DOLLAR INDEX (DXY)")
    print("="*80)
    print("CRITICAL: #1 inverse correlation with gold (80%)")
    print()

    # All possible DXY symbol variations
    candidates = [
        # NYBOT/ICE
        ('DX-Y.NYB', 'ICE Dollar Index Futures'),
        ('DXY.NYB', 'Dollar Index NYBOT'),
        ('DX.NYB', 'Dollar Index Short'),

        # Forex style
        ('DXY.FOREX', 'Dollar Index Forex'),
        ('USDX.FOREX', 'US Dollar Index Forex'),
        ('DXY.FX', 'Dollar Index FX'),

        # Index style
        ('DXY.INDX', 'Dollar Index'),
        ('^DXY.INDX', 'Dollar Index with caret'),
        ('DXY', 'Dollar Index plain'),
        ('^DXY', 'Dollar Index caret plain'),

        # Alternative formats
        ('USDOLLAR.INDX', 'US Dollar Index'),
        ('USD.INDX', 'USD Index'),
        ('USDU.INDX', 'USD Universe Index'),

        # ETF proxies
        ('UUP.US', 'Invesco DB USD Bullish ETF'),
        ('USDU.US', 'USD Bullish Fund'),

        # Currency pairs as proxy
        ('EURUSD.FOREX', 'EUR/USD (inverse proxy)'),
    ]

    results = []
    for symbol, desc in candidates:
        print(f"Testing: {symbol:20s} - {desc}")
        result = test_symbol(symbol, 'eod', desc)
        if result:
            results.append(result)
            if result['status'] == 'SUCCESS':
                print(f"  [OK] SUCCESS! Value: {result.get('sample_value', 'N/A')}")
            else:
                print(f"  [X] {result['status']}")
        else:
            print(f"  [X] Failed to test")

    return results


def research_treasury_yields():
    """Research 10Y Treasury Yield symbols"""
    print("\n" + "="*80)
    print("RESEARCHING: 10-YEAR TREASURY YIELD")
    print("="*80)
    print("CRITICAL: Real rates calculation for gold")
    print()

    candidates = [
        # Index symbols
        ('^TNX.INDX', '10Y Treasury Yield Index'),
        ('TNX.INDX', '10Y Yield without caret'),
        ('^TNX', '10Y Yield plain'),
        ('TNX', '10Y Yield short'),

        # Alternative formats
        ('US10Y.INDX', 'US 10Y Index'),
        ('^US10Y.INDX', 'US 10Y with caret'),
        ('US10YT.INDX', 'US 10Y Treasury'),

        # Bond symbols
        ('TY.COMM', '10Y Treasury Futures'),
        ('ZN.COMM', '10Y Note Futures'),

        # ETF proxies
        ('IEF.US', 'iShares 7-10Y Treasury ETF'),
        ('TLT.US', 'iShares 20+ Year Treasury ETF'),
        ('SHY.US', 'iShares 1-3Y Treasury ETF'),

        # FRED codes (if supported)
        ('DGS10.FRED', '10Y Treasury Constant Maturity'),
        ('GS10.FRED', '10 Year Treasury Rate'),
    ]

    results = []
    for symbol, desc in candidates:
        print(f"Testing: {symbol:20s} - {desc}")
        result = test_symbol(symbol, 'eod', desc)
        if result:
            results.append(result)
            if result['status'] == 'SUCCESS':
                print(f"  [OK] SUCCESS! Value: {result.get('sample_value', 'N/A')}")
            else:
                print(f"  [X] {result['status']}")

    return results


def research_nasdaq():
    """Research NASDAQ Composite symbols"""
    print("\n" + "="*80)
    print("RESEARCHING: NASDAQ COMPOSITE")
    print("="*80)
    print("CRITICAL: #1 correlation with Apple (high beta)")
    print()

    candidates = [
        # Official NASDAQ symbols
        ('^IXIC.INDX', 'NASDAQ Composite Index'),
        ('IXIC.INDX', 'NASDAQ without caret'),
        ('^IXIC', 'NASDAQ plain'),
        ('IXIC', 'NASDAQ short'),

        # Alternative names
        ('^COMP.INDX', 'NASDAQ COMP'),
        ('COMP.INDX', 'Composite Index'),
        ('COMPX.INDX', 'Composite X'),

        # NASDAQ 100
        ('^NDX.INDX', 'NASDAQ 100 Index'),
        ('NDX.INDX', 'NDX without caret'),
        ('^NDX', 'NDX plain'),

        # ETF proxies
        ('QQQ.US', 'Invesco QQQ NASDAQ 100 ETF'),
        ('ONEQ.US', 'Fidelity NASDAQ Composite ETF'),
        ('QQQM.US', 'Invesco NASDAQ 100 Mini ETF'),

        # Futures
        ('NQ.COMM', 'NASDAQ 100 Futures'),
    ]

    results = []
    for symbol, desc in candidates:
        print(f"Testing: {symbol:20s} - {desc}")
        result = test_symbol(symbol, 'eod', desc)
        if result:
            results.append(result)
            if result['status'] == 'SUCCESS':
                print(f"  [OK] SUCCESS! Value: {result.get('sample_value', 'N/A')}")
            else:
                print(f"  [X] {result['status']}")

    return results


def research_sp500():
    """Research S&P 500 symbols"""
    print("\n" + "="*80)
    print("RESEARCHING: S&P 500")
    print("="*80)
    print("IMPORTANT: Broad market sentiment indicator")
    print()

    candidates = [
        # Official S&P symbols
        ('^GSPC.INDX', 'S&P 500 Index'),
        ('GSPC.INDX', 'S&P without caret'),
        ('^GSPC', 'S&P plain'),
        ('GSPC', 'S&P short'),

        # Alternative formats
        ('^SPX.INDX', 'SPX Index'),
        ('SPX.INDX', 'SPX without caret'),
        ('^INX.INDX', 'INX Index'),
        ('INX.INDX', 'INX without caret'),

        # ETF proxies
        ('SPY.US', 'SPDR S&P 500 ETF'),
        ('VOO.US', 'Vanguard S&P 500 ETF'),
        ('IVV.US', 'iShares Core S&P 500 ETF'),

        # Futures
        ('ES.COMM', 'E-mini S&P 500 Futures'),
    ]

    results = []
    for symbol, desc in candidates:
        print(f"Testing: {symbol:20s} - {desc}")
        result = test_symbol(symbol, 'eod', desc)
        if result:
            results.append(result)
            if result['status'] == 'SUCCESS':
                print(f"  [OK] SUCCESS! Value: {result.get('sample_value', 'N/A')}")
            else:
                print(f"  [X] {result['status']}")

    return results


def research_fundamentals():
    """Research Apple fundamentals endpoint"""
    print("\n" + "="*80)
    print("RESEARCHING: APPLE FUNDAMENTALS")
    print("="*80)
    print("CRITICAL: P/E, revenue growth, margins for valuation")
    print()

    candidates = [
        ('AAPL.US', 'Apple Inc'),
        ('AAPL', 'Apple without exchange'),
        ('AAPL.NASDAQ', 'Apple NASDAQ'),
    ]

    results = []
    for symbol, desc in candidates:
        print(f"Testing: {symbol:20s} - {desc}")
        result = test_symbol(symbol, 'fundamentals', desc)
        if result:
            results.append(result)
            if result['status'] == 'SUCCESS':
                print(f"  [OK] SUCCESS!")
                print(f"     Highlights: {result.get('has_highlights', False)}")
                print(f"     Financials: {result.get('has_financials', False)}")
                print(f"     Ratings: {result.get('has_ratings', False)}")
            else:
                print(f"  [X] {result['status']}")

    return results


def research_additional_factors():
    """Research additional useful factors"""
    print("\n" + "="*80)
    print("RESEARCHING: ADDITIONAL FACTORS")
    print("="*80)
    print()

    print("\n--- CPI (Inflation) ---")
    cpi_candidates = [
        ('^CPI.INDX', 'CPI Index'),
        ('CPI.INDX', 'CPI without caret'),
        ('CPIAUCSL.FRED', 'CPI All Urban FRED'),
    ]

    cpi_results = []
    for symbol, desc in cpi_candidates:
        print(f"Testing: {symbol:20s} - {desc}")
        result = test_symbol(symbol, 'eod', desc)
        if result and result['status'] == 'SUCCESS':
            cpi_results.append(result)
            print(f"  [OK] SUCCESS! Value: {result.get('sample_value', 'N/A')}")
        else:
            print(f"  [X] Failed")

    print("\n--- Fed Funds Rate ---")
    fed_candidates = [
        ('FEDFUNDS.FRED', 'Federal Funds Rate'),
        ('EFFR.FRED', 'Effective Federal Funds Rate'),
        ('^IRX.INDX', '13 Week Treasury Bill'),
    ]

    fed_results = []
    for symbol, desc in fed_candidates:
        print(f"Testing: {symbol:20s} - {desc}")
        result = test_symbol(symbol, 'eod', desc)
        if result and result['status'] == 'SUCCESS':
            fed_results.append(result)
            print(f"  [OK] SUCCESS! Value: {result.get('sample_value', 'N/A')}")
        else:
            print(f"  [X] Failed")

    print("\n--- Gold Futures ---")
    gold_candidates = [
        ('GC.COMM', 'Gold Futures'),
        ('GOLD.COMM', 'Gold Commodity'),
        ('XAUUSD.FOREX', 'Gold Spot (current)'),
    ]

    gold_results = []
    for symbol, desc in gold_candidates:
        print(f"Testing: {symbol:20s} - {desc}")
        result = test_symbol(symbol, 'eod', desc)
        if result and result['status'] == 'SUCCESS':
            gold_results.append(result)
            print(f"  [OK] SUCCESS! Value: {result.get('sample_value', 'N/A')}")
        else:
            print(f"  [X] Failed")

    return {
        'cpi': cpi_results,
        'fed': fed_results,
        'gold': gold_results
    }


def main():
    """Main research function"""
    print("="*80)
    print("API SYMBOL RESEARCH FOR 100% FACTOR COVERAGE")
    print("="*80)
    print(f"Date: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print(f"API: EODHD")
    print(f"Goal: Find working symbols for all critical factors")
    print("="*80)

    all_results = {}

    # Research all critical factors
    all_results['dxy'] = research_dollar_index()
    all_results['treasury'] = research_treasury_yields()
    all_results['nasdaq'] = research_nasdaq()
    all_results['sp500'] = research_sp500()
    all_results['fundamentals'] = research_fundamentals()
    all_results['additional'] = research_additional_factors()

    # Summary
    print("\n" + "="*80)
    print("RESEARCH SUMMARY")
    print("="*80)

    def count_success(results):
        if isinstance(results, list):
            return sum(1 for r in results if r.get('status') == 'SUCCESS')
        return 0

    print("\nCRITICAL FACTORS:")
    print(f"  Dollar Index (DXY):     {count_success(all_results['dxy'])} working symbols")
    print(f"  10Y Treasury Yield:     {count_success(all_results['treasury'])} working symbols")
    print(f"  NASDAQ Composite:       {count_success(all_results['nasdaq'])} working symbols")
    print(f"  S&P 500:                {count_success(all_results['sp500'])} working symbols")
    print(f"  Apple Fundamentals:     {count_success(all_results['fundamentals'])} working symbols")

    print("\nADDITIONAL FACTORS:")
    print(f"  CPI (Inflation):        {count_success(all_results['additional']['cpi'])} working symbols")
    print(f"  Fed Funds Rate:         {count_success(all_results['additional']['fed'])} working symbols")
    print(f"  Gold Futures:           {count_success(all_results['additional']['gold'])} working symbols")

    # Find best symbols for each
    print("\n" + "="*80)
    print("RECOMMENDED SYMBOLS")
    print("="*80)

    recommendations = {}

    for factor, results in all_results.items():
        if factor == 'additional':
            continue
        if isinstance(results, list):
            successes = [r for r in results if r.get('status') == 'SUCCESS']
            if successes:
                best = successes[0]  # First success is usually best
                recommendations[factor] = {
                    'symbol': best['symbol'],
                    'description': best.get('description', ''),
                    'sample_value': best.get('sample_value', 'N/A')
                }
                print(f"\n{factor.upper()}:")
                print(f"  Symbol: {best['symbol']}")
                print(f"  Description: {best.get('description', '')}")
                if 'sample_value' in best:
                    print(f"  Current Value: {best['sample_value']}")
            else:
                print(f"\n{factor.upper()}: [X] NO WORKING SYMBOL FOUND")
                recommendations[factor] = None

    # Additional factors
    for subfactor, results in all_results['additional'].items():
        if results:
            best = results[0]
            recommendations[f'additional_{subfactor}'] = {
                'symbol': best['symbol'],
                'description': best.get('description', ''),
                'sample_value': best.get('sample_value', 'N/A')
            }
            print(f"\n{subfactor.upper()}:")
            print(f"  Symbol: {best['symbol']}")
            print(f"  Current Value: {best['sample_value']}")

    # Save results
    output = {
        'timestamp': datetime.now().isoformat(),
        'all_results': all_results,
        'recommendations': recommendations,
        'summary': {
            'dxy_found': count_success(all_results['dxy']) > 0,
            'treasury_found': count_success(all_results['treasury']) > 0,
            'nasdaq_found': count_success(all_results['nasdaq']) > 0,
            'sp500_found': count_success(all_results['sp500']) > 0,
            'fundamentals_found': count_success(all_results['fundamentals']) > 0
        }
    }

    with open('api_symbol_research_results.json', 'w') as f:
        json.dump(output, f, indent=2, default=str)

    print("\n" + "="*80)
    print("Results saved to: api_symbol_research_results.json")
    print("="*80)

    return recommendations


if __name__ == '__main__':
    main()
