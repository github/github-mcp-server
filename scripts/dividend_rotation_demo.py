#!/usr/bin/env python3
"""
Dividend Rotation V4 Strategy - DEMO VERSION
Simplified version that uses hardcoded ETF data to demonstrate functionality
"""

import os
import sys
import json
import logging
import argparse
from datetime import datetime, date, timedelta
from dataclasses import dataclass
from typing import Dict, List, Optional, Set, Tuple
import pandas as pd
import numpy as np

# Configure logging
logging.basicConfig(
    format='%(asctime)s [%(levelname)s] %(message)s',
    datefmt='%Y-%m-%d %H:%M:%S',
    level=logging.INFO
)
logger = logging.getLogger(__name__)

BASE_URL = "https://eodhd.com/api"
EODHD_API_TOKEN = os.getenv("EODHD_API_TOKEN", "")
DEFAULT_START = "2024-08-01"
DEFAULT_END = "2024-11-11"

# ===== DEMO DATA: Hardcoded high-dividend ETFs =====
DEMO_ETFS = [
    {"ticker": "SCHD.US", "name": "Schwab US Dividend Equity ETF", "dividend_yield": 0.038, "avgvol": 3500000},
    {"ticker": "VYM.US", "name": "Vanguard High Dividend Yield ETF", "dividend_yield": 0.032, "avgvol": 1800000},
    {"ticker": "HDV.US", "name": "iShares Core High Dividend ETF", "dividend_yield": 0.035, "avgvol": 950000},
    {"ticker": "DGRO.US", "name": "iShares Core Dividend Growth ETF", "dividend_yield": 0.025, "avgvol": 2100000},
    {"ticker": "NOBL.US", "name": "ProShares S&P 500 Dividend Aristocrats ETF", "dividend_yield": 0.027, "avgvol": 850000},
    {"ticker": "SDIV.US", "name": "Global X SuperDividend U.S. ETF", "dividend_yield": 0.089, "avgvol": 650000},
    {"ticker": "JEPI.US", "name": "Janus Henderson Equity Premium Income ETF", "dividend_yield": 0.072, "avgvol": 5200000},
    {"ticker": "XYLD.US", "name": "Global X S&P 500 Covered Call ETF", "dividend_yield": 0.083, "avgvol": 3800000},
]

DEMO_PRICES = {
    "2024-08-01": {"SCHD.US": 92.5, "VYM.US": 118.3, "HDV.US": 108.2, "DGRO.US": 62.1, "NOBL.US": 94.5, "SDIV.US": 14.2, "JEPI.US": 56.8, "XYLD.US": 42.3},
    "2024-08-15": {"SCHD.US": 93.2, "VYM.US": 119.1, "HDV.US": 109.0, "DGRO.US": 62.8, "NOBL.US": 95.2, "SDIV.US": 14.3, "JEPI.US": 57.2, "XYLD.US": 42.8},
    "2024-09-01": {"SCHD.US": 94.1, "VYM.US": 120.5, "HDV.US": 110.5, "DGRO.US": 63.5, "NOBL.US": 96.1, "SDIV.US": 14.5, "JEPI.US": 58.1, "XYLD.US": 43.5},
    "2024-09-15": {"SCHD.US": 93.8, "VYM.US": 119.8, "HDV.US": 109.8, "DGRO.US": 63.2, "NOBL.US": 95.8, "SDIV.US": 14.4, "JEPI.US": 57.9, "XYLD.US": 43.2},
    "2024-10-01": {"SCHD.US": 95.2, "VYM.US": 121.8, "HDV.US": 111.8, "DGRO.US": 64.2, "NOBL.US": 97.2, "SDIV.US": 14.6, "JEPI.US": 58.8, "XYLD.US": 44.2},
    "2024-10-15": {"SCHD.US": 96.5, "VYM.US": 123.5, "HDV.US": 113.5, "DGRO.US": 65.1, "NOBL.US": 98.5, "SDIV.US": 14.8, "JEPI.US": 59.8, "XYLD.US": 45.1},
    "2024-11-01": {"SCHD.US": 97.2, "VYM.US": 124.8, "HDV.US": 114.8, "DGRO.US": 65.8, "NOBL.US": 99.2, "SDIV.US": 15.0, "JEPI.US": 60.5, "XYLD.US": 45.8},
    "2024-11-11": {"SCHD.US": 98.1, "VYM.US": 126.1, "HDV.US": 116.1, "DGRO.US": 66.5, "NOBL.US": 100.1, "SDIV.US": 15.2, "JEPI.US": 61.2, "XYLD.US": 46.5},
}

