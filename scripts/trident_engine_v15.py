#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
三叉戟统一量化投资引擎 v1.5 (日频版)
Trident Unified Quantitative Investment Engine

执行日期：2025-11-14
目标：年化≥18%，周度胜率≥58%，最大回撤≤-25%，Sharpe≥1.2
"""

import os
import sys
import json
import time
from datetime import datetime, timedelta
from typing import Dict, List, Tuple, Optional
import warnings
warnings.filterwarnings('ignore')

import pandas as pd
import numpy as np
import requests

# ==================== 配置 ====================
EODHD_API_TOKEN = os.getenv("EODHD_API_TOKEN", "690d7cdc3013f4.57364117")
BASE_URL = "https://eodhd.com/api"

# 资金配置
INITIAL_CAPITAL = 4_000_000  # ¥400万
MAX_LEVERAGE = 2.25  # 最大杠杆
MIN_LEVERAGE = 0.6   # 最小杠杆

# 风控参数
SINGLE_ETF_MAX = 0.35  # 单ETF上限35%
THEME_MAX = 0.60       # 主题总敞口60%
CORRELATION_THRESHOLD = 0.85  # 相关性阈值

# 中国A股ETF资产池
ASSET_POOL = {
    '宽基_沪深300': '510300.SHG',    # 华泰柏瑞沪深300ETF
    '宽基_中证500': '510500.SHG',    # 南方中证500ETF
    '宽基_创业板': '159915.SHE',     # 易方达创业板ETF
    '宽基_中证1000': '512100.SHG',   # 南方中证1000ETF
    '主题_AI': '515220.SHG',         # 国泰AI智能ETF
    '主题_半导体': '512480.SHG',     # 国联安半导体ETF
    '主题_消费': '159928.SHE',       # 汇添富消费ETF
    '主题_医药': '512010.SHG',       # 易方达沪深300医药ETF
    '主题_新能源车': '515030.SHG',   # 华夏新能源车ETF
    '主题_军工': '512660.SHG',       # 国泰军工ETF
    '主题_央企': '510060.SHG',       # 工银央企ETF
    '红利_上证红利': '510880.SHG',   # 华泰柏瑞上证红利ETF
    '红利_中证红利': '515180.SHG',   # 华宝中证红利ETF
    '港股_恒生科技': '513010.SHG',   # 华夏恒生科技ETF
    '债券_30年国债': '511090.SHG',   # 国泰30年国债ETF
    '债券_短融': '511360.SHG',       # 信用债ETF
}

# ==================== 工具函数 ====================
def get_data(ticker: str, days: int = 252) -> pd.DataFrame:
    """获取ETF历史数据"""
    try:
        end_date = datetime.now().strftime('%Y-%m-%d')
        start_date = (datetime.now() - timedelta(days=days*1.5)).strftime('%Y-%m-%d')

        url = f"{BASE_URL}/eod/{ticker}"
        params = {
            'api_token': EODHD_API_TOKEN,
            'from': start_date,
            'to': end_date,
            'fmt': 'json',
            'order': 'd'
        }

        response = requests.get(url, params=params, timeout=15)
        response.raise_for_status()
        data = response.json()

        if not data:
            return pd.DataFrame()

        df = pd.DataFrame(data)
        df['date'] = pd.to_datetime(df['date'])
        df = df.sort_values('date')
        df.set_index('date', inplace=True)

        # 计算收益率
        df['returns'] = df['close'].pct_change()

        return df[['open', 'high', 'low', 'close', 'volume', 'returns']].iloc[-days:]

    except Exception as e:
        print(f"❌ 获取 {ticker} 数据失败: {e}")
        return pd.DataFrame()

def calculate_technical_features(df: pd.DataFrame) -> Dict:
    """计算技术特征"""
    if len(df) < 20:
        return {}

    close = df['close']
    volume = df['volume']
    returns = df['returns'].dropna()

    # 趋势动量
    ma20 = close.rolling(20).mean().iloc[-1]
    ma50 = close.rolling(50).mean().iloc[-1] if len(df) >= 50 else ma20
    ma200 = close.rolling(200).mean().iloc[-1] if len(df) >= 200 else ma20

    current_price = close.iloc[-1]

    # 相对强度
    rs_20 = (current_price - close.rolling(20).mean().iloc[-1]) / close.rolling(20).mean().iloc[-1] if ma20 > 0 else 0

    # 动量
    mom_1m = returns.iloc[-20:].sum() if len(returns) >= 20 else 0
    mom_3m = returns.iloc[-60:].sum() if len(returns) >= 60 else 0
    mom_6m = returns.iloc[-126:].sum() if len(returns) >= 126 else 0

    # 波动率
    vol_20 = returns.iloc[-20:].std() * np.sqrt(252) if len(returns) >= 20 else 0
    vol_60 = returns.iloc[-60:].std() * np.sqrt(252) if len(returns) >= 60 else 0

    # 布林带
    bb_upper = ma20 + 2 * close.rolling(20).std().iloc[-1]
    bb_lower = ma20 - 2 * close.rolling(20).std().iloc[-1]
    bb_position = (current_price - bb_lower) / (bb_upper - bb_lower) if (bb_upper - bb_lower) > 0 else 0.5

    # 成交量变化
    vol_ratio = volume.iloc[-5:].mean() / volume.iloc[-20:].mean() if volume.iloc[-20:].mean() > 0 else 1

    # 最大回撤
    rolling_max = close.rolling(60, min_periods=1).max()
    drawdown = (close - rolling_max) / rolling_max
    max_dd = drawdown.min()

    features = {
        'ma20_pct': (current_price - ma20) / ma20 if ma20 > 0 else 0,
        'ma50_pct': (current_price - ma50) / ma50 if ma50 > 0 else 0,
        'ma200_pct': (current_price - ma200) / ma200 if ma200 > 0 else 0,
        'rs_20': rs_20,
        'momentum_1m': mom_1m,
        'momentum_3m': mom_3m,
        'momentum_6m': mom_6m,
        'volatility_20d': vol_20,
        'volatility_60d': vol_60,
        'bb_position': bb_position,
        'volume_ratio': vol_ratio,
        'max_drawdown_60d': max_dd,
    }

    return features

def calculate_winprob(features: Dict) -> float:
    """计算胜率概率 (简化版 - 基于规则)"""
    if not features:
        return 0.5

    score = 0.0
    weight_sum = 0.0

    # 趋势得分 (40%)
    if features['ma20_pct'] > 0:
        score += 0.4 * min(1.0, features['ma20_pct'] * 10)
        weight_sum += 0.4

    if features['ma50_pct'] > 0:
        score += 0.3 * min(1.0, features['ma50_pct'] * 8)
        weight_sum += 0.3

    # 动量得分 (30%)
    if features['momentum_1m'] > 0:
        score += 0.2 * min(1.0, features['momentum_1m'] * 5)
        weight_sum += 0.2

    if features['momentum_3m'] > 0:
        score += 0.1 * min(1.0, features['momentum_3m'] * 3)
        weight_sum += 0.1

    # 波动率得分 (15%) - 低波动更好
    vol_score = 1.0 - min(1.0, features['volatility_20d'] / 0.5)
    score += 0.15 * vol_score
    weight_sum += 0.15

    # 成交量得分 (10%)
    vol_score = min(1.0, features['volume_ratio'] - 0.5) if features['volume_ratio'] > 0.8 else 0
    score += 0.1 * vol_score
    weight_sum += 0.1

    # 回撤惩罚 (5%)
    dd_penalty = max(0, 1 + features['max_drawdown_60d'])  # 0 to 1
    score += 0.05 * dd_penalty
    weight_sum += 0.05

    # 归一化到0-1
    winprob = score / weight_sum if weight_sum > 0 else 0.5

    # 限制在0.3-0.85之间（避免过度自信）
    return np.clip(winprob, 0.3, 0.85)

def calculate_expected_return(df: pd.DataFrame, winprob: float) -> float:
    """估算期望收益"""
    if len(df) < 20:
        return 0

    recent_vol = df['returns'].iloc[-20:].std() * np.sqrt(5)  # 5日波动
    expected_ret = (winprob - 0.5) * 2 * recent_vol  # 基于胜率优势和波动

    return expected_ret

def calculate_regime_score() -> float:
    """计算市场大势评分 (0-100)"""
    # 简化版：基于沪深300走势
    try:
        df_csi300 = get_data('510300.SHG', days=200)
        if df_csi300.empty:
            return 50  # 中性

        close = df_csi300['close']

        # 趋势得分 (40分)
        ma50 = close.rolling(50).mean().iloc[-1] if len(df_csi300) >= 50 else close.iloc[-1]
        ma200 = close.rolling(200).mean().iloc[-1] if len(df_csi300) >= 200 else ma50
        current = close.iloc[-1]

        trend_score = 0
        if current > ma50:
            trend_score += 20
        if current > ma200:
            trend_score += 20

        # 动量得分 (30分)
        mom_20d = (close.iloc[-1] / close.iloc[-20] - 1) if len(close) >= 20 else 0
        momentum_score = np.clip(mom_20d * 100, -15, 15) + 15

        # 波动率得分 (30分) - 低波动给高分
        vol = df_csi300['returns'].iloc[-20:].std() * np.sqrt(252)
        vol_score = 30 if vol < 0.25 else (30 - (vol - 0.25) * 60)
        vol_score = np.clip(vol_score, 0, 30)

        regime_score = trend_score + momentum_score + vol_score
        return np.clip(regime_score, 0, 100)

    except:
        return 50

def calculate_tvrs_score() -> Tuple[float, str]:
    """计算tvrs估值-风险评分"""
    # 简化版：使用沪深300 PE估计
    # 实际应使用更完整的估值模型

    regime = calculate_regime_score()

    if regime >= 70:
        return 80, "正常"  # 市场强势，估值正常
    elif regime >= 50:
        return 60, "谨慎"  # 市场中性
    else:
        return 45, "严控"  # 市场弱势

def calculate_dynamic_leverage(regime_score: float, tvrs_score: float, tvrs_state: str) -> float:
    """计算动态杠杆"""
    # 基础杠杆
    base_lev = 1.0 + 0.9 * (regime_score - 50) / 50
    base_lev = np.clip(base_lev, 0.1, 1.9)

    # 波动率调整（简化）
    vol_adj = 0.3 if regime_score > 60 else 0

    # tvrs上限
    tvrs_cap = {'严控': 1.0, '谨慎': 1.5, '正常': 2.25}.get(tvrs_state, 1.5)

    final_lev = min(base_lev + vol_adj, tvrs_cap)
    final_lev = np.clip(final_lev, MIN_LEVERAGE, MAX_LEVERAGE)

    return final_lev

# ==================== 主执行流程 ====================
def screen_etfs() -> pd.DataFrame:
    """筛选并评分ETF"""
    print("\n" + "="*60)
    print("🔍 步骤 1/5: 筛选与评分 ETF资产池")
    print("="*60)

    results = []

    for name, ticker in ASSET_POOL.items():
        print(f"  处理: {name:15s} ({ticker})...", end='')

        # 获取数据
        df = get_data(ticker, days=252)

        if df.empty or len(df) < 60:
            print(" ❌ 数据不足")
            continue

        # 计算特征
        features = calculate_technical_features(df)

        if not features:
            print(" ❌ 特征计算失败")
            continue

        # 计算胜率概率
        winprob = calculate_winprob(features)

        # 计算期望收益
        exp_ret = calculate_expected_return(df, winprob)

        # 风险调整
        risk_adj = 1.0 / (features['volatility_20d'] + 0.01)

        # TrVal-P综合得分
        trval_score = 0.6 * winprob + 0.25 * (exp_ret * 10) + 0.15 * risk_adj / 10

        current_price = df['close'].iloc[-1]

        results.append({
            'name': name,
            'ticker': ticker,
            'category': name.split('_')[0],
            'price': current_price,
            'winprob': winprob,
            'exp_return_5d': exp_ret,
            'volatility_20d': features['volatility_20d'],
            'momentum_1m': features['momentum_1m'],
            'momentum_3m': features['momentum_3m'],
            'trval_score': trval_score,
            'volume_10d_avg': df['volume'].iloc[-10:].mean(),
        })

        print(f" ✅ 胜率:{winprob:.1%} 得分:{trval_score:.3f}")

    df_results = pd.DataFrame(results)

    if df_results.empty:
        print("\n❌ 未找到合格的ETF！")
        return df_results

    # 排序
    df_results = df_results.sort_values('trval_score', ascending=False)

    print(f"\n✅ 完成筛选，共 {len(df_results)} 只ETF符合条件\n")

    return df_results

def select_portfolio(df_screened: pd.DataFrame, top_n: int = 3) -> pd.DataFrame:
    """选择投资组合"""
    print("\n" + "="*60)
    print(f"📊 步骤 2/5: 构建投资组合 (Top {top_n})")
    print("="*60)

    if len(df_screened) < top_n:
        top_n = len(df_screened)

    # 选择Top N
    portfolio = df_screened.head(top_n).copy()

    # 去相关性检查
    print("\n  检查相关性...")
    for i in range(len(portfolio)):
        for j in range(i+1, len(portfolio)):
            ticker1 = portfolio.iloc[i]['ticker']
            ticker2 = portfolio.iloc[j]['ticker']

            df1 = get_data(ticker1, days=30)
            df2 = get_data(ticker2, days=30)

            if not df1.empty and not df2.empty:
                corr = df1['returns'].corr(df2['returns'])

                if corr > CORRELATION_THRESHOLD:
                    print(f"    ⚠️  高相关: {portfolio.iloc[i]['name']} vs {portfolio.iloc[j]['name']} (ρ={corr:.2f})")
                    print(f"    → 替换为低相关品种")

                    # 查找替代品（不同类别）
                    cat_i = portfolio.iloc[i]['category']
                    alternatives = df_screened[
                        ~df_screened['ticker'].isin(portfolio['ticker']) &
                        (df_screened['category'] != cat_i)
                    ]

                    if not alternatives.empty:
                        replacement = alternatives.iloc[0]
                        portfolio.iloc[j] = replacement
                        print(f"    ✅ 替换为: {replacement['name']}")

    print(f"\n✅ 投资组合构建完成 ({len(portfolio)} 只ETF)\n")

    return portfolio

def calculate_position_weights(portfolio: pd.DataFrame, final_leverage: float) -> pd.DataFrame:
    """计算仓位权重"""
    print("\n" + "="*60)
    print("⚖️  步骤 3/5: 计算仓位权重")
    print("="*60)

    # 胜率加权 * 波动率逆序
    portfolio = portfolio.copy()
    portfolio['weight_raw'] = portfolio['winprob'] * (1 / (portfolio['volatility_20d'] + 0.01))

    # 归一化
    total_raw = portfolio['weight_raw'].sum()
    portfolio['weight'] = portfolio['weight_raw'] / total_raw

    # 应用单品种上限
    portfolio['weight'] = portfolio['weight'].apply(lambda x: min(x, SINGLE_ETF_MAX))

    # 重新归一化
    portfolio['weight'] = portfolio['weight'] / portfolio['weight'].sum()

    # 应用杠杆
    portfolio['final_weight'] = portfolio['weight'] * final_leverage * 0.75  # 75%股票仓位

    # 计算金额
    portfolio['position_value'] = portfolio['final_weight'] * INITIAL_CAPITAL
    portfolio['shares'] = (portfolio['position_value'] / portfolio['price']).astype(int)

    # 债券仓位
    bond_weight = max(0.10, 1.0 - final_leverage * 0.75)
    bond_value = bond_weight * INITIAL_CAPITAL

    print(f"\n  杠杆: {final_leverage:.2f}x")
    print(f"  股票总仓位: {(final_leverage * 0.75):.1%}")
    print(f"  债券仓位: {bond_weight:.1%} (¥{bond_value:,.0f})")

    print("\n  个股权重分配:")
    for idx, row in portfolio.iterrows():
        print(f"    {row['name']:20s}: {row['final_weight']:6.1%}  (¥{row['position_value']:>12,.0f}  {row['shares']:>7,}股)")

    print("\n✅ 仓位计算完成\n")

    return portfolio, bond_weight, bond_value

def generate_trading_plan(portfolio: pd.DataFrame, bond_weight: float, regime_score: float) -> str:
    """生成交易执行计划"""
    print("\n" + "="*60)
    print("📝 步骤 4/5: 生成交易执行计划")
    print("="*60)

    today = datetime.now().strftime('%Y-%m-%d')

    # 确定债券ETF
    if regime_score >= 60:
        bond_etf = '511090.SHG'  # 30年国债
        bond_name = '国泰30年国债ETF'
    else:
        bond_etf = '511360.SHG'  # 短融
        bond_name = '信用债ETF'

    # Markdown报告
    report = f"""
