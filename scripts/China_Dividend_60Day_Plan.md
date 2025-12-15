# 中国股票股息轮动计划 (China Dividend Rotation Plan)

**生成日期 (Generated):** 2025-11-15  
**预测期 (Lookahead Period):** 60 days  
**策略 (Strategy):** Buy 2 days before ex-dividend, Sell 1 day after ex-dividend  
**目标 (Target):** 最大化RMB投资组合收益 (Maximize RMB portfolio returns)

---

## 即将到来的分红事件 (Upcoming Dividend Events)

| # | 代码 (Ticker) | 名称 (Name) | 市场 (Market) | 分红日 (Ex-Date) | 分红额 (Div/Share) | 买入日 (Buy) | 卖出日 (Sell) | 持仓天数 (Hold) |
|---|---|---|---|---|---|---|---|---|
| 1 | 510300 |  | ETF | 2025-11-30 | ¥0.018 | 2025-11-26 | 2025-12-01 | 5 |
| 2 | 510500 |  | ETF | 2025-12-02 | ¥0.015 | 2025-11-28 | 2025-12-03 | 5 |
| 3 | 600000 |  | A-share | 2025-12-03 | ¥0.025 | 2025-12-01 | 2025-12-04 | 3 |
| 4 | 510880 |  | ETF | 2025-12-05 | ¥0.028 | 2025-12-03 | 2025-12-08 | 5 |
| 5 | 601988 |  | A-share | 2025-12-05 | ¥0.033 | 2025-12-03 | 2025-12-08 | 5 |
| 6 | 601398 |  | A-share | 2025-12-05 | ¥0.028 | 2025-12-03 | 2025-12-08 | 5 |
| 7 | 000858 |  | A-share | 2025-12-07 | ¥0.008 | 2025-12-03 | 2025-12-08 | 5 |
| 8 | 601288 |  | A-share | 2025-12-10 | ¥0.032 | 2025-12-08 | 2025-12-11 | 3 |
| 9 | 00939.HK |  | H-share | 2025-12-10 | ¥0.032 | 2025-12-08 | 2025-12-11 | 3 |
| 10 | 01288.HK |  | H-share | 2025-12-13 | ¥0.035 | 2025-12-10 | 2025-12-15 | 5 |
| 11 | 00700.HK |  | H-share | 2025-12-15 | ¥0.015 | 2025-12-11 | 2025-12-16 | 5 |
| 12 | 510300 |  | ETF | 2025-12-30 | ¥0.018 | 2025-12-26 | 2025-12-31 | 5 |
| 13 | 510500 |  | ETF | 2026-01-01 | ¥0.015 | 2025-12-30 | 2026-01-02 | 3 |
| 14 | 600000 |  | A-share | 2026-01-02 | ¥0.025 | 2025-12-31 | 2026-01-05 | 5 |
| 15 | 601398 |  | A-share | 2026-01-04 | ¥0.028 | 2025-12-31 | 2026-01-05 | 5 |
| 16 | 601988 |  | A-share | 2026-01-04 | ¥0.033 | 2025-12-31 | 2026-01-05 | 5 |
| 17 | 510880 |  | ETF | 2026-01-04 | ¥0.028 | 2025-12-31 | 2026-01-05 | 5 |
| 18 | 000858 |  | A-share | 2026-01-06 | ¥0.008 | 2026-01-02 | 2026-01-07 | 5 |
| 19 | 00939.HK |  | H-share | 2026-01-09 | ¥0.032 | 2026-01-07 | 2026-01-12 | 5 |
| 20 | 601288 |  | A-share | 2026-01-09 | ¥0.032 | 2026-01-07 | 2026-01-12 | 5 |
| 21 | 01288.HK |  | H-share | 2026-01-12 | ¥0.035 | 2026-01-08 | 2026-01-13 | 5 |
| 22 | 00700.HK |  | H-share | 2026-01-14 | ¥0.015 | 2026-01-12 | 2026-01-15 | 3 |

---

## 行动计划 (Action Plan by Week)

### 本周 (This Week)

- **Day 1**: Review upcoming ex-dates, prepare order list
- **Day 2-3**: Execute first round of buy orders
- **Day 4-5**: Monitor positions, prepare sell orders

### 关键指标 (Key Metrics)

| 指标 (Metric) | 数值 (Value) |
|---|---|
| 总事件数 (Total Events) | 22 |
| 日期范围 (Date Range) | 2025-11-26 - 2026-01-15 |
| 平均持仓期 (Avg Hold) | 4.5 days |
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

cd c:\Users\micha\github-mcp-server\scripts

# 验证系统
python verify_yields.py

# 快速演示
python demo_yields.py

# 生成报告
python trading_plan_report.pycd c:\Users\micha\github-mcp-server\scripts

# 验证系统
python verify_yields.py

# 快速演示
python demo_yields.py

# 生成报告
python trading_plan_report.pycd c:\Users\micha\github-mcp-server\scripts

# 验证系统
python verify_yields.py

# 快速演示
python demo_yields.py

# 生成报告
python trading_plan_report.pycd c:\Users\micha\github-mcp-server\scripts

# 验证系统
python verify_yields.py

# 快速演示
python demo_yields.py

# 生成报告
python trading_plan_report.pycd c:\Users\micha\github-mcp-server\scripts

# 验证系统
python verify_yields.py

# 快速演示
python demo_yields.py

# 生成报告
python trading_plan_report.py