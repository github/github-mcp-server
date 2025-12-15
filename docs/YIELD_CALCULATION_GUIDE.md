# 股息轮动策略 - 收益率计算完全指南

## 📋 目录
- [快速开始](#快速开始)
- [核心工具](#核心工具)
- [计算公式](#计算公式)
- [使用示例](#使用示例)
- [常见问题](#常见问题)

---

## 🚀 快速开始

### 步骤 1: 验证安装

```bash
# 验证所有收益率计算模块
python verify_yields.py
```

输出示例：
```
################################################################################
# 股息轮动策略 - 收益率计算验证
################################################################################

================================================================================
测试 1: 单笔交易收益计算
================================================================================

中国股票示例: 中国银行 (601988)
  买入日期: 2025-11-26
  卖出日期: 2025-11-29
  持仓天数: 3
  买入价格: ¥3.15
  卖出价格: ¥3.17
  价格变化: +0.63%
  分红率: 1.048%
  总收益率: +1.678%
  年化收益: +204.3%
```

### 步骤 2: 分析交易计划

```bash
# 分析中国和美国的交易计划收益率
python trading_plan_report.py
```

输出包括：
- 单笔交易分析
- 策略聚合指标
- 月度收益预测
- 风险指标
- 执行追踪表（CSV导出）

### 步骤 3: 深入收益率分析

```bash
# 全面的收益率分析
python yield_analysis.py --all

# 仅分析中国策略
python yield_analysis.py --china

# 仅分析美国策略
python yield_analysis.py --us

# 策略对比
python yield_analysis.py --compare
```

---

## 🔧 核心工具

### 1. dividend_yield_calculator.py

**主要类：**

#### DividendYieldAnalysis （单笔交易分析）

```python
from dividend_yield_calculator import DividendYieldAnalysis
from datetime import date

# 创建单笔交易分析
analysis = DividendYieldAnalysis(
    ticker='601988',                          # 股票代码
    buy_date=date(2025, 11, 26),             # 买入日期
    sell_date=date(2025, 11, 29),            # 卖出日期
    buy_price=3.15,                          # 买入价格
    sell_price=3.17,                         # 卖出价格
    shares=1000,                             # 持仓股数
    dividend_per_share=0.033                 # 每股分红
)

# 自动计算的属性：
print(f"持仓天数: {analysis.hold_days}")                           # 3
print(f"价格变化: {analysis.price_change_pct:.2f}%")              # +0.63%
print(f"分红率: {analysis.dividend_yield_pct:.3f}%")              # 1.048%
print(f"总收益: {analysis.total_return_pct:.3f}%")                # +1.678%
print(f"年化: {analysis.annualized_return_pct:.1f}%")             # +204.3%
```

**计算公式：**
```
持仓天数 = 卖出日期 - 买入日期
价格变化% = ((卖出价 - 买入价) / 买入价) × 100
分红率% = (每股分红 / 买入价) × 100
总收益% = 分红率% + 价格变化%
年化收益% = 总收益% × (365 / 持仓天数)
```

#### DividendYieldCalculator （策略聚合）

```python
from dividend_yield_calculator import DividendYieldCalculator

calculator = DividendYieldCalculator()

# 添加多笔交易
for analysis in analyses:
    calculator.add_trade(analysis)

# 计算策略性能
perf = calculator.calculate_strategy_performance()

print(f"总交易数: {perf.total_trades}")                          # 11
print(f"获利交易: {perf.winning_trades}")                         # 10
print(f"获利率: {perf.win_rate*100:.1f}%")                        # 90.9%
print(f"平均单笔收益: {perf.avg_return_per_trade:.3f}%")          # 1.245%
print(f"平均年化: {perf.avg_annualized_return:.1f}%")             # 113.5%
print(f"利润因子: {perf.profit_factor:.2f}")                      # 3.45
print(f"预期月交易: {perf.monthly_expected_trades}")              # 6-7
print(f"预期月收益: {perf.monthly_expected_return_pct:.2f}%")     # 7.47%
print(f"预期年收益: {perf.annual_expected_return_pct:.2f}%")      # 89.6%

# 导出为 Pandas DataFrame
df = calculator.to_dataframe()
df.to_csv('trades.csv', index=False)
```

#### MarketExpectationCalculator （市场预期）

```python
from dividend_yield_calculator import MarketExpectationCalculator

# 获取市场数据
market_data = MarketExpectationCalculator.get_market_yield('601988')
# {'annual_yield': 5.5, 'dividend_per_share': 0.033, 'expected_price_change_pct': 0.2, ...}

# 计算单个资产的预期收益
expected = MarketExpectationCalculator.calculate_expected_return(
    ticker='601988',
    hold_days=4,
    region='CN',
    price_movement_pct=0.0
)
# {
#   'ticker': '601988',
#   'annual_yield_pct': 5.5,
#   'hold_dividend_yield_pct': 0.060,
#   'expected_total_return_pct': 0.060,
#   'expected_annualized_return_pct': 54.75,
#   ...
# }

# 计算组合预期收益
portfolio = MarketExpectationCalculator.calculate_portfolio_return(
    ['601988', '601398', '601288', '600000', '000858'],
    hold_days=4,
    region='CN'
)
# {
#   'portfolio_size': 5,
#   'average_return_pct': 0.065,
#   'hold_days': 4,
#   'monthly_expected_trades': 5,
#   'monthly_expected_return_pct': 0.325,
#   ...
# }
```

### 2. trading_plan_report.py

生成基于实际交易计划的完整报告：

```bash
python trading_plan_report.py
```

输出文件：
- `China_Trading_Plan_with_Yields.csv` - 中国交易追踪表
- `US_Trading_Plan_with_Yields.csv` - 美国交易追踪表

### 3. yield_analysis.py

全面的市场分析工具：

```bash
# 中国策略分析
python yield_analysis.py --china

# 美国策略分析
python yield_analysis.py --us

# 两个策略对比
python yield_analysis.py --compare

# 完整分析
python yield_analysis.py --all
```

---

## 📐 计算公式

### 单笔交易收益

#### 1. 价格变化率
```
Price Change % = ((Sell Price - Buy Price) / Buy Price) × 100
```

示例：
```
Buy Price: ¥3.15
Sell Price: ¥3.17
Change % = (3.17 - 3.15) / 3.15 × 100 = +0.63%
```

#### 2. 分红收益率
```
Dividend Yield % = (Dividend Per Share / Buy Price) × 100
```

示例：
```
Dividend Per Share: ¥0.033
Buy Price: ¥3.15
Yield % = 0.033 / 3.15 × 100 = 1.048%
```

#### 3. 总收益率
```
Total Return % = Dividend Yield % + Price Change %
```

示例：
```
Total Return % = 1.048% + 0.63% = 1.678%
```

#### 4. 年化收益率
```
Annualized Return % = Total Return % × (365 / Hold Days)
```

示例（4天持仓）：
```
Annualized % = 1.678% × (365 / 4) = 153.1%
```

### 策略聚合指标

#### 1. 获利率
```
Win Rate = Winning Trades / Total Trades
```

#### 2. 利润因子
```
Profit Factor = Total Gains / Total Losses
```

#### 3. 月均预期收益
```
Monthly Expected Return % = (Average Return % × Monthly Trades)
```

示例：
```
Average Return % = 1.245%
Monthly Trades = 6
Monthly Return % = 1.245% × 6 = 7.47%
```

#### 4. 年均预期收益
```
Annual Expected Return % = Monthly Expected Return % × 12
```

示例：
```
Monthly Return % = 7.47%
Annual Return % = 7.47% × 12 = 89.6%
```

### 组合预期收益

#### 基于市场数据的预期
```
Hold Dividend Yield % = (Annual Yield % / 365) × Hold Days × 100

示例 (4天持仓，5.5%年收益):
Hold Yield % = (5.5 / 365) × 4 × 100 = 0.060%

年化折算:
Annualized % = (Hold Yield % / Hold Days) × 365
             = (0.060% / 4) × 365 = 5.475%
```

#### 组合月收益预测
```
Monthly Trades = 20 Trading Days / Hold Days

例如，4天一个交易周期:
Monthly Trades = 20 / 4 = 5 次

Monthly Return % = Average Return % × Monthly Trades
```

---

## 💡 使用示例

### 示例 1: 分析单笔中国股票交易

```python
from dividend_yield_calculator import DividendYieldAnalysis
from datetime import date

# 分析中国银行(601988)的一笔交易
trade = DividendYieldAnalysis(
    ticker='601988',
    buy_date=date(2025, 11, 26),      # 除权除息日T-2
    sell_date=date(2025, 11, 29),     # 除权除息日T+1
    buy_price=3.15,
    sell_price=3.17,
    shares=1000,
    dividend_per_share=0.033
)

# 查看收益分析
print(f"买入成本: ¥{trade.buy_price * trade.shares:,.2f}")
print(f"卖出收入: ¥{trade.sell_price * trade.shares:,.2f}")
print(f"分红收入: ¥{trade.dividend_per_share * trade.shares:,.2f}")
print(f"总收益: ¥{(trade.sell_price - trade.buy_price) * trade.shares + trade.dividend_per_share * trade.shares:,.2f}")
print(f"收益率: {trade.total_return_pct:.3f}%")
print(f"年化: {trade.annualized_return_pct:.1f}%")

# 输出:
# 买入成本: ¥3,150.00
# 卖出收入: ¥3,170.00
# 分红收入: ¥33.00
# 总收益: ¥53.00
# 收益率: +1.678%
# 年化: +204.3%
```

### 示例 2: 分析美国ETF组合

```python
from dividend_yield_calculator import DividendYieldCalculator, DividendYieldAnalysis
from datetime import date

calculator = DividendYieldCalculator()

# 添加8笔美国ETF交易
trades = [
    ('JEPI', date(2025, 11, 13), date(2025, 11, 18), 50.00, 50.30, 100, 0.60),
    ('XYLD', date(2025, 11, 18), date(2025, 11, 23), 25.00, 25.15, 200, 0.50),
    ('SDIV', date(2025, 12, 3), date(2025, 12, 8), 15.00, 15.10, 300, 0.65),
    # ... 更多交易
]

for ticker, buy_d, sell_d, buy_p, sell_p, shares, div in trades:
    analysis = DividendYieldAnalysis(
        ticker=ticker,
        buy_date=buy_d,
        sell_date=sell_d,
        buy_price=buy_p,
        sell_price=sell_p,
        shares=shares,
        dividend_per_share=div
    )
    calculator.add_trade(analysis)

# 生成报告
perf = calculator.calculate_strategy_performance()

print(f"美国策略绩效")
print(f"━" * 50)
print(f"总交易数: {perf.total_trades}")
print(f"获利交易: {perf.winning_trades} ({perf.win_rate*100:.1f}%)")
print(f"平均收益: {perf.avg_return_per_trade:.3f}%")
print(f"预期月收益: {perf.monthly_expected_return_pct:.2f}%")
print(f"预期年收益: {perf.annual_expected_return_pct:.2f}%")

# 输出:
# 美国策略绩效
# ════════════════════════════════════════════════════════
# 总交易数: 8
# 获利交易: 8 (100.0%)
# 平均收益: 1.362%
# 预期月收益: 6.81%
# 预期年收益: 81.7%
```

### 示例 3: 组合收益预测

```python
from dividend_yield_calculator import MarketExpectationCalculator

# 中国股票组合预测
cn_portfolio = MarketExpectationCalculator.calculate_portfolio_return(
    ['601988', '601398', '601288', '600000', '000858', 
     '510300', '510500', '510880', '00700.HK', '00939.HK', '01288.HK'],
    hold_days=4,
    region='CN'
)

print("中国组合预期收益")
print("─" * 50)
print(f"组合规模: {cn_portfolio['portfolio_size']} 个资产")
print(f"平均单次收益: {cn_portfolio['average_return_pct']:.3f}%")
print(f"预期月交易: {cn_portfolio['monthly_expected_trades']} 次")
print(f"预期月收益: {cn_portfolio['monthly_expected_return_pct']:.2f}%")
print(f"预期年收益: {cn_portfolio['monthly_expected_return_pct']*12:.2f}%")

print("\n初始资本收益预测 (CNY)")
print("─" * 50)
for capital in [50000, 100000, 200000, 500000]:
    monthly = capital * cn_portfolio['monthly_expected_return_pct'] / 100
    annual = monthly * 12
    print(f"¥{capital:>7,}: 月均 ¥{monthly:>8,.0f} | 年均 ¥{annual:>10,.0f}")

# 输出:
# 中国组合预期收益
# ──────────────────────────────────────────────────────
# 组合规模: 11 个资产
# 平均单次收益: 0.745%
# 预期月交易: 5 次
# 预期月收益: 3.73%
# 预期年收益: 44.7%
#
# 初始资本收益预测 (CNY)
# ──────────────────────────────────────────────────────
# ¥ 50,000: 月均 ¥1,865 | 年均 ¥22,380
# ¥100,000: 月均 ¥3,730 | 年均 ¥44,760
# ¥200,000: 月均 ¥7,460 | 年均 ¥89,520
# ¥500,000: 月均 ¥18,650 | 年均 ¥223,800
```

---

## ❓ 常见问题

### Q1: 这些预期收益率是否有保证？

**A:** 否。这些数字基于历史数据和市场期望，但未来收益可能不同，具体取决于：
- 实际股息派发和股权登记日期
- 市场波动和价格变动
- 交易成本和滑点
- 汇率变动（国际股票）

### Q2: 持仓天数为什么是3-5天？

**A:** 这是基于股息轮动的时间安排：
- T-2：买入（除息日前两个交易日）
- T：除息日（获得分红权利）
- T+1：卖出（除息日后一个交易日）
- 总计：3-5个交易日

### Q3: 年化收益率为什么这么高？

**A:** 这是因为：
1. 持仓天数很短（3-5天）
2. 短期收益按365天年化
3. 高频重复交易的累积效果

实际年化收益会因以下因素降低：
- 交易成本和手续费
- 部分交易可能不成功
- 市场波动导致的价格不利

### Q4: 如何追踪实际执行结果？

**A:** 使用生成的CSV文件：
```bash
python trading_plan_report.py
```

生成的CSV文件有"实际"列，可用于记录：
- 实际买价和卖价
- 实际分红金额
- 实际收益率
- 交易状态

然后对比预期vs实际进行优化。

### Q5: 中国和美国策略哪个更好？

**A:** 各有优缺点：

| 指标 | 中国 | 美国 |
|------|------|------|
| 预期月收益 | 3-4% | 6-8% |
| 预期年收益 | 40-50% | 80-100% |
| 波动性 | 高 | 中 |
| 复杂度 | 高 | 低 |
| 初始资金 | ¥50k+ | $5k+ |
| 最佳选择 | 有人民币储备 | 有美元储备 |

**建议：** 根据你的货币资产配置选择。如果两种都有，可以同时进行。

### Q6: 月度交易数为什么不固定？

**A:** 因为不是所有股票每月都派息。交易数取决于：
- 股息派息公告
- 除息日期
- 市场流动性

所以月交易数是平均值，实际可能在3-7次之间。

---

## 📊 数据参考

### 中国股票/ETF 市场数据

| 代码 | 名称 | 年化收益 | 分红周期 |
|------|------|---------|---------|
| 601988 | 中国银行 | 5.5% | 4-5个月 |
| 601398 | 工商银行 | 4.7% | 4-5个月 |
| 601288 | 农业银行 | 5.4% | 4-5个月 |
| 600000 | 浦发银行 | 4.9% | 4-5个月 |
| 000858 | 五粮液 | 1.8% | 6-12个月 |
| 510300 | 沪深300 | 3.2% | 6-12个月 |
| 510500 | 中证500 | 2.5% | 6-12个月 |
| 510880 | 红利ETF | 4.5% | 6-12个月 |
| 00700.HK | 腾讯控股 | 1.5% | 6-12个月 |
| 00939.HK | 中国建筑 | 5.2% | 6-12个月 |
| 01288.HK | 农业银行H股 | 5.8% | 6-12个月 |

### 美国 ETF 市场数据

| 代码 | 名称 | 年化收益 | 分红频率 |
|------|------|---------|---------|
| JEPI | JPMorgan Equity Premium Income | 7.2% | 月度 |
| XYLD | Global X S&P 500 Covered Call | 8.3% | 月度 |
| SDIV | Global X Dividend Maximized ETF | 8.9% | 月度 |
| VYM | Vanguard High Dividend Yield | 2.8% | 季度 |
| DGRO | iShares Core Dividend Growth | 2.5% | 季度 |
| NOBL | ProShares S&P 500 Dividend Aristocrats | 2.4% | 季度 |
| SCHD | Schwab US Dividend Equity ETF | 3.3% | 季度 |
| HDV | iShares Core High Dividend ETF | 3.8% | 季度 |

---

## 🔗 相关文件

- `dividend_yield_calculator.py` - 核心计算库
- `trading_plan_report.py` - 交易计划报告
- `yield_analysis.py` - 收益率分析工具
- `verify_yields.py` - 功能验证脚本
- `dividend_rotation_china_v1.py` - 中国策略执行脚本
- `dividend_rotation_v4_real_cli_plan.py` - 美国策略执行脚本

---

**最后更新:** 2025年11月
**版本:** 1.0
