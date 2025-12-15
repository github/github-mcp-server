#!/usr/bin/env python3
"""
REAL-TIME DATA-DRIVEN PRICE PREDICTOR
Incorporates latest macro and micro factors from EODHD API
"""

import pandas as pd
import numpy as np
from datetime import datetime, timedelta
import requests
import os
import warnings
import json
from scipy import stats

warnings.filterwarnings('ignore')

EODHD_API_KEY = os.environ.get('EODHD_APIKEY', '690d7cdc3013f4.57364117')


class RealTimeDataFetcher:
    """Fetch latest macro and micro data from EODHD"""

    def __init__(self, api_key):
        self.api_key = api_key
        self.base_url = "https://eodhd.com/api"

    def fetch_macro_indicators(self):
        """
        Fetch latest macro economic indicators
        - Fed Funds Rate
        - Inflation (CPI)
        - VIX (Market volatility)
        - USD Index
        - 10Y Treasury Yield
        """
        print("\n" + "="*80)
        print("FETCHING REAL-TIME MACRO INDICATORS")
        print("="*80)

        macro_data = {}

        # Fed Funds Rate (FEDFUNDS from FRED via EODHD)
        try:
            # Using economic indicators endpoint
            indicators = {
                'FED_FUNDS': 'FEDFUNDS.FRED',
                'CPI': 'CPIAUCSL.FRED',
                'UNEMPLOYMENT': 'UNRATE.FRED',
                'VIX': '^VIX.INDX',
                'USD_INDEX': 'DX-Y.FOREX',
                'TREASURY_10Y': '^TNX.INDX'
            }

            for name, symbol in indicators.items():
                try:
                    url = f"{self.base_url}/eod/{symbol}"
                    params = {
                        'api_token': self.api_key,
                        'fmt': 'json',
                        'period': 'd',
                        'order': 'd'
                    }
                    resp = requests.get(url, params=params, timeout=30)

                    if resp.status_code == 200:
                        data = resp.json()
                        if isinstance(data, list) and len(data) > 0:
                            latest = data[0]
                            macro_data[name] = {
                                'value': float(latest['close']),
                                'date': latest['date']
                            }
                            print(f"  {name}: {macro_data[name]['value']:.2f} (as of {macro_data[name]['date']})")
                except Exception as e:
                    print(f"  {name}: Failed to fetch - {e}")

        except Exception as e:
            print(f"  Error fetching macro data: {e}")

        # Calculate derived metrics
        if 'FED_FUNDS' in macro_data and 'CPI' in macro_data:
            real_rate = macro_data['FED_FUNDS']['value'] - (macro_data['CPI']['value'] / 100)
            macro_data['REAL_INTEREST_RATE'] = {
                'value': real_rate,
                'date': macro_data['FED_FUNDS']['date']
            }
            print(f"  REAL_INTEREST_RATE: {real_rate:.2f}%")

        return macro_data

    def fetch_apple_fundamentals(self):
        """
        Fetch Apple-specific fundamental data
        - Latest earnings
        - Revenue growth
        - P/E ratio
        - Analyst ratings
        """
        print("\n" + "="*80)
        print("FETCHING APPLE FUNDAMENTALS")
        print("="*80)

        fundamentals = {}

        try:
            # Fetch fundamentals
            url = f"{self.base_url}/fundamentals/AAPL.US"
            params = {
                'api_token': self.api_key,
                'fmt': 'json'
            }
            resp = requests.get(url, params=params, timeout=30)

            if resp.status_code == 200:
                data = resp.json()

                # Extract key metrics
                if 'Highlights' in data:
                    highlights = data['Highlights']
                    fundamentals['PE_Ratio'] = highlights.get('PERatio', None)
                    fundamentals['EPS'] = highlights.get('EarningsShare', None)
                    fundamentals['Dividend_Yield'] = highlights.get('DividendYield', None)
                    fundamentals['Market_Cap'] = highlights.get('MarketCapitalization', None)

                    print(f"  P/E Ratio: {fundamentals.get('PE_Ratio', 'N/A')}")
                    print(f"  EPS: ${fundamentals.get('EPS', 'N/A')}")
                    print(f"  Dividend Yield: {fundamentals.get('Dividend_Yield', 'N/A')}")

                # Get latest earnings
                if 'Earnings' in data and 'History' in data['Earnings']:
                    earnings_history = data['Earnings']['History']
                    if earnings_history:
                        latest_earnings = list(earnings_history.values())[0]
                        fundamentals['Latest_EPS_Actual'] = latest_earnings.get('epsActual', None)
                        fundamentals['Latest_EPS_Estimate'] = latest_earnings.get('epsEstimate', None)

                        if fundamentals['Latest_EPS_Actual'] and fundamentals['Latest_EPS_Estimate']:
                            surprise = ((fundamentals['Latest_EPS_Actual'] - fundamentals['Latest_EPS_Estimate']) /
                                      fundamentals['Latest_EPS_Estimate'] * 100)
                            fundamentals['Earnings_Surprise_Pct'] = surprise
                            print(f"  Latest Earnings Surprise: {surprise:+.1f}%")

                # Analyst ratings
                if 'AnalystRatings' in data:
                    ratings = data['AnalystRatings']
                    if 'Rating' in ratings:
                        fundamentals['Analyst_Rating'] = ratings['Rating']
                        fundamentals['Target_Price'] = ratings.get('TargetPrice', None)
                        print(f"  Analyst Rating: {fundamentals.get('Analyst_Rating', 'N/A')}")
                        print(f"  Target Price: ${fundamentals.get('Target_Price', 'N/A')}")

        except Exception as e:
            print(f"  Error fetching Apple fundamentals: {e}")

        return fundamentals

    def fetch_gold_factors(self):
        """
        Fetch Gold-specific factors
        - USD strength
        - Real yields
        - Central bank purchases
        - Geopolitical risk (VIX proxy)
        """
        print("\n" + "="*80)
        print("FETCHING GOLD MARKET FACTORS")
        print("="*80)

        factors = {}

        try:
            # USD Index
            url = f"{self.base_url}/eod/DX-Y.FOREX"
            params = {
                'api_token': self.api_key,
                'fmt': 'json',
                'period': 'd',
                'order': 'd'
            }
            resp = requests.get(url, params=params, timeout=30)

            if resp.status_code == 200:
                data = resp.json()
                if isinstance(data, list) and len(data) >= 20:
                    current_usd = float(data[0]['close'])
                    usd_20d_ago = float(data[19]['close'])
                    usd_change = ((current_usd - usd_20d_ago) / usd_20d_ago) * 100

                    factors['USD_Index'] = current_usd
                    factors['USD_Change_20d'] = usd_change
                    print(f"  USD Index: {current_usd:.2f}")
                    print(f"  USD 20-Day Change: {usd_change:+.2f}%")

            # Silver/Gold Ratio (diversification indicator)
            silver_data = requests.get(
                f"{self.base_url}/eod/XAG.FOREX",
                params={'api_token': self.api_key, 'fmt': 'json', 'period': 'd', 'order': 'd'},
                timeout=30
            )
            gold_data = requests.get(
                f"{self.base_url}/eod/XAUUSD.FOREX",
                params={'api_token': self.api_key, 'fmt': 'json', 'period': 'd', 'order': 'd'},
                timeout=30
            )

            if silver_data.status_code == 200 and gold_data.status_code == 200:
                silver_price = float(silver_data.json()[0]['close'])
                gold_price = float(gold_data.json()[0]['close'])
                gold_silver_ratio = gold_price / silver_price
                factors['Gold_Silver_Ratio'] = gold_silver_ratio
                print(f"  Gold/Silver Ratio: {gold_silver_ratio:.1f}")

        except Exception as e:
            print(f"  Error fetching gold factors: {e}")

        return factors

    def fetch_market_sentiment(self):
        """
        Fetch overall market sentiment indicators
        - VIX (Fear index)
        - Put/Call ratio
        - Market breadth
        """
        print("\n" + "="*80)
        print("FETCHING MARKET SENTIMENT")
        print("="*80)

        sentiment = {}

        try:
            # VIX
            url = f"{self.base_url}/eod/^VIX.INDX"
            params = {
                'api_token': self.api_key,
                'fmt': 'json',
                'period': 'd',
                'order': 'd'
            }
            resp = requests.get(url, params=params, timeout=30)

            if resp.status_code == 200:
                data = resp.json()
                if isinstance(data, list) and len(data) > 0:
                    vix = float(data[0]['close'])
                    sentiment['VIX'] = vix

                    # Interpret VIX
                    if vix < 15:
                        vix_level = "Low (Complacency)"
                    elif vix < 20:
                        vix_level = "Normal"
                    elif vix < 30:
                        vix_level = "Elevated (Caution)"
                    else:
                        vix_level = "High (Fear)"

                    print(f"  VIX: {vix:.2f} ({vix_level})")

            # S&P 500 trend
            url = f"{self.base_url}/eod/^GSPC.INDX"
            params = {
                'api_token': self.api_key,
                'fmt': 'json',
                'period': 'd',
                'order': 'd'
            }
            resp = requests.get(url, params=params, timeout=30)

            if resp.status_code == 200:
                data = resp.json()
                if isinstance(data, list) and len(data) >= 20:
                    current_sp = float(data[0]['close'])
                    sp_20d_ago = float(data[19]['close'])
                    sp_change = ((current_sp - sp_20d_ago) / sp_20d_ago) * 100

                    sentiment['SP500_Index'] = current_sp
                    sentiment['SP500_Change_20d'] = sp_change
                    print(f"  S&P 500: {current_sp:.2f}")
                    print(f"  S&P 500 20-Day Change: {sp_change:+.2f}%")

        except Exception as e:
            print(f"  Error fetching sentiment: {e}")

        return sentiment


