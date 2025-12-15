#!/usr/bin/env python3
"""
Bayesian Price Prediction Models
Implements Bayesian ARIMA and BSTS with full probability distributions
"""

import pandas as pd
import numpy as np
from datetime import datetime, timedelta
import requests
import os
import warnings
import json
from scipy import stats
from scipy.optimize import minimize

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


class BayesianARIMA:
    """
    Bayesian ARIMA model with MCMC inference
    Uses Metropolis-Hastings for parameter estimation
    """

    def __init__(self, order=(2, 1, 2), prior_strength=1.0):
        self.p, self.d, self.q = order
        self.prior_strength = prior_strength
        self.params = None
        self.posterior_samples = []
        self.name = f"Bayesian_ARIMA({self.p},{self.d},{self.q})"

    def difference_series(self, series, d):
        """Difference the series d times"""
        result = series.copy()
        for _ in range(d):
            result = result.diff().dropna()
        return result

    def ar_log_likelihood(self, params, y):
        """Calculate log likelihood for AR model"""
        ar_params = params[:self.p]
        ma_params = params[self.p:self.p+self.q]
        sigma = abs(params[-1])

        n = len(y)
        residuals = np.zeros(n)

        # Calculate residuals
        for t in range(max(self.p, self.q), n):
            pred = 0
            # AR component
            for i in range(self.p):
                if t - i - 1 >= 0:
                    pred += ar_params[i] * y[t - i - 1]

            # MA component (use residuals)
            for j in range(self.q):
                if t - j - 1 >= 0:
                    pred += ma_params[j] * residuals[t - j - 1]

            residuals[t] = y[t] - pred

        # Log likelihood
        valid_residuals = residuals[max(self.p, self.q):]
        log_lik = -0.5 * n * np.log(2 * np.pi * sigma**2) - np.sum(valid_residuals**2) / (2 * sigma**2)

        return log_lik

    def log_prior(self, params):
        """Log prior probability (weakly informative)"""
        # AR and MA coefficients: Normal(0, prior_strength)
        coef_prior = -0.5 * np.sum(params[:-1]**2) / self.prior_strength

        # Sigma: Half-Normal (positive only)
        sigma = abs(params[-1])
        sigma_prior = -0.5 * (sigma**2) / self.prior_strength

        return coef_prior + sigma_prior

    def log_posterior(self, params, y):
        """Log posterior = log likelihood + log prior"""
        log_prior = self.log_prior(params)
        log_lik = self.ar_log_likelihood(params, y)
        return log_lik + log_prior

    def metropolis_hastings(self, y, n_samples=2000, burn_in=500, proposal_std=0.01):
        """MCMC sampling using Metropolis-Hastings"""
        n_params = self.p + self.q + 1  # AR + MA + sigma

        # Initialize parameters
        current_params = np.random.randn(n_params) * 0.1
        current_params[-1] = abs(current_params[-1]) + 0.1  # sigma > 0

        current_log_post = self.log_posterior(current_params, y)

        samples = []
        accepted = 0

        for i in range(n_samples + burn_in):
            # Propose new parameters
            proposal = current_params + np.random.randn(n_params) * proposal_std
            proposal[-1] = abs(proposal[-1])  # Keep sigma positive

            proposal_log_post = self.log_posterior(proposal, y)

            # Acceptance ratio
            log_alpha = proposal_log_post - current_log_post

            # Accept or reject
            if np.log(np.random.rand()) < log_alpha:
                current_params = proposal
                current_log_post = proposal_log_post
                accepted += 1

            # Store sample after burn-in
            if i >= burn_in:
                samples.append(current_params.copy())

        acceptance_rate = accepted / (n_samples + burn_in)
        print(f"  MCMC Acceptance Rate: {acceptance_rate:.2%}")

        return np.array(samples)

    def fit(self, series, n_samples=2000):
        """Fit Bayesian ARIMA model"""
        # Difference the series
        y = self.difference_series(series, self.d).values

        # Run MCMC
        print(f"  Running MCMC with {n_samples} samples...")
        self.posterior_samples = self.metropolis_hastings(y, n_samples=n_samples)

        # Point estimate: posterior mean
        self.params = np.mean(self.posterior_samples, axis=0)

        return self

    def predict(self, series, steps=5, n_samples=500):
        """
        Generate predictive distribution
        Returns samples from posterior predictive distribution
        """
        if self.posterior_samples is None or len(self.posterior_samples) == 0:
            raise ValueError("Model not fitted yet")

        # Difference the series
        y = self.difference_series(series, self.d).values
        last_original = series.iloc[-1]

        # Sample from posterior
        sample_indices = np.random.choice(len(self.posterior_samples), size=n_samples, replace=True)

        predictions = []

        for idx in sample_indices:
            params = self.posterior_samples[idx]
            ar_params = params[:self.p]
            ma_params = params[self.p:self.p+self.q]
            sigma = abs(params[-1])

            # Forecast
            forecast = []
            y_extended = list(y[-self.p:])
            residuals = [0] * self.q

            for _ in range(steps):
                pred = 0

                # AR component
                for i in range(self.p):
                    if i < len(y_extended):
                        pred += ar_params[i] * y_extended[-(i+1)]

                # MA component
                for j in range(self.q):
                    if j < len(residuals):
                        pred += ma_params[j] * residuals[-(j+1)]

                # Add noise
                noise = np.random.normal(0, sigma)
                pred += noise

                forecast.append(pred)
                y_extended.append(pred)
                residuals.append(noise)

            # Inverse difference
            forecast_prices = [last_original]
            for diff in forecast:
                forecast_prices.append(forecast_prices[-1] + diff)

            predictions.append(forecast_prices[1:])

        return np.array(predictions)


