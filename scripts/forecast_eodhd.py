#!/usr/bin/env python3
"""Fetch recent EOD series from EODHD MCP proxy and produce a short-term forecast.

Usage:
  # set API key (or pass via --apikey)
  $Env:EODHD_APIKEY = '690d7cdc3013f4.57364117'
  python scripts/forecast_eodhd.py --symbol AAPL.US --days 200 --horizon 30

This script fits a simple ARIMA(1,1,1) model to the 'close' series and
prints point forecasts and 95% confidence intervals for the next `horizon`
business days.

Notes:
- Requires: requests, pandas, statsmodels, numpy
- Install: pip install requests pandas statsmodels numpy
"""
from __future__ import annotations
import os
import sys
import argparse
import urllib.parse
import requests
import pandas as pd
import numpy as np
from statsmodels.tsa.arima.model import ARIMA


def fetch_eod_series(apikey: str, symbol: str, days: int = 200):
    # Build encoded inner URL for the MCP proxy
    inner = f"/api/eod/{symbol}?fmt=json&period=d&limit={days}"
    encoded = urllib.parse.quote(inner, safe="")
    url = f"https://mcp.eodhd.dev/mcp?apikey={apikey}&url={encoded}"
    resp = requests.get(url, timeout=30)
    resp.raise_for_status()
    data = resp.json()
    if not isinstance(data, (list, tuple)):
        # some proxies wrap result; try to find list
        if isinstance(data, dict) and 'data' in data and isinstance(data['data'], list):
            data = data['data']
        else:
            raise RuntimeError(f"Unexpected response format: {type(data)}")

    # Convert to DataFrame
    df = pd.DataFrame(data)
    if 'date' not in df.columns or 'close' not in df.columns:
        raise RuntimeError(f"Response JSON missing required fields: {df.columns.tolist()}")
    df['date'] = pd.to_datetime(df['date'])
    df = df.sort_values('date')
    df = df.set_index('date')
    # Ensure numeric
    df['close'] = pd.to_numeric(df['close'], errors='coerce')
    df = df[['close']].dropna()
    return df


def fit_and_forecast(series: pd.Series, horizon: int = 30):
    # Use a simple ARIMA(1,1,1) for short-term forecasting
    model = ARIMA(series, order=(1, 1, 1))
    fitted = model.fit()
    fc = fitted.get_forecast(steps=horizon)
    mean = fc.predicted_mean
    ci = fc.conf_int(alpha=0.05)
    return fitted, mean, ci


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--symbol', required=True, help='Symbol to fetch (e.g. AAPL.US or GC.F or XAUUSD)')
    p.add_argument('--apikey', default=None, help='EODHD MCP API key')
    p.add_argument('--days', type=int, default=200, help='Number of historical days to fetch')
    p.add_argument('--horizon', type=int, default=30, help='Forecast horizon in days')
    args = p.parse_args()

    apikey = args.apikey or os.environ.get('EODHD_APIKEY') or os.environ.get('EODHD_API_KEY') or os.environ.get('EODHD_API') or os.environ.get('EODHD_MCP_APIKEY')
    if not apikey:
        apikey = '690d7cdc3013f4.57364117'

    print(f"Fetching {args.days} days for {args.symbol} via MCP proxy...")
    try:
        df = fetch_eod_series(apikey, args.symbol, args.days)
    except Exception as e:
        print(f"Failed to fetch series: {e}")
        sys.exit(2)

    if df.empty:
        print("No data returned for symbol")
        sys.exit(2)

    series = df['close']
    print(f"Got {len(series)} points, last date: {series.index[-1].date()}, last close: {series.iloc[-1]:.4f}")

    print(f"Fitting ARIMA(1,1,1) and forecasting {args.horizon} days...")
    try:
        model, mean, ci = fit_and_forecast(series, args.horizon)
    except Exception as e:
        print(f"Model fitting/forecast failed: {e}")
        sys.exit(3)

    out = []
    last_date = series.index[-1]
    for i, (m, row) in enumerate(zip(mean, ci.values)):
        d = last_date + pd.Timedelta(days=(i + 1))
        lower = row[0]
        upper = row[1]
        out.append((d.strftime('%Y-%m-%d'), float(m), float(lower), float(upper)))

    print('\nForecast (date, mean, ci_lower, ci_upper)')
    for r in out:
        print(f"{r[0]}\t{r[1]:.4f}\t{r[2]:.4f}\t{r[3]:.4f}")

    forecast_mean = np.array([r[1] for r in out])
    print('\nSummary:')
    print(f"Last close: {series.iloc[-1]:.4f}")
    print(f"Mean forecast (next {args.horizon} days) first 7-day avg: {forecast_mean[:7].mean():.4f}")


if __name__ == '__main__':
    main()
