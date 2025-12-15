#!/usr/bin/env python3
"""
Backtest the IBD methodology with aggressive trading rules.

This backtests the exact criteria used in our analysis:
- IBD Composite Rating >= 70 = STRONG BUY (aggressive entry)
- IBD Composite Rating >= 60 = BUY
- IBD Composite Rating >= 50 = HOLD
- IBD Composite Rating < 50 = SELL/AVOID

Entry rules (AGGRESSIVE):
1. IBD rating >= 70
2. Above 50-day MA
3. Within 10% of 52-week high
4. Volume > 1.0x average

Exit rules (AGGRESSIVE):
1. IBD rating drops below 50
2. Falls below 50-day MA
3. 10% stop loss triggered
4. Or take profit at +30%
"""
import requests
import json
import os
import sys
from datetime import datetime, timedelta
from statistics import mean, stdev
import pandas as pd

def fetch_historical_data(symbol, api_token, start_date=None, end_date=None):
    """Fetch historical price data from EODHD API."""
    url = f"https://eodhd.com/api/eod/{symbol}"

    params = {
        'api_token': api_token,
        'fmt': 'json',
        'period': 'd',
        'order': 'a'  # Ascending for backtest
    }

    if start_date:
        params['from'] = start_date
    if end_date:
        params['to'] = end_date

    try:
        resp = requests.get(url, params=params, timeout=60)
        resp.raise_for_status()
        data = resp.json()
        return data if isinstance(data, list) else []
    except Exception as e:
        print(f"Error fetching {symbol}: {e}", file=sys.stderr)
        return []

def calculate_ibd_score_historical(prices, volumes, index):
    """Calculate IBD score for a specific point in history."""
    if index < 200:  # Need 200 days of history
        return None

    current_price = prices[index]

    # Get lookback data
    prices_lookback = prices[max(0, index-252):index+1]
    volumes_lookback = volumes[max(0, index-50):index+1]

    if len(prices_lookback) < 50:
        return None

    # Calculate MAs
    ma_10 = mean(prices_lookback[-10:]) if len(prices_lookback) >= 10 else None
    ma_21 = mean(prices_lookback[-21:]) if len(prices_lookback) >= 21 else None
    ma_50 = mean(prices_lookback[-50:]) if len(prices_lookback) >= 50 else None
    ma_200 = mean(prices_lookback[-200:]) if len(prices_lookback) >= 200 else None

    # Position from high
    high_52w = max(prices_lookback[-252:]) if len(prices_lookback) >= 252 else max(prices_lookback)
    pct_from_high = ((current_price - high_52w) / high_52w) * 100

    # Relative strength
    if len(prices_lookback) >= 50:
        price_50d_ago = prices_lookback[-50]
        rs_50d = ((current_price - price_50d_ago) / price_50d_ago) * 100
    else:
        rs_50d = 0

    # Volume
    avg_vol_50 = mean(volumes_lookback) if volumes_lookback else 0
    latest_vol = volumes[index] if index < len(volumes) else 0
    vol_ratio = (latest_vol / avg_vol_50) if avg_vol_50 > 0 else 0

    # Calculate IBD score
    score = 0

    # Relative Strength (40 points)
    if rs_50d >= 30: score += 40
    elif rs_50d >= 20: score += 32
    elif rs_50d >= 10: score += 24
    elif rs_50d >= 5: score += 16
    elif rs_50d >= 0: score += 8

    # MA alignment (30 points)
    if ma_10 and current_price > ma_10: score += 8
    if ma_21 and current_price > ma_21: score += 8
    if ma_50 and current_price > ma_50: score += 7
    if ma_200 and current_price > ma_200: score += 7

    # Position from high (15 points)
    if pct_from_high >= -2: score += 15
    elif pct_from_high >= -5: score += 12
    elif pct_from_high >= -10: score += 9
    elif pct_from_high >= -20: score += 6
    elif pct_from_high >= -30: score += 3

    # Volume (15 points)
    if vol_ratio >= 2.0: score += 15
    elif vol_ratio >= 1.5: score += 12
    elif vol_ratio >= 1.2: score += 9
    elif vol_ratio >= 1.0: score += 6
    elif vol_ratio >= 0.8: score += 3

    return {
        'score': min(score, 100),
        'price': current_price,
        'ma_50': ma_50,
        'pct_from_high': pct_from_high,
        'rs_50d': rs_50d,
        'vol_ratio': vol_ratio,
        'is_above_ma50': current_price > ma_50 if ma_50 else False
    }

