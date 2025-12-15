#!/usr/bin/env python3
"""
Generate Final Price Predictions with Multiple Timeframes
Uses the best performing ARIMA models from backtesting
"""

import pandas as pd
import numpy as np
from datetime import datetime, timedelta
import requests
import os
import warnings
from statsmodels.tsa.arima.model import ARIMA
import json

warnings.filterwarnings('ignore')

EODHD_API_KEY = os.environ.get('EODHD_APIKEY', '690d7cdc3013f4.57364117')


def fetch_historical_data(symbol, days=500):
    """Fetch historical price data from EODHD"""
    url = f"https://eodhd.com/api/eod/{symbol}"
    params = {
        'api_token': EODHD_API_KEY,
        'fmt': 'json',
        'period': 'd',
        'order': 'd'
    }

    try:
        resp = requests.get(url, params=params, timeout=30)
        resp.raise_for_status()
        data = resp.json()

        if isinstance(data, list) and data:
            df = pd.DataFrame(data[:days])
            df['date'] = pd.to_datetime(df['date'])
            df = df.sort_values('date')
            df.set_index('date', inplace=True)

            for col in ['open', 'high', 'low', 'close', 'volume']:
                if col in df.columns:
                    df[col] = pd.to_numeric(df[col], errors='coerce')

            return df
        return None
    except Exception as e:
        print(f"Error fetching {symbol}: {e}")
        return None


def train_arima_and_forecast(series, order, forecast_days):
    """Train ARIMA model and generate forecasts"""
    try:
        model = ARIMA(series, order=order)
        fitted = model.fit()
        forecast = fitted.get_forecast(steps=forecast_days)

        pred_mean = forecast.predicted_mean
        conf_int = forecast.conf_int(alpha=0.05)

        return {
            'predictions': pred_mean.values,
            'lower_bound': conf_int.iloc[:, 0].values,
            'upper_bound': conf_int.iloc[:, 1].values,
            'dates': pd.date_range(start=series.index[-1] + timedelta(days=1), periods=forecast_days, freq='B')
        }
    except Exception as e:
        print(f"Error forecasting: {e}")
        return None


def generate_multi_timeframe_predictions(symbol, asset_name):
    """Generate predictions for multiple timeframes"""
    print(f"\n{'='*80}")
    print(f"MULTI-TIMEFRAME PREDICTIONS: {asset_name}")
    print(f"{'='*80}\n")

    # Fetch data
    df = fetch_historical_data(symbol, days=500)
    if df is None:
        print(f"Failed to fetch data for {symbol}")
        return None

    current_price = df['close'].iloc[-1]
    current_date = df.index[-1]

    print(f"Current Date: {current_date.strftime('%Y-%m-%d')}")
    print(f"Current Price: ${current_price:.2f}\n")

    # Use best model from backtest: ARIMA(2,1,2)
    best_order = (2, 1, 2)
    series = df['close']

    # Generate forecasts for multiple horizons
    timeframes = {
        '5-Day': 5,
        '10-Day': 10,
        '20-Day': 20,
        '30-Day': 30
    }

    all_predictions = {}

    for name, days in timeframes.items():
        print(f"Generating {name} Forecast...")
        forecast = train_arima_and_forecast(series, best_order, days)

        if forecast:
            final_price = forecast['predictions'][-1]
            final_lower = forecast['lower_bound'][-1]
            final_upper = forecast['upper_bound'][-1]

            price_change = final_price - current_price
            price_change_pct = (price_change / current_price) * 100

            print(f"  Target Date: {forecast['dates'][-1].strftime('%Y-%m-%d')}")
            print(f"  Predicted Price: ${final_price:.2f}")
            print(f"  95% CI: [${final_lower:.2f}, ${final_upper:.2f}]")
            print(f"  Expected Change: ${price_change:+.2f} ({price_change_pct:+.2f}%)")
            print()

            all_predictions[name] = {
                'days': days,
                'target_date': forecast['dates'][-1].strftime('%Y-%m-%d'),
                'predicted_price': round(final_price, 2),
                'lower_bound': round(final_lower, 2),
                'upper_bound': round(final_upper, 2),
                'price_change': round(price_change, 2),
                'price_change_pct': round(price_change_pct, 2),
                'full_series': [round(p, 2) for p in forecast['predictions']],
                'dates': [d.strftime('%Y-%m-%d') for d in forecast['dates']]
            }

    return {
        'symbol': symbol,
        'asset_name': asset_name,
        'current_date': current_date.strftime('%Y-%m-%d'),
        'current_price': round(current_price, 2),
        'predictions': all_predictions
    }


