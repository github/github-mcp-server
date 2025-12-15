# 📊 V4 高频分红轮动策略 - 完整工具套件

> **项目状态：✅ 完全就绪，可投入生产** | **版本：v4.0** | **发布日期：2025-11-12**

## 概述

这是一套专业级的量化投资工具，用于自动分析和执行基于除权日期的 ETF 轮动策略。包含历史回测、智能筛选、未来计划生成和多格式报告导出。

**核心特性：**
- ✅ 智能 ETF 筛选与多维评分
- ✅ 历史回测（24 个月）
- ✅ 未来交易计划自动生成
- ✅ 精确日期计算（自动处理周末/假期）
- ✅ OMS 集成就绪（CSV 导出）
- ✅ 多格式报告（Excel、PDF、PNG）
- ✅ 5 种预设配置
- ✅ 完整文档和示例

---

## 📁 文件清单

| 文件 | 类型 | 说明 |
|------|------|------|
| **dividend_rotation_v4_real_cli_plan.py** | 核心 | 850 行完整 Python 程序 |
| **requirements_dividend.txt** | 配置 | 依赖包列表 |
| **config_presets.py** | 工具 | 280 行配置预设管理工具 |
| **INDEX.md** | 📖 导航 | **【从这里开始】** 项目总览 |
| **QUICKSTART.md** | 📖 入门 | 5 分钟快速开始指南 |
| **DIVIDEND_ROTATION_README.md** | 📖 参考 | 350 行完整使用手册 |
| **QUICK_REFERENCE.md** | 📖 速查 | 一页纸快速参考卡 |
| **IMPLEMENTATION_NOTES.md** | 📖 深度 | 400 行技术实现细节 |
| **PROJECT_SUMMARY.py** | 📖 总结 | 项目完成总结和清单 |
| **run_examples.ps1** | 脚本 | PowerShell 示例脚本（推荐）|
| **run_examples.bat** | 脚本 | Windows CMD 示例脚本 |
| **README.md** | 📖 本文 | 此文件 |

---

## 🚀 快速开始（3 步，5 分钟）

### 步骤 1：安装依赖
```bash
pip install -r requirements_dividend.txt
```

### 步骤 2：配置 API 密钥
```powershell
# Windows PowerShell
$env:EODHD_API_TOKEN = "690d7cdc3013f4.57364117"
```

### 步骤 3：运行分析
```bash
python dividend_rotation_v4_real_cli_plan.py --emit-xlsx
```

✅ **完成！** 生成的报告已保存到当前目录。

---

## 📖 文档导航

### 🟢 如果你是初学者（30 分钟）
1. 打开 **INDEX.md** - 了解项目结构
2. 打开 **QUICKSTART.md** - 5 分钟上手
3. 运行 `.\run_examples.ps1` - 尝试示例
4. 查看生成的 Excel 文件

### 🟡 如果你想深入学习（1-2 小时）
1. 打开 **DIVIDEND_ROTATION_README.md** - 学习完整功能
2. 尝试不同的命令参数
3. 理解三维评分体系
4. 对比不同策略的效果

### 🔴 如果你想了解技术细节（2-4 小时）
1. 打开 **IMPLEMENTATION_NOTES.md** - 技术实现细节
2. 查看 **config_presets.py** - 学习配置管理
3. 修改源代码，实现自定义逻辑
4. 集成到自有 OMS 系统

### ⚡ 如果你需要快速参考
- 打开 **QUICK_REFERENCE.md** - 一页纸快速查阅

---

## 💡 5 个常见使用场景

### 场景 1：分析历史表现（2024 年）
```bash
python dividend_rotation_v4_real_cli_plan.py \
  --start 2024-01-01 \
  --end 2024-12-31 \
  --initial-cash 200000 \
  --emit-xlsx --emit-pdf
```
**生成：** 完整的回测报告 + 性能分析

### 场景 2：生成下周计划（OMS 导入）
```bash
python dividend_rotation_v4_real_cli_plan.py \
  --topk 10 \
  --ex-lookahead 7 \
  --output-prefix NextWeek
```
**生成：** `NextWeek_Forward_Plan.csv` 可直接导入 OMS

