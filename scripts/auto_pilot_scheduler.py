#!/usr/bin/env python3
"""
Auto-Pilot Scheduler for Shanghai Gold Options Strategy
Runs daily strategy updates automatically at scheduled time
"""

import schedule
import time
import os
import sys
import logging
from datetime import datetime
import argparse

# Import our modules
import daily_strategy_engine
import backtesting_module
try:
    import deep_learning_enhancer
    DL_AVAILABLE = True
except ImportError:
    DL_AVAILABLE = False

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler('auto_pilot.log'),
        logging.StreamHandler()
    ]
)
logger = logging.getLogger(__name__)


class AutoPilotScheduler:
    """
    Automated scheduling system for daily strategy updates
    Runs prediction engine, generates reports, and performs backtesting
    """

    def __init__(self, api_key: str, run_time: str = "08:00", enable_dl: bool = False):
        """
        Initialize auto-pilot scheduler

        Args:
            api_key: EODHD API key
            run_time: Time to run daily update (HH:MM format, 24-hour)
            enable_dl: Enable deep learning enhancement
        """
        self.api_key = api_key
        self.run_time = run_time
        self.enable_dl = enable_dl and DL_AVAILABLE

        # Initialize components
        self.strategy_engine = None
        self.backtester = backtesting_module.StrategyBacktester()

        logger.info("=" * 60)
        logger.info("Auto-Pilot Scheduler Initialized")
        logger.info(f"Scheduled run time: {run_time}")
        logger.info(f"Deep learning: {'ENABLED' if self.enable_dl else 'DISABLED'}")
        logger.info("=" * 60)

    def daily_update_job(self):
        """Main job that runs daily - orchestrates all components"""

        logger.info("\n" + "=" * 60)
        logger.info(f"DAILY UPDATE JOB STARTED - {datetime.now()}")
        logger.info("=" * 60)

        try:
            # Step 1: Initialize strategy engine
            logger.info("\n[Step 1/6] Initializing strategy engine...")
            self.strategy_engine = daily_strategy_engine.ShanghaGoldOptionsStrategy(self.api_key)

            # Step 2: Get current market data
            logger.info("\n[Step 2/6] Fetching current market data...")
            market_data = self.strategy_engine.get_current_market_data()

            if not market_data or 'shanghai_gold_equivalent' not in market_data:
                logger.error("Failed to fetch market data. Aborting daily update.")
                return

            current_gold_cny = market_data['shanghai_gold_equivalent']
            logger.info(f"Current Shanghai Gold: CNY {current_gold_cny:.2f}/gram")

            # Log actual price for backtesting
            self.backtester.log_actual(
                timestamp=datetime.now().isoformat(),
                actual_price=current_gold_cny,
                market_data={
                    'XAUUSD': market_data.get('XAUUSD', {}).get('price', 0),
                    'USDCNY': market_data.get('USDCNY', {}).get('rate', 0)
                }
            )

            # Step 3: Run enhanced prediction engine
            logger.info("\n[Step 3/6] Running enhanced prediction engine...")

            # Use the strategy engine's enhanced prediction method
            predictions = self.strategy_engine.get_gold_predictions(current_gold_cny)
            final_prediction = predictions.get('predicted_1d', current_gold_cny * 1.001)

            # If deep learning enabled, blend with LSTM prediction
            if self.enable_dl:
                logger.info("Deep learning enhancement: Creating LSTM prediction...")
                try:
                    lstm_model = deep_learning_enhancer.GoldPriceLSTM()
                    if lstm_model.load_model():
                        lstm_pred = lstm_model.predict_next(current_gold_cny)
                        # Ensemble: 60% ARIMA/factors, 40% LSTM
                        final_prediction = final_prediction * 0.6 + lstm_pred * 0.4
                        logger.info(f"LSTM prediction: CNY {lstm_pred:.2f}, Ensemble: CNY {final_prediction:.2f}")
                except Exception as e:
                    logger.warning(f"LSTM prediction failed: {e}")

            logger.info(f"Predicted price (1-day): CNY {final_prediction:.2f}")
            logger.info(f"Prediction confidence: {predictions.get('confidence', 'N/A')}")

            # Log prediction for backtesting
            self.backtester.log_prediction(
                timestamp=datetime.now().isoformat(),
                current_price=current_gold_cny,
                predicted_price=final_prediction,
                horizon='1d'
            )

            # Step 4: Calculate position metrics and generate signals
            logger.info("\n[Step 4/6] Calculating position metrics...")
            position_metrics = self.strategy_engine.calculate_option_metrics(current_gold_cny)

            signals = self.strategy_engine.generate_trading_signals(
                current_gold_cny,
                final_prediction,
                position_metrics
            )

            # Log strategy decisions
            for action in signals['position_actions']:
                self.backtester.log_strategy_decision(
                    timestamp=datetime.now().isoformat(),
                    action=action['action'],
                    position=action['position'],
                    price=current_gold_cny,
                    reason=action['reason'],
                    executed=False
                )

            logger.info(f"Overall signal: {signals['overall_action']} (Risk: {signals['risk_level']})")

            # Step 5: Generate daily markdown report
            logger.info("\n[Step 5/6] Generating daily strategy report...")

            probabilities = self.strategy_engine.calculate_probabilities(current_gold_cny, final_prediction)

            predictions = {
                'predicted_1d': final_prediction,
                'predicted_change_pct': ((final_prediction - current_gold_cny) / current_gold_cny) * 100
            }

            report_path = self.generate_report(market_data, predictions, position_metrics, signals, probabilities)
            logger.info(f"Daily report saved: {report_path}")

            # Step 6: Run backtest validation
            logger.info("\n[Step 6/6] Running backtest validation...")
            validation_metrics = self.backtester.validate_predictions()

            logger.info(f"Backtest metrics: {validation_metrics.get('validated_predictions', 0)} validated, "
                       f"Avg error: {validation_metrics.get('avg_error_pct', 0):.2f}%, "
                       f"Directional accuracy: {validation_metrics.get('directional_accuracy', 0)*100:.1f}%")

            # Generate backtest report (weekly, only on Mondays)
            if datetime.now().weekday() == 0:  # Monday
                logger.info("Generating weekly backtest report...")
                backtest_report = self.backtester.generate_backtest_report()
                logger.info(f"Backtest report: {backtest_report}")

            logger.info("\n" + "=" * 60)
            logger.info(f"✅ DAILY UPDATE JOB COMPLETED - {datetime.now()}")
            logger.info("=" * 60 + "\n")

        except Exception as e:
            logger.error(f"❌ Error in daily update job: {e}")
            import traceback
            traceback.print_exc()

    def generate_report(self, market_data, predictions, position_metrics, signals, probabilities):
        """Generate and save the daily markdown report"""
        report = self.strategy_engine.generate_markdown_report(
            market_data,
            predictions,
            position_metrics,
            signals,
            probabilities
        )

        # Save report
        report_filename = f"daily_strategy_update_{datetime.now().strftime('%Y%m%d')}.md"
        report_path = os.path.join(self.strategy_engine.data_dir if hasattr(self.strategy_engine, 'data_dir') else os.path.dirname(__file__), report_filename)

        with open(report_path, 'w', encoding='utf-8') as f:
            f.write(report)

        return report_path

    def run_now(self):
        """Run the daily update immediately (for testing)"""
        logger.info("Running daily update immediately...")
        self.daily_update_job()

    def start(self):
        """Start the scheduler - runs continuously"""
        logger.info(f"Scheduler starting... Will run daily at {self.run_time}")

        # Schedule the daily job
        schedule.every().day.at(self.run_time).do(self.daily_update_job)

        # Run continuously
        try:
            while True:
                schedule.run_pending()
                time.sleep(60)  # Check every minute

        except KeyboardInterrupt:
            logger.info("\nScheduler stopped by user")
        except Exception as e:
            logger.error(f"Scheduler error: {e}")
            import traceback
            traceback.print_exc()


