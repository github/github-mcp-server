# V4 高频分红轮动策略 - 完整工具包

## 📋 项目概览

这是一个专业级的量化投资工具套件，用于分析和执行基于除权日期的 ETF 轮动策略。

**核心功能：**
- ✅ 历史回测（24 个月）
- ✅ 智能 ETF 筛选与评分
- ✅ 未来交易计划生成
- ✅ 多格式导出（Excel、PDF、PNG）
- ✅ OMS 集成就绪

---

## 📁 文件结构

```
scripts/
├── dividend_rotation_v4_real_cli_plan.py     # 主程序（850 行）
├── requirements_dividend.txt                 # 依赖清单
├── DIVIDEND_ROTATION_README.md               # 详细文档
├── QUICKSTART.md                             # 5 分钟快速开始
├── run_examples.bat                          # Windows CMD 示例脚本
├── run_examples.ps1                          # PowerShell 示例脚本
└── INDEX.md                                  # 本文件
```

---

## 🚀 快速开始（3 步）

### 第 1 步：安装依赖
```bash
pip install -r requirements_dividend.txt
```

### 第 2 步：设置 API 密钥
```powershell
$env:EODHD_API_TOKEN = "690d7cdc3013f4.57364117"
```

### 第 3 步：运行分析
```bash
python dividend_rotation_v4_real_cli_plan.py --start 2024-01-01 --end 2025-11-11 --emit-xlsx
```

✅ **完成！** 生成的文件已保存到当前目录。

---

## 📚 文档导航

### 🟢 初级用户
**从这里开始：**
1. `QUICKSTART.md` - 5 分钟入门指南
2. 运行 `run_examples.ps1` - 交互式示例菜单
3. 查看生成的 Excel 文件

### 🟡 中级用户
**深入学习：**
1. `DIVIDEND_ROTATION_README.md` - 完整参考手册
2. 查看参数说明表
3. 尝试不同的参数组合

### 🔴 高级用户
**自定义扩展：**
1. 修改主程序源代码
2. 添加自定义评分逻辑
3. 集成到自有 OMS 系统

---

## 💡 常见场景

### 场景 1：我想看过去的表现
```bash
python dividend_rotation_v4_real_cli_plan.py \
  --start 2023-01-01 \
  --end 2025-11-11 \
  --initial-cash 200000 \
  --emit-xlsx --emit-pdf
```
📊 **输出：** 完整的历史回测报告 + PDF

### 场景 2：我想生成下周计划
```bash
python dividend_rotation_v4_real_cli_plan.py \
  --topk 10 \
  --ex-lookahead 7 \
  --output-prefix NextWeek
```
📋 **输出：** `NextWeek_Forward_Plan.csv` 可直接导入 OMS

### 场景 3：我想对比策略效果
运行两次脚本，不同参数：
```bash
# 保守
python dividend_rotation_v4_real_cli_plan.py --min-div-yield 0.03 --output-prefix Conservative --emit-xlsx

# 激进
python dividend_rotation_v4_real_cli_plan.py --min-div-yield 0.01 --topk 20 --output-prefix Aggressive --emit-xlsx
```
📈 **对比：** 两个 Excel 文件中的成交量、收益率、胜率

### 场景 4：我想自动化日常执行
使用 `run_examples.ps1` 或创建任务计划：
```powershell
$action = New-ScheduledTaskAction -Execute "PowerShell.exe" -Argument "-File run_daily.ps1"
$trigger = New-ScheduledTaskTrigger -Daily -At 9:00AM
Register-ScheduledTask -TaskName "DividendRotation" -Action $action -Trigger $trigger
```
⏰ **每日自动执行**

---

## 🎯 核心算法

### 三维评分体系

每个候选 ETF 根据三个维度评分：

```
综合得分 = 0.4 × 股息率归一值 
         + 0.25 × 流动性归一值 
         + 0.35 × 除权日期近度值
```

**权重可调：** 使用 `--wY`, `--wL`, `--wS` 参数

### 交易日期计算

```
除权前 2 天买入 → [除权日] → 除权后 1 天卖出

自动处理：
  ✓ 周末跳过
  ✓ 假期避免
  ✓ 最近交易日选择
```

### 盈亏计算

```
P&L = (卖出价 × 股数) + (分红 × 股数) - (买入价 × 股数)
```

---