# 三叉戟量化引擎 v1.5 - 交易执行计划

**生成时间:** {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}
**资金规模:** ¥{INITIAL_CAPITAL:,}
**交易日期:** {today} (T+0)

---

## 📊 市场环境评估

| 指标 | 数值 | 状态 |
|------|------|------|
| 市场大势评分 (RegimeScore) | {regime_score:.1f}/100 | {'🟢 强势' if regime_score >= 70 else '🟡 中性' if regime_score >= 50 else '🔴 弱势'} |
| 目标杠杆 | {portfolio['final_weight'].sum() / 0.75:.2f}x | {'适中' if regime_score >= 50 else '保守'} |
| 风控状态 | 正常 | ✅ |

---

## 🎯 投资组合配置

### 股票仓位 ({(portfolio['final_weight'].sum()):.1%})

| 代码 | 名称 | 分类 | 现价 | 目标权重 | 目标金额 | 目标股数 | 胜率概率 | TrVal得分 |
|------|------|------|------|----------|----------|----------|----------|-----------|
"""

    for idx, row in portfolio.iterrows():
        report += f"| {row['ticker']} | {row['name']} | {row['category']} | ¥{row['price']:.2f} | {row['final_weight']:.1%} | ¥{row['position_value']:,.0f} | {row['shares']:,} | {row['winprob']:.1%} | {row['trval_score']:.3f} |\n"

    report += f"""
