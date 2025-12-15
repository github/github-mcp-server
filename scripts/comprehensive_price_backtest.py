#!/usr/bin/env python3
"""
Comprehensive Backtesting Framework for Gold and Apple Price Predictions
Tests multiple models and identifies the best performing approach
"""

import pandas as pd
import numpy as np
from datetime import datetime, timedelta
import requests
import os
import sys
import warnings
from sklearn.ensemble import RandomForestRegressor, GradientBoostingRegressor
from sklearn.linear_model import LinearRegression, Ridge, Lasso
from sklearn.preprocessing import StandardScaler
from sklearn.metrics import mean_squared_error, mean_absolute_error, r2_score
from statsmodels.tsa.arima.model import ARIMA
import json

warnings.filterwarnings('ignore')

# EODHD API Configuration
EODHD_API_KEY = os.environ.get('EODHD_APIKEY', '690d7cdc3013f4.57364117')

class PricePredictor:
    """Base class for price prediction models"""

    def __init__(self, name):
        self.name = name
        self.model = None
        self.scaler = StandardScaler()

    def prepare_features(self, df):
        """Prepare features for ML models"""
        features = pd.DataFrame(index=df.index)

        # Price-based features
        features['returns_1d'] = df['close'].pct_change(1)
        features['returns_5d'] = df['close'].pct_change(5)
        features['returns_10d'] = df['close'].pct_change(10)
        features['returns_20d'] = df['close'].pct_change(20)

        # Moving averages
        features['sma_10'] = df['close'].rolling(10).mean() / df['close']
        features['sma_20'] = df['close'].rolling(20).mean() / df['close']
        features['sma_50'] = df['close'].rolling(50).mean() / df['close']

        # Volatility
        features['volatility_10'] = df['close'].pct_change().rolling(10).std()
        features['volatility_20'] = df['close'].pct_change().rolling(20).std()

        # Volume (if available)
        if 'volume' in df.columns:
            features['volume_ratio'] = df['volume'] / df['volume'].rolling(20).mean()

        # RSI
        delta = df['close'].diff()
        gain = (delta.where(delta > 0, 0)).rolling(14).mean()
        loss = (-delta.where(delta < 0, 0)).rolling(14).mean()
        rs = gain / loss
        features['rsi'] = 100 - (100 / (1 + rs))

        # MACD
        ema_12 = df['close'].ewm(span=12).mean()
        ema_26 = df['close'].ewm(span=26).mean()
        features['macd'] = (ema_12 - ema_26) / df['close']

        # Bollinger Bands
        sma_20 = df['close'].rolling(20).mean()
        std_20 = df['close'].rolling(20).std()
        features['bb_position'] = (df['close'] - (sma_20 - 2*std_20)) / (4*std_20)

        # Momentum
        features['momentum_10'] = df['close'] / df['close'].shift(10) - 1
        features['momentum_20'] = df['close'] / df['close'].shift(20) - 1

        return features.dropna()

    def train(self, X, y):
        """Train the model"""
        raise NotImplementedError

    def predict(self, X):
        """Make predictions"""
        raise NotImplementedError


class ARIMAPredictor(PricePredictor):
    """ARIMA model for time series forecasting"""

    def __init__(self, order=(1,1,1)):
        super().__init__(f"ARIMA{order}")
        self.order = order

    def train(self, series):
        """Train ARIMA model on price series"""
        try:
            self.model = ARIMA(series, order=self.order)
            self.fitted_model = self.model.fit()
            return True
        except:
            return False

    def predict(self, steps=1):
        """Forecast future prices"""
        if self.fitted_model:
            forecast = self.fitted_model.forecast(steps=steps)
            return forecast
        return None


class RandomForestPredictor(PricePredictor):
    """Random Forest Regressor"""

    def __init__(self, **kwargs):
        super().__init__("RandomForest")
        self.model = RandomForestRegressor(
            n_estimators=kwargs.get('n_estimators', 100),
            max_depth=kwargs.get('max_depth', 10),
            random_state=42
        )

    def train(self, X, y):
        X_scaled = self.scaler.fit_transform(X)
        self.model.fit(X_scaled, y)
        return True

    def predict(self, X):
        X_scaled = self.scaler.transform(X)
        return self.model.predict(X_scaled)