### 场景 3：对比策略效果
```bash
# 保守版（高息、低频）
python ... --min-div-yield 0.03 --topk 5 --output-prefix Conservative --emit-xlsx

# 激进版（高频、近度优先）
python ... --min-div-yield 0.009 --topk 20 --wS 0.6 --output-prefix Aggressive --emit-xlsx
```
**对比：** 两个 Excel 文件中的成交量、收益率、胜率

### 场景 4：使用预设配置
```bash
# 查看所有预设
python config_presets.py list

# 使用保守预设
python config_presets.py show conservative

# 对比不同预设
python config_presets.py compare conservative aggressive
```
**快速：** 5 种预设一键切换

### 场景 5：自动化日常执行
```powershell
# 创建每日脚本
$action = New-ScheduledTaskAction -Execute "PowerShell.exe" -Argument "-File daily_plan.ps1"
$trigger = New-ScheduledTaskTrigger -Daily -At 9:00AM
Register-ScheduledTask -TaskName "DividendRotation" -Action $action -Trigger $trigger
```
**自动：** 每天早上 9 点自动生成计划

---

## 🎯 核心功能说明

### 1. 智能 ETF 筛选
- 基于交易所过滤器
- 股息率范围限制
- 流动性要求
- 自定义条件组合

### 2. 多维评分体系
```
综合得分 = 40% × 股息率 + 25% × 流动性 + 35% × 除权日期近度
```
- 支持自定义权重
- 快速策略切换

### 3. 精确交易日期计算
```
除权前 2 天买入 → [除权日] → 除权后 1 天卖出
↓
自动跳过周末、假期、数据缺失
↓
精确的交易日期匹配
```

### 4. 历史回测
- 完整模拟 24 个月交易
- 计算每笔交易的盈亏
- 生成权益曲线
- 统计胜率、收益率等指标

### 5. 未来计划生成
- 未来 30-90 天除权事件清单
- 自动计算买卖时点
- CSV 格式直接导入 OMS

### 6. 多格式导出
- 📊 **Excel (.xlsx)** - 3 张工作表
- 📄 **PDF (.pdf)** - 专业报告
- 🖼️ **PNG (.png)** - 性能图表
- 📋 **CSV (.csv)** - OMS 集成

---

