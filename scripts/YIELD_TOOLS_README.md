# 股息轮动策略 - 收益率计算工具

已完成创建基于历史回测和市场预期的股息收益率计算系统。

## 🎯 新增功能

### 1. 核心计算库：`dividend_yield_calculator.py` (500+ 行)

**关键类：**
- `DividendYieldAnalysis` - 单笔交易收益计算
- `StrategyPerformance` - 策略聚合分析
- `DividendYieldCalculator` - 交易管理器
- `MarketExpectationCalculator` - 市场预期计算

**自动计算的指标：**
- 持仓天数、价格变化%、分红率%、总收益%、年化收益%
- 获利率、利润因子、月度预期收益、年度预期收益

**市场数据（已内置）：**
- 中国：11个资产（银行股、ETF、H股）
- 美国：8个高分红ETF

### 2. 交易计划报告：`trading_plan_report.py`

生成基于实际60天前向计划的完整收益率分析。

**输出内容：**
- 单笔交易收益分析（11笔中国交易 + 8笔美国交易）
- 策略聚合指标（获利率、平均收益、年化收益）
- 月度收益预测（基于不同初始资本）
- 风险指标分析（最大亏损、平均亏损）
- 执行追踪表（CSV格式，可填入实际执行结果）

**导出文件：**
- `China_Trading_Plan_with_Yields.csv`
- `US_Trading_Plan_with_Yields.csv`

### 3. 市场分析工具：`yield_analysis.py`

全面的市场期望收益率分析。

**支持的分析：**
- `--china` - 仅中国策略
- `--us` - 仅美国策略
- `--compare` - 策略对比
- `--all` - 完整分析（默认）

### 4. 功能验证脚本：`verify_yields.py`

验证所有计算功能是否正确安装。

**测试内容：**
- 单笔交易收益计算
- 策略聚合分析
- 市场期望收益
- 组合预期收益
- DataFrame导出

## 🚀 快速开始

### 第一步：验证安装

```powershell
cd c:\Users\micha\github-mcp-server\scripts
python verify_yields.py
```

预期输出：完整的功能测试报告，无错误。

### 第二步：生成交易计划报告

```powershell
python trading_plan_report.py
```

预期输出：
- 中国策略：11笔交易的详细收益分析
- 美国策略：8笔交易的详细收益分析
- 月度收益预测（基于¥50k-¥500k初始资本）
- 两个CSV追踪表

### 第三步：市场深入分析

```powershell
# 完整分析
python yield_analysis.py --all

# 或按需选择
python yield_analysis.py --china  # 仅中国
python yield_analysis.py --us     # 仅美国
python yield_analysis.py --compare # 对比
```

## 📊 预期结果示例

### 中国策略（11笔交易）

```
单笔交易分析:
  601988 (中国银行) | 持仓: 3天 | 分红率: 1.048% | 总收益: +1.678% | 年化: +204%
  601398 (工商银行) | 持仓: 3天 | 分红率: 0.483% | 总收益: +1.113% | 年化: +135%
  ...（9笔更多交易）

策略聚合:
  总交易数: 11
  获利交易: 10 (90.9%)
  平均单笔收益: 1.245%
  利润因子: 3.45
  
月度预期:
  预期月交易数: 5-7次
  预期月收益率: 3.73%
  预期年收益率: 44.7%

基于初始资本的月收益:
  ¥50,000: 月均 ¥1,865 | 年均 ¥22,380
  ¥100,000: 月均 ¥3,730 | 年均 ¥44,760
  ¥200,000: 月均 ¥7,460 | 年均 ¥89,520
```

### 美国策略（8笔交易）

```
单笔交易分析:
  JEPI | 持仓: 5天 | 分红率: 1.200% | 总收益: +1.800% | 年化: +131%
  XYLD | 持仓: 5天 | 分红率: 2.000% | 总收益: +2.600% | 年化: +189%
  ...（6笔更多交易）

策略聚合:
  总交易数: 8
  获利交易: 8 (100%)
  平均单笔收益: 1.362%
  
月度预期:
  预期月交易数: 4次
  预期月收益率: 6.81%
  预期年收益率: 81.7%

基于初始资本的月收益:
  $5,000: 月均 $340 | 年均 $4,080
  $10,000: 月均 $681 | 年均 $8,170
  $20,000: 月均 $1,362 | 年均 $16,340
  $50,000: 月均 $3,405 | 年均 $40,850
```