class GradientBoostingPredictor(PricePredictor):
    """Gradient Boosting Regressor"""

    def __init__(self, **kwargs):
        super().__init__("GradientBoosting")
        self.model = GradientBoostingRegressor(
            n_estimators=kwargs.get('n_estimators', 100),
            max_depth=kwargs.get('max_depth', 5),
            learning_rate=kwargs.get('learning_rate', 0.1),
            random_state=42
        )

    def train(self, X, y):
        X_scaled = self.scaler.fit_transform(X)
        self.model.fit(X_scaled, y)
        return True

    def predict(self, X):
        X_scaled = self.scaler.transform(X)
        return self.model.predict(X_scaled)


class LinearRegressionPredictor(PricePredictor):
    """Linear Regression Model"""

    def __init__(self):
        super().__init__("LinearRegression")
        self.model = LinearRegression()

    def train(self, X, y):
        X_scaled = self.scaler.fit_transform(X)
        self.model.fit(X_scaled, y)
        return True

    def predict(self, X):
        X_scaled = self.scaler.transform(X)
        return self.model.predict(X_scaled)


class RidgePredictor(PricePredictor):
    """Ridge Regression with L2 regularization"""

    def __init__(self, alpha=1.0):
        super().__init__(f"Ridge(alpha={alpha})")
        self.model = Ridge(alpha=alpha)

    def train(self, X, y):
        X_scaled = self.scaler.fit_transform(X)
        self.model.fit(X_scaled, y)
        return True

    def predict(self, X):
        X_scaled = self.scaler.transform(X)
        return self.model.predict(X_scaled)


class EnsemblePredictor(PricePredictor):
    """Ensemble of multiple models with weighted averaging"""

    def __init__(self, models, weights=None):
        super().__init__("Ensemble")
        self.models = models
        self.weights = weights or [1/len(models)] * len(models)

    def train(self, X, y):
        for model in self.models:
            model.train(X, y)
        return True

    def predict(self, X):
        predictions = []
        for model in self.models:
            pred = model.predict(X)
            predictions.append(pred)

        # Weighted average
        ensemble_pred = np.average(predictions, axis=0, weights=self.weights)
        return ensemble_pred


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

            # Convert to numeric
            for col in ['open', 'high', 'low', 'close', 'volume']:
                if col in df.columns:
                    df[col] = pd.to_numeric(df[col], errors='coerce')

            return df
        return None
    except Exception as e:
        print(f"Error fetching {symbol}: {e}")
        return None


def backtest_model(model, df, forecast_days=5, train_size=0.8):
    """
    Backtest a model using walk-forward validation
    Returns metrics: RMSE, MAE, R2, direction accuracy
    """
    results = {
        'predictions': [],
        'actuals': [],
        'dates': [],
        'errors': []
    }

    # Prepare features
    if not isinstance(model, ARIMAPredictor):
        features = model.prepare_features(df)
        features['target'] = df['close'].shift(-forecast_days)
        features = features.dropna()

        split_idx = int(len(features) * train_size)

        # Walk-forward validation
        for i in range(split_idx, len(features) - 1):
            train_X = features.iloc[:i].drop('target', axis=1)
            train_y = features.iloc[:i]['target']
            test_X = features.iloc[i:i+1].drop('target', axis=1)
            test_y = features.iloc[i:i+1]['target'].values[0]

            # Train model
            try:
                model.train(train_X, train_y)
                pred = model.predict(test_X)[0]

                results['predictions'].append(pred)
                results['actuals'].append(test_y)
                results['dates'].append(features.index[i])
                results['errors'].append(abs(pred - test_y))
            except:
                continue
    else:
        # ARIMA specific backtesting
        split_idx = int(len(df) * train_size)

        for i in range(split_idx, len(df) - forecast_days):
            train_series = df['close'].iloc[:i]
            test_value = df['close'].iloc[i + forecast_days]

            try:
                model.train(train_series)
                pred = model.predict(steps=forecast_days).iloc[-1]

                results['predictions'].append(pred)
                results['actuals'].append(test_value)
                results['dates'].append(df.index[i])
                results['errors'].append(abs(pred - test_value))
            except:
                continue

    if not results['predictions']:
        return None

    # Calculate metrics
    predictions = np.array(results['predictions'])
    actuals = np.array(results['actuals'])

    rmse = np.sqrt(mean_squared_error(actuals, predictions))
    mae = mean_absolute_error(actuals, predictions)
    r2 = r2_score(actuals, predictions)

    # Direction accuracy
    actual_direction = np.diff(actuals) > 0
    pred_direction = np.diff(predictions) > 0
    direction_accuracy = np.mean(actual_direction == pred_direction) * 100

    # MAPE
    mape = np.mean(np.abs((actuals - predictions) / actuals)) * 100

    return {
        'model_name': model.name,
        'rmse': rmse,
        'mae': mae,
        'r2': r2,
        'mape': mape,
        'direction_accuracy': direction_accuracy,
        'predictions': results['predictions'],
        'actuals': results['actuals'],
        'dates': results['dates']
    }


