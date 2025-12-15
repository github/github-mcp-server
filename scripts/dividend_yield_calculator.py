#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Dividend Yield Calculator - 股息收益率计算器
基于历史回测与市场预期的收益率分析

支持:
  - 历史回测收益率 (Historical backtest returns)
  - 市场预期收益率 (Market expected yields)
  - 单笔交易收益计算 (Per-trade P&L)
  - 月度/年度复合收益 (Monthly/Annual compounding)
  - 分策略对比分析 (Strategy comparison)
"""

import logging
import sys
from datetime import datetime, date, timedelta
from typing import List, Dict, Tuple, Optional
from dataclasses import dataclass
import pandas as pd
import numpy as np

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(message)s',
    handlers=[logging.StreamHandler(sys.stdout)]
)
logger = logging.getLogger(__name__)

# ========================================
# Data Models
# ========================================

@dataclass
class DividendYieldAnalysis:
    """单笔交易的收益率分析"""
    ticker: str
    trade_date: date
    buy_date: date
    sell_date: date
    buy_price: float
    sell_price: float
    dividend_per_share: float
    shares: int
    
    # 计算字段
    hold_days: int = None
    price_change_pct: float = None  # 价格变动百分比
    dividend_yield_pct: float = None  # 分红收益率
    total_return_pct: float = None   # 总收益率
    annualized_return_pct: float = None  # 年化收益率
    
    def __post_init__(self):
        """自动计算收益率"""
        self.hold_days = (self.sell_date - self.buy_date).days
        
        # 价格变动百分比
        if self.buy_price > 0:
            self.price_change_pct = ((self.sell_price - self.buy_price) / self.buy_price) * 100
        else:
            self.price_change_pct = 0
        
        # 分红收益率
        if self.buy_price > 0:
            self.dividend_yield_pct = (self.dividend_per_share / self.buy_price) * 100
        else:
            self.dividend_yield_pct = 0
        
        # 总收益率 (分红 + 价格变动)
        self.total_return_pct = self.dividend_yield_pct + self.price_change_pct
        
        # 年化收益率
        if self.hold_days > 0:
            days_per_year = 365
            self.annualized_return_pct = self.total_return_pct * (days_per_year / self.hold_days)
        else:
            self.annualized_return_pct = 0

@dataclass
class StrategyPerformance:
    """策略整体表现分析"""
    strategy_name: str
    total_trades: int
    winning_trades: int
    losing_trades: int
    
    total_capital_deployed: float  # 总投入资本
    total_dividends_received: float  # 总收到的分红
    total_price_gains: float  # 总价差收益
    total_pnl: float  # 总盈亏
    
    avg_hold_days: float  # 平均持仓天数
    avg_return_per_trade: float  # 平均每笔交易收益率
    avg_annualized_return: float  # 平均年化收益率
    
    win_rate: float  # 胜率
    profit_factor: float  # 盈亏因子 (总胜/总负)
    
    # 预期指标
    monthly_expected_trades: int  # 预期月交易数
    monthly_expected_return_pct: float  # 预期月收益率
    annual_expected_return_pct: float  # 预期年收益率

# ========================================
# 核心计算函数
# ========================================

class DividendYieldCalculator:
    """股息收益率计算器"""
    
    def __init__(self):
        self.trades: List[DividendYieldAnalysis] = []
        self.logger = logger
    
    def add_trade(self, ticker: str, trade_date: date, buy_date: date, sell_date: date,
                  buy_price: float, sell_price: float, dividend_per_share: float, 
                  shares: int = 100):
        """添加单笔交易"""
        trade = DividendYieldAnalysis(
            ticker=ticker,
            trade_date=trade_date,
            buy_date=buy_date,
            sell_date=sell_date,
            buy_price=buy_price,
            sell_price=sell_price,
            dividend_per_share=dividend_per_share,
            shares=shares
        )
        self.trades.append(trade)
    
    def calculate_trade_pnl(self, trade: DividendYieldAnalysis) -> Dict:
        """计算单笔交易的P&L"""
        buy_cost = trade.buy_price * trade.shares
        sell_revenue = trade.sell_price * trade.shares
        dividend_cash = trade.dividend_per_share * trade.shares
        
        total_pnl = (sell_revenue - buy_cost) + dividend_cash
        
        return {
            'buy_cost': buy_cost,
            'sell_revenue': sell_revenue,
            'dividend_cash': dividend_cash,
            'total_pnl': total_pnl,
            'price_gain': sell_revenue - buy_cost,
            'dividend_gain': dividend_cash
        }
    
    def calculate_strategy_performance(self, strategy_name: str = "Default",
                                      monthly_trades: int = 5) -> StrategyPerformance:
        """计算策略整体表现"""
        if not self.trades:
            self.logger.warning("No trades to analyze")
            return None
        
        # 统计基础数据
        total_trades = len(self.trades)
        winning_trades = sum(1 for t in self.trades if t.total_return_pct > 0)
        losing_trades = total_trades - winning_trades
        
        win_rate = winning_trades / total_trades if total_trades > 0 else 0
        
        # 计算各项收益
        total_capital = sum(t.buy_price * t.shares for t in self.trades)
        total_dividends = sum(t.dividend_per_share * t.shares for t in self.trades)
        
        total_price_gains = sum(
            (t.sell_price - t.buy_price) * t.shares for t in self.trades
        )
        
        total_pnl = total_price_gains + total_dividends
        
        # 平均指标
        avg_return_per_trade = np.mean([t.total_return_pct for t in self.trades])
        avg_annualized = np.mean([t.annualized_return_pct for t in self.trades])
        avg_hold_days = np.mean([t.hold_days for t in self.trades])
        
        # 盈亏因子
        win_pnl = sum(self.calculate_trade_pnl(t)['total_pnl'] 
                     for t in self.trades if t.total_return_pct > 0)
        lose_pnl = abs(sum(self.calculate_trade_pnl(t)['total_pnl'] 
                          for t in self.trades if t.total_return_pct < 0))
        
        profit_factor = win_pnl / lose_pnl if lose_pnl > 0 else float('inf')
        
        # 预期指标 (基于历史平均)
        monthly_expected_trades = monthly_trades
        monthly_expected_return = avg_return_per_trade * monthly_expected_trades
        annual_expected_return = monthly_expected_return * 12
        
        return StrategyPerformance(
            strategy_name=strategy_name,
            total_trades=total_trades,
            winning_trades=winning_trades,
            losing_trades=losing_trades,
            total_capital_deployed=total_capital,
            total_dividends_received=total_dividends,
            total_price_gains=total_price_gains,
            total_pnl=total_pnl,
            avg_hold_days=avg_hold_days,
            avg_return_per_trade=avg_return_per_trade,
            avg_annualized_return=avg_annualized,
            win_rate=win_rate,
            profit_factor=profit_factor,
            monthly_expected_trades=monthly_expected_trades,
            monthly_expected_return_pct=monthly_expected_return,
            annual_expected_return_pct=annual_expected_return
        )
    
    def to_dataframe(self) -> pd.DataFrame:
        """转换为DataFrame便于分析"""
        data = []
        for trade in self.trades:
            data.append({
                'Ticker': trade.ticker,
                'Buy Date': trade.buy_date,
                'Sell Date': trade.sell_date,
                'Hold Days': trade.hold_days,
                'Buy Price': trade.buy_price,
                'Sell Price': trade.sell_price,
                'Price Change %': trade.price_change_pct,
                'Dividend/Share': trade.dividend_per_share,
                'Dividend Yield %': trade.dividend_yield_pct,
                'Total Return %': trade.total_return_pct,
                'Annualized Return %': trade.annualized_return_pct,
                'Shares': trade.shares
            })
        return pd.DataFrame(data)

# ========================================
# 市场预期计算
# ========================================

class MarketExpectationCalculator:
    """市场预期收益率计算"""
    
    # 中国高分红股票/ETF的市场数据
    CHINA_DIVIDEND_DATA = {
        # A股
        '601988': {'name': '中国银行', 'market': 'A-share', 'annual_yield': 0.055, 'avg_dividend': 0.033},
        '601398': {'name': '工商银行', 'market': 'A-share', 'annual_yield': 0.047, 'avg_dividend': 0.028},
        '601288': {'name': '农业银行', 'market': 'A-share', 'annual_yield': 0.054, 'avg_dividend': 0.032},
        '600000': {'name': '浦发银行', 'market': 'A-share', 'annual_yield': 0.049, 'avg_dividend': 0.025},
        '000858': {'name': '五粮液', 'market': 'A-share', 'annual_yield': 0.018, 'avg_dividend': 0.008},
        
        # ETF
        '510300': {'name': '沪深300ETF', 'market': 'ETF', 'annual_yield': 0.032, 'avg_dividend': 0.018},
        '510500': {'name': '中证500ETF', 'market': 'ETF', 'annual_yield': 0.025, 'avg_dividend': 0.015},
        '510880': {'name': '红利ETF', 'market': 'ETF', 'annual_yield': 0.045, 'avg_dividend': 0.028},
        
        # H股
        '00700.HK': {'name': '腾讯控股', 'market': 'H-share', 'annual_yield': 0.015, 'avg_dividend': 0.015},
        '00939.HK': {'name': '中国建筑', 'market': 'H-share', 'annual_yield': 0.052, 'avg_dividend': 0.032},
        '01288.HK': {'name': '农业银行H股', 'market': 'H-share', 'annual_yield': 0.058, 'avg_dividend': 0.035},
    }
    
    # 美国高分红ETF数据
    US_DIVIDEND_DATA = {
        'JEPI': {'name': 'JPMorgan Equity Premium Income', 'annual_yield': 0.072, 'avg_dividend': 0.60},
        'XYLD': {'name': 'Global X Russell 2000 Covered Call', 'annual_yield': 0.083, 'avg_dividend': 0.50},
        'SDIV': {'name': 'Global X SuperDividend US ETF', 'annual_yield': 0.089, 'avg_dividend': 0.65},
        'VYM': {'name': 'Vanguard High Dividend Yield', 'annual_yield': 0.028, 'avg_dividend': 1.20},
        'DGRO': {'name': 'iShares Core Dividend Growth', 'annual_yield': 0.025, 'avg_dividend': 0.85},
        'NOBL': {'name': 'ProShares S&P 500 Dividend Aristocrats', 'annual_yield': 0.024, 'avg_dividend': 1.45},
        'SCHD': {'name': 'Schwab U.S. Dividend Equity', 'annual_yield': 0.033, 'avg_dividend': 0.90},
        'HDV': {'name': 'iShares Core High Dividend ETF', 'annual_yield': 0.038, 'avg_dividend': 1.32},
    }
    
    @staticmethod
    def get_market_yield(ticker: str, region: str = 'CN') -> Optional[Dict]:
        """获取市场预期收益率"""
        if region == 'CN':
            return MarketExpectationCalculator.CHINA_DIVIDEND_DATA.get(ticker)
        elif region == 'US':
            return MarketExpectationCalculator.US_DIVIDEND_DATA.get(ticker)
        return None
    
    @staticmethod
    def calculate_expected_return(ticker: str, hold_days: int = 4,
                                  region: str = 'CN',
                                  price_movement_pct: float = 0.0) -> Optional[Dict]:
        """计算预期收益 (分红 + 价格变动)"""
        data = MarketExpectationCalculator.get_market_yield(ticker, region)
        if not data:
            return None
        
        annual_yield = data['annual_yield']
        
        # 按持仓天数计算分红收益率
        hold_dividend_yield = (annual_yield / 365) * hold_days * 100
        
        # 总收益率
        total_return = hold_dividend_yield + price_movement_pct
        
        # 年化收益率
        annualized_return = (total_return / hold_days) * 365 if hold_days > 0 else 0
        
        return {
            'ticker': ticker,
            'name': data['name'],
            'market': data['market'],
            'annual_yield_pct': annual_yield * 100,
            'expected_dividend_per_share': data['avg_dividend'],
            'hold_days': hold_days,
            'hold_dividend_yield_pct': hold_dividend_yield,
            'price_movement_pct': price_movement_pct,
            'expected_total_return_pct': total_return,
            'expected_annualized_return_pct': annualized_return
        }
    
    @staticmethod
    def calculate_portfolio_return(tickers: List[str], hold_days: int = 4,
                                  region: str = 'CN') -> Dict:
        """计算组合预期收益"""
        returns = []
        total_return = 0
        
        for ticker in tickers:
            ret = MarketExpectationCalculator.calculate_expected_return(
                ticker, hold_days, region
            )
            if ret:
                returns.append(ret)
                total_return += ret['expected_total_return_pct']
        
        avg_return = total_return / len(returns) if returns else 0
        
        return {
            'portfolio_size': len(returns),
            'individual_returns': returns,
            'average_return_pct': avg_return,
            'total_return_if_all_executed': total_return,
            'hold_days': hold_days,
            'monthly_expected_trades': 20 // hold_days if hold_days > 0 else 0,
            'monthly_expected_return_pct': avg_return * (20 // hold_days) if hold_days > 0 else 0
        }

# ========================================
# 报告生成
# ========================================

def generate_yield_report(calculator: DividendYieldCalculator,
                         strategy_name: str = "Dividend Rotation",
                         output_file: Optional[str] = None) -> str:
    """生成股息收益率报告"""
    
    if not calculator.trades:
        return "No trades to report"
    
    perf = calculator.calculate_strategy_performance(strategy_name)
    df = calculator.to_dataframe()
    
    report = f"""
{'='*80}
股息轮动策略收益率分析报告
Dividend Rotation Yield Analysis Report
{'='*80}

