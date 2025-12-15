#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
China Dividend Rotation Strategy v1
Dividend rotation strategy optimized for China A-shares, H-shares, and CNY ETFs.

Strategy: Buy 2 days before ex-dividend date, hold through dividend payment, sell 1 day after.
Target: Maximize RMB portfolio returns through high-frequency dividend capture.

Supports:
  - A-shares (Shanghai/Shenzhen exchanges): 中国银行, 工商银行, etc.
  - H-shares (Hong Kong): SEHK listed Chinese companies
  - China ETFs (CNY-denominated): 沪深300, 中证500, etc.
  
Data Source: TuShare free API (requires token) + fallback mock data
"""

import argparse
import logging
import sys
from datetime import datetime, date, timedelta
from typing import List, Dict, Set, Optional, Tuple
import json
from dataclasses import dataclass

import pandas as pd
import numpy as np

# ===========================
# Configuration
# ===========================

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(message)s',
    handlers=[logging.StreamHandler(sys.stdout)]
)
logger = logging.getLogger(__name__)

# TuShare API (free tier available)
TUSHARE_BASE_URL = "http://api.tushare.pro"
TUSHARE_TOKEN = ""  # Set via environment or argument

# China trading calendar (simplified)
CHINA_HOLIDAYS_2025 = {
    date(2025, 1, 1): "New Year",
    date(2025, 1, 29): "Spring Festival",
    date(2025, 1, 30): "Spring Festival",
    date(2025, 1, 31): "Spring Festival",
    date(2025, 2, 1): "Spring Festival",
    date(2025, 2, 2): "Spring Festival",
    date(2025, 2, 3): "Spring Festival",
    date(2025, 4, 4): "Qingming Festival",
    date(2025, 4, 5): "Qingming Festival",
    date(2025, 4, 6): "Qingming Festival",
    date(2025, 6, 2): "Dragon Boat Festival",
    date(2025, 9, 3): "Mid-Autumn Festival",
    date(2025, 10, 1): "National Day",
    date(2025, 10, 2): "National Day",
    date(2025, 10, 3): "National Day",
    date(2025, 10, 4): "National Day",
    date(2025, 10, 5): "National Day",
    date(2025, 10, 6): "National Day",
    date(2025, 10, 7): "National Day",
}

# ===========================
# Data Models
# ===========================

@dataclass
class ChineseDividend:
    """Represents a dividend payment for a Chinese security."""
    symbol: str        # A-share: 000001 or 600000; H-share: 00700.HK; ETF: 510300
    name: str          # Chinese or English name
    ex_date: date      # Ex-dividend date
    amount: float      # Dividend per share (RMB)
    yield_pct: float   # Annual dividend yield %
    market: str        # "A-share", "H-share", "ETF"
    exchange: str      # "SHSE", "SZSE", "HKEX", "ETF"
    currency: str      # "CNY" or "HKD"

@dataclass
class ChineseTrade:
    """Represents a completed dividend rotation trade."""
    ticker: str
    name: str
    buy_date: date
    sell_date: date
    buy_price: float
    sell_price: float
    shares: int
    dividend_cash: float
    pnl: float
    return_pct: float
    market: str

# ===========================
# Core Functions
# ===========================

def get_china_trading_days(start_d: date, end_d: date) -> List[date]:
    """
    Generate list of trading days on China exchanges (Mon-Fri, excluding holidays).
    """
    bdays = pd.bdate_range(start=start_d, end=end_d).date
    trading_days = [d for d in bdays if d not in CHINA_HOLIDAYS_2025]
    return trading_days

def shift_china_trading_day(target_d: date, offset: int, trading_cal: List[date]) -> date:
    """
    Shift target date by N trading days using China trading calendar.
    Offset > 0 = forward, < 0 = backward.
    """
    if target_d not in trading_cal:
        # If target is not a trading day, find nearest previous trading day
        idx = 0
        for i, d in enumerate(trading_cal):
            if d >= target_d:
                idx = max(0, i - 1)
                break
        else:
            idx = len(trading_cal) - 1
    else:
        idx = trading_cal.index(target_d)
    
    new_idx = max(0, min(len(trading_cal) - 1, idx + offset))
    return trading_cal[new_idx]

def get_chinese_dividend_stocks() -> pd.DataFrame:
    """
    Return list of high-dividend Chinese stocks suitable for rotation.
    Falls back to curated list if API fails.
    """
    try:
        # Try TuShare API (requires token)
        if TUSHARE_TOKEN:
            import requests
            params = {
                'ts_code': '',
                'fields': 'ts_code,name,dv_yield,ann_date,ex_date,div_procafterdate',
                'api_token': TUSHARE_TOKEN
            }
            # This is simplified; actual TuShare API requires specific method calls
            # For now, use fallback data
        raise Exception("TuShare API not configured; using fallback")
    except Exception as e:
        logger.warning(f"Could not fetch from TuShare API: {e}. Using fallback data.")
        
        # Curated list of high-dividend Chinese stocks (A-shares and ETFs)
        fallback_data = [
            # A-share banks (高分红)
            {"code": "601988", "name": "中国银行", "yield": 0.055, "market": "A-share", "exchange": "SHSE"},
            {"code": "601398", "name": "工商银行", "yield": 0.047, "market": "A-share", "exchange": "SHSE"},
            {"code": "601288", "name": "农业银行", "yield": 0.054, "market": "A-share", "exchange": "SHSE"},
            {"code": "600000", "name": "浦发银行", "yield": 0.049, "market": "A-share", "exchange": "SHSE"},
            {"code": "000858", "name": "五粮液", "yield": 0.018, "market": "A-share", "exchange": "SZSE"},
            
            # A-share ETFs (指数基金，分红稳定)
            {"code": "510300", "name": "沪深300ETF", "yield": 0.032, "market": "ETF", "exchange": "ETF"},
            {"code": "510500", "name": "中证500ETF", "yield": 0.025, "market": "ETF", "exchange": "ETF"},
            {"code": "510880", "name": "红利ETF", "yield": 0.045, "market": "ETF", "exchange": "ETF"},
            
            # H-shares (港股通，以HKD计价)
            {"code": "00700.HK", "name": "腾讯控股", "yield": 0.015, "market": "H-share", "exchange": "HKEX"},
            {"code": "00939.HK", "name": "中国建筑", "yield": 0.052, "market": "H-share", "exchange": "HKEX"},
            {"code": "01288.HK", "name": "农业银行H股", "yield": 0.058, "market": "H-share", "exchange": "HKEX"},
        ]
        
        df = pd.DataFrame(fallback_data)
        df["code"] = df["code"].astype(str)
        df["yield"] = df["yield"].astype(float)
        return df

def get_china_dividend_calendar(start: str, end: str, symbols: Optional[List[str]] = None) -> pd.DataFrame:
    """
    Fetch upcoming dividend events for Chinese stocks.
    Returns dataframe with columns: symbol, name, ex_date, amount, currency
    """
    try:
        # Try TuShare API dividend calendar
        if TUSHARE_TOKEN:
            # Actual implementation would call TuShare dividend API
            # pro = ts.pro_connect(api_name='tushare')
            # df = pro.dividend(ts_code='', start_date=start, end_date=end)
            raise Exception("TuShare API not fully implemented")
        raise Exception("No API token")
    except Exception as e:
        logger.warning(f"Could not fetch dividend calendar: {e}. Using fallback mock data.")
        
        # Fallback: Generate realistic dividend dates for Chinese stocks
        # Most Chinese companies pay dividends annually, typically in June-August
        today = datetime.now().date()
        to_date = today + timedelta(days=60)
        events = []
        
        # High-dividend A-share stocks and ETFs (payment dates in future)
        # Realistic dividend amounts (in CNY per share)
        dividend_schedule = {
            "601988": ("中国银行", 0.033, [20, 50]),  # Multiple possible ex-dates
            "601398": ("工商银行", 0.028, [20, 50]),
            "601288": ("农业银行", 0.032, [25, 55]),
            "600000": ("浦发银行", 0.025, [18, 48]),
            "000858": ("五粮液", 0.008, [22, 52]),
            "510300": ("沪深300ETF", 0.018, [15, 45]),
            "510500": ("中证500ETF", 0.015, [17, 47]),
            "510880": ("红利ETF", 0.028, [20, 50]),
            "00700.HK": ("腾讯控股", 0.015, [30, 60]),
            "00939.HK": ("中国建筑", 0.032, [25, 55]),
            "01288.HK": ("农业银行H股", 0.035, [28, 58]),
        }
        
        # Generate next 60 days of dividend events
        for symbol, (name, div_amount, offset_days) in dividend_schedule.items():
            for offset in offset_days:
                try:
                    ex_date = today + timedelta(days=offset)
                    if today <= ex_date <= to_date:
                        events.append({
                            "symbol": symbol,
                            "name": name,
                            "ex_date": ex_date.strftime("%Y-%m-%d"),
                            "amount": div_amount,
                            "currency": "CNY" if "HK" not in symbol else "HKD"
                        })
                except ValueError:
                    continue
        
        df = pd.DataFrame(events)
        if not df.empty:
            df["ex_date"] = pd.to_datetime(df["ex_date"]).dt.date
        return df

def build_china_forward_plan(
    stocks: pd.DataFrame,
    hold_pre: int = 2,
    hold_post: int = 1,
    lookahead_days: int = 60
) -> pd.DataFrame:
    """
    Build 60-day forward trading plan for Chinese dividend rotation.
    """
    today = date.today()
    to_day = today + timedelta(days=lookahead_days)
    
    # Get trading calendar
    trading_cal = get_china_trading_days(today - timedelta(days=30), to_day + timedelta(days=5))
    
    # Get dividend events
    symbols_list = stocks["code"].tolist() if "code" in stocks.columns else []
    calendar = get_china_dividend_calendar(today.isoformat(), to_day.isoformat(), symbols_list)
    
    if calendar.empty:
        logger.warning("No dividend events found for upcoming 60 days")
        return pd.DataFrame()
    
    # Merge with stock info
    calendar = calendar.rename(columns={"symbol": "code"})
    calendar = calendar.merge(stocks, on="code", how="left")
    
    # Generate buy/sell dates
    plans = []
    for _, row in calendar.sort_values("ex_date").iterrows():
        ex_date = row["ex_date"]
        
        # Calculate buy/sell dates
        buy_date = shift_china_trading_day(ex_date, -hold_pre, trading_cal)
        sell_date = shift_china_trading_day(ex_date, hold_post, trading_cal)
        
        plans.append({
            "ticker": row["code"],
            "name": row.get("name", ""),
            "market": row.get("market", ""),
            "ex_date": ex_date,
            "dividend_amount": row.get("amount", 0.0),
            "currency": row.get("currency", "CNY"),
            "buy_date": buy_date,
            "sell_date": sell_date,
            "hold_days": (sell_date - buy_date).days,
        })
    
    return pd.DataFrame(plans)

def export_china_plan_markdown(plan_df: pd.DataFrame, output_file: str):
    """Export China forward plan as markdown table."""
    if plan_df.empty:
        logger.warning("Empty plan dataframe, nothing to export")
        return
    
    md_content = """# 中国股票股息轮动计划 (China Dividend Rotation Plan)

