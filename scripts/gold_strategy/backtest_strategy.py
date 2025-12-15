#!/usr/bin/env python3
"""
Backtest Gold Options Strategy using EODHD Historical Data
Tests Bull Call Spread strategy performance over historical periods
"""

import pandas as pd
import numpy as np
from scipy.stats import norm
from datetime import datetime, timedelta
import json

def load_historical_data():
    """Load historical gold data"""
    df = pd.read_csv('gold_historical_with_indicators.csv', parse_dates=['date'])
    df.set_index('date', inplace=True)
    return df

def black_scholes_call(S, K, T, r, sigma):
    """Calculate call option price"""
    if T <= 0:
        return max(S - K, 0)
    d1 = (np.log(S / K) + (r + 0.5 * sigma**2) * T) / (sigma * np.sqrt(T))
    d2 = d1 - sigma * np.sqrt(T)
    return S * norm.cdf(d1) - K * np.exp(-r * T) * norm.cdf(d2)

def calculate_historical_volatility(df, window=20):
    """Calculate rolling historical volatility"""
    returns = df['close'].pct_change()
    vol = returns.rolling(window=window).std() * np.sqrt(252)
    return vol

def backtest_bull_call_spread(df, holding_period=15, lower_pct=0.98, upper_pct=1.03):
    """
    Backtest bull call spread strategy
    Enter at beginning of each period, hold for specified days
    """
    results = []

    # Calculate volatility for option pricing
    df['hist_vol'] = calculate_historical_volatility(df, 20)

    # Skip first 252 days for proper indicator calculation
    start_idx = 252

    # Risk-free rate assumption
    r = 0.02

    print(f"Running backtest from {df.index[start_idx]} to {df.index[-1]}")
    print(f"Holding period: {holding_period} days")
    print(f"Lower strike: {lower_pct*100:.0f}% of spot")
    print(f"Upper strike: {upper_pct*100:.0f}% of spot")
    print("="*60)

    # Test every month (approximately every 21 trading days)
    test_dates = list(range(start_idx, len(df) - holding_period, 21))

    for i in test_dates:
        entry_date = df.index[i]
        exit_date = df.index[i + holding_period]

        # Entry prices
        entry_price = df['close'].iloc[i]
        exit_price = df['close'].iloc[i + holding_period]

        # Strike prices
        lower_strike = entry_price * lower_pct
        upper_strike = entry_price * upper_pct

        # Volatility at entry
        vol = df['hist_vol'].iloc[i]
        if pd.isna(vol):
            vol = 0.15  # default

        # Time to expiry in years
        T = holding_period / 365

        # Option prices at entry
        long_call_entry = black_scholes_call(entry_price, lower_strike, T, r, vol)
        short_call_entry = black_scholes_call(entry_price, upper_strike, T, r, vol)
        spread_cost = long_call_entry - short_call_entry

        # Payoff at expiry
        long_call_payoff = max(exit_price - lower_strike, 0)
        short_call_payoff = -max(exit_price - upper_strike, 0)
        total_payoff = long_call_payoff + short_call_payoff

        # P&L
        pl = total_payoff - spread_cost
        roi = (pl / spread_cost) * 100 if spread_cost > 0 else 0

        # Underlying return
        underlying_return = ((exit_price / entry_price) - 1) * 100

        results.append({
            'entry_date': entry_date,
            'exit_date': exit_date,
            'entry_price': entry_price,
            'exit_price': exit_price,
            'underlying_return': underlying_return,
            'lower_strike': lower_strike,
            'upper_strike': upper_strike,
            'spread_cost': spread_cost,
            'payoff': total_payoff,
            'pl': pl,
            'roi': roi,
            'volatility': vol
        })

    return pd.DataFrame(results)

