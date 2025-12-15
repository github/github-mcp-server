# V4 高频分红轮动策略 - 实盘 CLI 工具

## 概述

`dividend_rotation_v4_real_cli_plan.py` 是一个专业级的量化投资工具，用于：

1. **历史回测**：模拟过去 24 个月的分红轮动策略表现
2. **未来计划**：根据除权日期自动计算最优的买入/卖出时点
3. **多维评分**：综合股息率、流动性和除权日期近度进行 ETF 选择
4. **完整导出**：生成 Excel、PDF 和 PNG 格式的报告

## 核心特性

### 1. 智能筛选与评分
- 基于交易所过滤器（行业、流动性、股息率）筛选 ETF
- 三维评分体系：
  - **Y**：股息率（权重 40%）
  - **L**：平均成交量（权重 25%）
  - **S**：除权日期近度（权重 35%）

### 2. 交易日历管理
- 自动获取美国交易所假期
- 精确计算除权前/后的买入/卖出交易日
- 处理周末和假期跳过

### 3. 历史回测
- 模拟 24 个月内所有分红事件
- 计算每笔交易的盈亏（含分红现金）
- 生成权益曲线和累计回报率

### 4. 前向计划表
- 未来 90 天（可配置）的除权日期清单
- 自动计算每个事件的计划买入/卖出日期
- 可直接集成到 OMS（订单管理系统）

## 安装

### 1. 安装依赖

```bash
pip install -r requirements_dividend.txt
```

### 2. 配置 API 密钥

设置环境变量 `EODHD_API_TOKEN`：

**Windows (PowerShell):**
```powershell
$env:EODHD_API_TOKEN = "690d7cdc3013f4.57364117"
```

**Windows (CMD):**
```cmd
set EODHD_API_TOKEN=690d7cdc3013f4.57364117
```

**Linux/macOS:**
```bash
export EODHD_API_TOKEN="690d7cdc3013f4.57364117"
```

也可以在脚本中硬编码（不推荐用于生产环境）。

## 使用方法

### 基础用法

```bash
python dividend_rotation_v4_real_cli_plan.py \
  --start 2023-11-01 \
  --end 2025-11-11 \
  --initial-cash 200000 \
  --emit-xlsx --emit-pdf --emit-png
```

### 完整参数说明

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--start` | 2023-11-01 | 回测开始日期（YYYY-MM-DD） |
| `--end` | 昨日 | 回测结束日期（YYYY-MM-DD） |
| `--initial-cash` | 100000 | 初始资金（USD） |
| `--exchange` | US | 交易所（US/HK/L 等） |
| `--min-div-yield` | 0.009 | 最低股息率阈值（0.009 = 0.9%） |
| `--min-avg-vol` | 200000 | 最低平均成交量 |
| `--topk` | 10 | 选择前 N 个候选 |
| `--ex-lookahead` | 90 | 未来计划窗口（天数） |
| `--hold-pre` | 2 | 除权前买入偏移（交易日数） |
| `--hold-post` | 1 | 除权后卖出偏移（交易日数） |
| `--alloc-per-event` | 0.33 | 每个事件分配的现金比例 |
| `--wY` | 0.4 | 股息率权重 |
| `--wL` | 0.25 | 流动性权重 |
| `--wS` | 0.35 | 近度权重 |
| `--output-prefix` | Dividend_Rotation | 输出文件前缀 |
| `--emit-xlsx` | - | 导出 Excel |
| `--emit-pdf` | - | 导出 PDF |
| `--emit-png` | - | 导出图表 |

## 使用示例

### 示例 1：保守策略（低频、高息）
```bash
python dividend_rotation_v4_real_cli_plan.py \
  --initial-cash 200000 \
  --min-div-yield 0.025 \
  --min-avg-vol 500000 \
  --topk 5 \
  --hold-pre 1 --hold-post 2 \
  --wY 0.6 --wL 0.2 --wS 0.2 \
  --output-prefix Conservative_Strategy \
  --emit-xlsx
