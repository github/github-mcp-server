#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
股息轮动策略 - 收益率计算验证脚本
Dividend Rotation Strategy - Yield Calculation Verification

测试所有收益率计算功能：
  1. 单笔交易收益计算
  2. 策略聚合分析
  3. 市场预期收益
  4. 组合收益预测
"""

import sys
import logging
from datetime import date, timedelta
from typing import List

# 配置日志
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(message)s',
    handlers=[
        logging.StreamHandler(sys.stdout)
    ]
)
logger = logging.getLogger(__name__)

def test_single_trade_yields():
    """测试单笔交易收益计算"""
    
    logger.info("="*80)
    logger.info("测试 1: 单笔交易收益计算")
    logger.info("="*80)
    
    from dividend_yield_calculator import DividendYieldAnalysis
    
    # 示例1: 中国股票
    analysis_cn = DividendYieldAnalysis(
        ticker='601988',
        trade_date=date(2025, 11, 28),
        buy_date=date(2025, 11, 26),
        sell_date=date(2025, 11, 29),
        buy_price=3.15,
        sell_price=3.17,
        shares=1000,
        dividend_per_share=0.033
    )
    
    logger.info(f"\n中国股票示例: 中国银行 (601988)")
    logger.info(f"  买入日期: {analysis_cn.buy_date}")
    logger.info(f"  卖出日期: {analysis_cn.sell_date}")
    logger.info(f"  持仓天数: {analysis_cn.hold_days}")
    logger.info(f"  买入价格: ¥{analysis_cn.buy_price}")
    logger.info(f"  卖出价格: ¥{analysis_cn.sell_price}")
    logger.info(f"  价格变化: {analysis_cn.price_change_pct:+.2f}%")
    logger.info(f"  分红率: {analysis_cn.dividend_yield_pct:.3f}%")
    logger.info(f"  总收益率: {analysis_cn.total_return_pct:+.3f}%")
    logger.info(f"  年化收益: {analysis_cn.annualized_return_pct:+.1f}%")
    
    # 示例2: 美国ETF
    analysis_us = DividendYieldAnalysis(
        ticker='JEPI',
        trade_date=date(2025, 11, 15),
        buy_date=date(2025, 11, 13),
        sell_date=date(2025, 11, 18),
        buy_price=50.00,
        sell_price=50.30,
        shares=100,
        dividend_per_share=0.60
    )
    
    logger.info(f"\n美国ETF示例: JEPI")
    logger.info(f"  买入日期: {analysis_us.buy_date}")
    logger.info(f"  卖出日期: {analysis_us.sell_date}")
    logger.info(f"  持仓天数: {analysis_us.hold_days}")
    logger.info(f"  买入价格: ${analysis_us.buy_price}")
    logger.info(f"  卖出价格: ${analysis_us.sell_price}")
    logger.info(f"  价格变化: {analysis_us.price_change_pct:+.2f}%")
    logger.info(f"  分红率: {analysis_us.dividend_yield_pct:.3f}%")
    logger.info(f"  总收益率: {analysis_us.total_return_pct:+.3f}%")
    logger.info(f"  年化收益: {analysis_us.annualized_return_pct:+.1f}%")
    
    return [analysis_cn, analysis_us]

def test_strategy_aggregation(trades: List):
    """测试策略聚合分析"""
    
    logger.info("\n" + "="*80)
    logger.info("测试 2: 策略聚合分析")
    logger.info("="*80)
    
    from dividend_yield_calculator import DividendYieldCalculator
    
    calculator = DividendYieldCalculator()
    
    # 添加多笔交易
    for trade in trades:
        calculator.add_trade(trade)
    
    logger.info(f"\n已添加 {len(trades)} 笔交易")
    
    # 计算策略性能
    perf = calculator.calculate_strategy_performance()
    
    logger.info(f"\n策略聚合指标:")
    logger.info(f"  总交易笔数: {perf.total_trades}")
    logger.info(f"  获利笔数: {perf.winning_trades}")
    logger.info(f"  亏损笔数: {perf.losing_trades}")
    logger.info(f"  获利率: {perf.win_rate*100:.1f}%")
    logger.info(f"  平均单笔收益: {perf.avg_return_per_trade:.3f}%")
    logger.info(f"  平均年化收益: {perf.avg_annualized_return:.1f}%")
    logger.info(f"  利润因子: {perf.profit_factor:.2f}")
    logger.info(f"  预期月交易数: {perf.monthly_expected_trades}")
    logger.info(f"  预期月收益: {perf.monthly_expected_return_pct:.2f}%")
    logger.info(f"  预期年收益: {perf.annual_expected_return_pct:.2f}%")
    
    return calculator, perf

def test_market_expectations():
    """测试市场预期收益计算"""
    
    logger.info("\n" + "="*80)
    logger.info("测试 3: 市场预期收益计算")
    logger.info("="*80)
    
    from dividend_yield_calculator import MarketExpectationCalculator
    
    # 测试中国股票
    logger.info(f"\n中国股票市场数据:")
    
    cn_tickers = ['601988', '601398', '601288', '600000', '000858']
    
    for ticker in cn_tickers:
        mkt_data = MarketExpectationCalculator.get_market_yield(ticker)
        if mkt_data:
            exp_return = MarketExpectationCalculator.calculate_expected_return(
                ticker, hold_days=4, region='CN'
            )
            logger.info(
                f"  {ticker} | 年化: {mkt_data['annual_yield']:.1f}% | "
                f"4天预期: {exp_return['hold_dividend_yield_pct']:.3f}% | "
                f"年化预期: {exp_return['expected_annualized_return_pct']:.1f}%"
            )
    
    # 测试美国ETF
    logger.info(f"\n美国ETF市场数据:")
    
    us_tickers = ['JEPI', 'XYLD', 'SDIV', 'VYM', 'DGRO']
    
    for ticker in us_tickers:
        mkt_data = MarketExpectationCalculator.get_market_yield(ticker)
        if mkt_data:
            exp_return = MarketExpectationCalculator.calculate_expected_return(
                ticker, hold_days=5, region='US'
            )
            logger.info(
                f"  {ticker} | 年化: {mkt_data['annual_yield']:.1f}% | "
                f"5天预期: {exp_return['hold_dividend_yield_pct']:.3f}% | "
                f"年化预期: {exp_return['expected_annualized_return_pct']:.1f}%"
            )

def test_portfolio_expectations():
    """测试组合预期收益"""
    
    logger.info("\n" + "="*80)
    logger.info("测试 4: 组合预期收益")
    logger.info("="*80)
    
    from dividend_yield_calculator import MarketExpectationCalculator
    
    # 中国股票组合
    logger.info(f"\n中国股票组合预期:")
    
    cn_portfolio = MarketExpectationCalculator.calculate_portfolio_return(
        ['601988', '601398', '601288', '600000', '000858', '510300', '510500', '510880'],
        hold_days=4,
        region='CN'
    )
    
    logger.info(f"  组合规模: {cn_portfolio['portfolio_size']} 个资产")
    logger.info(f"  平均单次收益: {cn_portfolio['average_return_pct']:.3f}%")
    logger.info(f"  单次持仓: {cn_portfolio['hold_days']} 天")
    logger.info(f"  预期月交易: {cn_portfolio['monthly_expected_trades']} 次")
    logger.info(f"  预期月收益: {cn_portfolio['monthly_expected_return_pct']:.2f}%")
    logger.info(f"  预期年收益: {cn_portfolio['monthly_expected_return_pct']*12:.2f}%")
    
    logger.info(f"\n基于初始资本的月收益预测 (CNY):")
    for capital in [50000, 100000, 200000]:
        monthly = capital * cn_portfolio['monthly_expected_return_pct'] / 100
        annual = monthly * 12
        logger.info(f"  ¥{capital:>7,}: 月 ¥{monthly:>8,.0f}, 年 ¥{annual:>10,.0f}")
    
    # 美国ETF组合
    logger.info(f"\n美国ETF组合预期:")
    
    us_portfolio = MarketExpectationCalculator.calculate_portfolio_return(
        ['JEPI', 'XYLD', 'SDIV', 'VYM', 'DGRO', 'NOBL', 'SCHD', 'HDV'],
        hold_days=5,
        region='US'
    )
    
    logger.info(f"  组合规模: {us_portfolio['portfolio_size']} 个资产")
    logger.info(f"  平均单次收益: {us_portfolio['average_return_pct']:.3f}%")
    logger.info(f"  单次持仓: {us_portfolio['hold_days']} 天")
    logger.info(f"  预期月交易: {us_portfolio['monthly_expected_trades']} 次")
    logger.info(f"  预期月收益: {us_portfolio['monthly_expected_return_pct']:.2f}%")
    logger.info(f"  预期年收益: {us_portfolio['monthly_expected_return_pct']*12:.2f}%")
    
    logger.info(f"\n基于初始资本的月收益预测 (USD):")
    for capital in [5000, 10000, 20000]:
        monthly = capital * us_portfolio['monthly_expected_return_pct'] / 100
        annual = monthly * 12
        logger.info(f"  ${capital:>6,}: 月 ${monthly:>7,.0f}, 年 ${annual:>9,.0f}")

def test_dataframe_export(calculator):
    """测试导出为DataFrame"""
    
    logger.info("\n" + "="*80)
    logger.info("测试 5: 导出为 Pandas DataFrame")
    logger.info("="*80)
    
    df = calculator.to_dataframe()
    
    logger.info(f"\nDataFrame 统计:")
    logger.info(f"  行数: {len(df)}")
    logger.info(f"  列数: {len(df.columns)}")
    logger.info(f"\nDataFrame 预览:")
    logger.info("\n" + df.to_string())
    
    return df

def main():
    """运行所有测试"""
    
    logger.info("\n")
    logger.info("#"*80)
    logger.info("# 股息轮动策略 - 收益率计算验证")
    logger.info("#"*80)
    
    try:
        # 测试1: 单笔交易
        trades = test_single_trade_yields()
        
        # 测试2: 策略聚合
        calculator, perf = test_strategy_aggregation(trades)
        
        # 测试3: 市场预期
        test_market_expectations()
        
        # 测试4: 组合预期
        test_portfolio_expectations()
        
        # 测试5: 导出DataFrame
        test_dataframe_export(calculator)
        
        logger.info("\n" + "#"*80)
        logger.info("# 所有测试完成 ✓")
        logger.info("#"*80 + "\n")
        
        return 0
        
    except Exception as e:
        logger.error(f"测试失败: {e}", exc_info=True)
        return 1

if __name__ == '__main__':
    sys.exit(main())
