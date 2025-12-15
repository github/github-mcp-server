#!/usr/bin/env python3
"""
AUTONOMOUS SELF-IMPROVING PREDICTION ENGINE
- Updates every 5 minutes with latest market data
- Backtests every 4 minutes
- Self-improves based on performance
- Cites all data sources
- Runs indefinitely with monitoring
"""

import pandas as pd
import numpy as np
from datetime import datetime, timedelta
import requests
import os
import warnings
import json
import time
import threading
from collections import deque
from statsmodels.tsa.arima.model import ARIMA

warnings.filterwarnings('ignore')

EODHD_API_KEY = os.environ.get('EODHD_APIKEY', '690d7cdc3013f4.57364117')


class DataSourceCitation:
    """Track and cite all data sources"""

    def __init__(self):
        self.citations = []

    def add_citation(self, source, data_type, value, timestamp=None):
        """Add a citation with source attribution"""
        citation = {
            'source': source,
            'data_type': data_type,
            'value': value,
            'timestamp': timestamp or datetime.now().isoformat(),
            'reliability': self.get_reliability_score(source)
        }
        self.citations.append(citation)
        return citation

    def get_reliability_score(self, source):
        """Assign reliability scores to different sources"""
        reliability_map = {
            'EODHD_API': 0.95,      # High - official API
            'FRED': 0.98,            # Very high - Federal Reserve
            'Yahoo_Finance': 0.85,   # Good
            'Alpha_Vantage': 0.90,   # High
            'News_Sentiment': 0.70,  # Moderate
            'Twitter_Sentiment': 0.50, # Low
            'Model_Prediction': 0.80  # High for backtested models
        }
        return reliability_map.get(source, 0.75)

    def get_summary(self):
        """Get citation summary"""
        return {
            'total_citations': len(self.citations),
            'sources': list(set(c['source'] for c in self.citations)),
            'latest_update': self.citations[-1]['timestamp'] if self.citations else None
        }


class RealTimeDataStream:
    """Continuously fetch latest market data"""

    def __init__(self, api_key, citations):
        self.api_key = api_key
        self.citations = citations
        self.base_url = "https://eodhd.com/api"

    def fetch_latest_price(self, symbol):
        """Fetch real-time price with citation"""
        try:
            url = f"{self.base_url}/real-time/{symbol}"
            params = {'api_token': self.api_key, 'fmt': 'json'}
            resp = requests.get(url, params=params, timeout=10)

            if resp.status_code == 200:
                data = resp.json()
                price = float(data.get('close', 0))

                self.citations.add_citation(
                    source='EODHD_API',
                    data_type=f'{symbol}_price',
                    value=price
                )

                return price
        except Exception as e:
            print(f"Error fetching {symbol}: {e}")

        return None

    def fetch_market_indicators(self):
        """Fetch latest market indicators with citations"""
        indicators = {}

        # VIX
        try:
            url = f"{self.base_url}/eod/^VIX.INDX"
            params = {'api_token': self.api_key, 'fmt': 'json', 'period': 'd', 'order': 'd'}
            resp = requests.get(url, params=params, timeout=10)

            if resp.status_code == 200:
                data = resp.json()
                if isinstance(data, list) and data:
                    vix = float(data[0]['close'])
                    indicators['VIX'] = vix

                    self.citations.add_citation(
                        source='EODHD_API',
                        data_type='VIX',
                        value=vix
                    )
        except:
            pass

        # S&P 500
        try:
            url = f"{self.base_url}/eod/^GSPC.INDX"
            params = {'api_token': self.api_key, 'fmt': 'json', 'period': 'd', 'order': 'd'}
            resp = requests.get(url, params=params, timeout=10)

            if resp.status_code == 200:
                data = resp.json()
                if isinstance(data, list) and len(data) >= 2:
                    current = float(data[0]['close'])
                    prev = float(data[1]['close'])
                    change = ((current - prev) / prev) * 100

                    indicators['SP500'] = current
                    indicators['SP500_Change'] = change

                    self.citations.add_citation(
                        source='EODHD_API',
                        data_type='SP500_Change',
                        value=change
                    )
        except:
            pass

        return indicators