策略名称: {perf.strategy_name}
报告日期: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}

{'─'*80}
1. 回测统计 (Backtest Statistics)
{'─'*80}

总交易笔数: {perf.total_trades}
成功交易: {perf.winning_trades} ({perf.win_rate*100:.1f}%)
失败交易: {perf.losing_trades} ({(1-perf.win_rate)*100:.1f}%)

{'─'*80}
2. 收益指标 (Return Metrics)
{'─'*80}

总投入资本: ¥{perf.total_capital_deployed:,.2f}
总分红收入: ¥{perf.total_dividends_received:,.2f}
总价差收益: ¥{perf.total_price_gains:,.2f}
总盈亏 (P&L): ¥{perf.total_pnl:,.2f}

平均持仓天数: {perf.avg_hold_days:.1f} days
平均每笔收益率: {perf.avg_return_per_trade:.2f}%
平均年化收益率: {perf.avg_annualized_return:.2f}%

盈亏因子: {perf.profit_factor:.2f}x

{'─'*80}
3. 预期收益 (Expected Returns)
{'─'*80}

预期月交易笔数: {perf.monthly_expected_trades}
预期月度收益率: {perf.monthly_expected_return_pct:.2f}%
预期年度收益率: {perf.annual_expected_return_pct:.2f}%