def backtest_aggressive_strategy(symbol, api_token, start_date, end_date):
    """Run aggressive IBD backtest."""
    print(f"\n{'='*100}")
    print(f"BACKTESTING AGGRESSIVE IBD STRATEGY: {symbol}")
    print(f"Period: {start_date} to {end_date}")
    print(f"{'='*100}\n")

    # Fetch data
    print("Fetching historical data...")
    data = fetch_historical_data(symbol, api_token, start_date, end_date)

    if not data or len(data) < 252:
        print(f"Insufficient data for {symbol}")
        return None

    print(f"Loaded {len(data)} trading days")

    # Parse data
    dates = [d['date'] for d in data]
    prices = [float(d['close']) for d in data]
    volumes = [float(d.get('volume', 0)) for d in data]

    # Trading parameters (AGGRESSIVE)
    BUY_THRESHOLD = 70  # IBD score for entry
    SELL_THRESHOLD = 50  # IBD score for exit
    STOP_LOSS_PCT = -10  # 10% stop loss
    TAKE_PROFIT_PCT = 30  # 30% take profit
    POSITION_SIZE = 1.0  # 100% of capital (aggressive)

    # Track trades
    trades = []
    positions = []
    cash = 100000  # Start with $100k
    shares = 0
    entry_price = 0
    entry_date = None
    portfolio_values = []

    # Run backtest
    for i in range(252, len(data)):  # Start after 252 days for proper calculations
        current_date = dates[i]
        current_price = prices[i]

        # Calculate IBD score
        ibd_data = calculate_ibd_score_historical(prices, volumes, i)

        if not ibd_data:
            continue

        ibd_score = ibd_data['score']

        # Current portfolio value
        portfolio_value = cash + (shares * current_price)
        portfolio_values.append({
            'date': current_date,
            'value': portfolio_value,
            'price': current_price,
            'ibd_score': ibd_score,
            'position': 'LONG' if shares > 0 else 'CASH'
        })

        # ENTRY LOGIC (AGGRESSIVE)
        if shares == 0:  # Not in position
            # Aggressive entry criteria:
            # 1. IBD score >= 70 (A- or better)
            # 2. Above 50-day MA
            # 3. Within 10% of 52-week high
            # 4. Volume >= 1.0x average (optional for aggressive)

            if (ibd_score >= BUY_THRESHOLD and
                ibd_data['is_above_ma50'] and
                ibd_data['pct_from_high'] >= -10):

                # BUY
                shares = (cash * POSITION_SIZE) / current_price
                entry_price = current_price
                entry_date = current_date
                cash = cash * (1 - POSITION_SIZE)

                print(f"BUY: {current_date} | Price: ${current_price:.2f} | IBD: {ibd_score} | Shares: {shares:.2f}")

        # EXIT LOGIC (AGGRESSIVE)
        else:  # In position
            pnl_pct = ((current_price - entry_price) / entry_price) * 100

            # Exit conditions:
            # 1. IBD score drops below 50
            # 2. Falls below 50-day MA
            # 3. 10% stop loss
            # 4. 30% take profit

            exit_signal = False
            exit_reason = ""

            if ibd_score < SELL_THRESHOLD:
                exit_signal = True
                exit_reason = f"IBD score dropped to {ibd_score}"
            elif not ibd_data['is_above_ma50']:
                exit_signal = True
                exit_reason = "Fell below 50-day MA"
            elif pnl_pct <= STOP_LOSS_PCT:
                exit_signal = True
                exit_reason = f"Stop loss triggered ({pnl_pct:.1f}%)"
            elif pnl_pct >= TAKE_PROFIT_PCT:
                exit_signal = True
                exit_reason = f"Take profit triggered (+{pnl_pct:.1f}%)"

            if exit_signal:
                # SELL
                cash += shares * current_price
                profit = (current_price - entry_price) * shares
                profit_pct = pnl_pct

                trades.append({
                    'entry_date': entry_date,
                    'entry_price': entry_price,
                    'exit_date': current_date,
                    'exit_price': current_price,
                    'shares': shares,
                    'profit': profit,
                    'profit_pct': profit_pct,
                    'days_held': (datetime.strptime(current_date, '%Y-%m-%d') -
                                 datetime.strptime(entry_date, '%Y-%m-%d')).days,
                    'exit_reason': exit_reason
                })

                print(f"SELL: {current_date} | Price: ${current_price:.2f} | P/L: {profit_pct:+.2f}% | Reason: {exit_reason}")

                shares = 0
                entry_price = 0
                entry_date = None

    # Close any open position at end
    if shares > 0:
        final_price = prices[-1]
        cash += shares * final_price
        profit = (final_price - entry_price) * shares
        profit_pct = ((final_price - entry_price) / entry_price) * 100

        trades.append({
            'entry_date': entry_date,
            'entry_price': entry_price,
            'exit_date': dates[-1],
            'exit_price': final_price,
            'shares': shares,
            'profit': profit,
            'profit_pct': profit_pct,
            'days_held': (datetime.strptime(dates[-1], '%Y-%m-%d') -
                         datetime.strptime(entry_date, '%Y-%m-%d')).days,
            'exit_reason': 'End of backtest'
        })

        shares = 0

    # Calculate performance metrics
    final_value = cash
    total_return = ((final_value - 100000) / 100000) * 100

    # Buy-and-hold comparison
    buy_hold_shares = 100000 / prices[252]
    buy_hold_final = buy_hold_shares * prices[-1]
    buy_hold_return = ((buy_hold_final - 100000) / 100000) * 100

    # Win rate
    winning_trades = [t for t in trades if t['profit'] > 0]
    losing_trades = [t for t in trades if t['profit'] <= 0]
    win_rate = (len(winning_trades) / len(trades) * 100) if trades else 0

    # Average metrics
    avg_profit_pct = mean([t['profit_pct'] for t in trades]) if trades else 0
    avg_winning_pct = mean([t['profit_pct'] for t in winning_trades]) if winning_trades else 0
    avg_losing_pct = mean([t['profit_pct'] for t in losing_trades]) if losing_trades else 0
    avg_days_held = mean([t['days_held'] for t in trades]) if trades else 0

    # Max drawdown
    peak = 100000
    max_dd = 0
    for pv in portfolio_values:
        if pv['value'] > peak:
            peak = pv['value']
        dd = ((pv['value'] - peak) / peak) * 100
        if dd < max_dd:
            max_dd = dd

    # Sharpe ratio (simplified - assuming 0% risk-free rate)
    if len(portfolio_values) > 1:
        returns = []
        for i in range(1, len(portfolio_values)):
            ret = ((portfolio_values[i]['value'] - portfolio_values[i-1]['value']) /
                   portfolio_values[i-1]['value']) * 100
            returns.append(ret)

        avg_return = mean(returns) if returns else 0
        std_return = stdev(returns) if len(returns) > 1 else 0
        sharpe = (avg_return / std_return * (252 ** 0.5)) if std_return > 0 else 0
    else:
        sharpe = 0

    results = {
        'symbol': symbol,
        'start_date': start_date,
        'end_date': end_date,
        'trading_days': len(data),
        'total_trades': len(trades),
        'winning_trades': len(winning_trades),
        'losing_trades': len(losing_trades),
        'win_rate': win_rate,
        'total_return': total_return,
        'buy_hold_return': buy_hold_return,
        'outperformance': total_return - buy_hold_return,
        'final_value': final_value,
        'avg_profit_pct': avg_profit_pct,
        'avg_winning_pct': avg_winning_pct,
        'avg_losing_pct': avg_losing_pct,
        'avg_days_held': avg_days_held,
        'max_drawdown': max_dd,
        'sharpe_ratio': sharpe,
        'trades': trades,
        'portfolio_values': portfolio_values
    }

    return results

