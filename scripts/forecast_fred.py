#!/usr/bin/env python3
"""Multivariate forecast for FEDFUNDS using exogenous predictors from FRED.

This upgrades the previous univariate ARIMA to a SARIMAX-style model that
uses CPI (`CPIAUCSL`) and unemployment (`UNRATE`) as exogenous variables.

Approach:
- Fetch FEDFUNDS, CPIAUCSL, UNRATE monthly series from FRED
- Align on dates and drop NaNs
- Fit SARIMAX(endog=FEDFUNDS, exog=[CPI,UNRATE], order=(1,1,1))
- Forecast exogenous variables for the horizon using ARIMA(1,1,0) per-series
- Produce 1- and 3-month forecasts, save CSV and a PNG plot

Limitations:
- Exogenous forecasts use simple ARIMA(1,1,0) per-series; for production use

    apikey = args.apikey or os.environ.get('FRED_APIKEY') or os.environ.get('FRED_API_KEY')
    if not apikey:
        print('FRED API key required via --apikey or FRED_APIKEY env var', file=sys.stderr)
        sys.exit(2)

    print('Fetching series from FRED...')
    endog = fetch_fred_series(apikey, FRED_SERIES_MAIN)
    exogs = {s: fetch_fred_series(apikey, s) for s in EXOG_SERIES}

    # Align series
    df = pd.DataFrame({'fedfunds': endog})
    for name, s in exogs.items():
        df[name] = s
    df = df.dropna()
    if df.empty:
        print('No overlapping data after alignment', file=sys.stderr)
        sys.exit(3)

    print(f'Using data from {df.index[0].date()} to {df.index[-1].date()} ({len(df)} points)')

    # Fit model
    endog_aligned = df['fedfunds']
    exog_aligned = df[EXOG_SERIES]
    print('Fitting SARIMAX with exogenous CPI and UNRATE...')
    res = fit_sarimax(endog_aligned, exog_aligned)

    # Forecast exogenous vars for horizon
    future_exog = []
    for name in EXOG_SERIES:
        vals = forecast_exog(df[name], args.horizon)
        future_exog.append(vals)
    future_exog = np.column_stack(future_exog)

    # Generate forecast with provided future exog
    fc = res.get_forecast(steps=args.horizon, exog=future_exog)
    mean = fc.predicted_mean
    ci = fc.conf_int(alpha=0.05)

    last_date = df.index[-1]
    print(f'Latest observation: {last_date.date()} -> {endog_aligned.iloc[-1]:.2f}%')
    print('\nForecasts (monthly steps)')
    rows = []
    for i in range(args.horizon):
        step_date = last_date + pd.DateOffset(months=(i + 1))
        m = mean.iloc[i]
        lower = ci.iloc[i, 0]
        upper = ci.iloc[i, 1]
        print(f'{i+1}-month ({step_date.date()}): {m:.2f}%  95% CI [{lower:.2f}%, {upper:.2f}%]')
        rows.append({'date': step_date.date().isoformat(), 'mean': float(m), 'ci_lower': float(lower), 'ci_upper': float(upper)})

    # Save CSV
    out_dir = os.path.join(os.getcwd(), 'reports')
    os.makedirs(out_dir, exist_ok=True)
    csv_path = os.path.join(out_dir, 'fedfunds_forecast.csv')
    pd.DataFrame(rows).to_csv(csv_path, index=False)
    print(f'Forecast CSV written to: {csv_path}')

    # Plot
    fig, ax = plt.subplots(figsize=(10, 5))
    ax.plot(df.index, df['fedfunds'], label='Historical FEDFUNDS')
    # create future dates
    future_dates = [last_date + pd.DateOffset(months=(i + 1)) for i in range(args.horizon)]
    ax.plot(future_dates, mean.values, marker='o', label='Forecast')
    ax.fill_between(future_dates, ci.iloc[:, 0], ci.iloc[:, 1], color='gray', alpha=0.3, label='95% CI')
    ax.set_title('FEDFUNDS Forecast (SARIMAX with CPI, UNRATE)')
    ax.set_ylabel('Percent')
    ax.legend()
    png_path = os.path.join(out_dir, 'fedfunds_forecast.png')
    fig.autofmt_xdate()
    fig.savefig(png_path, dpi=150)
    print(f'Forecast plot written to: {png_path}')


if __name__ == '__main__':
    main()
#!/usr/bin/env python3
"""Fetch FRED series (FEDFUNDS) and produce a 1- and 3-month ARIMA forecast.

Usage:
  python scripts/forecast_fred.py --apikey <FRED_API_KEY>

Output: prints latest observation and forecasts for 1 and 3 months ahead (point + 95% CI).
"""
from __future__ import annotations
import os
import sys
import argparse
import requests
import pandas as pd
import numpy as np
from statsmodels.tsa.arima.model import ARIMA


FRED_SERIES = 'FEDFUNDS'


def fetch_fred_series(apikey: str, series_id: str = FRED_SERIES):
    url = f"https://api.stlouisfed.org/fred/series/observations?series_id={series_id}&api_key={apikey}&file_type=json&units=lin"
    resp = requests.get(url, timeout=30)
    resp.raise_for_status()
    j = resp.json()
    obs = j.get('observations')
    if not obs:
        raise RuntimeError('No observations in FRED response')
    df = pd.DataFrame(obs)
    if 'date' not in df.columns or 'value' not in df.columns:
        raise RuntimeError('Unexpected FRED response format')
    df['date'] = pd.to_datetime(df['date'])
    # FRED uses '.' for missing values
    df['value'] = pd.to_numeric(df['value'].replace('.', np.nan), errors='coerce')
    df = df.set_index('date').sort_index()
    return df['value'].dropna()


def fit_forecast(series: pd.Series, steps: int = 3):
    # FEDFUNDS is monthly - use ARIMA(1,1,1) for short horizons
    model = ARIMA(series, order=(1, 1, 1))
    fitted = model.fit()
    fc = fitted.get_forecast(steps=steps)
    mean = fc.predicted_mean
    ci = fc.conf_int(alpha=0.05)
    return mean, ci


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--apikey', default=None, help='FRED API key')
    args = p.parse_args()

    apikey = args.apikey or os.environ.get('FRED_APIKEY') or os.environ.get('FRED_API_KEY')
    if not apikey:
        print('FRED API key required via --apikey or FRED_APIKEY env var', file=sys.stderr)
        sys.exit(2)

    print('Fetching FRED series FEDFUNDS...')
    try:
        series = fetch_fred_series(apikey)
    except Exception as e:
        *** End Patch
        sys.exit(3)