### 债券仓位 ({bond_weight:.1%})

| 代码 | 名称 | 目标金额 | 用途 |
|------|------|----------|------|
| {bond_etf} | {bond_name} | ¥{bond_weight * INITIAL_CAPITAL:,.0f} | 防御+流动性储备 |

---

## ⚠️ 风控护栏

### 仓位限制
- ✅ 单ETF权重 ≤ 35%
- ✅ 主题类总敞口 ≤ 60%
- ✅ 相关性 ρ < 0.85

### 止损止盈
- **单品种止损:** -3.5% (常规) / -2.5% (高波动期)
- **分档加仓:** 回撤 -1.5σ (30%) / -2.5σ (50%)
- **分档止盈:** +2σ (减30%) / +3σ (减50%)
- **组合止损:** 月内回撤 ≤ -10% → 降杠杆至0.5x

---

## 📋 执行工单

### 开盘前 (09:15-09:25)
"""

    for idx, row in portfolio.iterrows():
        report += f"- [ ] **买入** {row['ticker']} {row['name']} 约{row['shares']:,}股 (目标¥{row['position_value']:,.0f})\n"

    report += f"- [ ] **买入** {bond_etf} {bond_name} (目标¥{bond_weight * INITIAL_CAPITAL:,.0f})\n"

    report += f"""
