#!/usr/bin/env python3
"""
Gold Price Data Collector and Analyzer for China Futures Market Strategy
Collects historical data and economic indicators from EODHD API
"""

import requests
import json
import pandas as pd
import numpy as np
from datetime import datetime, timedelta
import os

API_TOKEN = os.environ.get('EODHD_API_TOKEN', '690d7cdc3013f4.57364117')
BASE_URL = 'https://eodhd.com/api'

def fetch_gold_historical_data():
    """Fetch 10+ years of gold price history"""
    print("Fetching historical gold price data...")
    url = f"{BASE_URL}/eod/XAUUSD.FOREX"
    params = {
        'api_token': API_TOKEN,
        'fmt': 'json',
        'from': '2010-01-01',
        'to': '2025-11-16'
    }
    response = requests.get(url, params=params)
    data = response.json()
    df = pd.DataFrame(data)
    df['date'] = pd.to_datetime(df['date'])
    df.set_index('date', inplace=True)
    print(f"Fetched {len(df)} records from {df.index[0]} to {df.index[-1]}")
    print(f"Latest gold price: ${df['close'].iloc[-1]:.2f}")
    return df

def fetch_us_macro_indicators():
    """Fetch key US macroeconomic indicators that affect gold"""
    print("\nFetching US macroeconomic indicators...")
    indicators = {
        'inflation_consumer_prices_annual': 'CPI Inflation',
        'real_interest_rate': 'Real Interest Rate',
        'central_bank_interest_rate': 'Fed Funds Rate',
        'gdp_growth_annual': 'GDP Growth',
        'unemployment_rate': 'Unemployment Rate',
        'government_debt_to_gdp': 'Debt to GDP'
    }

    macro_data = {}
    for indicator_code, indicator_name in indicators.items():
        url = f"{BASE_URL}/macro-indicator/USA"
        params = {
            'api_token': API_TOKEN,
            'fmt': 'json',
            'indicator': indicator_code
        }
        try:
            response = requests.get(url, params=params)
            data = response.json()
            if data:
                macro_data[indicator_name] = data
                print(f"  {indicator_name}: Latest = {data[0]['Value']:.2f}% ({data[0]['Date']})")
        except Exception as e:
            print(f"  Error fetching {indicator_name}: {e}")

    return macro_data

def fetch_china_macro_indicators():
    """Fetch China macroeconomic indicators"""
    print("\nFetching China macroeconomic indicators...")
    indicators = {
        'inflation_consumer_prices_annual': 'China CPI',
        'gdp_growth_annual': 'China GDP Growth',
        'central_bank_interest_rate': 'China Interest Rate'
    }

    macro_data = {}
    for indicator_code, indicator_name in indicators.items():
        url = f"{BASE_URL}/macro-indicator/CHN"
        params = {
            'api_token': API_TOKEN,
            'fmt': 'json',
            'indicator': indicator_code
        }
        try:
            response = requests.get(url, params=params)
            data = response.json()
            if data:
                macro_data[indicator_name] = data
                print(f"  {indicator_name}: Latest = {data[0]['Value']:.2f}% ({data[0]['Date']})")
        except Exception as e:
            print(f"  Error fetching {indicator_name}: {e}")

    return macro_data

def fetch_upcoming_economic_events():
    """Fetch economic events calendar for November 2025"""
    print("\nFetching November 2025 economic events...")
    url = f"{BASE_URL}/economic-events"
    params = {
        'api_token': API_TOKEN,
        'fmt': 'json',
        'from': '2025-11-01',
        'to': '2025-11-30'
    }
    response = requests.get(url, params=params)
    events = response.json()

    # Filter for high-impact events (US and China)
    important_events = []
    key_countries = ['US', 'CN', 'EU']
    key_event_types = ['Interest Rate', 'Inflation', 'GDP', 'Employment', 'PMI',
                       'Fed', 'FOMC', 'Non-Farm', 'CPI', 'PPI']

    for event in events:
        if event.get('country') in key_countries:
            for key_type in key_event_types:
                if key_type.lower() in event.get('type', '').lower():
                    important_events.append(event)
                    break

    print(f"Found {len(important_events)} key economic events in November 2025")
    for event in important_events[:10]:
        print(f"  {event['date'][:10]} - {event['country']}: {event['type']}")

    return events

def fetch_usd_cny_exchange():
    """Fetch USD/CNY exchange rate data"""
    print("\nFetching USD/CNY exchange rate...")
    url = f"{BASE_URL}/eod/USDCNY.FOREX"
    params = {
        'api_token': API_TOKEN,
        'fmt': 'json',
        'from': '2020-01-01'
    }
    response = requests.get(url, params=params)
    data = response.json()
    df = pd.DataFrame(data)
    df['date'] = pd.to_datetime(df['date'])
    df.set_index('date', inplace=True)
    print(f"Latest USD/CNY: {df['close'].iloc[-1]:.4f}")
    return df