DEMO_DIVIDENDS = {
    "SCHD.US": [(date(2024, 8, 15), 0.65), (date(2024, 10, 15), 0.65)],
    "VYM.US": [(date(2024, 8, 10), 0.72), (date(2024, 11, 8), 0.72)],
    "HDV.US": [(date(2024, 8, 20), 0.75), (date(2024, 10, 22), 0.76)],
    "DGRO.US": [(date(2024, 8, 25), 0.39), (date(2024, 10, 25), 0.40)],
}

@dataclass
class Trade:
    ticker: str
    ex_date: date
    buy_date: date
    sell_date: date
    buy_price: float
    sell_price: float
    shares: float
    dividend_cash: float
    pnl: float


def normalize(x: float, x_min: float, x_max: float) -> float:
    """Normalize value to [0, 1] range."""
    if x_max == x_min:
        return 0.5
    return max(0.0, min(1.0, (x - x_min) / (x_max - x_min)))


def get_mock_price(ticker: str, price_date: str) -> float:
    """Get mock price from demo data."""
    # Find closest date in demo data
    for demo_date in sorted(DEMO_PRICES.keys(), reverse=True):
        if demo_date <= price_date:
            return DEMO_PRICES[demo_date].get(ticker, 100.0)
    return 100.0


def build_scored_candidates(topk: int, min_div: float) -> pd.DataFrame:
    """Build scored candidate ETFs from demo data."""
    logger.info(f"Building scored candidates (topk={topk}, min_div={min_div:.2%})")
    
    candidates = [e for e in DEMO_ETFS if e["dividend_yield"] >= min_div]
    logger.info(f"Found {len(candidates)} ETFs meeting min dividend yield filter")
    
    df = pd.DataFrame(candidates)
    df["score"] = df["dividend_yield"]
    df = df.nlargest(topk, "score")
    
    logger.info(f"Selected top {len(df)} ETFs by dividend yield")
    return df


def simulate_rotation(candidates: pd.DataFrame, initial_cash: float, start_date: str, end_date: str) -> Tuple[List[Trade], float]:
    """Simulate dividend rotation strategy."""
    logger.info(f"Simulating rotation from {start_date} to {end_date}")
    
    trades = []
    cash = initial_cash
    portfolio = {}
    
    # Simplified simulation: buy on start date, sell 30 days later
    start = datetime.strptime(start_date, "%Y-%m-%d").date()
    end = datetime.strptime(end_date, "%Y-%m-%d").date()
    
    # Buy phase
    for _, row in candidates.iterrows():
        ticker = row["ticker"]
        buy_price = get_mock_price(ticker, start_date)
        buy_shares = (cash / len(candidates)) / buy_price
        
        logger.info(f"Buy {buy_shares:.2f} shares of {ticker} at ${buy_price:.2f}")
        portfolio[ticker] = (buy_shares, buy_price)
        cash -= buy_shares * buy_price
    
    # Sell phase (30 days later)
    end_date_obj = start + timedelta(days=30)
    end_date_str = end_date_obj.strftime("%Y-%m-%d")
    
    for ticker, (shares, buy_price) in portfolio.items():
        sell_price = get_mock_price(ticker, end_date_str)
        dividend = DEMO_DIVIDENDS.get(ticker, [])
        dividend_cash = sum(d[1] for d in dividend if start <= d[0] <= end_date_obj) * shares
        
        pnl = (sell_price - buy_price) * shares + dividend_cash
        
        logger.info(f"Sell {shares:.2f} shares of {ticker} at ${sell_price:.2f}, dividend: ${dividend_cash:.2f}, P&L: ${pnl:.2f}")
        
        trades.append(Trade(
            ticker=ticker,
            ex_date=start,
            buy_date=start,
            sell_date=end_date_obj,
            buy_price=buy_price,
            sell_price=sell_price,
            shares=shares,
            dividend_cash=dividend_cash,
            pnl=pnl
        ))
        
        cash += shares * sell_price + dividend_cash
    
    return trades, cash