def run_comprehensive_backtest(symbol, asset_name):
    """Run backtest on all models for a given symbol"""
    print(f"\n{'='*80}")
    print(f"COMPREHENSIVE BACKTEST: {asset_name} ({symbol})")
    print(f"{'='*80}\n")

    # Fetch data
    print(f"Fetching historical data for {symbol}...")
    df = fetch_historical_data(symbol, days=500)

    if df is None or len(df) < 100:
        print(f"Insufficient data for {symbol}")
        return None

    print(f"Loaded {len(df)} days of data from {df.index[0].date()} to {df.index[-1].date()}")
    print(f"Current price: ${df['close'].iloc[-1]:.2f}\n")

    # Initialize models
    models = [
        ARIMAPredictor(order=(1,1,1)),
        ARIMAPredictor(order=(2,1,2)),
        RandomForestPredictor(n_estimators=100, max_depth=10),
        RandomForestPredictor(n_estimators=200, max_depth=15),
        GradientBoostingPredictor(n_estimators=100, max_depth=5),
        GradientBoostingPredictor(n_estimators=150, max_depth=7),
        LinearRegressionPredictor(),
        RidgePredictor(alpha=1.0),
        RidgePredictor(alpha=10.0),
    ]

    # Run backtests
    results = []
    for i, model in enumerate(models, 1):
        print(f"[{i}/{len(models)}] Testing {model.name}...", end=' ')
        try:
            result = backtest_model(model, df, forecast_days=5, train_size=0.8)
            if result:
                results.append(result)
                print(f"OK - RMSE: {result['rmse']:.2f}, R2: {result['r2']:.4f}, Dir Acc: {result['direction_accuracy']:.1f}%")
            else:
                print("FAILED")
        except Exception as e:
            print(f"ERROR: {e}")

    if not results:
        print("No successful backtests!")
        return None

    # Sort by R² score
    results.sort(key=lambda x: x['r2'], reverse=True)

    # Print summary
    print(f"\n{'='*80}")
    print(f"BACKTEST RESULTS SUMMARY - {asset_name}")
    print(f"{'='*80}")
    print(f"{'Rank':<6} {'Model':<25} {'RMSE':<12} {'MAE':<12} {'R2':<10} {'MAPE':<10} {'Dir Acc':<10}")
    print(f"{'-'*80}")

    for i, result in enumerate(results, 1):
        print(f"{i:<6} {result['model_name']:<25} "
              f"{result['rmse']:<12.2f} {result['mae']:<12.2f} "
              f"{result['r2']:<10.4f} {result['mape']:<10.2f}% "
              f"{result['direction_accuracy']:<10.1f}%")

    # Best model
    best = results[0]
    print(f"\nBEST MODEL: {best['model_name']}")
    print(f"   R2 Score: {best['r2']:.4f}")
    print(f"   RMSE: ${best['rmse']:.2f}")
    print(f"   Direction Accuracy: {best['direction_accuracy']:.1f}%")

    return {
        'symbol': symbol,
        'asset_name': asset_name,
        'current_price': df['close'].iloc[-1],
        'results': results,
        'best_model': best,
        'data': df
    }


def create_improved_ensemble(backtest_results):
    """Create an improved ensemble based on backtest performance"""
    # Only use models with positive R2 scores (better than mean predictor)
    good_models = [r for r in backtest_results['results'] if r['r2'] > 0]

    if not good_models:
        # Fallback: use best model only
        print("\nNo models with positive R2 found, using best model only")
        return [backtest_results['results'][0]], [1.0]

    # Take top 3 good models
    top_models = good_models[:min(3, len(good_models))]

    # Weight by R2 score (normalized)
    r2_scores = [r['r2'] for r in top_models]
    total_r2 = sum(r2_scores)
    weights = [r2 / total_r2 for r2 in r2_scores]

    print(f"\n{'='*80}")
    print("IMPROVED ENSEMBLE MODEL")
    print(f"{'='*80}")
    print(f"\nSelected {len(top_models)} models with positive R2 scores:")
    for i, (model, weight) in enumerate(zip(top_models, weights), 1):
        print(f"  {i}. {model['model_name']:<25} Weight: {weight:.3f} (R2: {model['r2']:.4f})")

    return top_models, weights