def calculate_technical_indicators(df):
    """Calculate key technical indicators"""
    current_price = df['close'].iloc[-1]

    # Moving averages
    sma_20 = df['close'].rolling(20).mean().iloc[-1]
    sma_50 = df['close'].rolling(50).mean().iloc[-1]
    sma_200 = df['close'].rolling(200).mean().iloc[-1]

    # RSI
    delta = df['close'].diff()
    gain = (delta.where(delta > 0, 0)).rolling(14).mean().iloc[-1]
    loss = (-delta.where(delta < 0, 0)).rolling(14).mean().iloc[-1]
    rs = gain / loss
    rsi = 100 - (100 / (1 + rs))

    # Recent performance
    perf_1w = ((current_price / df['close'].iloc[-5]) - 1) * 100 if len(df) >= 5 else 0
    perf_1m = ((current_price / df['close'].iloc[-21]) - 1) * 100 if len(df) >= 21 else 0
    perf_3m = ((current_price / df['close'].iloc[-63]) - 1) * 100 if len(df) >= 63 else 0

    return {
        'sma_20': round(sma_20, 2),
        'sma_50': round(sma_50, 2),
        'sma_200': round(sma_200, 2),
        'rsi': round(rsi, 2),
        'above_sma_20': bool(current_price > sma_20),
        'above_sma_50': bool(current_price > sma_50),
        'above_sma_200': bool(current_price > sma_200),
        'performance_1w': round(perf_1w, 2),
        'performance_1m': round(perf_1m, 2),
        'performance_3m': round(perf_3m, 2)
    }


def main():
    print("="*80)
    print("FINAL PRICE PREDICTIONS - GOLD AND APPLE")
    print(f"Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("="*80)

    assets = [
        {'symbol': 'XAUUSD.FOREX', 'name': 'Gold (XAU/USD)'},
        {'symbol': 'AAPL.US', 'name': 'Apple Inc. (AAPL)'}
    ]

    final_results = {}

    for asset in assets:
        # Generate predictions
        result = generate_multi_timeframe_predictions(asset['symbol'], asset['name'])

        if result:
            # Add technical analysis
            df = fetch_historical_data(asset['symbol'], days=250)
            if df is not None:
                technicals = calculate_technical_indicators(df)
                result['technical_indicators'] = technicals

                print(f"Technical Indicators for {asset['name']}:")
                print(f"  SMA 20:  ${technicals['sma_20']:<10} [{'ABOVE' if technicals['above_sma_20'] else 'BELOW'}]")
                print(f"  SMA 50:  ${technicals['sma_50']:<10} [{'ABOVE' if technicals['above_sma_50'] else 'BELOW'}]")
                print(f"  SMA 200: ${technicals['sma_200']:<10} [{'ABOVE' if technicals['above_sma_200'] else 'BELOW'}]")
                print(f"  RSI:     {technicals['rsi']:.1f}")
                print(f"  Performance:")
                print(f"    1-Week:  {technicals['performance_1w']:+.2f}%")
                print(f"    1-Month: {technicals['performance_1m']:+.2f}%")
                print(f"    3-Month: {technicals['performance_3m']:+.2f}%")
                print()

            final_results[asset['name']] = result

    # Save to file
    output_file = 'final_price_predictions.json'
    with open(output_file, 'w') as f:
        json.dump({
            'timestamp': datetime.now().isoformat(),
            'model': 'ARIMA(2,1,2) - Best from Backtesting',
            'assets': final_results
        }, f, indent=2)

    print(f"\n{'='*80}")
    print(f"SUCCESS: Final predictions saved to: {output_file}")
    print(f"{'='*80}")

    # Print summary
    print(f"\n{'='*80}")
    print("PREDICTION SUMMARY")
    print(f"{'='*80}\n")

    for asset_name, result in final_results.items():
        print(f"{asset_name}:")
        print(f"  Current: ${result['current_price']}")
        for timeframe, pred in result['predictions'].items():
            print(f"  {timeframe:8} ${pred['predicted_price']:<8.2f} ({pred['price_change_pct']:+.2f}%)")
        print()

    return final_results


if __name__ == '__main__':
    results = main()
