#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
快速演示：股息收益率计算系统
Quick Demo: Dividend Yield Calculation System

这个脚本演示了所有核心功能，适合快速了解系统。
"""

import sys
import logging
from datetime import date
from dividend_yield_calculator import (
    DividendYieldAnalysis,
    DividendYieldCalculator,
    MarketExpectationCalculator,
    generate_yield_report
)

logging.basicConfig(level=logging.INFO, format='%(message)s')
logger = logging.getLogger(__name__)

def demo_single_trade():
    """演示 1: 单笔交易收益计算"""
    
    logger.info("="*80)
    logger.info("演示 1: 单笔交易收益计算")
    logger.info("="*80)
    
    # 中国股票示例
    trade_cn = DividendYieldAnalysis(
        ticker='601988',
        trade_date=date(2025, 11, 28),
        buy_date=date(2025, 11, 26),
        sell_date=date(2025, 11, 29),
        buy_price=3.15,
        sell_price=3.17,
        shares=1000,
        dividend_per_share=0.033
    )
    
    logger.info("\n中国银行 (601988) 交易分析:")
    logger.info(f"  买入: {trade_cn.buy_date} @ ¥{trade_cn.buy_price}")
    logger.info(f"  卖出: {trade_cn.sell_date} @ ¥{trade_cn.sell_price}")
    logger.info(f"  分红: ¥{trade_cn.dividend_per_share * trade_cn.shares:,.2f}")
    logger.info(f"  ├─ 价格变化: {trade_cn.price_change_pct:+.2f}%")
    logger.info(f"  ├─ 分红收益: {trade_cn.dividend_yield_pct:.3f}%")
    logger.info(f"  ├─ 总收益: {trade_cn.total_return_pct:+.3f}%")
    logger.info(f"  └─ 年化收益: {trade_cn.annualized_return_pct:+.1f}%")
    
    # 美国ETF示例
    trade_us = DividendYieldAnalysis(
        ticker='JEPI',
        trade_date=date(2025, 11, 15),
        buy_date=date(2025, 11, 13),
        sell_date=date(2025, 11, 18),
        buy_price=50.00,
        sell_price=50.30,
        shares=100,
        dividend_per_share=0.60
    )
    
    logger.info("\nJEPI ETF 交易分析:")
    logger.info(f"  买入: {trade_us.buy_date} @ ${trade_us.buy_price}")
    logger.info(f"  卖出: {trade_us.sell_date} @ ${trade_us.sell_price}")
    logger.info(f"  分红: ${trade_us.dividend_per_share * trade_us.shares:,.2f}")
    logger.info(f"  ├─ 价格变化: {trade_us.price_change_pct:+.2f}%")
    logger.info(f"  ├─ 分红收益: {trade_us.dividend_yield_pct:.3f}%")
    logger.info(f"  ├─ 总收益: {trade_us.total_return_pct:+.3f}%")
    logger.info(f"  └─ 年化收益: {trade_us.annualized_return_pct:+.1f}%")
    
    return [trade_cn, trade_us]

def demo_strategy_analysis(trades):
    """演示 2: 策略聚合分析"""
    
    logger.info("\n" + "="*80)
    logger.info("演示 2: 策略聚合分析")
    logger.info("="*80)
    
    calculator = DividendYieldCalculator()
    
    # 添加更多交易
    all_trades = trades + [
        DividendYieldAnalysis('601398', date(2025, 12, 3), date(2025, 12, 3), date(2025, 12, 6), 5.80, 5.83, 1000, 0.028),
        DividendYieldAnalysis('601288', date(2025, 12, 8), date(2025, 12, 8), date(2025, 12, 11), 3.90, 3.92, 1000, 0.032),
        DividendYieldAnalysis('600000', date(2025, 12, 1), date(2025, 12, 1), date(2025, 12, 4), 8.50, 8.53, 1000, 0.42),
        DividendYieldAnalysis('XYLD', date(2025, 11, 18), date(2025, 11, 20), date(2025, 11, 23), 25.00, 25.15, 200, 0.50),
        DividendYieldAnalysis('SDIV', date(2025, 12, 3), date(2025, 12, 3), date(2025, 12, 8), 15.00, 15.10, 300, 0.65),
    ]
    
    for trade in all_trades:
        calculator.add_trade(trade)
    
    perf = calculator.calculate_strategy_performance()
    
    logger.info(f"\n已添加 {perf.total_trades} 笔交易")
    logger.info(f"├─ 获利交易: {perf.winning_trades} 笔 ({perf.win_rate*100:.1f}%)")
    logger.info(f"├─ 亏损交易: {perf.losing_trades} 笔")
    logger.info(f"├─ 平均单笔收益: {perf.avg_return_per_trade:.3f}%")
    logger.info(f"├─ 平均年化收益: {perf.avg_annualized_return:.1f}%")
    logger.info(f"└─ 利润因子: {perf.profit_factor:.2f}")
    
    logger.info(f"\n月度预期:")
    logger.info(f"├─ 预期月交易: {perf.monthly_expected_trades} 次")
    logger.info(f"├─ 预期月收益: {perf.monthly_expected_return_pct:.2f}%")
    logger.info(f"└─ 预期年收益: {perf.annual_expected_return_pct:.2f}%")
    
    return calculator, perf

def demo_market_expectation():
    """演示 3: 市场期望收益"""
    
    logger.info("\n" + "="*80)
    logger.info("演示 3: 市场期望收益")
    logger.info("="*80)
    
    # 中国股票
    logger.info("\n中国资产市场期望 (4天持仓):")
    cn_tickers = ['601988', '601398', '601288', '600000', '000858']
    
    for ticker in cn_tickers:
        expected = MarketExpectationCalculator.calculate_expected_return(
            ticker, hold_days=4, region='CN'
        )
        logger.info(
            f"  {ticker} | 年化: {expected['annual_yield_pct']:.1f}% | "
            f"4天: {expected['hold_dividend_yield_pct']:.3f}%"
        )
    
    # 美国ETF
    logger.info("\n美国资产市场期望 (5天持仓):")
    us_tickers = ['JEPI', 'XYLD', 'SDIV', 'VYM', 'DGRO']
    
    for ticker in us_tickers:
        expected = MarketExpectationCalculator.calculate_expected_return(
            ticker, hold_days=5, region='US'
        )
        logger.info(
            f"  {ticker} | 年化: {expected['annual_yield_pct']:.1f}% | "
            f"5天: {expected['hold_dividend_yield_pct']:.3f}%"
        )

def demo_portfolio_expectation():
    """演示 4: 组合预期收益"""
    
    logger.info("\n" + "="*80)
    logger.info("演示 4: 组合预期收益")
    logger.info("="*80)
    
    # 中国组合
    logger.info("\n中国组合预期 (11个资产, 4天持仓):")
    cn_portfolio = MarketExpectationCalculator.calculate_portfolio_return(
        ['601988', '601398', '601288', '600000', '000858', 
         '510300', '510500', '510880', '00700.HK', '00939.HK', '01288.HK'],
        hold_days=4,
        region='CN'
    )
    
    logger.info(f"  平均单次收益: {cn_portfolio['average_return_pct']:.3f}%")
    logger.info(f"  预期月交易: {cn_portfolio['monthly_expected_trades']} 次")
    logger.info(f"  预期月收益: {cn_portfolio['monthly_expected_return_pct']:.2f}%")
    logger.info(f"  预期年收益: {cn_portfolio['monthly_expected_return_pct']*12:.2f}%")
    
    logger.info(f"\n  初始资本收益预测 (CNY):")
    for capital in [50000, 100000, 200000]:
        monthly = capital * cn_portfolio['monthly_expected_return_pct'] / 100
        logger.info(f"    ¥{capital:>7,}: 月均 ¥{monthly:>8,.0f}")
    
    # 美国组合
    logger.info("\n美国组合预期 (8个ETF, 5天持仓):")
    us_portfolio = MarketExpectationCalculator.calculate_portfolio_return(
        ['JEPI', 'XYLD', 'SDIV', 'VYM', 'DGRO', 'NOBL', 'SCHD', 'HDV'],
        hold_days=5,
        region='US'
    )
    
    logger.info(f"  平均单次收益: {us_portfolio['average_return_pct']:.3f}%")
    logger.info(f"  预期月交易: {us_portfolio['monthly_expected_trades']} 次")
    logger.info(f"  预期月收益: {us_portfolio['monthly_expected_return_pct']:.2f}%")
    logger.info(f"  预期年收益: {us_portfolio['monthly_expected_return_pct']*12:.2f}%")
    
    logger.info(f"\n  初始资本收益预测 (USD):")
    for capital in [5000, 10000, 20000]:
        monthly = capital * us_portfolio['monthly_expected_return_pct'] / 100
        logger.info(f"    ${capital:>6,}: 月均 ${monthly:>7,.0f}")

def demo_comparison():
    """演示 5: 策略对比"""
    
    logger.info("\n" + "="*80)
    logger.info("演示 5: 策略对比")
    logger.info("="*80)
    
    cn = MarketExpectationCalculator.calculate_portfolio_return(
        ['601988', '601398', '601288', '600000', '000858', 
         '510300', '510500', '510880', '00700.HK', '00939.HK', '01288.HK'],
        hold_days=4, region='CN'
    )
    
    us = MarketExpectationCalculator.calculate_portfolio_return(
        ['JEPI', 'XYLD', 'SDIV', 'VYM', 'DGRO', 'NOBL', 'SCHD', 'HDV'],
        hold_days=5, region='US'
    )
    
    logger.info("\n策略对比:")
    logger.info(f"{'指标':<20} {'中国策略':<20} {'美国策略':<20}")
    logger.info(f"{'-'*60}")
    logger.info(f"{'资产数量':<20} {cn['portfolio_size']:<20} {us['portfolio_size']:<20}")
    logger.info(f"{'持仓天数':<20} {cn['hold_days']:<20} {us['hold_days']:<20}")
    logger.info(f"{'月交易数':<20} {cn['monthly_expected_trades']:<20} {us['monthly_expected_trades']:<20}")
    logger.info(f"{'月收益%':<20} {cn['monthly_expected_return_pct']:.2f}%{'':<14} {us['monthly_expected_return_pct']:.2f}%")
    logger.info(f"{'年收益%':<20} {cn['monthly_expected_return_pct']*12:.2f}%{'':<14} {us['monthly_expected_return_pct']*12:.2f}%")
    
    logger.info("\n建议:")
    if cn['monthly_expected_return_pct'] > us['monthly_expected_return_pct']:
        logger.info("  • 中国策略预期收益更高")
        logger.info("  • 适合有人民币资产的投资者")
    else:
        logger.info("  • 美国策略预期收益更高")
        logger.info("  • 适合有美元资产的投资者")
    logger.info("  • 建议混合配置两个策略，平衡风险和收益")

def main():
    """运行所有演示"""
    
    logger.info("\n")
    logger.info("#"*80)
    logger.info("# 股息轮动策略 - 收益率计算系统演示")
    logger.info("#"*80)
    
    try:
        # 演示1: 单笔交易
        trades = demo_single_trade()
        
        # 演示2: 策略聚合
        calculator, perf = demo_strategy_analysis(trades)
        
        # 演示3: 市场期望
        demo_market_expectation()
        
        # 演示4: 组合预期
        demo_portfolio_expectation()
        
        # 演示5: 策略对比
        demo_comparison()
        
        logger.info("\n" + "#"*80)
        logger.info("# 演示完成!")
        logger.info("#"*80)
        logger.info("\n下一步:")
        logger.info("  1. python verify_yields.py          # 完整功能验证")
        logger.info("  2. python trading_plan_report.py    # 生成交易报告")
        logger.info("  3. python yield_analysis.py --all   # 市场深入分析")
        logger.info("\n更多信息请查看:")
        logger.info("  • YIELD_TOOLS_README.md")
        logger.info("  • YIELD_CALCULATION_GUIDE.md")
        logger.info("  • YIELD_SYSTEM_SUMMARY.md")
        logger.info("")
        
        return 0
        
    except Exception as e:
        logger.error(f"演示失败: {e}", exc_info=True)
        return 1

if __name__ == '__main__':
    sys.exit(main())