def predict_future_price(df, top_models, weights, forecast_days=5):
    """Generate future price prediction using ensemble of best models"""
    predictions = []

    for model_result in top_models:
        model_name = model_result['model_name']

        # Recreate and train model
        if 'ARIMA' in model_name:
            order = (1,1,1) if '(1, 1, 1)' in model_name else (2,1,2)
            model = ARIMAPredictor(order=order)
            model.train(df['close'])
            pred = model.predict(steps=forecast_days).iloc[-1]
        else:
            # ML models
            if 'RandomForest' in model_name:
                model = RandomForestPredictor()
            elif 'GradientBoosting' in model_name:
                model = GradientBoostingPredictor()
            elif 'Ridge' in model_name:
                model = RidgePredictor()
            else:
                model = LinearRegressionPredictor()

            features = model.prepare_features(df)
            features['target'] = df['close'].shift(-forecast_days)
            features = features.dropna()

            X = features.drop('target', axis=1)
            y = features['target']

            model.train(X, y)

            # Predict on latest data
            latest_features = model.prepare_features(df).iloc[-1:].drop('target', axis=1, errors='ignore')
            pred = model.predict(latest_features)[0]

        predictions.append(pred)

    # Weighted ensemble prediction
    ensemble_prediction = np.average(predictions, weights=weights)

    return ensemble_prediction, predictions


def main():
    """Main backtesting and prediction pipeline"""
    print("="*80)
    print("COMPREHENSIVE PRICE PREDICTION BACKTEST AND IMPROVEMENT")
    print(f"Analysis Date: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("="*80)

    # Assets to test
    assets = [
        {'symbol': 'XAUUSD.FOREX', 'name': 'Gold (XAU/USD)'},
        {'symbol': 'AAPL.US', 'name': 'Apple Inc. (AAPL)'}
    ]

    all_results = {}

    # Run backtests
    for asset in assets:
        result = run_comprehensive_backtest(asset['symbol'], asset['name'])
        if result:
            all_results[asset['name']] = result

    # Generate predictions with improved ensembles
    print(f"\n{'='*80}")
    print("GENERATING FINAL PREDICTIONS WITH IMPROVED ENSEMBLES")
    print(f"{'='*80}")

    final_predictions = {}

    for asset_name, backtest_result in all_results.items():
        print(f"\n{asset_name}:")
        top_models, weights = create_improved_ensemble(backtest_result)

        # Generate prediction
        current_price = backtest_result['current_price']
        forecast_price, individual_preds = predict_future_price(
            backtest_result['data'],
            top_models,
            weights,
            forecast_days=5
        )

        price_change = forecast_price - current_price
        price_change_pct = (price_change / current_price) * 100

        print(f"\n5-Day Price Forecast:")
        print(f"  Current Price:     ${current_price:.2f}")
        print(f"  Predicted Price:   ${forecast_price:.2f}")
        print(f"  Expected Change:   ${price_change:+.2f} ({price_change_pct:+.2f}%)")
        print(f"  Individual Model Predictions:")
        for i, (pred, model) in enumerate(zip(individual_preds, top_models), 1):
            print(f"    {i}. {model['model_name']:<25} ${pred:.2f}")

        final_predictions[asset_name] = {
            'current_price': current_price,
            'predicted_price': forecast_price,
            'price_change': price_change,
            'price_change_pct': price_change_pct,
            'individual_predictions': individual_preds,
            'top_models': [m['model_name'] for m in top_models],
            'weights': weights
        }

    # Save results
    output_file = 'backtest_and_predictions.json'
    with open(output_file, 'w') as f:
        json.dump({
            'timestamp': datetime.now().isoformat(),
            'backtest_results': {
                name: {
                    'symbol': res['symbol'],
                    'current_price': res['current_price'],
                    'best_model': res['best_model']['model_name'],
                    'best_r2': res['best_model']['r2'],
                    'all_models': [{
                        'name': r['model_name'],
                        'rmse': r['rmse'],
                        'r2': r['r2'],
                        'direction_accuracy': r['direction_accuracy']
                    } for r in res['results']]
                }
                for name, res in all_results.items()
            },
            'predictions': final_predictions
        }, f, indent=2)

    print(f"\n{'='*80}")
    print(f"SUCCESS: Backtest and prediction results saved to: {output_file}")
    print(f"{'='*80}")

    return all_results, final_predictions


if __name__ == '__main__':
    all_results, final_predictions = main()