def main():
    """Main entry point for auto-pilot scheduler"""

    parser = argparse.ArgumentParser(description='Auto-Pilot Scheduler for Shanghai Gold Options')
    parser.add_argument('--api-key', help='EODHD API key (or set EODHD_API_KEY env var)')
    parser.add_argument('--time', default='08:00', help='Daily run time (HH:MM, 24-hour format)')
    parser.add_argument('--enable-dl', action='store_true', help='Enable deep learning enhancement')
    parser.add_argument('--run-now', action='store_true', help='Run immediately instead of scheduling')
    parser.add_argument('--test', action='store_true', help='Test mode - run once and exit')

    args = parser.parse_args()

    # Get API key - Default to hardcoded key if not provided
    DEFAULT_API_KEY = '690d7cdc3013f4.57364117'
    api_key = args.api_key or os.getenv('EODHD_API_KEY') or DEFAULT_API_KEY

    # Check deep learning availability
    if args.enable_dl and not DL_AVAILABLE:
        logger.warning("Deep learning requested but TensorFlow not installed")
        logger.warning("Install with: pip install tensorflow scikit-learn")
        logger.warning("Continuing without deep learning...")

    # Initialize scheduler
    scheduler = AutoPilotScheduler(
        api_key=api_key,
        run_time=args.time,
        enable_dl=args.enable_dl
    )

    # Run based on mode
    if args.run_now or args.test:
        # Run immediately
        scheduler.run_now()

        if args.test:
            print("\n✅ Test run complete")
            sys.exit(0)

    else:
        # Start continuous scheduler
        print(f"\n🤖 Auto-Pilot Mode Activated!")
        print(f"Daily updates scheduled for {args.time} (your local time)")
        print(f"Deep learning: {'ENABLED' if args.enable_dl and DL_AVAILABLE else 'DISABLED'}")
        print(f"\nPress Ctrl+C to stop\n")

        scheduler.start()


if __name__ == '__main__':
    main()