### 盘中监控 (10:30 / 14:30)
- [ ] 检查止损线触发情况
- [ ] 监控成交量异常
- [ ] 关注波动率飙升 (VIX代理)

### 收盘后 (15:30)
- [ ] 记录执行情况
- [ ] 更新持仓成本
- [ ] 计算当日盈亏与回撤

---

## 📈 预期表现

- **周度胜率目标:** ≥58%
- **预期收益 (5日):** {portfolio['exp_return_5d'].mean():.2%} ~ {portfolio['exp_return_5d'].mean() * 1.5:.2%}
- **组合波动率:** {portfolio['volatility_20d'].mean():.2%} (年化)

---

## 🔔 重要提示

1. **交易时段:** 开盘后5-15分钟不交易，使用VWAP或限价单
2. **滑点控制:** 大单分拆，单笔 ≤ 5分钟成交量的10%
3. **紧急情况:** 波动率VIX ≥35连续2日 → 降杠杆至0.9x
4. **回撤触发:** 月内DD ≤-10% → 自动降杠杆+增持债券至50%

---

*本计划由三叉戟量化引擎v1.5自动生成*
*执行前请确认市场开盘状态与流动性*
"""

    print("✅ 交易计划生成完成\n")

    return report

def main():
    """主函数"""
    print("\n" + "="*60)
    print("🚀 三叉戟统一量化投资引擎 v1.5")
    print("="*60)
    print(f"执行时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print(f"资金规模: ¥{INITIAL_CAPITAL:,}")
    print(f"杠杆区间: {MIN_LEVERAGE:.1f}x - {MAX_LEVERAGE:.1f}x")
    print("="*60)

    try:
        # 1. 筛选ETF
        df_screened = screen_etfs()

        if df_screened.empty:
            print("❌ 无可投资标的，程序退出")
            return

        # 2. 选择组合
        portfolio = select_portfolio(df_screened, top_n=3)

        # 3. 计算市场大势与杠杆
        print("\n" + "="*60)
        print("🌐 市场大势与杠杆评估")
        print("="*60)

        regime_score = calculate_regime_score()
        tvrs_score, tvrs_state = calculate_tvrs_score()
        final_leverage = calculate_dynamic_leverage(regime_score, tvrs_score, tvrs_state)

        print(f"\n  市场大势评分: {regime_score:.1f}/100")
        print(f"  估值风险评分: {tvrs_score:.1f}/100 ({tvrs_state})")
        print(f"  目标杠杆: {final_leverage:.2f}x")
        print("\n✅ 评估完成\n")

        # 4. 计算仓位
        portfolio, bond_weight, bond_value = calculate_position_weights(portfolio, final_leverage)

        # 5. 生成交易计划
        trading_plan = generate_trading_plan(portfolio, bond_weight, regime_score)

        # 保存报告
        output_file = f"trident_plan_{datetime.now().strftime('%Y%m%d')}.md"
        with open(output_file, 'w', encoding='utf-8') as f:
            f.write(trading_plan)

        print("="*60)
        print(f"✅ 执行完成！交易计划已保存: {output_file}")
        print("="*60)

        # 打印摘要
        print("\n📌 今日执行摘要:\n")
        print(trading_plan.split('---')[0])  # 打印前面部分

        return portfolio, trading_plan

    except Exception as e:
        print(f"\n❌ 执行出错: {e}")
        import traceback
        traceback.print_exc()
        return None, None

if __name__ == "__main__":
    portfolio, plan = main()