```

### 示例 2：激进策略（高频、近度优先）
```bash
python dividend_rotation_v4_real_cli_plan.py \
  --initial-cash 500000 \
  --min-div-yield 0.009 \
  --min-avg-vol 100000 \
  --topk 20 \
  --hold-pre 3 --hold-post 0 \
  --wY 0.2 --wL 0.2 --wS 0.6 \
  --ex-lookahead 30 \
  --output-prefix Aggressive_Strategy \
  --emit-xlsx --emit-pdf --emit-png
```

### 示例 3：完整分析（所有导出）
```bash
python dividend_rotation_v4_real_cli_plan.py \
  --start 2023-01-01 \
  --end 2025-11-11 \
  --initial-cash 1000000 \
  --exchange US \
  --topk 15 \
  --output-prefix Full_Analysis \
  --emit-xlsx --emit-pdf --emit-png
```

## 输出文件说明

### 1. Excel 文件 (`*_Buy_Sell_Plan.xlsx`)

包含三张工作表：

**Sheet1: Top_Candidates**
- 入选的 ETF 清单
- 股息率、流动性、最近除权日期等信息
- 综合评分排序

**Sheet2: Buy_Sell_History**
- 历史回测期间的所有交易
- 买入日期、卖出日期、价格、分红现金、盈亏

**Sheet3: Forward_Plan**
- 未来除权事件及计划交易时点
- 可直接导入 OMS 执行

### 2. PDF 报告 (`*_Backtest_Report.pdf`)

包含：
- 回测总结（时间、资金、收益率、胜率）
- Top 候选列表（表格）
- 交易明细（详细记录）
- 未来计划表

### 3. 图表 (`*_Performance_Chart.png`)

- X 轴：时间
- Y 轴：累计收益率 (%)
- 显示策略全周期表现

## 关键概念

### 除权日期处理

```
除权前 2 天买入 → [除权日] → 除权后 1 天卖出

例如：除权日 2025-11-14（周五）
  - 买入：2025-11-12（周三）
  - 卖出：2025-11-17（周一）
  
自动跳过周末和假期，精确匹配交易日。
```

### 风险管理

1. **单笔分配**：`--alloc-per-event` 限制单笔风险
2. **流动性过滤**：`--min-avg-vol` 确保可快速平仓
3. **多头篮子**：`--topk` 分散风险

## 数据来源

- **API**：EODHD (https://eodhd.com)
- **数据类型**：
  - EOD 价格数据（开/高/低/收/调整收）
  - 成交量
  - 分红历史
  - 交易所假期

## 环境变量配置

也可通过环境变量控制所有参数（便于自动化）：

```bash
set EODHD_API_TOKEN=your_token
set START=2024-01-01
set END=2025-11-11
set INITIAL_CASH=500000
set TOP_K=15
set EXCHANGE=US
set MIN_DIVIDEND_YIELD=0.015
set HOLD_PRE_DAYS=2
set HOLD_POST_DAYS=1
set OUTPUT_PREFIX=MyStrategy
```

## 故障排除

### 错误：`EODHD_API_TOKEN 未设置`
**解决**：确保环境变量已设置，重启终端后再运行。

### 错误：`筛选结果为空`
**解决**：调低 `--min-div-yield` 或 `--min-avg-vol` 参数。

### 警告：`获取交易所假期失败，使用工作日近似`
**影响**：轻微，脚本会用周一-周五作为交易日近似。

### 率限制（429 错误）
**处理**：脚本内置自动重试与指数退避，会自动等待并重试。

## 性能建议

- 首次运行会缓存数据，后续速度更快
- 对于大型回测（>12 个月），可能需要 5-10 分钟
- 建议在离峰时段运行，避免 API 限流

## 集成到 OMS

导出的 CSV 计划表格式：

```
ticker,name,ex_date,amount,currency,plan_buy_date,plan_sell_date,note
VYM,Vanguard High Dividend Yield ETF,2025-11-21,1.02,USD,2025-11-19,2025-11-24,
SCHD,Schwab US Dividend Equity ETF,2025-11-28,0.85,USD,2025-11-26,2025-12-01,
```

直接导入 OMS，按 `plan_buy_date` 和 `plan_sell_date` 自动下单。

## 开源许可

MIT License - 详见 LICENSE 文件

## 支持与反馈

如有问题或建议，请联系开发团队或提交 Issue。

---

**最后更新**：2025-11-12
**版本**：4.0