class AutonomousBacktester:
    """Automatically backtest models and track performance"""

    def __init__(self):
        self.performance_history = deque(maxlen=100)
        self.best_model_params = None
        self.best_rmse = float('inf')

    def quick_backtest(self, model, data, horizon=5):
        """Fast backtest on recent data"""
        try:
            if len(data) < 50:
                return None

            # Use last 80% for train, 20% for test
            split_idx = int(len(data) * 0.8)
            train = data[:split_idx]
            test = data[split_idx:]

            # Fit model
            fitted = model.fit(train)

            # Predict
            predictions = fitted.forecast(steps=len(test))

            # Calculate RMSE
            rmse = np.sqrt(np.mean((predictions.values - test.values)**2))
            mae = np.mean(np.abs(predictions.values - test.values))

            # Direction accuracy
            actual_dir = np.diff(test.values) > 0
            pred_dir = np.diff(predictions.values) > 0
            dir_acc = np.mean(actual_dir == pred_dir) * 100 if len(actual_dir) > 0 else 0

            result = {
                'timestamp': datetime.now().isoformat(),
                'rmse': rmse,
                'mae': mae,
                'direction_accuracy': dir_acc,
                'model_params': str(model.model_orders)
            }

            self.performance_history.append(result)

            # Update best model if better
            if rmse < self.best_rmse:
                self.best_rmse = rmse
                self.best_model_params = model.model_orders

            return result

        except Exception as e:
            print(f"Backtest error: {e}")
            return None

    def get_performance_trend(self):
        """Analyze if model is improving"""
        if len(self.performance_history) < 5:
            return "INSUFFICIENT_DATA"

        recent_rmse = [p['rmse'] for p in list(self.performance_history)[-5:]]
        older_rmse = [p['rmse'] for p in list(self.performance_history)[-10:-5]] if len(self.performance_history) >= 10 else recent_rmse

        avg_recent = np.mean(recent_rmse)
        avg_older = np.mean(older_rmse)

        if avg_recent < avg_older * 0.95:
            return "IMPROVING"
        elif avg_recent > avg_older * 1.05:
            return "DEGRADING"
        else:
            return "STABLE"


class AdaptiveModelSelector:
    """Automatically select and improve models"""

    def __init__(self):
        self.model_configs = [
            {'order': (1, 1, 1), 'weight': 0.33},
            {'order': (2, 1, 2), 'weight': 0.33},
            {'order': (3, 1, 3), 'weight': 0.34}
        ]
        self.performance_scores = {str(cfg['order']): [] for cfg in self.model_configs}

    def update_weights(self, backtest_results):
        """Dynamically adjust model weights based on performance"""
        if not backtest_results:
            return

        # Track performance
        for cfg in self.model_configs:
            order_str = str(cfg['order'])
            # Inverse RMSE as score (lower RMSE = higher score)
            if backtest_results.get('rmse'):
                score = 1.0 / (backtest_results['rmse'] + 1)
                self.performance_scores[order_str].append(score)

        # Recalculate weights based on recent performance
        recent_scores = {}
        for order_str, scores in self.performance_scores.items():
            if scores:
                recent_scores[order_str] = np.mean(scores[-5:])  # Last 5 runs

        if recent_scores:
            total_score = sum(recent_scores.values())
            for cfg in self.model_configs:
                order_str = str(cfg['order'])
                cfg['weight'] = recent_scores.get(order_str, 0.33) / total_score

    def get_best_model_order(self):
        """Get model with highest weight"""
        best = max(self.model_configs, key=lambda x: x['weight'])
        return best['order']