**生成日期 (Generated):** {date}  
**预测期 (Lookahead Period):** 60 days  
**策略 (Strategy):** Buy 2 days before ex-dividend, Sell 1 day after ex-dividend  
**目标 (Target):** 最大化RMB投资组合收益 (Maximize RMB portfolio returns)

---

## 即将到来的分红事件 (Upcoming Dividend Events)

| # | 代码 (Ticker) | 名称 (Name) | 市场 (Market) | 分红日 (Ex-Date) | 分红额 (Div/Share) | 买入日 (Buy) | 卖出日 (Sell) | 持仓天数 (Hold) |
|---|---|---|---|---|---|---|---|---|
""".format(date=datetime.now().strftime("%Y-%m-%d"))
    
    for idx, row in plan_df.iterrows():
        md_content += f"""| {idx + 1} | {row['ticker']} | {row['name']} | {row['market']} | {row['ex_date']} | ¥{row['dividend_amount']:.3f} | {row['buy_date']} | {row['sell_date']} | {row['hold_days']} |
"""
    
    md_content += """
---

## 行动计划 (Action Plan by Week)

### 本周 (This Week)

- **Day 1**: Review upcoming ex-dates, prepare order list
- **Day 2-3**: Execute first round of buy orders
- **Day 4-5**: Monitor positions, prepare sell orders