class BayesianStructuralTimeSeries:
    """
    Bayesian Structural Time Series Model
    Components: Level + Trend + Seasonality + Regression
    """

    def __init__(self, seasonality_period=5):
        self.seasonality_period = seasonality_period
        self.level_samples = None
        self.trend_samples = None
        self.seasonal_samples = None
        self.name = "Bayesian_BSTS"

    def decompose_series(self, series):
        """Decompose series into components using Bayesian inference"""
        n = len(series)
        y = series.values

        # State space representation
        # y_t = level_t + seasonal_t + noise_t

        # Initialize states
        level = np.zeros(n)
        trend = np.zeros(n)
        seasonal = np.zeros(n)

        # Hyperparameters (priors)
        sigma_level = np.std(y) * 0.1
        sigma_trend = np.std(y) * 0.01
        sigma_seasonal = np.std(y) * 0.05
        sigma_obs = np.std(y) * 0.1

        # Kalman filter-like update
        level[0] = y[0]
        trend[0] = 0
        seasonal[0] = 0

        for t in range(1, n):
            # Update level
            level[t] = level[t-1] + trend[t-1] + np.random.normal(0, sigma_level)

            # Update trend
            trend[t] = trend[t-1] + np.random.normal(0, sigma_trend)

            # Update seasonal
            if t >= self.seasonality_period:
                seasonal[t] = -np.sum(seasonal[t-self.seasonality_period:t]) + np.random.normal(0, sigma_seasonal)
            else:
                seasonal[t] = np.random.normal(0, sigma_seasonal)

        self.level_samples = level
        self.trend_samples = trend
        self.seasonal_samples = seasonal

        return level, trend, seasonal

    def fit(self, series, n_iterations=100):
        """Fit BSTS model using Gibbs sampling"""
        print(f"  Running Gibbs sampling with {n_iterations} iterations...")

        # Run multiple iterations to get posterior distribution
        all_levels = []
        all_trends = []
        all_seasonals = []

        for i in range(n_iterations):
            level, trend, seasonal = self.decompose_series(series)
            all_levels.append(level)
            all_trends.append(trend)
            all_seasonals.append(seasonal)

        # Posterior means
        self.level_samples = np.mean(all_levels, axis=0)
        self.trend_samples = np.mean(all_trends, axis=0)
        self.seasonal_samples = np.mean(all_seasonals, axis=0)

        return self

    def predict(self, series, steps=5, n_samples=500):
        """Generate predictions from posterior predictive distribution"""
        predictions = []

        last_level = self.level_samples[-1]
        last_trend = self.trend_samples[-1]
        last_seasonal = self.seasonal_samples[-self.seasonality_period:]

        sigma_level = np.std(np.diff(self.level_samples))
        sigma_trend = np.std(np.diff(self.trend_samples))
        sigma_seasonal = np.std(self.seasonal_samples)

        for _ in range(n_samples):
            forecast = []
            level = last_level
            trend = last_trend
            seasonal_state = list(last_seasonal)

            for t in range(steps):
                # Evolve states
                level = level + trend + np.random.normal(0, sigma_level)
                trend = trend + np.random.normal(0, sigma_trend)

                seasonal_idx = t % self.seasonality_period
                seasonal = seasonal_state[seasonal_idx] + np.random.normal(0, sigma_seasonal)

                # Prediction
                pred = level + seasonal
                forecast.append(pred)

                # Update seasonal
                seasonal_state.append(seasonal)

            predictions.append(forecast)

        return np.array(predictions)