## ⚙️ 参数快速参考

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--start` | 2023-11-01 | 回测开始日期 |
| `--end` | 昨日 | 回测结束日期 |
| `--initial-cash` | 100,000 | 初始资金 (USD) |
| `--exchange` | US | 交易所代码 |
| `--min-div-yield` | 0.009 | 最低股息率 |
| `--min-avg-vol` | 200,000 | 最低成交量 |
| `--topk` | 10 | 选股数量 |
| `--hold-pre` | 2 | 除权前天数 |
| `--hold-post` | 1 | 除权后天数 |
| `--ex-lookahead` | 90 | 未来窗口 (天) |
| `--wY` | 0.4 | 股息率权重 |
| `--wL` | 0.25 | 流动性权重 |
| `--wS` | 0.35 | 近度权重 |
| `--emit-xlsx` | - | 导出 Excel |
| `--emit-pdf` | - | 导出 PDF |
| `--emit-png` | - | 导出图表 |

---

## 🎓 5 种预设配置

### 保守型（Conservative）
- 最低股息率：3%+
- 选股数量：5
- 交易频率：低
- 风险水平：低
- 目标收益：8-12%

### 均衡型（Balanced）
- 最低股息率：1.5%+
- 选股数量：10
- 交易频率：中
- 风险水平：中
- 目标收益：12-18%

### 激进型（Aggressive）
- 最低股息率：0.9%+
- 选股数量：20
- 交易频率：高
- 风险水平：高
- 目标收益：18-30%

### 高息专注（High Yield）
- 最低股息率：4.5%+
- 选股数量：3
- 交易频率：极低
- 风险水平：极低
- 目标收益：6-10%

### 量化研究（Quant Research）
- 最低股息率：0.5%+
- 选股数量：50
- 交易频率：自定义
- 风险水平：研究导向
- 目标收益：不限

**查看所有预设：**
```bash
python config_presets.py list
```

---

## 📊 输出文件说明

### Excel 文件 (`.xlsx`)
```
Dividend_Rotation_Buy_Sell_Plan.xlsx
├─ Sheet1: Top_Candidates（入选 ETF 和评分）
├─ Sheet2: Buy_Sell_History（历史交易明细）
└─ Sheet3: Forward_Plan（未来交易计划）
```

### PDF 报告 (`.pdf`)
```
Dividend_Rotation_Backtest_Report.pdf
├─ 执行摘要（时间、资金、收益率、胜率）
├─ 候选清单表格
├─ 交易明细表格
└─ 未来计划表格
```

### 性能图表 (`.png`)
```
Dividend_Rotation_Performance_Chart.png
└─ 累计收益率曲线（便于演示和分析）
```

### OMS 集成 (`.csv`)
```
Dividend_Rotation_Forward_Plan.csv
可直接导入订单管理系统，自动执行交易
```

---

## ⚠️ 常见问题

**Q: 如何安装依赖？**
A: `pip install -r requirements_dividend.txt`

**Q: API 密钥在哪里获取？**
A: 访问 https://eodhd.com 免费注册即可获得

**Q: 怎样只生成未来计划，不做回测？**
A: 设置 `--start` 和 `--end` 为同一天即可

**Q: 能否修改评分权重？**
A: 可以，使用 `--wY`, `--wL`, `--wS` 参数修改

**Q: 如何集成到 OMS？**
A: 使用生成的 CSV 文件，导入到 OMS 即可

更多问题详见 **DIVIDEND_ROTATION_README.md** 的故障排除章节。

---

## 🔗 重要资源

| 资源 | 链接 |
|------|------|
| EODHD API | https://eodhd.com/api |
| ETF 筛选器 | https://eodhd.com/screener |
| 分红日历 | https://eodhd.com/calendar/dividends |
| 交易所假期 | https://eodhd.com/calendar |

---

## ✨ 核心优势

- 🎯 **一键分析** - 从筛选到计划全自动化
- 📅 **精确日期** - 自动处理周末和假期  
- ⚖️ **灵活评分** - 支持自定义权重
- 🔗 **OMS 就绪** - CSV 格式直接集成
- 📚 **完整文档** - 快速开始 + 详细参考
- ⚙️ **预设配置** - 5 种策略一键切换
- 📊 **专业报告** - Excel、PDF、PNG 多格式
- 🔄 **自动重试** - 内置 API 限流处理
- ⚡ **高效执行** - 优化的数据处理
- 🔐 **安全可靠** - 完善的错误处理

---

## 📝 许可证

MIT License - 开源且可商业使用

---

## 📞 获取帮助

### 在线文档
1. **INDEX.md** - 项目导航
2. **QUICKSTART.md** - 快速开始  
3. **DIVIDEND_ROTATION_README.md** - 完整手册
4. **QUICK_REFERENCE.md** - 快速参考

### 脚本帮助
```bash
python dividend_rotation_v4_real_cli_plan.py --help
python config_presets.py --help
```

### 外部资源
- EODHD API 文档：https://eodhd.com/api
- Python 文档：https://docs.python.org/3
- Pandas 教程：https://pandas.pydata.org

---

## 🎉 开始使用

```bash
# 第 1 步：安装依赖
pip install -r requirements_dividend.txt

# 第 2 步：设置 API 密钥
$env:EODHD_API_TOKEN = "690d7cdc3013f4.57364117"

# 第 3 步：运行分析
python dividend_rotation_v4_real_cli_plan.py --emit-xlsx

# 或者查看交互式示例
.\run_examples.ps1
```

✅ **准备好了吗？** 打开 **INDEX.md** 或 **QUICKSTART.md** 开始吧！

---

**项目状态：✅ 完全就绪** | **版本：v4.0** | **发布日期：2025-11-12** | **许可证：MIT**