### 关键指标 (Key Metrics)

| 指标 (Metric) | 数值 (Value) |
|---|---|
| 总事件数 (Total Events) | {count} |
| 日期范围 (Date Range) | {start} - {end} |
| 平均持仓期 (Avg Hold) | {avg_hold:.1f} days |
| 预期货币 (Currency) | CNY + HKD |

---

## 执行说明 (Execution Notes)

### A-股 (A-Shares) 交易注意事项:
- 交易时间: 09:30-11:30, 13:00-15:00 (Beijing Time)
- T+1结算 (T+1 settlement)
- 分红需持有至分红除权日 (Must hold through ex-date)
- 交易费用: 券商佣金 + 印花税 0.1% (sell only)

### H-股 (H-Shares) via 港股通:
- 交易时间: 09:30-16:00 HK time
- T+2结算 (T+2 settlement)
- 交易费用: 佣金 + 港币汇兑成本
- 风险: HKD/CNY 汇率波动

### ETF (指数基金):
- 高流动性，低手续费
- 分红自动复投或分配 (check fund terms)
- 适合稳妥的长期持仓轮动

### 风险管理 (Risk Management):
- 单个头寸最大: ¥10,000 (或可用资本的5%)
- 止损: -2% (if dividend cut or price gap down)
- 监控分红公告变化 (dividend cut announcements)
- 跟踪汇率风险 (for H-shares)

