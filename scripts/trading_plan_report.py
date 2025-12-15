#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
股息轮动策略 - 交易计划收益率报告
Dividend Rotation Strategy - Trading Plan Yield Report

将生成的60天前向计划与收益率计算结合，提供：
  1. 每笔交易的预期收益率
  2. 累计月度收益预测
  3. 风险调整指标
  4. 实际执行跟踪表格
"""

import sys
import logging
from datetime import date, timedelta
from typing import List, Dict
import pandas as pd

from dividend_yield_calculator import (
    DividendYieldCalculator,
    DividendYieldAnalysis,
    MarketExpectationCalculator
)

logging.basicConfig(level=logging.INFO, format='%(message)s')
logger = logging.getLogger(__name__)

# ========================================
# 中国策略交易计划
# ========================================

class ChinaTradingPlanAnalyzer:
    """分析中国股息交易计划"""
    
    def __init__(self):
        self.trades: List[Dict] = [
            # 11月份交易
            {'ticker': '601988', 'name': '中国银行', 'ex_date': date(2025, 11, 28), 'buy_price': 3.15, 'div_per_share': 0.033},
            {'ticker': '510300', 'name': '沪深300', 'ex_date': date(2025, 11, 30), 'buy_price': 5.25, 'div_per_share': 0.018},
            {'ticker': '601398', 'name': '工商银行', 'ex_date': date(2025, 12, 5), 'buy_price': 5.80, 'div_per_share': 0.028},
            {'ticker': '601288', 'name': '农业银行', 'ex_date': date(2025, 12, 10), 'buy_price': 3.90, 'div_per_share': 0.032},
            {'ticker': '000858', 'name': '五粮液', 'ex_date': date(2025, 12, 7), 'buy_price': 120.00, 'div_per_share': 2.50},
            
            # 12月份交易
            {'ticker': '600000', 'name': '浦发银行', 'ex_date': date(2025, 12, 3), 'buy_price': 8.50, 'div_per_share': 0.42},
            {'ticker': '510880', 'name': '红利ETF', 'ex_date': date(2025, 12, 5), 'buy_price': 2.80, 'div_per_share': 0.028},
            {'ticker': '510500', 'name': '中证500', 'ex_date': date(2025, 12, 2), 'buy_price': 6.40, 'div_per_share': 0.015},
            {'ticker': '00700.HK', 'name': '腾讯控股', 'ex_date': date(2025, 12, 15), 'buy_price': 390.00, 'div_per_share': 5.40},
            {'ticker': '00939.HK', 'name': '中国建筑', 'ex_date': date(2025, 12, 10), 'buy_price': 4.25, 'div_per_share': 0.22},
            
            # 1月份交易
            {'ticker': '01288.HK', 'name': '农业银行H', 'ex_date': date(2026, 1, 13), 'buy_price': 2.85, 'div_per_share': 0.045},
        ]
        
        self.calculator = DividendYieldCalculator()
    
    def generate_report(self):
        """生成完整的交易计划收益率报告"""
        
        logger.info("="*100)
        logger.info("中国股息轮动策略 - 60天交易计划收益率分析")
        logger.info("="*100)
        
        # 分析每笔交易
        logger.info("\n" + "─"*100)
        logger.info("单笔交易分析 (基于市场历史数据)")
        logger.info("─"*100)
        
        analyses = []
        for trade in self.trades:
            # 计算持仓天数 (T-2到T+1)
            buy_date = trade['ex_date'] - timedelta(days=2)
            sell_date = trade['ex_date'] + timedelta(days=1)
            hold_days = (sell_date - buy_date).days
            
            # 获取市场数据
            mkt_data = MarketExpectationCalculator.get_market_yield(trade['ticker'], region='CN')
            # 预期价格变化: 3-4天持有中预期不大波动，假设+0.2%
            expected_price_change_pct = 0.2
            sell_price = trade['buy_price'] * (1 + (expected_price_change_pct / 100))
            
            # 创建分析对象
            analysis = DividendYieldAnalysis(
                ticker=trade['ticker'],
                trade_date=trade['ex_date'],
                buy_date=buy_date,
                sell_date=sell_date,
                buy_price=trade['buy_price'],
                sell_price=sell_price,
                shares=1000 if trade['ticker'].startswith(('60', '00')) else 2000,
                dividend_per_share=trade['div_per_share']
            )
            
            analyses.append(analysis)
            self.calculator.add_trade(
                ticker=analysis.ticker,
                trade_date=analysis.trade_date,
                buy_date=analysis.buy_date,
                sell_date=analysis.sell_date,
                buy_price=analysis.buy_price,
                sell_price=analysis.sell_price,
                dividend_per_share=analysis.dividend_per_share,
                shares=analysis.shares
            )
            
            # 打印单笔分析
            logger.info(
                f"{trade['ticker']:10} {trade['name']:15} | "
                f"持仓: {hold_days}天 | "
                f"分红率: {analysis.dividend_yield_pct:6.3f}% | "
                f"价差: {analysis.price_change_pct:+6.2f}% | "
                f"总收益: {analysis.total_return_pct:+6.3f}% | "
                f"年化: {analysis.annualized_return_pct:+6.1f}%"
            )
        
        # 策略聚合分析
        logger.info("\n" + "─"*100)
        logger.info("策略聚合分析")
        logger.info("─"*100)
        
        perf = self.calculator.calculate_strategy_performance()
        
        logger.info(f"总交易笔数: {perf.total_trades}")
        logger.info(f"获利笔数: {perf.winning_trades} ({perf.win_rate*100:.1f}%)")
        logger.info(f"平均单笔收益: {perf.avg_return_per_trade:.3f}%")
        logger.info(f"平均年化收益: {perf.avg_annualized_return:.1f}%")
        logger.info(f"利润因子: {perf.profit_factor:.2f}")
        
        # 预期月度收益
        logger.info("\n" + "─"*100)
        logger.info("月度收益预测")
        logger.info("─"*100)
        
        logger.info(f"预期月交易笔数: {perf.monthly_expected_trades}")
        logger.info(f"预期月平均收益: {perf.monthly_expected_return_pct:.2f}%")
        logger.info(f"预期年平均收益: {perf.annual_expected_return_pct:.2f}%")
        
        # 基于不同资本的收益预测
        logger.info("\n" + "─"*100)
        logger.info("收益预测 (基于初始资本)")
        logger.info("─"*100)
        
        capital_amounts = [50000, 100000, 200000, 500000]
        
        for capital in capital_amounts:
            monthly_income = capital * perf.monthly_expected_return_pct / 100
            annual_income = monthly_income * 12
            
            logger.info(
                f"初始资本 ¥{capital:>7,} | "
                f"月均收益 ¥{monthly_income:>8,.0f} | "
                f"年均收益 ¥{annual_income:>10,.0f}"
            )
        
        # 风险分析
        logger.info("\n" + "─"*100)
        logger.info("风险指标")
        logger.info("─"*100)
        
        negative_trades = len([a for a in analyses if a.total_return_pct < 0])
        max_loss = min([a.total_return_pct for a in analyses])
        max_gain = max([a.total_return_pct for a in analyses])
        avg_loss = sum([a.total_return_pct for a in analyses if a.total_return_pct < 0]) / max(negative_trades, 1)
        
        logger.info(f"亏损交易数: {negative_trades} ({negative_trades/len(analyses)*100:.1f}%)")
        logger.info(f"最大单笔亏损: {max_loss:.3f}%")
        logger.info(f"最大单笔收益: {max_gain:.3f}%")
        logger.info(f"平均亏损额: {avg_loss:.3f}%")
        
        # 执行追踪表
        logger.info("\n" + "─"*100)
        logger.info("执行追踪表 (用于记录实际执行结果)")
        logger.info("─"*100)
        
        df_data = []
        for i, analysis in enumerate(analyses, 1):
            df_data.append({
                '序号': i,
                '代码': analysis.ticker,
                '买入日': analysis.buy_date.strftime('%Y-%m-%d'),
                '卖出日': analysis.sell_date.strftime('%Y-%m-%d'),
                '预期分红%': f"{analysis.dividend_yield_pct:.3f}%",
                '预期收益%': f"{analysis.total_return_pct:.3f}%",
                '实际买价': '',
                '实际卖价': '',
                '实际分红': '',
                '实际收益%': '',
                '状态': '待执行'
            })
        
        df = pd.DataFrame(df_data)
        logger.info("\n" + df.to_string(index=False))
        
        # 导出到CSV
        csv_path = 'China_Trading_Plan_with_Yields.csv'
        df.to_csv(csv_path, index=False, encoding='utf-8-sig')
        logger.info(f"\n已保存到: {csv_path}")
        
        return perf

# ========================================
# 美国策略交易计划
# ========================================

class USTradingPlanAnalyzer:
    """分析美国股息交易计划"""
    
    def __init__(self):
        self.trades: List[Dict] = [
            # 根据FORWARD_PLAN_60DAY.md生成的示例
            {'ticker': 'JEPI', 'ex_date': date(2025, 11, 15), 'buy_price': 50.00, 'div_per_share': 0.60},
            {'ticker': 'XYLD', 'ex_date': date(2025, 11, 20), 'buy_price': 25.00, 'div_per_share': 0.50},
            {'ticker': 'SDIV', 'ex_date': date(2025, 12, 5), 'buy_price': 15.00, 'div_per_share': 0.65},
            {'ticker': 'VYM', 'ex_date': date(2025, 12, 10), 'buy_price': 130.00, 'div_per_share': 3.50},
            {'ticker': 'DGRO', 'ex_date': date(2025, 12, 15), 'buy_price': 85.00, 'div_per_share': 2.10},
            {'ticker': 'NOBL', 'ex_date': date(2025, 12, 20), 'buy_price': 65.00, 'div_per_share': 1.55},
            {'ticker': 'SCHD', 'ex_date': date(2025, 12, 25), 'buy_price': 75.00, 'div_per_share': 2.40},
            {'ticker': 'HDV', 'ex_date': date(2026, 1, 10), 'buy_price': 110.00, 'div_per_share': 4.10},
        ]
        
        self.calculator = DividendYieldCalculator()
    
    def generate_report(self):
        """生成完整的交易计划收益率报告"""
        
        logger.info("\n" + "="*100)
        logger.info("美国股息轮动策略 - 60天交易计划收益率分析")
        logger.info("="*100)
        
        # 分析每笔交易
        logger.info("\n" + "─"*100)
        logger.info("单笔交易分析 (基于市场历史数据)")
        logger.info("─"*100)
        
        analyses = []
        for trade in self.trades:
            # 计算持仓天数 (T-2到T+1)
            buy_date = trade['ex_date'] - timedelta(days=2)
            sell_date = trade['ex_date'] + timedelta(days=1)
            hold_days = (sell_date - buy_date).days
            
            # 获取市场数据
            mkt_data = MarketExpectationCalculator.get_market_yield(trade['ticker'], region='US')
            # 预期价格变化: 3-4天持有中预期不大波动，假设+0.3%
            expected_price_change_pct = 0.3
            sell_price = trade['buy_price'] * (1 + (expected_price_change_pct / 100))
            
            # 创建分析对象
            analysis = DividendYieldAnalysis(
                ticker=trade['ticker'],
                trade_date=trade['ex_date'],
                buy_date=buy_date,
                sell_date=sell_date,
                buy_price=trade['buy_price'],
                sell_price=sell_price,
                shares=100,
                dividend_per_share=trade['div_per_share']
            )
            
            analyses.append(analysis)
            self.calculator.add_trade(
                ticker=analysis.ticker,
                trade_date=analysis.trade_date,
                buy_date=analysis.buy_date,
                sell_date=analysis.sell_date,
                buy_price=analysis.buy_price,
                sell_price=analysis.sell_price,
                dividend_per_share=analysis.dividend_per_share,
                shares=analysis.shares
            )
            
            # 打印单笔分析
            logger.info(
                f"{trade['ticker']:6} | "
                f"持仓: {hold_days}天 | "
                f"分红率: {analysis.dividend_yield_pct:6.3f}% | "
                f"价差: {analysis.price_change_pct:+6.2f}% | "
                f"总收益: {analysis.total_return_pct:+6.3f}% | "
                f"年化: {analysis.annualized_return_pct:+6.1f}%"
            )
        
        # 策略聚合分析
        logger.info("\n" + "─"*100)
        logger.info("策略聚合分析")
        logger.info("─"*100)
        
        perf = self.calculator.calculate_strategy_performance()
        
        logger.info(f"总交易笔数: {perf.total_trades}")
        logger.info(f"获利笔数: {perf.winning_trades} ({perf.win_rate*100:.1f}%)")
        logger.info(f"平均单笔收益: {perf.avg_return_per_trade:.3f}%")
        logger.info(f"平均年化收益: {perf.avg_annualized_return:.1f}%")
        logger.info(f"利润因子: {perf.profit_factor:.2f}")
        
        # 预期月度收益
        logger.info("\n" + "─"*100)
        logger.info("月度收益预测")
        logger.info("─"*100)
        
        logger.info(f"预期月交易笔数: {perf.monthly_expected_trades}")
        logger.info(f"预期月平均收益: {perf.monthly_expected_return_pct:.2f}%")
        logger.info(f"预期年平均收益: {perf.annual_expected_return_pct:.2f}%")
        
        # 基于不同资本的收益预测
        logger.info("\n" + "─"*100)
        logger.info("收益预测 (基于初始资本)")
        logger.info("─"*100)
        
        capital_amounts = [5000, 10000, 20000, 50000]
        
        for capital in capital_amounts:
            monthly_income = capital * perf.monthly_expected_return_pct / 100
            annual_income = monthly_income * 12
            
            logger.info(
                f"初始资本 ${capital:>6,} | "
                f"月均收益 ${monthly_income:>7,.0f} | "
                f"年均收益 ${annual_income:>9,.0f}"
            )
        
        # 风险分析
        logger.info("\n" + "─"*100)
        logger.info("风险指标")
        logger.info("─"*100)
        
        negative_trades = len([a for a in analyses if a.total_return_pct < 0])
        max_loss = min([a.total_return_pct for a in analyses])
        max_gain = max([a.total_return_pct for a in analyses])
        avg_loss = sum([a.total_return_pct for a in analyses if a.total_return_pct < 0]) / max(negative_trades, 1)
        
        logger.info(f"亏损交易数: {negative_trades} ({negative_trades/len(analyses)*100:.1f}%)")
        logger.info(f"最大单笔亏损: {max_loss:.3f}%")
        logger.info(f"最大单笔收益: {max_gain:.3f}%")
        logger.info(f"平均亏损额: {avg_loss:.3f}%")
        
        # 执行追踪表
        logger.info("\n" + "─"*100)
        logger.info("执行追踪表 (用于记录实际执行结果)")
        logger.info("─"*100)
        
        df_data = []
        for i, analysis in enumerate(analyses, 1):
            df_data.append({
                '序号': i,
                '代码': analysis.ticker,
                '买入日': analysis.buy_date.strftime('%Y-%m-%d'),
                '卖出日': analysis.sell_date.strftime('%Y-%m-%d'),
                '预期分红%': f"{analysis.dividend_yield_pct:.3f}%",
                '预期收益%': f"{analysis.total_return_pct:.3f}%",
                '实际买价': '',
                '实际卖价': '',
                '实际分红': '',
                '实际收益%': '',
                '状态': '待执行'
            })
        
        df = pd.DataFrame(df_data)
        logger.info("\n" + df.to_string(index=False))
        
        # 导出到CSV
        csv_path = 'US_Trading_Plan_with_Yields.csv'
        df.to_csv(csv_path, index=False, encoding='utf-8-sig')
        logger.info(f"\n已保存到: {csv_path}")
        
        return perf

# ========================================
# 主函数
# ========================================

def main():
    """生成完整的交易计划收益率报告"""
    
    # 中国策略分析
    cn_analyzer = ChinaTradingPlanAnalyzer()
    cn_perf = cn_analyzer.generate_report()
    
    # 美国策略分析
    us_analyzer = USTradingPlanAnalyzer()
    us_perf = us_analyzer.generate_report()
    
    # 总体对比
    logger.info("\n" + "="*100)
    logger.info("双策略组合分析")
    logger.info("="*100)
    
    logger.info("\n" + "─"*100)
    logger.info("策略对比")
    logger.info("─"*100)
    
    comparison = pd.DataFrame({
        '指标': [
            '交易笔数',
            '获利率',
            '平均收益',
            '预期月收益',
            '预期年收益',
        ],
        '中国策略': [
            f"{cn_perf.total_trades}",
            f"{cn_perf.win_rate*100:.1f}%",
            f"{cn_perf.avg_return_per_trade:.3f}%",
            f"{cn_perf.monthly_expected_return_pct:.2f}%",
            f"{cn_perf.annual_expected_return_pct:.2f}%",
        ],
        '美国策略': [
            f"{us_perf.total_trades}",
            f"{us_perf.win_rate*100:.1f}%",
            f"{us_perf.avg_return_per_trade:.3f}%",
            f"{us_perf.monthly_expected_return_pct:.2f}%",
            f"{us_perf.annual_expected_return_pct:.2f}%",
        ]
    })
    
    logger.info("\n" + comparison.to_string(index=False))
    
    logger.info("\n" + "="*100)
    logger.info("报告生成完成")
    logger.info("交易执行追踪表已保存到CSV文件")
    logger.info("="*100)

if __name__ == '__main__':
    main()