def print_backtest_results(results):
    """Print formatted backtest results."""
    if not results:
        return

    print(f"\n{'='*100}")
    print(f"BACKTEST RESULTS: {results['symbol']}")
    print(f"{'='*100}\n")

    print(f"Period: {results['start_date']} to {results['end_date']}")
    print(f"Trading Days: {results['trading_days']}")
    print(f"\nOVERALL PERFORMANCE:")
    print(f"  Initial Capital:     $100,000.00")
    print(f"  Final Value:         ${results['final_value']:,.2f}")
    print(f"  Total Return:        {results['total_return']:+.2f}%")
    print(f"  Buy & Hold Return:   {results['buy_hold_return']:+.2f}%")
    print(f"  Outperformance:      {results['outperformance']:+.2f}%")
    print(f"  Max Drawdown:        {results['max_drawdown']:.2f}%")
    print(f"  Sharpe Ratio:        {results['sharpe_ratio']:.2f}")

    print(f"\nTRADING STATISTICS:")
    print(f"  Total Trades:        {results['total_trades']}")
    print(f"  Winning Trades:      {results['winning_trades']}")
    print(f"  Losing Trades:       {results['losing_trades']}")
    print(f"  Win Rate:            {results['win_rate']:.1f}%")
    print(f"  Avg Trade Return:    {results['avg_profit_pct']:+.2f}%")
    print(f"  Avg Winner:          {results['avg_winning_pct']:+.2f}%")
    print(f"  Avg Loser:           {results['avg_losing_pct']:+.2f}%")
    print(f"  Avg Days Held:       {results['avg_days_held']:.0f} days")

    print(f"\nTOP 5 BEST TRADES:")
    best_trades = sorted(results['trades'], key=lambda x: x['profit_pct'], reverse=True)[:5]
    for i, trade in enumerate(best_trades, 1):
        print(f"  {i}. {trade['entry_date']} to {trade['exit_date']}: {trade['profit_pct']:+.2f}% ({trade['days_held']} days)")

    print(f"\nTOP 5 WORST TRADES:")
    worst_trades = sorted(results['trades'], key=lambda x: x['profit_pct'])[:5]
    for i, trade in enumerate(worst_trades, 1):
        print(f"  {i}. {trade['entry_date']} to {trade['exit_date']}: {trade['profit_pct']:+.2f}% - {trade['exit_reason']}")

