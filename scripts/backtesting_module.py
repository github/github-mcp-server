#!/usr/bin/env python3
"""
Backtesting Module for Shanghai Gold Options Strategy
Tracks predictions vs actuals, calculates accuracy, evaluates strategy performance
"""

import json
import os
from datetime import datetime, timedelta
from typing import Dict, List, Tuple, Optional
import logging
import pandas as pd
import numpy as np

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


class StrategyBacktester:
    """
    Backtesting framework for Shanghai Gold options trading strategy
    Stores historical predictions, actuals, and strategy decisions
    """

    def __init__(self, data_dir: str = None):
        """Initialize backtester with data storage directory"""
        self.data_dir = data_dir or os.path.dirname(__file__)
        self.predictions_file = os.path.join(self.data_dir, 'backtest_predictions.json')
        self.actuals_file = os.path.join(self.data_dir, 'backtest_actuals.json')
        self.strategies_file = os.path.join(self.data_dir, 'backtest_strategies.json')
        self.metrics_file = os.path.join(self.data_dir, 'backtest_metrics.json')

        # Load existing data
        self.predictions = self._load_json(self.predictions_file, [])
        self.actuals = self._load_json(self.actuals_file, [])
        self.strategies = self._load_json(self.strategies_file, [])
        self.metrics = self._load_json(self.metrics_file, {})

    def _load_json(self, filepath: str, default):
        """Load JSON file or return default if not exists"""
        if os.path.exists(filepath):
            try:
                with open(filepath, 'r', encoding='utf-8') as f:
                    return json.load(f)
            except Exception as e:
                logger.warning(f"Error loading {filepath}: {e}")
        return default

    def _save_json(self, filepath: str, data):
        """Save data to JSON file"""
        try:
            with open(filepath, 'w', encoding='utf-8') as f:
                json.dump(data, f, indent=2, ensure_ascii=False)
            return True
        except Exception as e:
            logger.error(f"Error saving {filepath}: {e}")
            return False

    def log_prediction(self, timestamp: str, current_price: float, predicted_price: float,
                      horizon: str, factors: Dict = None):
        """
        Log a prediction for later validation

        Args:
            timestamp: ISO format timestamp of prediction
            current_price: Current gold price (CNY/gram) at prediction time
            predicted_price: Predicted future price
            horizon: Prediction horizon ('1d', '5d', etc.)
            factors: Dict of factor adjustments used in prediction
        """
        prediction_record = {
            'timestamp': timestamp,
            'current_price': current_price,
            'predicted_price': predicted_price,
            'horizon': horizon,
            'factors': factors or {},
            'validated': False
        }

        self.predictions.append(prediction_record)
        self._save_json(self.predictions_file, self.predictions)

        logger.info(f"Logged prediction: {predicted_price:.2f} CNY/g at {timestamp}")

    def log_actual(self, timestamp: str, actual_price: float, market_data: Dict = None):
        """
        Log actual market price for validation

        Args:
            timestamp: ISO format timestamp
            actual_price: Actual gold price (CNY/gram)
            market_data: Additional market data (DXY, VIX, etc.)
        """
        actual_record = {
            'timestamp': timestamp,
            'actual_price': actual_price,
            'market_data': market_data or {}
        }

        self.actuals.append(actual_record)
        self._save_json(self.actuals_file, self.actuals)

        logger.info(f"Logged actual: {actual_price:.2f} CNY/g at {timestamp}")

    def log_strategy_decision(self, timestamp: str, action: str, position: str,
                             price: float, reason: str, executed: bool = False):
        """
        Log a strategy decision (hold, exit, take profit, etc.)

        Args:
            timestamp: ISO format timestamp
            action: Action taken (HOLD, EXIT, TAKE_PROFIT, etc.)
            position: Position name
            price: Gold price at decision time
            reason: Reason for decision
            executed: Whether action was executed
        """
        strategy_record = {
            'timestamp': timestamp,
            'action': action,
            'position': position,
            'price': price,
            'reason': reason,
            'executed': executed
        }

        self.strategies.append(strategy_record)
        self._save_json(self.strategies_file, self.strategies)

        logger.info(f"Logged strategy: {action} for {position} at {price:.2f} CNY/g")

    def validate_predictions(self) -> Dict:
        """
        Validate predictions against actual prices
        Returns dict with accuracy metrics
        """
        validated_count = 0
        total_error = 0
        directional_correct = 0
        total_directional = 0

        for pred in self.predictions:
            if pred.get('validated'):
                continue

            # Find matching actual price
            pred_time = datetime.fromisoformat(pred['timestamp'])

            # Determine target time based on horizon
            if pred['horizon'] == '1d':
                target_time = pred_time + timedelta(days=1)
            elif pred['horizon'] == '5d':
                target_time = pred_time + timedelta(days=5)
            else:
                continue

            # Find closest actual within 6 hours of target
            closest_actual = None
            min_diff = timedelta(hours=6)

            for actual in self.actuals:
                actual_time = datetime.fromisoformat(actual['timestamp'])
                time_diff = abs(actual_time - target_time)

                if time_diff < min_diff:
                    min_diff = time_diff
                    closest_actual = actual

            if closest_actual:
                # Calculate error
                predicted = pred['predicted_price']
                actual = closest_actual['actual_price']
                current = pred['current_price']

                error_pct = abs((predicted - actual) / actual) * 100
                total_error += error_pct

                # Check directional accuracy
                predicted_direction = 'up' if predicted > current else 'down' if predicted < current else 'flat'
                actual_direction = 'up' if actual > current else 'down' if actual < current else 'flat'

                if predicted_direction == actual_direction:
                    directional_correct += 1
                total_directional += 1

                # Mark as validated
                pred['validated'] = True
                pred['actual_price'] = actual
                pred['error_pct'] = error_pct
                pred['directional_correct'] = (predicted_direction == actual_direction)

                validated_count += 1

        # Save updated predictions
        self._save_json(self.predictions_file, self.predictions)

        # Calculate metrics
        metrics = {
            'total_predictions': len(self.predictions),
            'validated_predictions': validated_count,
            'avg_error_pct': total_error / validated_count if validated_count > 0 else 0,
            'directional_accuracy': directional_correct / total_directional if total_directional > 0 else 0,
            'last_validation': datetime.now().isoformat()
        }

        # Update metrics file
        self.metrics.update(metrics)
        self._save_json(self.metrics_file, self.metrics)

        return metrics

    def calculate_strategy_performance(self) -> Dict:
        """
        Calculate strategy performance metrics
        Returns dict with win rate, avg profit, etc.
        """
        if not self.strategies:
            return {'error': 'No strategy decisions logged'}

        # Group by position
        position_results = {}

        for strategy in self.strategies:
            position = strategy['position']
            action = strategy['action']

            if position not in position_results:
                position_results[position] = {
                    'actions': [],
                    'hold_count': 0,
                    'exit_count': 0,
                    'profit_count': 0
                }

            position_results[position]['actions'].append(strategy)

            if 'HOLD' in action:
                position_results[position]['hold_count'] += 1
            elif 'EXIT' in action:
                position_results[position]['exit_count'] += 1
            elif 'PROFIT' in action:
                position_results[position]['profit_count'] += 1

        performance = {
            'positions': position_results,
            'total_decisions': len(self.strategies),
            'last_updated': datetime.now().isoformat()
        }

        return performance

    def generate_backtest_report(self) -> str:
        """Generate comprehensive backtest report in markdown format"""

        # Validate predictions first
        validation_metrics = self.validate_predictions()
        strategy_performance = self.calculate_strategy_performance()

        report = f"""# Backtesting Report

**Generated:** {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}

---

## 📊 Prediction Accuracy

### Overall Metrics
- **Total Predictions:** {validation_metrics.get('total_predictions', 0)}
- **Validated Predictions:** {validation_metrics.get('validated_predictions', 0)}
- **Average Error:** {validation_metrics.get('avg_error_pct', 0):.2f}%
- **Directional Accuracy:** {validation_metrics.get('directional_accuracy', 0)*100:.1f}%

### Performance Benchmark
"""

        # Add benchmark comparison
        avg_error = validation_metrics.get('avg_error_pct', 0)
        if avg_error < 1.0:
            report += "🟢 **Excellent** - Error < 1%\n"
        elif avg_error < 2.0:
            report += "🟡 **Good** - Error < 2% (Target range)\n"
        elif avg_error < 5.0:
            report += "🟠 **Fair** - Error < 5%\n"
        else:
            report += "🔴 **Needs Improvement** - Error > 5%\n"

        report += "\n---\n\n## 📈 Strategy Performance\n\n"

        # Add strategy metrics
        if 'positions' in strategy_performance:
            for position, data in strategy_performance['positions'].items():
                report += f"""### {position}
- Total Decisions: {len(data['actions'])}
- Hold Signals: {data['hold_count']}
- Exit Signals: {data['exit_count']}
- Profit Taking: {data['profit_count']}

"""

        report += f"""---

## 🔍 Recent Predictions

"""
        # Add last 10 validated predictions
        validated_preds = [p for p in self.predictions if p.get('validated')][-10:]

        if validated_preds:
            report += "| Date | Current | Predicted | Actual | Error % | Direction |\n"
            report += "|------|---------|-----------|--------|---------|----------|\n"

            for pred in reversed(validated_preds):
                date = pred['timestamp'][:10]
                current = pred['current_price']
                predicted = pred['predicted_price']
                actual = pred.get('actual_price', 0)
                error = pred.get('error_pct', 0)
                correct = '✅' if pred.get('directional_correct') else '❌'

                report += f"| {date} | {current:.2f} | {predicted:.2f} | {actual:.2f} | {error:.2f}% | {correct} |\n"
        else:
            report += "*No validated predictions yet*\n"

        report += """
---

## 💡 Insights & Recommendations

"""

        # Generate insights based on data
        dir_acc = validation_metrics.get('directional_accuracy', 0)
        if dir_acc > 0.6:
            report += "✅ **Directional Accuracy Strong** - Model correctly predicts price direction\n\n"
        else:
            report += "⚠️ **Directional Accuracy Weak** - Consider reviewing factor weights\n\n"

        if avg_error < 2.0:
            report += "✅ **Price Accuracy Good** - Predictions within target range\n\n"
        else:
            report += "⚠️ **Price Accuracy Needs Tuning** - Consider adjusting factor correlations\n\n"

        report += """---

## 📋 Next Steps

1. Continue logging daily predictions and actuals
2. Monitor directional accuracy trend
3. Adjust factor weights if error > 2% persists
4. Review strategy decisions against actual market outcomes
5. Implement deep learning enhancement for pattern recognition

---

*Backtest Module v1.0*
"""

        # Save report
        report_filename = f"backtest_report_{datetime.now().strftime('%Y%m%d')}.md"
        report_path = os.path.join(self.data_dir, report_filename)

        with open(report_path, 'w', encoding='utf-8') as f:
            f.write(report)

        logger.info(f"Backtest report saved to: {report_path}")

        return report

    def get_recommendation_for_model_tuning(self) -> Dict:
        """
        Analyze backtest results and provide recommendations for model tuning
        """
        validation_metrics = self.validate_predictions()

        recommendations = {
            'timestamp': datetime.now().isoformat(),
            'actions': []
        }

        # Check prediction accuracy
        avg_error = validation_metrics.get('avg_error_pct', 0)
        if avg_error > 2.0:
            recommendations['actions'].append({
                'priority': 'HIGH',
                'area': 'Factor Weights',
                'issue': f'Average error {avg_error:.2f}% exceeds 2% target',
                'recommendation': 'Reduce DXY correlation weight from 0.5 to 0.3 and test'
            })

        # Check directional accuracy
        dir_acc = validation_metrics.get('directional_accuracy', 0)
        if dir_acc < 0.55:
            recommendations['actions'].append({
                'priority': 'HIGH',
                'area': 'Directional Model',
                'issue': f'Directional accuracy {dir_acc*100:.1f}% below 55% threshold',
                'recommendation': 'Review ARIMA parameters and consider LSTM enhancement'
            })

        # Check validation rate
        validation_rate = validation_metrics.get('validated_predictions', 0) / max(validation_metrics.get('total_predictions', 1), 1)
        if validation_rate < 0.5:
            recommendations['actions'].append({
                'priority': 'MEDIUM',
                'area': 'Data Collection',
                'issue': f'Only {validation_rate*100:.1f}% of predictions validated',
                'recommendation': 'Increase frequency of actual price logging'
            })

        return recommendations


def main():
    """Test the backtesting module"""
    backtester = StrategyBacktester()

    # Example: Log a prediction
    backtester.log_prediction(
        timestamp=datetime.now().isoformat(),
        current_price=955.38,
        predicted_price=958.50,
        horizon='1d',
        factors={'DXY': 0.00281, 'VIX': 0, 'total': 0.00283}
    )

    # Example: Log an actual (would be done next day)
    backtester.log_actual(
        timestamp=(datetime.now() + timedelta(days=1)).isoformat(),
        actual_price=957.20,
        market_data={'DXY': 99.2, 'VIX': 17.5}
    )

    # Validate and generate report
    backtester.validate_predictions()
    report = backtester.generate_backtest_report()

    print(f"\n✅ Backtest report generated")
    print(f"\n📊 Sample Metrics: {backtester.metrics}")


if __name__ == '__main__':
    main()