class DataDrivenPriors:
    """
    Create informed priors based on real-time data
    """

    @staticmethod
    def calculate_gold_priors(macro_data, gold_factors, sentiment):
        """
        Calculate Gold priors from real macro/market data
        """
        print("\n" + "="*80)
        print("CALCULATING DATA-DRIVEN GOLD PRIORS")
        print("="*80)

        bullish_score = 0
        factors = {}

        # Fed rate trajectory (lower rates = bullish for gold)
        if 'FED_FUNDS' in macro_data:
            fed_rate = macro_data['FED_FUNDS']['value']
            # Below 5% and likely to cut = bullish
            if fed_rate < 5.0:
                factors['fed_rate_supportive'] = True
                bullish_score += 1
                print("  + Fed rate supportive (< 5%)")
            else:
                factors['fed_rate_supportive'] = False
                print("  - Fed rate restrictive (>= 5%)")

        # Real interest rates (negative = very bullish for gold)
        if 'REAL_INTEREST_RATE' in macro_data:
            real_rate = macro_data['REAL_INTEREST_RATE']['value']
            if real_rate < 1.0:
                factors['real_rates_low'] = True
                bullish_score += 1
                print("  + Real rates low (< 1%)")
            else:
                factors['real_rates_low'] = False
                print("  - Real rates elevated (>= 1%)")

        # USD weakness (weak USD = bullish for gold)
        if 'USD_Change_20d' in gold_factors:
            usd_change = gold_factors['USD_Change_20d']
            if usd_change < 0:
                factors['usd_weakening'] = True
                bullish_score += 1
                print(f"  + USD weakening ({usd_change:+.2f}%)")
            else:
                factors['usd_weakening'] = False
                print(f"  - USD strengthening ({usd_change:+.2f}%)")

        # Market fear (high VIX = bullish for gold safe haven)
        if 'VIX' in sentiment:
            vix = sentiment['VIX']
            if vix > 20:
                factors['high_volatility'] = True
                bullish_score += 1
                print(f"  + High volatility/fear (VIX {vix:.2f})")
            else:
                factors['high_volatility'] = False
                print(f"  - Low volatility (VIX {vix:.2f})")

        # Stock market weakness (falling stocks = gold bid)
        if 'SP500_Change_20d' in sentiment:
            sp_change = sentiment['SP500_Change_20d']
            if sp_change < 0:
                factors['equity_weakness'] = True
                bullish_score += 1
                print(f"  + Equity weakness (S&P {sp_change:+.2f}%)")
            else:
                factors['equity_weakness'] = False
                print(f"  - Equity strength (S&P {sp_change:+.2f}%)")

        # Gold/Silver ratio (high ratio = gold relatively expensive but safe)
        if 'Gold_Silver_Ratio' in gold_factors:
            ratio = gold_factors['Gold_Silver_Ratio']
            # Ratio > 80 suggests gold bid for safety
            if ratio > 80:
                factors['gold_premium'] = True
                bullish_score += 1
                print(f"  + Gold premium (ratio {ratio:.1f} > 80)")
            else:
                factors['gold_premium'] = False
                print(f"  - Normal gold/silver (ratio {ratio:.1f})")

        # Calculate prior mean return based on bullish score
        # 0-1: -0.1%, 2-3: 0%, 4-5: +0.1%, 6: +0.2%
        if bullish_score <= 1:
            prior_return = -0.001
        elif bullish_score <= 3:
            prior_return = 0.0
        elif bullish_score <= 5:
            prior_return = 0.001
        else:
            prior_return = 0.002

        # Prior volatility based on VIX
        vix = sentiment.get('VIX', 20)
        prior_volatility = 0.015 * (vix / 20)  # Scale by VIX

        print(f"\n  Bullish Score: {bullish_score}/6")
        print(f"  Prior Daily Return: {prior_return*100:+.2f}%")
        print(f"  Prior Volatility: {prior_volatility*100:.2f}%")

        return {
            'name': 'Gold (XAU/USD)',
            'factors': factors,
            'bullish_score': bullish_score,
            'prior_mean_return': prior_return,
            'prior_volatility': prior_volatility,
            'confidence': 0.7,
            'data_sources': {
                'macro': list(macro_data.keys()),
                'gold_specific': list(gold_factors.keys()),
                'sentiment': list(sentiment.keys())
            }
        }

    @staticmethod
    def calculate_apple_priors(fundamentals, macro_data, sentiment):
        """
        Calculate Apple priors from real fundamental/market data
        """
        print("\n" + "="*80)
        print("CALCULATING DATA-DRIVEN APPLE PRIORS")
        print("="*80)

        bullish_score = 0
        factors = {}

        # Earnings surprise (positive = bullish)
        if 'Earnings_Surprise_Pct' in fundamentals:
            surprise = fundamentals['Earnings_Surprise_Pct']
            if surprise > 0:
                factors['positive_earnings_surprise'] = True
                bullish_score += 1
                print(f"  + Positive earnings surprise ({surprise:+.1f}%)")
            else:
                factors['positive_earnings_surprise'] = False
                print(f"  - Earnings miss ({surprise:+.1f}%)")

        # P/E ratio (reasonable valuation = bullish)
        if 'PE_Ratio' in fundamentals and fundamentals['PE_Ratio']:
            pe = fundamentals['PE_Ratio']
            # Tech sector average ~25-30
            if 20 < pe < 35:
                factors['reasonable_valuation'] = True
                bullish_score += 1
                print(f"  + Reasonable valuation (P/E {pe:.1f})")
            else:
                factors['reasonable_valuation'] = False
                print(f"  - Stretched valuation (P/E {pe:.1f})")

        # Analyst rating
        if 'Target_Price' in fundamentals and fundamentals['Target_Price']:
            # We'd need current price to calculate upside
            # For now, check if target exists and is high
            factors['analyst_support'] = True
            bullish_score += 1
            print(f"  + Analyst target: ${fundamentals['Target_Price']}")

        # Tech sector momentum (S&P rising = bullish for AAPL)
        if 'SP500_Change_20d' in sentiment:
            sp_change = sentiment['SP500_Change_20d']
            if sp_change > 0:
                factors['sector_momentum'] = True
                bullish_score += 1
                print(f"  + Sector momentum (S&P {sp_change:+.2f}%)")
            else:
                factors['sector_momentum'] = False
                print(f"  - Sector weakness (S&P {sp_change:+.2f}%)")

        # Low volatility environment (low VIX = risk-on for stocks)
        if 'VIX' in sentiment:
            vix = sentiment['VIX']
            if vix < 20:
                factors['low_volatility'] = True
                bullish_score += 1
                print(f"  + Low volatility (VIX {vix:.2f})")
            else:
                factors['low_volatility'] = False
                print(f"  - High volatility (VIX {vix:.2f})")

        # Fed policy (easing = bullish for growth stocks)
        if 'FED_FUNDS' in macro_data:
            fed_rate = macro_data['FED_FUNDS']['value']
            if fed_rate < 5.0:
                factors['accommodative_policy'] = True
                bullish_score += 1
                print(f"  + Accommodative Fed (rate {fed_rate:.2f}%)")
            else:
                factors['accommodative_policy'] = False
                print(f"  - Restrictive Fed (rate {fed_rate:.2f}%)")

        # Calculate prior return
        if bullish_score <= 1:
            prior_return = -0.001
        elif bullish_score <= 3:
            prior_return = 0.0
        elif bullish_score <= 5:
            prior_return = 0.0005
        else:
            prior_return = 0.001

        # Prior volatility (tech stocks more volatile)
        vix = sentiment.get('VIX', 20)
        prior_volatility = 0.02 * (vix / 20)

        print(f"\n  Bullish Score: {bullish_score}/6")
        print(f"  Prior Daily Return: {prior_return*100:+.2f}%")
        print(f"  Prior Volatility: {prior_volatility*100:.2f}%")

        return {
            'name': 'Apple Inc. (AAPL)',
            'factors': factors,
            'bullish_score': bullish_score,
            'prior_mean_return': prior_return,
            'prior_volatility': prior_volatility,
            'confidence': 0.6,
            'data_sources': {
                'fundamentals': list(fundamentals.keys()),
                'macro': list(macro_data.keys()),
                'sentiment': list(sentiment.keys())
            }
        }