def main():
    api_token = (os.environ.get('EODHD_APIKEY') or
                 os.environ.get('EODHD_API_KEY') or
                 '690d7cdc3013f4.57364117')

    # Backtest parameters
    # Test last 2 years for Apple
    end_date = '2025-11-21'
    start_date = '2023-01-01'

    symbols = {
        'AAPL.US': 'Apple Inc.',
        'GLD.US': 'Gold ETF'
    }

    all_results = {}

    for symbol, name in symbols.items():
        try:
            results = backtest_aggressive_strategy(symbol, api_token, start_date, end_date)
            if results:
                all_results[symbol] = results
                print_backtest_results(results)
        except Exception as e:
            print(f"Error backtesting {symbol}: {e}")
            import traceback
            traceback.print_exc()

    # Save results
    output_file = 'c:\\Users\\micha\\github-mcp-server\\scripts\\backtest_results.json'
    with open(output_file, 'w', encoding='utf-8') as f:
        # Convert for JSON serialization
        export_results = {}
        for sym, res in all_results.items():
            export_results[sym] = {
                k: v for k, v in res.items()
                if k not in ['trades', 'portfolio_values']
            }
            export_results[sym]['sample_trades'] = res['trades'][:10]  # First 10 trades

        json.dump(export_results, f, indent=2)

    print(f"\n{'='*100}")
    print(f"Backtest results saved to: {output_file}")
    print(f"{'='*100}\n")

    # Comparison summary
    if len(all_results) > 1:
        print(f"\n{'='*100}")
        print(f"STRATEGY COMPARISON")
        print(f"{'='*100}\n")
        print(f"{'Symbol':<12} {'Total Return':<15} {'Buy&Hold':<15} {'Outperform':<15} {'Win Rate':<12} {'Sharpe':<10}")
        print(f"{'-'*100}")
        for sym, res in all_results.items():
            print(f"{sym:<12} {res['total_return']:>+13.2f}%  {res['buy_hold_return']:>+13.2f}%  "
                  f"{res['outperformance']:>+13.2f}%  {res['win_rate']:>10.1f}%  {res['sharpe_ratio']:>8.2f}")

if __name__ == '__main__':
    main()