按初始资本 ¥100,000 计算:
  月均收益: ¥{100000 * perf.monthly_expected_return_pct / 100:,.2f}
  年均收益: ¥{100000 * perf.annual_expected_return_pct / 100:,.2f}

{'─'*80}
4. 交易详情 (Trade Details)
{'─'*80}

{df.to_string(index=False)}

{'─'*80}
5. 分析总结 (Summary)
{'─'*80}

• 胜率: {perf.win_rate*100:.1f}% - {'高' if perf.win_rate > 0.8 else '中等' if perf.win_rate > 0.6 else '偏低'}
• 风险/收益比: {1/perf.profit_factor:.2f} - {'良好' if perf.profit_factor > 1.5 else '中等' if perf.profit_factor > 1.0 else '需要改进'}
• 年化收益: {perf.annual_expected_return_pct:.1f}% - {'优秀' if perf.annual_expected_return_pct > 30 else '良好' if perf.annual_expected_return_pct > 15 else '可接受'}

"""
    
    if output_file:
        with open(output_file, 'w', encoding='utf-8') as f:
            f.write(report)
        logger.info(f"Report saved to {output_file}")
    
    return report

# ========================================
# 示例和测试
# ========================================

def example_china_strategy():
    """中国策略示例回测"""
    calc = DividendYieldCalculator()
    
    # 添加示例交易 (基于实际中国股票)
    trades = [
        # (ticker, trade_date, buy_date, sell_date, buy_price, sell_price, dividend_per_share, shares)
        ('601988', date(2025, 11, 26), date(2025, 11, 26), date(2025, 12, 1), 4.50, 4.52, 0.033, 1000),
        ('601398', date(2025, 12, 3), date(2025, 12, 3), date(2025, 12, 8), 5.80, 5.83, 0.028, 1000),
        ('601288', date(2025, 12, 8), date(2025, 12, 8), date(2025, 12, 11), 3.90, 3.92, 0.032, 1000),
        ('510300', date(2025, 11, 26), date(2025, 11, 26), date(2025, 12, 1), 5.25, 5.28, 0.018, 2000),
        ('510880', date(2025, 12, 3), date(2025, 12, 3), date(2025, 12, 8), 2.80, 2.82, 0.028, 2000),
    ]
    
    for ticker, trade_date, buy_date, sell_date, buy_price, sell_price, dividend, shares in trades:
        calc.add_trade(ticker, trade_date, buy_date, sell_date, buy_price, sell_price, dividend, shares)
    
    return calc

def example_us_strategy():
    """美国策略示例回测"""
    calc = DividendYieldCalculator()
    
    trades = [
        ('JEPI', date(2025, 11, 15), date(2025, 11, 15), date(2025, 11, 20), 50.00, 50.30, 0.60, 100),
        ('XYLD', date(2025, 11, 20), date(2025, 11, 20), date(2025, 11, 25), 25.00, 25.15, 0.50, 200),
        ('SDIV', date(2025, 11, 25), date(2025, 11, 25), date(2025, 12, 1), 15.00, 15.10, 0.65, 300),
    ]
    
    for ticker, trade_date, buy_date, sell_date, buy_price, sell_price, dividend, shares in trades:
        calc.add_trade(ticker, trade_date, buy_date, sell_date, buy_price, sell_price, dividend, shares)
    
    return calc

if __name__ == '__main__':
    logger.info("=" * 80)
    logger.info("中国策略回测分析")
    logger.info("=" * 80)
    
    calc_cn = example_china_strategy()
    report_cn = generate_yield_report(calc_cn, "China Dividend Rotation", "China_Yield_Report.md")
    print(report_cn)
    
    logger.info("\n" + "=" * 80)
    logger.info("美国策略回测分析")
    logger.info("=" * 80)
    
    calc_us = example_us_strategy()
    report_us = generate_yield_report(calc_us, "US Dividend Rotation", "US_Yield_Report.md")
    print(report_us)
    
    logger.info("\n" + "=" * 80)
    logger.info("市场预期收益率计算")
    logger.info("=" * 80)
    
    # 中国组合预期
    cn_portfolio = MarketExpectationCalculator.calculate_portfolio_return(
        ['601988', '601398', '601288', '510300', '510880'],
        hold_days=4,
        region='CN'
    )
    
    logger.info(f"中国组合预期收益: {cn_portfolio['average_return_pct']:.2f}% (每次 {cn_portfolio['hold_days']} 天)")
    logger.info(f"预期月交易: {cn_portfolio['monthly_expected_trades']} 次")
    logger.info(f"预期月收益: {cn_portfolio['monthly_expected_return_pct']:.2f}%")
    logger.info(f"预期年收益: {cn_portfolio['monthly_expected_return_pct'] * 12:.2f}%")
    
    # 美国组合预期
    us_portfolio = MarketExpectationCalculator.calculate_portfolio_return(
        ['JEPI', 'XYLD', 'SDIV', 'VYM', 'DGRO', 'NOBL', 'SCHD', 'HDV'],
        hold_days=5,
        region='US'
    )
    
    logger.info(f"\n美国组合预期收益: {us_portfolio['average_return_pct']:.2f}% (每次 {us_portfolio['hold_days']} 天)")
    logger.info(f"预期月交易: {us_portfolio['monthly_expected_trades']} 次")
    logger.info(f"预期月收益: {us_portfolio['monthly_expected_return_pct']:.2f}%")
    logger.info(f"预期年收益: {us_portfolio['monthly_expected_return_pct'] * 12:.2f}%")