def main():
    print("="*80)
    print("REAL-TIME DATA-DRIVEN PRICE PREDICTION")
    print(f"Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("="*80)

    # Initialize data fetcher
    fetcher = RealTimeDataFetcher(EODHD_API_KEY)

    # Fetch all real-time data
    print("\nFETCHING REAL-TIME MARKET DATA...")
    macro_data = fetcher.fetch_macro_indicators()
    apple_fundamentals = fetcher.fetch_apple_fundamentals()
    gold_factors = fetcher.fetch_gold_factors()
    sentiment = fetcher.fetch_market_sentiment()

    # Calculate data-driven priors
    gold_priors = DataDrivenPriors.calculate_gold_priors(macro_data, gold_factors, sentiment)
    apple_priors = DataDrivenPriors.calculate_apple_priors(apple_fundamentals, macro_data, sentiment)

    # Save to file
    output = {
        'timestamp': datetime.now().isoformat(),
        'data_fetched': {
            'macro_indicators': macro_data,
            'apple_fundamentals': apple_fundamentals,
            'gold_factors': gold_factors,
            'market_sentiment': sentiment
        },
        'calculated_priors': {
            'gold': gold_priors,
            'apple': apple_priors
        }
    }

    output_file = 'real_time_market_data.json'
    with open(output_file, 'w') as f:
        json.dump(output, f, indent=2, default=str)

    print(f"\n{'='*80}")
    print(f"SUCCESS: Real-time market data saved to: {output_file}")
    print(f"{'='*80}")

    print(f"\n{'='*80}")
    print("DATA-DRIVEN PRIORS SUMMARY")
    print(f"{'='*80}\n")

    print(f"Gold (XAU/USD):")
    print(f"  Bullish Score: {gold_priors['bullish_score']}/6")
    print(f"  Expected Daily Return: {gold_priors['prior_mean_return']*100:+.3f}%")
    print(f"  Volatility: {gold_priors['prior_volatility']*100:.3f}%")
    print(f"  Signal: {'BULLISH' if gold_priors['bullish_score'] >= 4 else 'BEARISH' if gold_priors['bullish_score'] <= 2 else 'NEUTRAL'}")

    print(f"\nApple (AAPL):")
    print(f"  Bullish Score: {apple_priors['bullish_score']}/6")
    print(f"  Expected Daily Return: {apple_priors['prior_mean_return']*100:+.3f}%")
    print(f"  Volatility: {apple_priors['prior_volatility']*100:.3f}%")
    print(f"  Signal: {'BULLISH' if apple_priors['bullish_score'] >= 4 else 'BEARISH' if apple_priors['bullish_score'] <= 2 else 'NEUTRAL'}")

    return output


if __name__ == '__main__':
    results = main()