def export_csv(trades: List[Trade], final_cash: float, output_prefix: str) -> str:
    """Export results to CSV."""
    filename = f"{output_prefix}_results.csv"
    
    df = pd.DataFrame([
        {
            "Ticker": t.ticker,
            "Buy Date": t.buy_date,
            "Buy Price": f"${t.buy_price:.2f}",
            "Sell Date": t.sell_date,
            "Sell Price": f"${t.sell_price:.2f}",
            "Shares": f"{t.shares:.2f}",
            "Dividend": f"${t.dividend_cash:.2f}",
            "P&L": f"${t.pnl:.2f}",
        }
        for t in trades
    ])
    
    df.to_csv(filename, index=False)
    logger.info(f"Exported results to {filename}")
    
    # Print summary
    total_pnl = sum(t.pnl for t in trades)
    logger.info(f"Total P&L: ${total_pnl:.2f}")
    logger.info(f"Final Cash: ${final_cash:.2f}")
    logger.info(f"Total Return: {(total_pnl / 50000):.2%}")
    
    return filename


def main():
    parser = argparse.ArgumentParser(
        description="V4 High-Frequency Dividend Rotation Strategy (DEMO VERSION)",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  python dividend_rotation_demo.py --topk 3 --min-div-yield 0.02
  python dividend_rotation_demo.py --topk 5 --min-div-yield 0.015 --initial-cash 100000
        """
    )
    
    # Time & Capital
    parser.add_argument("--start", default=DEFAULT_START, help="Start date YYYY-MM-DD")
    parser.add_argument("--end", default=DEFAULT_END, help="End date YYYY-MM-DD")
    parser.add_argument("--initial-cash", type=float, default=50000.0, help="Initial capital in USD")
    
    # Screening
    parser.add_argument("--topk", type=int, default=3, help="Number of top candidates to select")
    parser.add_argument("--min-div-yield", type=float, default=0.02, help="Minimum dividend yield (e.g., 0.02 = 2 pct)")
    
    # Output
    parser.add_argument("--output-prefix", default="Dividend_Rotation_Demo", help="Output filename prefix")
    
    args = parser.parse_args()
    
    logger.info("=" * 70)
    logger.info("Dividend Rotation V4 Strategy - DEMO VERSION")
    logger.info("=" * 70)
    logger.info(f"Start Date: {args.start}")
    logger.info(f"End Date: {args.end}")
    logger.info(f"Initial Capital: ${args.initial_cash:.2f}")
    logger.info(f"Top K: {args.topk}")
    logger.info(f"Min Dividend Yield: {args.min_div_yield:.2%}")
    logger.info("=" * 70)
    
    # Build candidates
    candidates = build_scored_candidates(args.topk, args.min_div_yield)
    
    if candidates.empty:
        logger.error("No candidates found matching criteria")
        return 1
    
    logger.info(f"\nSelected Candidates:")
    for _, row in candidates.iterrows():
        logger.info(f"  {row['ticker']:12} | {row['name']:40} | Dividend: {row['dividend_yield']:6.2%} | Volume: {row['avgvol']:,}")
    
    # Simulate rotation
    trades, final_cash = simulate_rotation(candidates, args.initial_cash, args.start, args.end)
    
    # Export results
    logger.info("\n" + "=" * 70)
    logger.info("STRATEGY RESULTS")
    logger.info("=" * 70)
    csv_file = export_csv(trades, final_cash, args.output_prefix)
    
    logger.info(f"\nResults saved to: {csv_file}")
    logger.info("=" * 70)
    
    return 0


if __name__ == "__main__":
    sys.exit(main())