## 📊 输出格式

### Excel 文件 (`.xlsx`)

三张工作表：
1. **Top_Candidates** - 入选 ETF 及评分
2. **Buy_Sell_History** - 历史交易详情
3. **Forward_Plan** - 未来计划表

### CSV 文件 (`.csv`)

直接用于 OMS 集成：
```csv
ticker,ex_date,plan_buy_date,plan_sell_date,amount
VYM,2025-11-21,2025-11-19,2025-11-24,1.02
SCHD,2025-11-28,2025-11-26,2025-12-01,0.85
```

### PDF 报告 (`.pdf`)

专业格式报告，包含：
- 回测总结（收益率、胜率、最大回撤）
- Top 候选列表
- 交易明细表
- 未来计划表

### 性能图表 (`.png`)

累计收益率曲线，便于演示

---

## 🔧 参数快速参考

| 参数 | 默认值 | 说明 | 调整建议 |
|------|--------|------|---------|
| `--initial-cash` | 100,000 | 初始资金 | 根据账户规模调整 |
| `--min-div-yield` | 0.009 | 最低股息率 | ↑ = 更稳妥；↓ = 选择更多 |
| `--min-avg-vol` | 200,000 | 最低成交量 | ↑ = 更容易平仓；↓ = 更多选择 |
| `--topk` | 10 | 选择 Top N | ↑ = 更分散；↓ = 更集中 |
| `--hold-pre` | 2 | 除权前天数 | ↑ = 早买；↓ = 迟买 |
| `--hold-post` | 1 | 除权后天数 | ↑ = 迟卖；↓ = 早卖 |
| `--wY` | 0.4 | 股息率权重 | 偏重收入时↑ |
| `--wL` | 0.25 | 流动性权重 | 风险回避时↑ |
| `--wS` | 0.35 | 近度权重 | 高频时↑ |
| `--ex-lookahead` | 90 | 未来窗口 | 短期规划用 30，长期用 180 |

---

## 🐛 常见问题

**Q: 我没有 EODHD API 密钥？**
A: 访问 https://eodhd.com 免费注册即可获得。

**Q: 为什么有些 ETF 没有数据？**
A: 可能因为：
- ETF 太新（历史数据不足）
- 交易量太小（不符合 `--min-avg-vol` 条件）
- 股息率过低（不符合 `--min-div-yield` 条件）

**Q: 如何加快执行速度？**
A: 
- 减少 `--topk` 数值
- 缩短回测时间窗口（`--start` 到 `--end`）
- 使用更严格的筛选条件

**Q: 能否只生成未来计划，不做回测？**
A: 不能完全跳过，但可以：
1. 设置 `--start` 和 `--end` 为同一天
2. 获得空的回测结果
3. 重点关注 `Forward_Plan.csv`

---

## 📋 执行清单

- [ ] 安装 Python 3.8+
- [ ] 安装依赖：`pip install -r requirements_dividend.txt`
- [ ] 获取 EODHD API 密钥
- [ ] 设置环境变量：`$env:EODHD_API_TOKEN = "key"`
- [ ] 运行第一个测试：`python dividend_rotation_v4_real_cli_plan.py --help`
- [ ] 尝试 `run_examples.ps1` 中的示例
- [ ] 根据需要定制参数
- [ ] 集成到 OMS 或任务计划

---

## 🔗 资源链接

- **EODHD API 文档**：https://eodhd.com/api
- **ETF 筛选器**：https://eodhd.com/screener
- **分红日历**：https://eodhd.com/calendar/dividends
- **Python 文档**：https://docs.python.org/3
- **Pandas 教程**：https://pandas.pydata.org

---

## 📞 支持

遇到问题？

1. **查看日志**：脚本输出详细的执行信息
2. **阅读文档**：`DIVIDEND_ROTATION_README.md`
3. **检查参数**：运行 `--help` 查看所有选项
4. **API 限流**：脚本内置自动重试机制

---

## 📄 许可证

MIT License

---

## 👨‍💻 版本历史

- **v4.0** (2025-11-12) - 完整实现，含 OMS 集成
- **v3.5** (2025-10-15) - 添加 PDF 导出
- **v3.0** (2025-09-01) - 首个稳定版

---

**最后更新：2025-11-12**

**下一步：** 打开 `QUICKSTART.md` 开始使用吧！ 🚀
