#!/usr/bin/env python3
"""
ENHANCED AUTONOMOUS SELF-IMPROVING PREDICTION ENGINE v2.0

NEW FEATURES:
1. Multiple forecast horizons (1, 5, 10, 20 days)
2. Alert system for significant changes
3. News sentiment integration
4. Industry trend analysis
5. Enhanced financial performance tracking
6. 24-hour+ operation optimized
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
            'EODHD_API': 0.95,
            'EODHD_News': 0.85,
            'EODHD_Fundamentals': 0.95,
            'FRED': 0.98,
            'Yahoo_Finance': 0.85,
            'Alpha_Vantage': 0.90,
            'News_Sentiment': 0.70,
            'Model_Prediction': 0.80
        }
        return reliability_map.get(source, 0.75)

    def get_summary(self):
        """Get citation summary"""
        return {
            'total_citations': len(self.citations),
            'sources': list(set(c['source'] for c in self.citations)),
            'latest_update': self.citations[-1]['timestamp'] if self.citations else None
        }


class NewsDataFetcher:
    """Fetch and analyze news sentiment"""

    def __init__(self, api_key, citations):
        self.api_key = api_key
        self.citations = citations
        self.base_url = "https://eodhd.com/api"

    def fetch_recent_news(self, symbol, days=5):
        """Fetch recent news for symbol"""
        try:
            # Extract ticker without exchange
            ticker = symbol.split('.')[0]

            end_date = datetime.now()
            start_date = end_date - timedelta(days=days)

            url = f"{self.base_url}/news"
            params = {
                'api_token': self.api_key,
                's': ticker,
                'from': start_date.strftime('%Y-%m-%d'),
                'to': end_date.strftime('%Y-%m-%d'),
                'limit': 50
            }

            resp = requests.get(url, params=params, timeout=15)

            if resp.status_code == 200:
                news_data = resp.json()

                self.citations.add_citation(
                    source='EODHD_News',
                    data_type=f'{symbol}_news',
                    value=f'{len(news_data)} articles'
                )

                return news_data

        except Exception as e:
            print(f"  News fetch error: {e}")

        return []

    def analyze_sentiment(self, news_list):
        """Simple sentiment analysis"""
        if not news_list:
            return {'score': 0, 'count': 0}

        # Simple keyword-based sentiment
        positive_keywords = ['growth', 'profit', 'beat', 'upgrade', 'rise', 'gain',
                           'strong', 'success', 'record', 'high', 'boost', 'rally']
        negative_keywords = ['loss', 'miss', 'downgrade', 'fall', 'decline', 'weak',
                           'concern', 'cut', 'layoff', 'lawsuit', 'investigation', 'drop']

        sentiment_score = 0
        for article in news_list:
            title = article.get('title', '').lower()
            content = article.get('content', '').lower() if article.get('content') else ''
            text = title + ' ' + content

            # Count keywords
            pos_count = sum(1 for word in positive_keywords if word in text)
            neg_count = sum(1 for word in negative_keywords if word in text)

            sentiment_score += (pos_count - neg_count)

        # Normalize to -1 to +1 range
        normalized = np.tanh(sentiment_score / max(len(news_list), 1))

        return {
            'score': normalized,
            'count': len(news_list),
            'articles': news_list[:5]  # Keep top 5
        }


class IndustryTrendAnalyzer:
    """Analyze industry/sector trends"""

    def __init__(self, api_key, citations):
        self.api_key = api_key
        self.citations = citations
        self.base_url = "https://eodhd.com/api"

        # Map symbols to sector ETFs
        self.sector_map = {
            'AAPL.US': 'XLK.US',      # Technology
            'MSFT.US': 'XLK.US',
            'GOOGL.US': 'XLK.US',
            'XAUUSD.FOREX': 'GLD.US', # Gold ETF
        }

    def get_sector_etf(self, symbol):
        """Get relevant sector ETF for symbol"""
        return self.sector_map.get(symbol, 'SPY.US')  # Default to S&P 500

    def fetch_sector_performance(self, symbol, days=20):
        """Fetch sector performance trend"""
        try:
            sector_etf = self.get_sector_etf(symbol)

            url = f"{self.base_url}/eod/{sector_etf}"
            params = {
                'api_token': self.api_key,
                'fmt': 'json',
                'period': 'd',
                'order': 'd'
            }

            resp = requests.get(url, params=params, timeout=10)

            if resp.status_code == 200:
                data = resp.json()
                if isinstance(data, list) and len(data) >= days:
                    prices = [float(d['close']) for d in data[:days]]

                    # Calculate momentum
                    recent_return = (prices[0] - prices[4]) / prices[4] if len(prices) > 4 else 0
                    medium_return = (prices[0] - prices[9]) / prices[9] if len(prices) > 9 else 0
                    long_return = (prices[0] - prices[-1]) / prices[-1]

                    # Calculate trend strength
                    trend_strength = np.mean([recent_return, medium_return, long_return])

                    self.citations.add_citation(
                        source='EODHD_API',
                        data_type=f'{symbol}_sector_trend',
                        value=trend_strength
                    )

                    return {
                        'sector_etf': sector_etf,
                        'recent_return': recent_return * 100,
                        'medium_return': medium_return * 100,
                        'long_return': long_return * 100,
                        'trend_strength': trend_strength * 100
                    }

        except Exception as e:
            print(f"  Sector analysis error: {e}")

        return None


class EnhancedFundamentalsTracker:
    """Track comprehensive financial performance"""

    def __init__(self, api_key, citations):
        self.api_key = api_key
        self.citations = citations
        self.base_url = "https://eodhd.com/api"

    def fetch_fundamentals(self, symbol):
        """Fetch comprehensive fundamentals"""
        try:
            # Only for equity symbols
            if not symbol.endswith('.US'):
                return None

            url = f"{self.base_url}/fundamentals/{symbol}"
            params = {'api_token': self.api_key}

            resp = requests.get(url, params=params, timeout=15)

            if resp.status_code == 200:
                data = resp.json()

                fundamentals = {}

                # Valuation
                highlights = data.get('Highlights', {})
                fundamentals['pe_ratio'] = highlights.get('PERatio')
                fundamentals['eps'] = highlights.get('EarningsShare')
                fundamentals['market_cap'] = highlights.get('MarketCapitalization')
                fundamentals['dividend_yield'] = highlights.get('DividendYield')

                # Analyst ratings
                ratings = data.get('AnalystRatings', {})
                fundamentals['analyst_rating'] = ratings.get('Rating')
                fundamentals['target_price'] = ratings.get('TargetPrice')

                # Earnings trends
                earnings = data.get('Earnings', {})
                if 'History' in earnings and earnings['History']:
                    recent_earnings = earnings['History'][:4]  # Last 4 quarters

                    # Calculate earnings growth
                    if len(recent_earnings) >= 2:
                        latest = recent_earnings[0].get('epsActual', 0)
                        previous = recent_earnings[1].get('epsActual', 0)
                        fundamentals['earnings_growth'] = ((latest - previous) / abs(previous)) * 100 if previous else 0

                # Financial statements
                financials = data.get('Financials', {})
                if 'Income_Statement' in financials and financials['Income_Statement']:
                    latest_income = financials['Income_Statement'].get('quarterly', {})
                    if latest_income:
                        latest_quarter = list(latest_income.values())[0]
                        fundamentals['revenue'] = latest_quarter.get('totalRevenue')
                        fundamentals['net_income'] = latest_quarter.get('netIncome')

                self.citations.add_citation(
                    source='EODHD_Fundamentals',
                    data_type=f'{symbol}_fundamentals',
                    value='comprehensive'
                )

                return fundamentals

        except Exception as e:
            print(f"  Fundamentals fetch error: {e}")

        return None


class AlertManager:
    """Manage alerts for significant prediction changes"""

    def __init__(self, threshold_pct=1.0):
        self.threshold = threshold_pct
        self.previous_predictions = {}
        self.alerts = deque(maxlen=100)

    def check_for_alert(self, symbol, current_prediction, previous_prediction):
        """Check if prediction changed significantly"""
        if previous_prediction is None:
            return None

        curr_price = current_prediction.get('predicted_price', 0)
        prev_price = previous_prediction.get('predicted_price', 0)

        if prev_price == 0:
            return None

        change_pct = abs((curr_price - prev_price) / prev_price) * 100

        if change_pct >= self.threshold:
            alert = {
                'timestamp': datetime.now().isoformat(),
                'symbol': symbol,
                'change_pct': change_pct,
                'previous_prediction': prev_price,
                'current_prediction': curr_price,
                'message': f'ALERT: {symbol} prediction changed by {change_pct:.2f}%'
            }

            self.alerts.append(alert)
            return alert

        return None

    def update_predictions(self, symbol, prediction):
        """Update stored predictions"""
        self.previous_predictions[symbol] = prediction


class RealTimeDataStream:
    """Enhanced data stream with multiple sources"""

    def __init__(self, api_key, citations):
        self.api_key = api_key
        self.citations = citations
        self.base_url = "https://eodhd.com/api"

    def fetch_latest_price(self, symbol):
        """Fetch real-time price"""
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
            print(f"  Error fetching {symbol}: {e}")

        return None

    def fetch_market_indicators(self):
        """Fetch market indicators"""
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
    """Backtest with multiple horizons"""

    def __init__(self):
        self.performance_history = deque(maxlen=100)
        self.best_model_params = {}
        self.best_rmse = {}

    def quick_backtest(self, model, data, horizon=5):
        """Fast backtest"""
        try:
            if len(data) < 50:
                return None

            split_idx = int(len(data) * 0.8)
            train = data[:split_idx]
            test = data[split_idx:]

            fitted = model.fit(train)
            predictions = fitted.forecast(steps=min(horizon, len(test)))

            actual = test.values[:len(predictions)]
            pred = predictions.values

            rmse = np.sqrt(np.mean((pred - actual)**2))
            mae = np.mean(np.abs(pred - actual))

            # Direction accuracy
            if len(actual) > 1:
                actual_dir = np.diff(actual) > 0
                pred_dir = np.diff(pred) > 0
                dir_acc = np.mean(actual_dir == pred_dir) * 100
            else:
                dir_acc = 0

            result = {
                'timestamp': datetime.now().isoformat(),
                'rmse': rmse,
                'mae': mae,
                'direction_accuracy': dir_acc,
                'model_params': str(model.model_orders),
                'horizon': horizon
            }

            self.performance_history.append(result)

            # Update best model for this horizon
            if horizon not in self.best_rmse or rmse < self.best_rmse[horizon]:
                self.best_rmse[horizon] = rmse
                self.best_model_params[horizon] = model.model_orders

            return result

        except Exception as e:
            print(f"  Backtest error: {e}")
            return None

    def get_performance_trend(self):
        """Check if improving"""
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
    """Model selection with horizon-specific weights"""

    def __init__(self):
        self.model_configs = [
            {'order': (1, 1, 1), 'weight': 0.33},
            {'order': (2, 1, 2), 'weight': 0.33},
            {'order': (3, 1, 3), 'weight': 0.34}
        ]
        self.performance_scores = {str(cfg['order']): [] for cfg in self.model_configs}

    def update_weights(self, backtest_results):
        """Update weights based on performance"""
        if not backtest_results or not backtest_results.get('rmse'):
            return

        for cfg in self.model_configs:
            order_str = str(cfg['order'])
            score = 1.0 / (backtest_results['rmse'] + 1)
            self.performance_scores[order_str].append(score)

        recent_scores = {}
        for order_str, scores in self.performance_scores.items():
            if scores:
                recent_scores[order_str] = np.mean(scores[-5:])

        if recent_scores:
            total_score = sum(recent_scores.values())
            for cfg in self.model_configs:
                order_str = str(cfg['order'])
                cfg['weight'] = recent_scores.get(order_str, 0.33) / total_score

    def get_best_model_order(self):
        """Get best model"""
        best = max(self.model_configs, key=lambda x: x['weight'])
        return best['order']


class EnhancedAutonomousEngine:
    """Enhanced engine with all new features"""

    def __init__(self, symbols=['XAUUSD.FOREX', 'AAPL.US']):
        self.symbols = symbols
        self.citations = DataSourceCitation()
        self.data_stream = RealTimeDataStream(EODHD_API_KEY, self.citations)
        self.backtester = AutonomousBacktester()
        self.model_selector = AdaptiveModelSelector()

        # New components
        self.news_fetcher = NewsDataFetcher(EODHD_API_KEY, self.citations)
        self.industry_analyzer = IndustryTrendAnalyzer(EODHD_API_KEY, self.citations)
        self.fundamentals_tracker = EnhancedFundamentalsTracker(EODHD_API_KEY, self.citations)
        self.alert_manager = AlertManager(threshold_pct=1.0)

        self.predictions_log = deque(maxlen=1000)
        self.running = False

        # Forecast horizons
        self.horizons = [1, 5, 10, 20]

        # Timing
        self.update_interval = 300  # 5 minutes
        self.backtest_interval = 240  # 4 minutes

        self.last_update = None
        self.last_backtest = None

    def fetch_historical_data(self, symbol, days=100):
        """Fetch historical data"""
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
            print(f"  Error fetching historical: {e}")

        return None

    def generate_multi_horizon_prediction(self, symbol):
        """Generate predictions for multiple horizons"""
        print(f"\n{'='*80}")
        print(f"MULTI-HORIZON PREDICTION: {symbol}")
        print(f"Time: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
        print(f"{'='*80}")

        # Fetch data
        historical = self.fetch_historical_data(symbol, days=200)
        if historical is None or len(historical) < 50:
            print("Insufficient data")
            return None

        current_price = historical.iloc[-1]
        print(f"Current Price: ${current_price:.2f}")

        # Fetch all contextual data
        print(f"\n--- Contextual Analysis ---")

        # Market indicators
        indicators = self.data_stream.fetch_market_indicators()
        if indicators:
            print(f"Market Indicators:")
            for name, value in indicators.items():
                print(f"  {name}: {value:.2f}")

        # News sentiment
        news = self.news_fetcher.fetch_recent_news(symbol, days=5)
        sentiment = self.news_fetcher.analyze_sentiment(news)
        print(f"\nNews Sentiment (last 5 days):")
        print(f"  Articles: {sentiment['count']}")
        print(f"  Sentiment Score: {sentiment['score']:+.2f} (-1 bearish, +1 bullish)")

        # Industry trends
        sector_trend = self.industry_analyzer.fetch_sector_performance(symbol)
        if sector_trend:
            print(f"\nIndustry Trend ({sector_trend['sector_etf']}):")
            print(f"  5-day: {sector_trend['recent_return']:+.2f}%")
            print(f"  10-day: {sector_trend['medium_return']:+.2f}%")
            print(f"  20-day: {sector_trend['long_return']:+.2f}%")
            print(f"  Trend Strength: {sector_trend['trend_strength']:+.2f}%")

        # Fundamentals
        fundamentals = self.fundamentals_tracker.fetch_fundamentals(symbol)
        if fundamentals:
            print(f"\nFundamentals:")
            if fundamentals.get('pe_ratio'):
                print(f"  P/E Ratio: {fundamentals['pe_ratio']:.2f}")
            if fundamentals.get('eps'):
                print(f"  EPS: ${fundamentals['eps']:.2f}")
            if fundamentals.get('target_price'):
                print(f"  Analyst Target: ${fundamentals['target_price']:.2f}")
            if fundamentals.get('earnings_growth'):
                print(f"  Earnings Growth: {fundamentals['earnings_growth']:+.2f}%")

        # Generate predictions for each horizon
        print(f"\n--- Multi-Horizon Forecasts ---")

        predictions = {}
        best_order = self.model_selector.get_best_model_order()

        for horizon in self.horizons:
            try:
                model = ARIMA(historical, order=best_order)
                fitted = model.fit()
                forecast = fitted.forecast(steps=horizon)
                forecast_ci = fitted.get_forecast(steps=horizon).conf_int(alpha=0.05)

                pred_price = forecast.iloc[-1]
                lower = forecast_ci.iloc[-1, 0]
                upper = forecast_ci.iloc[-1, 1]

                # Apply adjustments
                adjustment = 0

                # VIX adjustment
                if 'VIX' in indicators:
                    vix = indicators['VIX']
                    if vix > 25:
                        adjustment -= 0.002

                # Market momentum
                if 'SP500_Change' in indicators:
                    sp_change = indicators['SP500_Change']
                    adjustment += sp_change * 0.1 / 100

                # News sentiment
                adjustment += sentiment['score'] * 0.005

                # Industry trend
                if sector_trend:
                    adjustment += sector_trend['trend_strength'] * 0.001

                adjusted_pred = pred_price * (1 + adjustment)

                predictions[f'{horizon}day'] = {
                    'horizon_days': horizon,
                    'predicted_price': adjusted_pred,
                    'raw_prediction': pred_price,
                    'change_pct': ((adjusted_pred - current_price) / current_price) * 100,
                    'lower_95': lower,
                    'upper_95': upper,
                    'adjustment': adjustment * 100
                }

                print(f"\n{horizon}-Day Forecast:")
                print(f"  Predicted: ${adjusted_pred:.2f}")
                print(f"  Change: {predictions[f'{horizon}day']['change_pct']:+.2f}%")
                print(f"  95% CI: [${lower:.2f}, ${upper:.2f}]")
                print(f"  Adjustments: {adjustment*100:+.3f}%")

            except Exception as e:
                print(f"  Error for {horizon}-day: {e}")
                predictions[f'{horizon}day'] = None

        # Create comprehensive prediction object
        prediction_obj = {
            'symbol': symbol,
            'timestamp': datetime.now().isoformat(),
            'current_price': current_price,
            'predictions': predictions,
            'model_order': best_order,
            'context': {
                'market_indicators': indicators,
                'news_sentiment': {
                    'score': sentiment['score'],
                    'article_count': sentiment['count']
                },
                'sector_trend': sector_trend,
                'fundamentals': fundamentals
            },
            'citations': self.citations.get_summary()
        }

        # Check for alerts
        prev_pred = self.alert_manager.previous_predictions.get(symbol)
        if prev_pred and predictions.get('5day'):
            alert = self.alert_manager.check_for_alert(symbol, predictions['5day'],
                                                      prev_pred.get('predictions', {}).get('5day'))
            if alert:
                print(f"\n{'!'*80}")
                print(f"ALERT: {alert['message']}")
                print(f"{'!'*80}")

        self.alert_manager.update_predictions(symbol, prediction_obj)
        self.predictions_log.append(prediction_obj)

        return prediction_obj

    def run_backtest_cycle(self):
        """Run backtests"""
        print(f"\n{'='*80}")
        print(f"AUTOMATIC BACKTEST CYCLE")
        print(f"Time: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
        print(f"{'='*80}")

        for symbol in self.symbols:
            historical = self.fetch_historical_data(symbol, days=200)
            if historical is None:
                continue

            print(f"\nBacktesting {symbol}...")

            for cfg in self.model_selector.model_configs:
                try:
                    model = ARIMA(historical, order=cfg['order'])
                    # Backtest for 5-day horizon (primary)
                    result = self.backtester.quick_backtest(model, historical, horizon=5)

                    if result:
                        print(f"  ARIMA{cfg['order']}: RMSE=${result['rmse']:.2f}, Dir={result['direction_accuracy']:.1f}%")
                        self.model_selector.update_weights(result)
                except:
                    pass

        trend = self.backtester.get_performance_trend()
        print(f"\nPerformance Trend: {trend}")

        if trend == "DEGRADING":
            print("  WARNING: Consider retraining")
        elif trend == "IMPROVING":
            print("  SUCCESS: Performance improving")

        self.last_backtest = datetime.now()

    def save_state(self):
        """Save state"""
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
            'best_models_by_horizon': {str(k): str(v) for k, v in self.backtester.best_model_params.items()},
            'best_rmse_by_horizon': {str(k): v for k, v in self.backtester.best_rmse.items()},
            'alerts': list(self.alert_manager.alerts)[-20:],
            'citations': self.citations.citations[-50:],
            'uptime_seconds': (datetime.now() - self.start_time).total_seconds() if hasattr(self, 'start_time') else 0
        }

        with open('enhanced_engine_state.json', 'w') as f:
            json.dump(state, f, indent=2, default=str)

    def run_update_cycle(self):
        """Update cycle"""
        print(f"\n{'#'*80}")
        print(f"UPDATE CYCLE #{len(self.predictions_log) + 1}")
        print(f"{'#'*80}")

        for symbol in self.symbols:
            prediction = self.generate_multi_horizon_prediction(symbol)

        self.save_state()
        self.last_update = datetime.now()

    def run(self, duration_minutes=None):
        """Run engine"""
        self.running = True
        self.start_time = datetime.now()

        print("="*80)
        print("ENHANCED AUTONOMOUS PREDICTION ENGINE v2.0")
        print("="*80)
        print(f"Forecast Horizons: {', '.join([str(h) + 'd' for h in self.horizons])}")
        print(f"Update Interval: {self.update_interval}s (5 min)")
        print(f"Backtest Interval: {self.backtest_interval}s (4 min)")
        print(f"Symbols: {', '.join(self.symbols)}")
        print(f"Features: News Sentiment, Industry Trends, Alerts")
        if duration_minutes:
            print(f"Duration: {duration_minutes} minutes")
        else:
            print("Duration: INDEFINITE (24/7 mode)")
        print("="*80)

        # Initial run
        self.run_update_cycle()
        self.run_backtest_cycle()

        try:
            while self.running:
                time.sleep(60)

                now = datetime.now()

                if self.last_update is None or (now - self.last_update).total_seconds() >= self.update_interval:
                    self.run_update_cycle()

                if self.last_backtest is None or (now - self.last_backtest).total_seconds() >= self.backtest_interval:
                    self.run_backtest_cycle()

                if duration_minutes and (now - self.start_time).total_seconds() >= duration_minutes * 60:
                    print("\nDuration reached")
                    break

        except KeyboardInterrupt:
            print("\n\nStopped by user")

        finally:
            self.running = False
            self.save_state()

            print("\n" + "="*80)
            print("FINAL STATISTICS")
            print("="*80)
            print(f"Runtime: {(datetime.now() - self.start_time).total_seconds()/60:.1f} minutes")
            print(f"Predictions: {len(self.predictions_log)}")
            print(f"Backtests: {len(self.backtester.performance_history)}")
            print(f"Alerts: {len(self.alert_manager.alerts)}")
            print(f"Citations: {len(self.citations.citations)}")
            print("\nState saved to: enhanced_engine_state.json")
            print("="*80)


def main():
    """Entry point"""
    import argparse

    parser = argparse.ArgumentParser(description='Enhanced Autonomous Engine v2.0')
    parser.add_argument('--duration', type=int, default=0,
                       help='Duration in minutes (0 for indefinite)')
    parser.add_argument('--symbols', nargs='+', default=['XAUUSD.FOREX', 'AAPL.US'],
                       help='Symbols to predict')

    args = parser.parse_args()

    duration = None if args.duration == 0 else args.duration

    engine = EnhancedAutonomousEngine(symbols=args.symbols)
    engine.run(duration_minutes=duration)


if __name__ == '__main__':
    main()
