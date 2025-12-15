#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
股息轮动策略 - 收益率分析工具
Dividend Rotation Strategy - Yield Analysis Tool

基于生成的60天前向计划，分析:
  1. 历史回测收益率 (如果有)
  2. 市场预期收益率 (基于实际股息数据)
  3. 复合收益预测 (月度/年度)
  4. 风险调整后的收益
"""

import argparse
import logging
import sys
from datetime import date, timedelta
from typing import List, Dict
import pandas as pd
import json

# 导入计算器
from dividend_yield_calculator import (
    DividendYieldCalculator,
    MarketExpectationCalculator,
    generate_yield_report
)

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(message)s',
    handlers=[logging.StreamHandler(sys.stdout)]
)
logger = logging.getLogger(__name__)

# ========================================
# 中国策略分析
# ========================================

def analyze_china_strategy(lookahead_days: int = 60):
    """分析中国策略的收益率"""
    
    logger.info("="*80)
    logger.info("中国股息轮动策略 - 收益率分析")
    logger.info("="*80)
    
    # 中国股票/ETF数据 (来自生成的计划)
    china_tickers = {
        '601988': {'name': '中国银行', 'market': 'A-share', 'ex_dates': [date(2025, 12, 5), date(2026, 1, 4)]},
        '601398': {'name': '工商银行', 'market': 'A-share', 'ex_dates': [date(2025, 12, 5), date(2026, 1, 4)]},
        '601288': {'name': '农业银行', 'market': 'A-share', 'ex_dates': [date(2025, 12, 10), date(2026, 1, 9)]},
        '600000': {'name': '浦发银行', 'market': 'A-share', 'ex_dates': [date(2025, 12, 3), date(2026, 1, 2)]},
        '000858': {'name': '五粮液', 'market': 'A-share', 'ex_dates': [date(2025, 12, 7), date(2026, 1, 6)]},
        
        '510300': {'name': '沪深300ETF', 'market': 'ETF', 'ex_dates': [date(2025, 11, 30), date(2025, 12, 30)]},
        '510500': {'name': '中证500ETF', 'market': 'ETF', 'ex_dates': [date(2025, 12, 2), date(2026, 1, 1)]},
        '510880': {'name': '红利ETF', 'market': 'ETF', 'ex_dates': [date(2025, 12, 5), date(2026, 1, 4)]},
        
        '00700.HK': {'name': '腾讯控股', 'market': 'H-share', 'ex_dates': [date(2025, 12, 15)]},
        '00939.HK': {'name': '中国建筑', 'market': 'H-share', 'ex_dates': [date(2025, 12, 10), date(2026, 1, 9)]},
        '01288.HK': {'name': '农业银行H股', 'market': 'H-share', 'ex_dates': [date(2025, 12, 13), date(2026, 1, 12)]},
    }
    
    # 分别计算各个股票的预期收益
    results = []
    
    logger.info("\n" + "─"*80)
    logger.info("单个股票预期收益率分析 (4-5天持仓周期)")
    logger.info("─"*80)
    
    for ticker, info in china_tickers.items():
        ret = MarketExpectationCalculator.calculate_expected_return(
            ticker, hold_days=4, region='CN', price_movement_pct=0.0
        )
        
        if ret:
            results.append(ret)
            logger.info(
                f"{ticker:10} {info['name']:12} | "
                f"年化分红: {ret['annual_yield_pct']:5.2f}% | "
                f"4天分红: {ret['hold_dividend_yield_pct']:5.3f}% | "
                f"预期总收益: {ret['expected_total_return_pct']:6.3f}% | "
                f"年化: {ret['expected_annualized_return_pct']:6.1f}%"
            )
    
    # 组合分析
    logger.info("\n" + "─"*80)
    logger.info("组合收益分析")
    logger.info("─"*80)
    
    portfolio = MarketExpectationCalculator.calculate_portfolio_return(
        list(china_tickers.keys()),
        hold_days=4,
        region='CN'
    )
    
    logger.info(f"组合规模: {portfolio['portfolio_size']} 个资产")
    logger.info(f"平均单次收益: {portfolio['average_return_pct']:.3f}%")
    logger.info(f"单次持仓: {portfolio['hold_days']} 天")
    logger.info(f"预期月交易: {portfolio['monthly_expected_trades']} 次")
    logger.info(f"预期月收益: {portfolio['monthly_expected_return_pct']:.2f}%")
    logger.info(f"预期年收益: {portfolio['monthly_expected_return_pct']*12:.2f}%")
    
    # 基于不同资本的收益预测
    logger.info("\n" + "─"*80)
    logger.info("收益预测 (基于初始资本)")
    logger.info("─"*80)
    
    capital_amounts = [50000, 100000, 200000, 500000]
    monthly_return_pct = portfolio['monthly_expected_return_pct']
    
    for capital in capital_amounts:
        monthly_income = capital * monthly_return_pct / 100
        annual_income = monthly_income * 12
        
        logger.info(
            f"初始资本 ¥{capital:>7,} | "
            f"月均收益 ¥{monthly_income:>8,.0f} | "
            f"年均收益 ¥{annual_income:>10,.0f}"
        )
    
    return portfolio

# ========================================
# 美国策略分析
# ========================================

def analyze_us_strategy():
    """分析美国策略的收益率"""
    
    logger.info("\n" + "="*80)
    logger.info("美国股息轮动策略 - 收益率分析")
    logger.info("="*80)
    
    us_etfs = {
        'JEPI': '7.2%',
        'XYLD': '8.3%',
        'SDIV': '8.9%',
        'VYM': '2.8%',
        'DGRO': '2.5%',
        'NOBL': '2.4%',
        'SCHD': '3.3%',
        'HDV': '3.8%',
    }
    
    results = []
    
    logger.info("\n" + "─"*80)
    logger.info("单个ETF预期收益率分析 (5-6天持仓周期)")
    logger.info("─"*80)
    
    for ticker in us_etfs.keys():
        ret = MarketExpectationCalculator.calculate_expected_return(
            ticker, hold_days=5, region='US', price_movement_pct=0.0
        )
        
        if ret:
            results.append(ret)
            logger.info(
                f"{ticker:6} {ret['name']:45} | "
                f"年化: {ret['annual_yield_pct']:5.2f}% | "
                f"5天收益: {ret['hold_dividend_yield_pct']:5.3f}% | "
                f"年化: {ret['expected_annualized_return_pct']:6.1f}%"
            )
    
    # 组合分析
    logger.info("\n" + "─"*80)
    logger.info("组合收益分析")
    logger.info("─"*80)
    
    portfolio = MarketExpectationCalculator.calculate_portfolio_return(
        list(us_etfs.keys()),
        hold_days=5,
        region='US'
    )
    
    logger.info(f"组合规模: {portfolio['portfolio_size']} 个ETF")
    logger.info(f"平均单次收益: {portfolio['average_return_pct']:.3f}%")
    logger.info(f"单次持仓: {portfolio['hold_days']} 天")
    logger.info(f"预期月交易: {portfolio['monthly_expected_trades']} 次")
    logger.info(f"预期月收益: {portfolio['monthly_expected_return_pct']:.2f}%")
    logger.info(f"预期年收益: {portfolio['monthly_expected_return_pct']*12:.2f}%")
    
    # 基于不同资本的收益预测
    logger.info("\n" + "─"*80)
    logger.info("收益预测 (基于初始资本)")
    logger.info("─"*80)
    
    capital_amounts = [5000, 10000, 20000, 50000]
    monthly_return_pct = portfolio['monthly_expected_return_pct']
    
    for capital in capital_amounts:
        monthly_income = capital * monthly_return_pct / 100
        annual_income = monthly_income * 12
        
        logger.info(
            f"初始资本 ${capital:>6,} | "
            f"月均收益 ${monthly_income:>7,.0f} | "
            f"年均收益 ${annual_income:>9,.0f}"
        )
    
    return portfolio

# ========================================
# 对比分析
# ========================================

def compare_strategies(cn_portfolio: Dict, us_portfolio: Dict):
    """对比两个策略"""
    
    logger.info("\n" + "="*80)
    logger.info("策略对比分析")
    logger.info("="*80)
    
    logger.info("\n" + "─"*80)
    logger.info("关键指标对比")
    logger.info("─"*80)
    
    comparison_data = {
        '指标': [
            '资产数量',
            '单次持仓天数',
            '单次平均收益率',
            '预期月交易笔数',
            '预期月收益率',
            '预期年收益率',
            '交易货币',
            '难度等级'
        ],
        '中国策略': [
            f"{cn_portfolio['portfolio_size']} 个",
            f"{cn_portfolio['hold_days']} 天",
            f"{cn_portfolio['average_return_pct']:.3f}%",
            f"{cn_portfolio['monthly_expected_trades']} 次",
            f"{cn_portfolio['monthly_expected_return_pct']:.2f}%",
            f"{cn_portfolio['monthly_expected_return_pct']*12:.2f}%",
            'CNY + HKD',
            '中等'
        ],
        '美国策略': [
            f"{us_portfolio['portfolio_size']} 个",
            f"{us_portfolio['hold_days']} 天",
            f"{us_portfolio['average_return_pct']:.3f}%",
            f"{us_portfolio['monthly_expected_trades']} 次",
            f"{us_portfolio['monthly_expected_return_pct']:.2f}%",
            f"{us_portfolio['monthly_expected_return_pct']*12:.2f}%",
            'USD',
            '简单'
        ]
    }
    
    df = pd.DataFrame(comparison_data)
    logger.info("\n" + df.to_string(index=False))
    
    # 组合策略分析
    logger.info("\n" + "─"*80)
    logger.info("组合策略分析 (40% US + 60% CN)")
    logger.info("─"*80)
    
    us_weight = 0.4
    cn_weight = 0.6
    
    blended_monthly_return = (
        us_portfolio['monthly_expected_return_pct'] * us_weight +
        cn_portfolio['monthly_expected_return_pct'] * cn_weight
    )
    
    blended_annual_return = blended_monthly_return * 12
    
    logger.info(f"混合月收益率: {blended_monthly_return:.2f}%")
    logger.info(f"混合年收益率: {blended_annual_return:.2f}%")
    
    logger.info("\n基于初始资本预测 (USD/CNY各一半):")
    
    # 假设 $20k + ¥200k
    capital_usd = 20000
    capital_cny = 200000
    
    monthly_income_usd = capital_usd * us_portfolio['monthly_expected_return_pct'] / 100
    monthly_income_cny = capital_cny * cn_portfolio['monthly_expected_return_pct'] / 100
    
    logger.info(f"美国账户月均收益: ${monthly_income_usd:,.0f}")
    logger.info(f"中国账户月均收益: ¥{monthly_income_cny:,.0f}")
    logger.info(f"总月均收益 (按美元): ${monthly_income_usd + monthly_income_cny/6.5:,.0f}")
    logger.info(f"总年均收益 (按美元): ${(monthly_income_usd + monthly_income_cny/6.5)*12:,.0f}")

# ========================================
# 主函数
# ========================================

def main():
    parser = argparse.ArgumentParser(
        description='股息轮动策略收益率分析工具',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  python yield_analysis.py --all
  python yield_analysis.py --china
  python yield_analysis.py --us
  python yield_analysis.py --compare
        """
    )
    
    parser.add_argument('--all', action='store_true', help='分析所有策略')
    parser.add_argument('--china', action='store_true', help='仅分析中国策略')
    parser.add_argument('--us', action='store_true', help='仅分析美国策略')
    parser.add_argument('--compare', action='store_true', help='策略对比')
    parser.add_argument('--lookahead', type=int, default=60, help='前向窗口(天)')
    
    args = parser.parse_args()
    
    # 默认分析所有
    if not (args.all or args.china or args.us or args.compare):
        args.all = True
    
    cn_portfolio = None
    us_portfolio = None
    
    if args.all or args.china:
        cn_portfolio = analyze_china_strategy(args.lookahead)
    
    if args.all or args.us:
        us_portfolio = analyze_us_strategy()
    
    if args.all or args.compare:
        if not cn_portfolio:
            cn_portfolio = analyze_china_strategy(args.lookahead)
        if not us_portfolio:
            us_portfolio = analyze_us_strategy()
        compare_strategies(cn_portfolio, us_portfolio)
    
    logger.info("\n" + "="*80)
    logger.info("分析完成")
    logger.info("="*80)

if __name__ == '__main__':
    main()