def calculate_technical_indicators(df):
    """Calculate technical indicators for gold price"""
    print("\nCalculating technical indicators...")

    # Moving averages
    df['SMA_20'] = df['close'].rolling(window=20).mean()
    df['SMA_50'] = df['close'].rolling(window=50).mean()
    df['SMA_200'] = df['close'].rolling(window=200).mean()
    df['EMA_12'] = df['close'].ewm(span=12, adjust=False).mean()
    df['EMA_26'] = df['close'].ewm(span=26, adjust=False).mean()

    # MACD
    df['MACD'] = df['EMA_12'] - df['EMA_26']
    df['Signal'] = df['MACD'].ewm(span=9, adjust=False).mean()
    df['MACD_Hist'] = df['MACD'] - df['Signal']

    # RSI
    delta = df['close'].diff()
    gain = (delta.where(delta > 0, 0)).rolling(window=14).mean()
    loss = (-delta.where(delta < 0, 0)).rolling(window=14).mean()
    rs = gain / loss
    df['RSI'] = 100 - (100 / (1 + rs))

    # Bollinger Bands
    df['BB_Middle'] = df['close'].rolling(window=20).mean()
    bb_std = df['close'].rolling(window=20).std()
    df['BB_Upper'] = df['BB_Middle'] + (bb_std * 2)
    df['BB_Lower'] = df['BB_Middle'] - (bb_std * 2)

    # Volatility
    df['Returns'] = df['close'].pct_change()
    df['Volatility_20'] = df['Returns'].rolling(window=20).std() * np.sqrt(252) * 100

    # Momentum
    df['Momentum_10'] = df['close'].pct_change(periods=10) * 100
    df['Momentum_30'] = df['close'].pct_change(periods=30) * 100

    latest = df.iloc[-1]
    print(f"  Current Price: ${latest['close']:.2f}")
    print(f"  SMA 20: ${latest['SMA_20']:.2f}")
    print(f"  SMA 50: ${latest['SMA_50']:.2f}")
    print(f"  SMA 200: ${latest['SMA_200']:.2f}")
    print(f"  RSI: {latest['RSI']:.2f}")
    print(f"  MACD: {latest['MACD']:.2f}")
    print(f"  Volatility (20-day annualized): {latest['Volatility_20']:.2f}%")
    print(f"  10-day Momentum: {latest['Momentum_10']:.2f}%")
    print(f"  30-day Momentum: {latest['Momentum_30']:.2f}%")

    return df

def analyze_price_drivers():
    """Analyze key price drivers and their current state"""
    print("\n" + "="*60)
    print("GOLD PRICE DRIVER ANALYSIS")
    print("="*60)

    drivers = {
        'US Real Interest Rates': {
            'impact': 'Negative - Lower real rates = Higher gold',
            'current_state': 'Fed cutting cycle, real rates declining',
            'outlook': 'Bullish for gold'
        },
        'USD Strength': {
            'impact': 'Negative - Stronger USD = Lower gold',
            'current_state': 'USD under pressure with rate cuts',
            'outlook': 'Bullish for gold'
        },
        'Geopolitical Risk': {
            'impact': 'Positive - Higher risk = Higher gold',
            'current_state': 'Elevated tensions globally',
            'outlook': 'Bullish for gold'
        },
        'Central Bank Buying': {
            'impact': 'Positive - More buying = Higher gold',
            'current_state': 'Record central bank purchases',
            'outlook': 'Very Bullish for gold'
        },
        'Inflation Expectations': {
            'impact': 'Positive - Higher inflation = Higher gold',
            'current_state': 'Sticky inflation concerns',
            'outlook': 'Neutral to Bullish'
        },
        'China Demand': {
            'impact': 'Positive - Higher demand = Higher gold',
            'current_state': 'Strong retail and PBOC demand',
            'outlook': 'Bullish for gold'
        }
    }

    for driver, analysis in drivers.items():
        print(f"\n{driver}:")
        for key, value in analysis.items():
            print(f"  {key}: {value}")

    return drivers

def main():
    print("="*60)
    print("GOLD OPTION TRADING STRATEGY - DATA COLLECTION")
    print("China Futures Market Analysis - November 2025 Outlook")
    print("="*60)

    # Collect all data
    gold_data = fetch_gold_historical_data()
    us_macro = fetch_us_macro_indicators()
    china_macro = fetch_china_macro_indicators()
    events = fetch_upcoming_economic_events()
    usd_cny = fetch_usd_cny_exchange()

    # Calculate technical indicators
    gold_data = calculate_technical_indicators(gold_data)

    # Analyze price drivers
    drivers = analyze_price_drivers()

    # Save data
    print("\nSaving data to files...")
    gold_data.to_csv('gold_historical_with_indicators.csv')
    usd_cny.to_csv('usd_cny_exchange.csv')

    with open('us_macro_indicators.json', 'w') as f:
        json.dump(us_macro, f, indent=2, default=str)

    with open('china_macro_indicators.json', 'w') as f:
        json.dump(china_macro, f, indent=2, default=str)

    with open('november_2025_events.json', 'w') as f:
        json.dump(events, f, indent=2)

    print("Data collection complete!")

    return gold_data, us_macro, china_macro, events, usd_cny

if __name__ == "__main__":
    main()