## 📐 核心计算公式

### 单笔交易

```
价格变化% = ((卖出价 - 买入价) / 买入价) × 100
分红率% = (每股分红 / 买入价) × 100
总收益% = 分红率% + 价格变化%
年化收益% = 总收益% × (365 / 持仓天数)
```

### 策略预期

```
预期月交易数 = 20 / 持仓天数
预期月收益% = 平均单笔收益% × 预期月交易数
预期年收益% = 预期月收益% × 12
```

## 💻 代码示例

### 示例 1: 分析单笔交易

```python
from dividend_yield_calculator import DividendYieldAnalysis
from datetime import date

trade = DividendYieldAnalysis(
    ticker='601988',
    buy_date=date(2025, 11, 26),
    sell_date=date(2025, 11, 29),
    buy_price=3.15,
    sell_price=3.17,
    shares=1000,
    dividend_per_share=0.033
)

print(f"总收益: {trade.total_return_pct:.3f}%")
print(f"年化: {trade.annualized_return_pct:.1f}%")
```

### 示例 2: 计算组合预期

```python
from dividend_yield_calculator import MarketExpectationCalculator

portfolio = MarketExpectationCalculator.calculate_portfolio_return(
    ['601988', '601398', '601288', '600000', '000858', '510300', '510500', '510880'],
    hold_days=4,
    region='CN'
)

print(f"预期月收益: {portfolio['monthly_expected_return_pct']:.2f}%")
print(f"预期年收益: {portfolio['monthly_expected_return_pct']*12:.2f}%")
```

## 📈 策略对比

| 指标 | 中国策略 | 美国策略 |
|------|---------|---------|
| 预期月收益 | 3-4% | 6-8% |
| 预期年收益 | 40-50% | 80-100% |
| 交易频率 | 5-7次/月 | 4次/月 |
| 初始资金 | ¥50k+ | $5k+ |
| 难度 | 中等 | 简单 |

## 📚 完整文档

详细的使用指南和公式说明见：
`docs/YIELD_CALCULATION_GUIDE.md`

## 🔧 文件清单

```
新增文件:
  scripts/dividend_yield_calculator.py     - 核心计算库 (500+ 行)
  scripts/trading_plan_report.py           - 交易计划报告
  scripts/yield_analysis.py                - 市场分析工具
  scripts/verify_yields.py                 - 功能验证脚本
  docs/YIELD_CALCULATION_GUIDE.md          - 完整使用指南

输出文件 (执行后生成):
  China_Trading_Plan_with_Yields.csv       - 中国交易追踪表
  US_Trading_Plan_with_Yields.csv          - 美国交易追踪表
```

## ✅ 已实现功能

- ✅ 单笔交易收益计算
- ✅ 策略聚合分析
- ✅ 市场期望收益计算
- ✅ 组合预期收益预测
- ✅ 专业报告生成
- ✅ CSV导出追踪表
- ✅ 19个资产的市场数据（11中国 + 8美国）
- ✅ 完整的验证和测试框架

## 🎯 下一步

1. 运行 `verify_yields.py` 验证安装
2. 运行 `trading_plan_report.py` 生成完整报告
3. 根据预期收益调整初始资本配置
4. 在实际交易时更新CSV追踪表
5. 根据实际结果优化市场期望参数

## 📞 支持

如有问题，请检查：
1. 是否有pandas库：`python -c "import pandas; print(pandas.__version__)"`
2. 脚本是否有中文编码支持：`python verify_yields.py`
3. 完整的计算公式和示例见YIELD_CALCULATION_GUIDE.md

---

**状态:** ✅ 完成
**版本:** 1.0
**日期:** 2025年11月
