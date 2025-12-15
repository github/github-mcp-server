#!/usr/bin/env python3
"""
Wrapper to run ultimate_autopilot_eod_v2.py and analyze its portfolio with FSDPredictor
"""
import runpy
import os
from datetime import datetime

# Load the autopilot script as a module namespace
script_path = os.path.join(os.path.dirname(__file__), 'ultimate_autopilot_eod_v2.py')
ns = runpy.run_path(script_path)

# Import FSD predictor
from fsd_modeling_engine import FSDPredictor, generate_fsd_report

# Retrieve PORTFOLIO from the autopilot namespace
PORTFOLIO = ns.get('PORTFOLIO', {})

predictor = FSDPredictor()

reports = []
summary_rows = []
total_alloc = 0.0

for key, asset in PORTFOLIO.items():
    try:
        name = asset.get('name', key)
        price = asset.get('current_price', asset.get('price', 0.0))
        allocated = asset.get('allocated_capital', asset.get('allocated', 0.0)) or 0.0
        total_alloc += allocated

        # Build minimal data dict for meta-factors
        data = {
            'price': price,
            'sma_20': asset.get('sma_20', price),
            'sma_50': asset.get('sma_50', price),
            'sma_200': asset.get('sma_200', price),
            'rsi': asset.get('rsi', 50),
            'ibd_composite': asset.get('ibd_composite', 50),
            'ibd_rs': asset.get('ibd_rs', 50),
            'ibd_eps': asset.get('ibd_eps', 50),
            'kalshi_sentiment': asset.get('kalshi_sentiment', 0.5),
            'polymarket_sentiment': asset.get('polymarket_sentiment', 0.5),
            'news_sentiment': asset.get('news_sentiment', 0.0),
            'reddit_sentiment': asset.get('reddit_sentiment', 0.0),
            'expense_ratio': asset.get('expense_ratio', 0.0),
            'premium_to_nav': asset.get('premium_to_nav', 0.0),
            'avg_volume': asset.get('avg_volume', 1000000),
            'fed_rate': asset.get('fed_rate', 5.0),
            'inflation': asset.get('inflation', 2.0),
            'vix': asset.get('vix', 18.0),
            'gdp_growth': asset.get('gdp_growth', 2.0),
            'unemployment': asset.get('unemployment', 4.0)
        }

        analysis = predictor.analyze_asset(data)
        report_text = generate_fsd_report(data, options_data=None)
        out_file = os.path.join(os.path.dirname(__file__), f"fsd_report_{key}.txt")
        with open(out_file, 'w', encoding='utf-8') as f:
            f.write(report_text)
        # Per-asset Monte Carlo and optional plot
        try:
            mc = predictor.run_monte_carlo(price if price else 1.0, volatility=0.20, drift=0.05, n_days=30)
            mc_stats = mc.get('statistics', {})
            # Append MC summary to text report
            with open(out_file, 'a', encoding='utf-8') as f:
                f.write('\n\nSECTION: Monte Carlo (30d)\n')
                f.write(f"Expected Price: ${mc_stats.get('mean_price', 0):.2f}\n")
                f.write(f"95% Range: ${mc_stats.get('percentile_5', 0):.2f} - ${mc_stats.get('percentile_95', 0):.2f}\n")
                f.write(f"VaR (95%): {mc_stats.get('var_95', 0):.2f}%\n")

            # Try to plot distribution if matplotlib exists
            try:
                import matplotlib.pyplot as plt
                paths = mc.get('price_paths')
                if paths is not None:
                    plt.figure(figsize=(6,3))
                    for p in paths[:100]:
                        plt.plot(p, color='gray', alpha=0.08)
                    plt.title(f"Monte Carlo price paths - {name}")
                    plt.xlabel('Days')
                    plt.ylabel('Price')
                    plot_file = os.path.join(os.path.dirname(__file__), f"mc_{key}.png")
                    plt.tight_layout()
                    plt.savefig(plot_file, dpi=150)
                    plt.close()
                    print(f"  Monte Carlo plot saved: {plot_file}")
            except Exception:
                pass
        except Exception:
            pass

        # Extract composite and predicted return from analysis
        composite = analysis.get('composite_score', 0.0)
        predicted = analysis.get('predicted_return', 0.0)
        signal = analysis.get('signal', 'NEUTRAL')

        summary_rows.append({
            'key': key,
            'name': name,
            'allocated': allocated,
            'price': price,
            'composite': composite,
            'predicted_return': predicted,
            'signal': signal
        })

        print(f"Wrote report for {name} -> {out_file}")
        reports.append(out_file)
    except Exception as e:
        print(f"Failed to analyze {key}: {e}")

print(f"\nDone. Generated {len(reports)} reports at {datetime.now()}")

# Aggregate portfolio-level summary and append to STRATEGY report
strategy_file = os.path.join(os.path.dirname(__file__), 'STRATEGY_REPORT_20251216.md')
portfolio_summary = []
portfolio_summary.append('# FSD Portfolio Aggregated Summary')
portfolio_summary.append(f'*Generated: {datetime.now().strftime("%Y-%m-%d %H:%M:%S") }*')
portfolio_summary.append('')
portfolio_summary.append('| Asset | Allocated | Price | Composite | Predicted Return | Signal |')
portfolio_summary.append('|---|---:|---:|---:|---:|---:')

# Compute weighted predicted return
weighted_sum = 0.0
for row in summary_rows:
    alloc = row['allocated']
    pr = row['predicted_return']
    weighted_sum += alloc * pr

portfolio_predicted = (weighted_sum / total_alloc) if total_alloc > 0 else 0.0

for row in summary_rows:
    portfolio_summary.append(f"| {row['name']} | {row['allocated']:.2f} | {row['price']:.2f} | {row['composite']:.3f} | {row['predicted_return']*100:+.2f}% | {row['signal']} |")

portfolio_summary.append('')
portfolio_summary.append(f'*Portfolio-weighted predicted return: **{portfolio_predicted*100:+.2f}%***')

with open(strategy_file, 'a', encoding='utf-8') as sf:
    sf.write('\n'.join(portfolio_summary) + '\n\n')

print(f"Appended portfolio summary to {strategy_file}")