def analyze_backtest_results(results_df):
    """Analyze backtest performance"""
    print("\n" + "="*60)
    print("BACKTEST RESULTS ANALYSIS")
    print("="*60)

    total_trades = len(results_df)
    winning_trades = len(results_df[results_df['pl'] > 0])
    losing_trades = len(results_df[results_df['pl'] <= 0])

    win_rate = (winning_trades / total_trades) * 100

    avg_roi = results_df['roi'].mean()
    median_roi = results_df['roi'].median()
    std_roi = results_df['roi'].std()

    max_profit = results_df['roi'].max()
    max_loss = results_df['roi'].min()

    # Calculate Sharpe-like ratio (assuming 0 risk-free rate for simplicity)
    sharpe = avg_roi / std_roi if std_roi > 0 else 0

    # Profit factor
    total_profit = results_df[results_df['pl'] > 0]['pl'].sum()
    total_loss = abs(results_df[results_df['pl'] <= 0]['pl'].sum())
    profit_factor = total_profit / total_loss if total_loss > 0 else float('inf')

    print(f"\nTOTAL TRADES: {total_trades}")
    print(f"TIME PERIOD: {results_df['entry_date'].iloc[0].strftime('%Y-%m-%d')} to {results_df['exit_date'].iloc[-1].strftime('%Y-%m-%d')}")

    print(f"\nWIN/LOSS STATISTICS:")
    print(f"  Winning Trades: {winning_trades} ({win_rate:.1f}%)")
    print(f"  Losing Trades: {losing_trades} ({100-win_rate:.1f}%)")
    print(f"  Profit Factor: {profit_factor:.2f}")

    print(f"\nRETURN STATISTICS:")
    print(f"  Average ROI: {avg_roi:.2f}%")
    print(f"  Median ROI: {median_roi:.2f}%")
    print(f"  Std Dev ROI: {std_roi:.2f}%")
    print(f"  Best Trade: {max_profit:.2f}%")
    print(f"  Worst Trade: {max_loss:.2f}%")
    print(f"  Sharpe Ratio: {sharpe:.3f}")

    # Analyze by year
    results_df['year'] = results_df['entry_date'].dt.year
    yearly_stats = results_df.groupby('year').agg({
        'roi': ['mean', 'count', 'std'],
        'pl': 'sum'
    }).round(2)

    print(f"\nYEARLY PERFORMANCE:")
    print(yearly_stats)

    # Analyze by volatility regime
    high_vol = results_df[results_df['volatility'] > 0.20]
    low_vol = results_df[results_df['volatility'] <= 0.20]

    print(f"\nPERFORMANCE BY VOLATILITY REGIME:")
    print(f"  High Volatility (>20%):")
    print(f"    Trades: {len(high_vol)}, Avg ROI: {high_vol['roi'].mean():.2f}%")
    print(f"  Low Volatility (<=20%):")
    print(f"    Trades: {len(low_vol)}, Avg ROI: {low_vol['roi'].mean():.2f}%")

    # Analyze by underlying direction
    bullish = results_df[results_df['underlying_return'] > 0]
    bearish = results_df[results_df['underlying_return'] <= 0]

    print(f"\nPERFORMANCE BY MARKET DIRECTION:")
    print(f"  Bullish Underlying:")
    print(f"    Trades: {len(bullish)}, Avg ROI: {bullish['roi'].mean():.2f}%")
    print(f"  Bearish/Flat Underlying:")
    print(f"    Trades: {len(bearish)}, Avg ROI: {bearish['roi'].mean():.2f}%")

    # Drawdown analysis
    cumulative_pl = results_df['pl'].cumsum()
    running_max = cumulative_pl.cummax()
    drawdown = cumulative_pl - running_max
    max_drawdown = drawdown.min()

    print(f"\nRISK METRICS:")
    print(f"  Max Drawdown: ${max_drawdown:.2f}")
    print(f"  Total P&L: ${cumulative_pl.iloc[-1]:.2f}")

    return {
        'total_trades': total_trades,
        'win_rate': win_rate,
        'avg_roi': avg_roi,
        'sharpe_ratio': sharpe,
        'profit_factor': profit_factor,
        'max_drawdown': max_drawdown,
        'total_pl': cumulative_pl.iloc[-1]
    }

def test_different_parameters(df):
    """Test strategy with different parameters"""
    print("\n" + "="*60)
    print("PARAMETER SENSITIVITY ANALYSIS")
    print("="*60)

    params_to_test = [
        (0.98, 1.03, 15),  # Current strategy
        (0.97, 1.03, 15),  # Wider lower strike
        (0.98, 1.05, 15),  # Wider upper strike
        (0.99, 1.02, 15),  # Tighter spread
        (0.98, 1.03, 10),  # Shorter holding
        (0.98, 1.03, 21),  # Longer holding
    ]

    results_summary = []

    for lower_pct, upper_pct, holding in params_to_test:
        print(f"\nTesting: Lower={lower_pct:.0%}, Upper={upper_pct:.0%}, Hold={holding}d")
        results = backtest_bull_call_spread(df, holding, lower_pct, upper_pct)

        win_rate = (len(results[results['pl'] > 0]) / len(results)) * 100
        avg_roi = results['roi'].mean()

        results_summary.append({
            'lower_pct': lower_pct,
            'upper_pct': upper_pct,
            'holding_period': holding,
            'avg_roi': avg_roi,
            'win_rate': win_rate
        })

        print(f"  Win Rate: {win_rate:.1f}%, Avg ROI: {avg_roi:.2f}%")

    return pd.DataFrame(results_summary)

def main():
    print("="*60)
    print("GOLD OPTIONS STRATEGY BACKTEST")
    print("Using EODHD Historical Data")
    print("="*60)

    # Load data
    print("\nLoading historical data...")
    df = load_historical_data()
    print(f"Loaded {len(df)} records from {df.index[0]} to {df.index[-1]}")

    # Run main backtest
    print("\n" + "="*60)
    print("MAIN STRATEGY BACKTEST: Bull Call Spread")
    print("="*60)

    results = backtest_bull_call_spread(df, holding_period=15, lower_pct=0.98, upper_pct=1.03)

    # Save detailed results
    results.to_csv('backtest_results_detailed.csv', index=False)
    print(f"\nDetailed results saved to backtest_results_detailed.csv")

    # Analyze results
    summary = analyze_backtest_results(results)

    # Test different parameters
    param_results = test_different_parameters(df)
    param_results.to_csv('parameter_sensitivity.csv', index=False)

    # Save summary
    with open('backtest_summary.json', 'w') as f:
        json.dump(summary, f, indent=2, default=float)

    print("\n" + "="*60)
    print("BACKTEST COMPLETE")
    print("="*60)
    print("\nFiles generated:")
    print("  - backtest_results_detailed.csv")
    print("  - parameter_sensitivity.csv")
    print("  - backtest_summary.json")

    # Final recommendation
    print("\n" + "="*60)
    print("RECOMMENDATION BASED ON BACKTEST")
    print("="*60)

    if summary['win_rate'] >= 50 and summary['avg_roi'] > 0:
        print("✓ Strategy shows POSITIVE expected value")
        print(f"  Win Rate: {summary['win_rate']:.1f}%")
        print(f"  Average ROI: {summary['avg_roi']:.2f}%")
        print("  PROCEED with caution, size positions appropriately")
    else:
        print("⚠ Strategy shows WEAK or NEGATIVE expected value")
        print(f"  Win Rate: {summary['win_rate']:.1f}%")
        print(f"  Average ROI: {summary['avg_roi']:.2f}%")
        print("  RECONSIDER strategy or adjust parameters")

    return results, summary

if __name__ == "__main__":
    results, summary = main()