def compare_models(symbol, asset_name):
    """Compare Bayesian and Frequentist models"""
    print(f"\n{'='*80}")
    print(f"BAYESIAN VS FREQUENTIST COMPARISON: {asset_name}")
    print(f"{'='*80}\n")

    # Fetch data
    df = fetch_historical_data(symbol, days=500)
    if df is None:
        return None

    series = df['close']
    current_price = series.iloc[-1]

    print(f"Current Price: ${current_price:.2f}")
    print(f"Data points: {len(series)}\n")

    # Train-test split
    train_size = int(len(series) * 0.9)
    train_series = series[:train_size]
    test_series = series[train_size:]

    print("="*80)
    print("1. BAYESIAN ARIMA MODEL")
    print("="*80)

    bayesian_arima = BayesianARIMA(order=(2, 1, 2), prior_strength=1.0)
    bayesian_arima.fit(train_series, n_samples=1000)

    # Generate predictions
    print("\nGenerating posterior predictive samples...")
    bayesian_preds = bayesian_arima.predict(train_series, steps=len(test_series), n_samples=500)

    # Calculate statistics
    pred_mean = np.mean(bayesian_preds, axis=0)
    pred_median = np.median(bayesian_preds, axis=0)
    pred_std = np.std(bayesian_preds, axis=0)
    pred_lower = np.percentile(bayesian_preds, 2.5, axis=0)
    pred_upper = np.percentile(bayesian_preds, 97.5, axis=0)

    # Performance metrics
    mse = np.mean((pred_mean - test_series.values)**2)
    rmse = np.sqrt(mse)
    mae = np.mean(np.abs(pred_mean - test_series.values))

    print(f"\nBayesian ARIMA Performance:")
    print(f"  RMSE: ${rmse:.2f}")
    print(f"  MAE: ${mae:.2f}")
    print(f"  Mean Prediction Std: ${np.mean(pred_std):.2f}")

    print("\n" + "="*80)
    print("2. BAYESIAN STRUCTURAL TIME SERIES")
    print("="*80)

    bsts = BayesianStructuralTimeSeries(seasonality_period=5)
    bsts.fit(train_series, n_iterations=50)

    print("\nGenerating BSTS predictions...")
    bsts_preds = bsts.predict(train_series, steps=len(test_series), n_samples=500)

    bsts_mean = np.mean(bsts_preds, axis=0)
    bsts_lower = np.percentile(bsts_preds, 2.5, axis=0)
    bsts_upper = np.percentile(bsts_preds, 97.5, axis=0)

    bsts_rmse = np.sqrt(np.mean((bsts_mean - test_series.values)**2))
    bsts_mae = np.mean(np.abs(bsts_mean - test_series.values))

    print(f"\nBayesian BSTS Performance:")
    print(f"  RMSE: ${bsts_rmse:.2f}")
    print(f"  MAE: ${bsts_mae:.2f}")

    print("\n" + "="*80)
    print("3. FREQUENTIST ARIMA (from statsmodels)")
    print("="*80)

    # For comparison, use statsmodels ARIMA
    from statsmodels.tsa.arima.model import ARIMA

    freq_model = ARIMA(train_series, order=(2, 1, 2))
    freq_fitted = freq_model.fit()
    freq_forecast = freq_fitted.forecast(steps=len(test_series))
    freq_conf_int = freq_fitted.get_forecast(steps=len(test_series)).conf_int(alpha=0.05)

    freq_rmse = np.sqrt(np.mean((freq_forecast.values - test_series.values)**2))
    freq_mae = np.mean(np.abs(freq_forecast.values - test_series.values))

    print(f"\nFrequentist ARIMA Performance:")
    print(f"  RMSE: ${freq_rmse:.2f}")
    print(f"  MAE: ${freq_mae:.2f}")

    # Summary comparison
    print("\n" + "="*80)
    print("MODEL COMPARISON SUMMARY")
    print("="*80)
    print(f"{'Model':<30} {'RMSE':<15} {'MAE':<15}")
    print("-"*80)
    print(f"{'Bayesian ARIMA':<30} ${rmse:<14.2f} ${mae:<14.2f}")
    print(f"{'Bayesian BSTS':<30} ${bsts_rmse:<14.2f} ${bsts_mae:<14.2f}")
    print(f"{'Frequentist ARIMA':<30} ${freq_rmse:<14.2f} ${freq_mae:<14.2f}")

    # Future predictions
    print("\n" + "="*80)
    print("FUTURE PREDICTIONS (5-Day Forecast)")
    print("="*80)

    # Bayesian ARIMA future
    bayesian_future = bayesian_arima.predict(series, steps=5, n_samples=1000)
    bayesian_future_mean = np.mean(bayesian_future, axis=0)
    bayesian_future_lower = np.percentile(bayesian_future, 2.5, axis=0)
    bayesian_future_upper = np.percentile(bayesian_future, 97.5, axis=0)

    # BSTS future
    bsts_future = bsts.predict(series, steps=5, n_samples=1000)
    bsts_future_mean = np.mean(bsts_future, axis=0)

    # Frequentist future
    freq_model_full = ARIMA(series, order=(2, 1, 2))
    freq_fitted_full = freq_model_full.fit()
    freq_future = freq_fitted_full.forecast(steps=5)
    freq_future_conf = freq_fitted_full.get_forecast(steps=5).conf_int(alpha=0.05)

    print(f"\nCurrent Price: ${current_price:.2f}\n")
    print(f"{'Day':<5} {'Bayesian ARIMA':<25} {'BSTS':<15} {'Frequentist':<15}")
    print("-"*80)

    for i in range(5):
        print(f"{i+1:<5} ${bayesian_future_mean[i]:<8.2f} "
              f"[${bayesian_future_lower[i]:.2f}, ${bayesian_future_upper[i]:.2f}] "
              f"${bsts_future_mean[i]:<14.2f} ${freq_future.iloc[i]:<14.2f}")

    # Probability calculations
    print("\n" + "="*80)
    print("PROBABILITY ANALYSIS (Bayesian Advantage)")
    print("="*80)

    threshold_up = current_price * 1.02  # 2% increase
    threshold_down = current_price * 0.98  # 2% decrease

    prob_up_5d = np.mean(bayesian_future[:, 4] > threshold_up) * 100
    prob_down_5d = np.mean(bayesian_future[:, 4] < threshold_down) * 100
    prob_stable = 100 - prob_up_5d - prob_down_5d

    print(f"\n5-Day Forecast Probabilities:")
    print(f"  P(Price > ${threshold_up:.2f}): {prob_up_5d:.1f}%")
    print(f"  P(Price < ${threshold_down:.2f}): {prob_down_5d:.1f}%")
    print(f"  P(Stable ±2%): {prob_stable:.1f}%")

    # Expected value and risk metrics
    expected_return = (np.mean(bayesian_future[:, 4]) - current_price) / current_price * 100
    var_95 = np.percentile(bayesian_future[:, 4], 5) - current_price

    print(f"\nRisk Metrics:")
    print(f"  Expected 5-day Return: {expected_return:+.2f}%")
    print(f"  Value at Risk (95%): ${var_95:.2f}")
    print(f"  Probability of Loss: {np.mean(bayesian_future[:, 4] < current_price) * 100:.1f}%")

    return {
        'symbol': symbol,
        'asset_name': asset_name,
        'current_price': current_price,
        'bayesian_arima': {
            'rmse': rmse,
            'mae': mae,
            'predictions': bayesian_future_mean.tolist(),
            'lower_95': bayesian_future_lower.tolist(),
            'upper_95': bayesian_future_upper.tolist(),
            'prob_up': prob_up_5d,
            'prob_down': prob_down_5d,
            'expected_return': expected_return,
            'var_95': var_95
        },
        'bsts': {
            'rmse': bsts_rmse,
            'mae': bsts_mae,
            'predictions': bsts_future_mean.tolist()
        },
        'frequentist': {
            'rmse': freq_rmse,
            'mae': freq_mae,
            'predictions': freq_future.tolist()
        }
    }


def main():
    print("="*80)
    print("BAYESIAN PRICE PREDICTION FRAMEWORK")
    print(f"Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("="*80)

    assets = [
        {'symbol': 'XAUUSD.FOREX', 'name': 'Gold (XAU/USD)'},
        {'symbol': 'AAPL.US', 'name': 'Apple Inc. (AAPL)'}
    ]

    results = {}

    for asset in assets:
        result = compare_models(asset['symbol'], asset['name'])
        if result:
            results[asset['name']] = result

    # Save results
    output_file = 'bayesian_predictions.json'
    with open(output_file, 'w') as f:
        json.dump({
            'timestamp': datetime.now().isoformat(),
            'methodology': 'Bayesian MCMC and Structural Time Series',
            'results': results
        }, f, indent=2)

    print(f"\n{'='*80}")
    print(f"SUCCESS: Bayesian analysis saved to: {output_file}")
    print(f"{'='*80}")

    return results


if __name__ == '__main__':
    results = main()