""".format(
        count=len(plan_df),
        start=plan_df["buy_date"].min(),
        end=plan_df["sell_date"].max(),
        avg_hold=plan_df["hold_days"].mean()
    )
    
    with open(output_file, 'w', encoding='utf-8') as f:
        f.write(md_content)
    
    logger.info(f"Exported China forward plan: {output_file}")

# ===========================
# Main Entry Point
# ===========================

def main():
    parser = argparse.ArgumentParser(
        description="China Dividend Rotation Strategy - 中国股票股息轮动策略",
        formatter_class=argparse.RawDescriptionHelpFormatter
    )
    
    parser.add_argument(
        '--lookahead', type=int, default=60,
        help='Days to look ahead for dividend events (default: 60)'
    )
    parser.add_argument(
        '--hold-pre', type=int, default=2,
        help='Days to hold before ex-dividend date (default: 2)'
    )
    parser.add_argument(
        '--hold-post', type=int, default=1,
        help='Days to hold after ex-dividend date (default: 1)'
    )
    parser.add_argument(
        '--min-yield', type=float, default=0.015,
        help='Minimum dividend yield threshold (default: 1.5 percent)'
    )
    parser.add_argument(
        '--output', type=str, default='China_Dividend_Forward_Plan.md',
        help='Output markdown file'
    )
    parser.add_argument(
        '--tushare-token', type=str, default='',
        help='TuShare API token (optional)'
    )
    
    args = parser.parse_args()
    
    if args.tushare_token:
        global TUSHARE_TOKEN
        TUSHARE_TOKEN = args.tushare_token
    
    logger.info(f"Starting China Dividend Rotation Strategy")
    logger.info(f"Lookahead: {args.lookahead} days | Hold Pre: {args.hold_pre} | Hold Post: {args.hold_post}")
    
    # Get available stocks
    stocks = get_chinese_dividend_stocks()
    logger.info(f"Found {len(stocks)} Chinese dividend stocks/ETFs")
    
    # Filter by yield
    stocks = stocks[stocks["yield"] >= args.min_yield]
    logger.info(f"After yield filter ({args.min_yield*100}%): {len(stocks)} stocks")
    
    if stocks.empty:
        logger.error("No stocks found matching criteria")
        return 1
    
    # Build forward plan
    plan = build_china_forward_plan(
        stocks,
        hold_pre=args.hold_pre,
        hold_post=args.hold_post,
        lookahead_days=args.lookahead
    )
    
    if plan.empty:
        logger.warning("No dividend events found in lookahead period")
    else:
        logger.info(f"Generated forward plan with {len(plan)} events")
        export_china_plan_markdown(plan, args.output)
    
    logger.info("China dividend rotation strategy complete!")
    return 0

if __name__ == "__main__":
    sys.exit(main())