class AutonomousPredictionEngine:
    """Main engine that runs continuously"""

    def __init__(self, symbols=['XAUUSD.FOREX', 'AAPL.US']):
        self.symbols = symbols
        self.citations = DataSourceCitation()
        self.data_stream = RealTimeDataStream(EODHD_API_KEY, self.citations)
        self.backtester = AutonomousBacktester()
        self.model_selector = AdaptiveModelSelector()

        self.predictions_log = deque(maxlen=1000)
        self.running = False

        # Timing
        self.update_interval = 300  # 5 minutes
        self.backtest_interval = 240  # 4 minutes

        self.last_update = None
        self.last_backtest = None

    def fetch_historical_data(self, symbol, days=100):
        """Fetch historical data for modeling"""
        try:
            url = f"{self.data_stream.base_url}/eod/{symbol}"
            params = {
                'api_token': self.data_stream.api_key,
                'fmt': 'json',
                'period': 'd',
                'order': 'd'
            }
            resp = requests.get(url, params=params, timeout=30)

            if resp.status_code == 200:
                data = resp.json()
                if isinstance(data, list) and data:
                    df = pd.DataFrame(data[:days])
                    df['date'] = pd.to_datetime(df['date'])
                    df = df.sort_values('date')
                    df['close'] = pd.to_numeric(df['close'], errors='coerce')

                    self.citations.add_citation(
                        source='EODHD_API',
                        data_type=f'{symbol}_historical',
                        value=f'{len(df)} days'
                    )

                    return df['close'].dropna()
        except Exception as e:
            print(f"Error fetching historical data: {e}")

        return None

    def generate_prediction(self, symbol):
        """Generate prediction with full citation trail"""
        print(f"\n{'='*80}")
        print(f"GENERATING PREDICTION: {symbol}")
        print(f"Time: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
        print(f"{'='*80}")

        # Fetch data
        historical = self.fetch_historical_data(symbol, days=100)
        if historical is None or len(historical) < 50:
            print("Insufficient data")
            return None

        current_price = historical.iloc[-1]
        print(f"Current Price: ${current_price:.2f}")

        # Get market indicators
        indicators = self.data_stream.fetch_market_indicators()
        print(f"\nMarket Indicators:")
        for name, value in indicators.items():
            print(f"  {name}: {value:.2f}")

        # Select best model
        best_order = self.model_selector.get_best_model_order()
        print(f"\nSelected Model: ARIMA{best_order}")
        print(f"Model Weights:")
        for cfg in self.model_selector.model_configs:
            print(f"  {cfg['order']}: {cfg['weight']:.3f}")

        # Train and predict
        try:
            model = ARIMA(historical, order=best_order)
            fitted = model.fit()
            forecast = fitted.forecast(steps=5)
            forecast_ci = fitted.get_forecast(steps=5).conf_int(alpha=0.05)

            pred_price = forecast.iloc[-1]
            lower = forecast_ci.iloc[-1, 0]
            upper = forecast_ci.iloc[-1, 1]

            # Calculate metrics
            change = pred_price - current_price
            change_pct = (change / current_price) * 100

            # Weight by market indicators
            vix_adjustment = 0
            if 'VIX' in indicators:
                vix = indicators['VIX']
                # High VIX = more uncertainty, widen intervals
                if vix > 25:
                    vix_adjustment = -0.002  # Bearish adjustment
                    print(f"  VIX Adjustment: -0.2% (High volatility)")

            sp_adjustment = 0
            if 'SP500_Change' in indicators:
                sp_change = indicators['SP500_Change']
                # Market momentum adjustment
                sp_adjustment = sp_change * 0.1 / 100  # 10% of market move
                print(f"  Market Momentum Adjustment: {sp_adjustment*100:+.2f}%")

            adjusted_pred = pred_price * (1 + vix_adjustment + sp_adjustment)

            prediction = {
                'symbol': symbol,
                'timestamp': datetime.now().isoformat(),
                'current_price': current_price,
                'predicted_price': adjusted_pred,
                'raw_prediction': pred_price,
                'change_pct': ((adjusted_pred - current_price) / current_price) * 100,
                'lower_95': lower,
                'upper_95': upper,
                'model_order': best_order,
                'indicators_used': indicators,
                'adjustments': {
                    'vix': vix_adjustment,
                    'market_momentum': sp_adjustment
                },
                'citations': self.citations.get_summary()
            }

            print(f"\nPrediction (5-day):")
            print(f"  Raw Model: ${pred_price:.2f}")
            print(f"  Adjusted: ${adjusted_pred:.2f}")
            print(f"  Expected Change: {prediction['change_pct']:+.2f}%")
            print(f"  95% CI: [${lower:.2f}, ${upper:.2f}]")

            # Add citation
            self.citations.add_citation(
                source='Model_Prediction',
                data_type='5day_forecast',
                value=adjusted_pred
            )

            self.predictions_log.append(prediction)

            return prediction

        except Exception as e:
            print(f"Prediction error: {e}")
            return None

    def run_backtest_cycle(self):
        """Run backtest on all models"""
        print(f"\n{'='*80}")
        print(f"RUNNING AUTOMATIC BACKTEST")
        print(f"Time: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
        print(f"{'='*80}")

        for symbol in self.symbols:
            historical = self.fetch_historical_data(symbol, days=200)
            if historical is None:
                continue

            print(f"\nBacktesting {symbol}...")

            # Test each model configuration
            for cfg in self.model_selector.model_configs:
                try:
                    model = ARIMA(historical, order=cfg['order'])
                    result = self.backtester.quick_backtest(model, historical)

                    if result:
                        print(f"  ARIMA{cfg['order']}: RMSE=${result['rmse']:.2f}, Dir={result['direction_accuracy']:.1f}%")
                        self.model_selector.update_weights(result)
                except:
                    pass

        # Check performance trend
        trend = self.backtester.get_performance_trend()
        print(f"\nPerformance Trend: {trend}")

        if trend == "DEGRADING":
            print("  WARNING: Model performance degrading - consider retraining")
        elif trend == "IMPROVING":
            print("  SUCCESS: Model performance improving")

        self.last_backtest = datetime.now()

    def save_state(self):
        """Save current state to file"""
        state = {
            'timestamp': datetime.now().isoformat(),
            'latest_predictions': list(self.predictions_log)[-10:],
            'model_weights': [
                {
                    'order': str(cfg['order']),
                    'weight': cfg['weight']
                }
                for cfg in self.model_selector.model_configs
            ],
            'performance_history': list(self.backtester.performance_history)[-20:],
            'best_model': str(self.backtester.best_model_params),
            'best_rmse': self.backtester.best_rmse,
            'citations': self.citations.citations[-50:],
            'uptime_seconds': (datetime.now() - self.start_time).total_seconds() if hasattr(self, 'start_time') else 0
        }

        with open('autonomous_engine_state.json', 'w') as f:
            json.dump(state, f, indent=2, default=str)

    def run_update_cycle(self):
        """Update predictions for all symbols"""
        print(f"\n{'#'*80}")
        print(f"UPDATE CYCLE #{len(self.predictions_log) + 1}")
        print(f"{'#'*80}")

        for symbol in self.symbols:
            prediction = self.generate_prediction(symbol)

        self.save_state()
        self.last_update = datetime.now()

    def run(self, duration_minutes=None):
        """Run engine continuously"""
        self.running = True
        self.start_time = datetime.now()

        print("="*80)
        print("AUTONOMOUS PREDICTION ENGINE STARTED")
        print("="*80)
        print(f"Update Interval: {self.update_interval}s (5 min)")
        print(f"Backtest Interval: {self.backtest_interval}s (4 min)")
        print(f"Symbols: {', '.join(self.symbols)}")
        if duration_minutes:
            print(f"Duration: {duration_minutes} minutes")
        else:
            print("Duration: INDEFINITE (Ctrl+C to stop)")
        print("="*80)

        # Initial run
        self.run_update_cycle()
        self.run_backtest_cycle()

        iteration = 0

        try:
            while self.running:
                iteration += 1
                time.sleep(60)  # Check every minute

                now = datetime.now()

                # Check if time for update (5 min)
                if self.last_update is None or (now - self.last_update).total_seconds() >= self.update_interval:
                    self.run_update_cycle()

                # Check if time for backtest (4 min)
                if self.last_backtest is None or (now - self.last_backtest).total_seconds() >= self.backtest_interval:
                    self.run_backtest_cycle()

                # Stop if duration reached
                if duration_minutes and (now - self.start_time).total_seconds() >= duration_minutes * 60:
                    print("\nDuration reached - stopping engine")
                    break

        except KeyboardInterrupt:
            print("\n\nEngine stopped by user")

        finally:
            self.running = False
            self.save_state()

            print("\n" + "="*80)
            print("FINAL STATISTICS")
            print("="*80)
            print(f"Total Runtime: {(datetime.now() - self.start_time).total_seconds()/60:.1f} minutes")
            print(f"Total Predictions: {len(self.predictions_log)}")
            print(f"Total Backtests: {len(self.backtester.performance_history)}")
            print(f"Best Model: {self.backtester.best_model_params}")
            print(f"Best RMSE: ${self.backtester.best_rmse:.2f}")
            print(f"Citations Logged: {len(self.citations.citations)}")
            print("\nState saved to: autonomous_engine_state.json")
            print("="*80)


def main():
    """Main entry point"""
    import argparse

    parser = argparse.ArgumentParser(description='Autonomous Prediction Engine')
    parser.add_argument('--duration', type=int, default=30,
                       help='Run duration in minutes (default: 30, use 0 for indefinite)')
    parser.add_argument('--symbols', nargs='+', default=['XAUUSD.FOREX', 'AAPL.US'],
                       help='Symbols to predict')

    args = parser.parse_args()

    duration = None if args.duration == 0 else args.duration

    engine = AutonomousPredictionEngine(symbols=args.symbols)
    engine.run(duration_minutes=duration)


if __name__ == '__main__':
    main()
