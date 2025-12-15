# 快速开始指南 - V4 高频分红轮动策略

## 5 分钟快速上手

### 第 1 步：安装依赖 (1 分钟)

```powershell
pip install -r requirements_dividend.txt
```

**包含以下库：**
- `requests` - HTTP 客户端
- `pandas` - 数据处理
- `numpy` - 数值计算
- `matplotlib` - 绘图
- `reportlab` - PDF 生成
- `xlsxwriter` - Excel 导出

### 第 2 步：配置 API 密钥 (1 分钟)

**选项 A - 环境变量（推荐）**

```powershell
# Windows PowerShell
$env:EODHD_API_TOKEN = "690d7cdc3013f4.57364117"

# 验证
Write-Host $env:EODHD_API_TOKEN
```

**选项 B - 持久化设置**

```powershell
# 在 PowerShell 配置文件中添加
# 文件位置：$PROFILE
echo '$env:EODHD_API_TOKEN = "690d7cdc3013f4.57364117"' >> $PROFILE
```

### 第 3 步：运行你的第一个分析 (3 分钟)

```bash
python dividend_rotation_v4_real_cli_plan.py ^
  --start 2024-01-01 ^
  --end 2025-11-11 ^
  --initial-cash 200000 ^
  --topk 10 ^
  --emit-xlsx
```

**预期输出：**
```
2025-11-12 10:30:45 [INFO] 候选ETF数量：120
2025-11-12 10:31:02 [INFO] Top10：VYM, SCHD, DGRO, ...
2025-11-12 10:31:15 [INFO] —— 执行完成 ——
2025-11-12 10:31:15 [INFO] 已导出 Excel：Dividend_Rotation_Buy_Sell_Plan.xlsx
```

**生成文件：**
- `Dividend_Rotation_Buy_Sell_Plan.xlsx` ✓
- `Dividend_Rotation_Forward_Plan.csv` ✓

---

## 常见场景

### 场景 1：我想看看过去 24 个月的表现

```bash
python dividend_rotation_v4_real_cli_plan.py ^
  --start 2023-11-01 ^
  --end 2025-11-11 ^
  --initial-cash 100000
```

**输出：**
- 历史交易清单
- 累计回报率
- 胜率

### 场景 2：我想生成下周的买卖计划

```bash
python dividend_rotation_v4_real_cli_plan.py ^
  --topk 10 ^
  --ex-lookahead 7 ^
  --output-prefix NextWeek
```

**输出：**
- `NextWeek_Forward_Plan.csv`
- 包含：股票代码、除权日、计划买入日、计划卖出日

**直接导入 OMS：**
```powershell
# 用 Excel 打开 CSV，复制到订单管理系统
Invoke-Item NextWeek_Forward_Plan.csv
```

### 场景 3：我想对比不同策略的效果

```powershell
# 保守策略（高息）
python dividend_rotation_v4_real_cli_plan.py ^
  --min-div-yield 0.03 ^
  --topk 5 ^
  --output-prefix Conservative ^
  --emit-xlsx

# 激进策略（高频）
python dividend_rotation_v4_real_cli_plan.py ^
  --min-div-yield 0.01 ^
  --topk 20 ^
  --wS 0.6 ^
  --output-prefix Aggressive ^
  --emit-xlsx
```

**然后对比：**
- `Conservative_Buy_Sell_Plan.xlsx` vs `Aggressive_Buy_Sell_Plan.xlsx`
- 观察成交量、收益率、胜率

### 场景 4：我想生成完整的投资报告

```bash
python dividend_rotation_v4_real_cli_plan.py ^
  --start 2024-01-01 ^
  --end 2025-11-11 ^
  --initial-cash 500000 ^
  --topk 15 ^
  --output-prefix MyReport ^
  --emit-xlsx --emit-pdf --emit-png
```

**生成三件套：**
1. `MyReport_Buy_Sell_Plan.xlsx` - 数据详表
2. `MyReport_Backtest_Report.pdf` - 专业报告（含图表）
3. `MyReport_Performance_Chart.png` - 收益曲线

**用途：**
- 向投资者展示
- 存档备案
- 性能跟踪

---

## 参数速查表

| 需求 | 参数 | 建议值 |
|------|------|--------|
| **提高息率** | `--min-div-yield` | 0.03 |
| **降低风险** | `--topk` | 5-10 |
| **提高频率** | `--ex-lookahead` | 30 |
| **增加资金** | `--initial-cash` | 500000 |
| **选择更多** | `--topk` | 20-30 |
| **提早买入** | `--hold-pre` | 3-5 |
| **延迟卖出** | `--hold-post` | 2-3 |

---

## 实时监控脚本

创建 `run_daily.ps1`，每天自动生成计划：

```powershell
# run_daily.ps1
$date = Get-Date -Format "yyyyMMdd_HHmmss"
$prefix = "DailyPlan_$date"

python dividend_rotation_v4_real_cli_plan.py `
  --topk 10 `
  --ex-lookahead 30 `
  --output-prefix $prefix `
  --emit-xlsx

Write-Host "计划已生成：${prefix}_Forward_Plan.csv"

# 可选：上传到云存储
# Copy-Item "${prefix}_*.xlsx" -Destination "C:\CloudFolder\"
```

**添加到 Windows 任务计划：**

```powershell
$action = New-ScheduledTaskAction -Execute "PowerShell.exe" -Argument "-NoProfile -File C:\Path\To\run_daily.ps1"
$trigger = New-ScheduledTaskTrigger -Daily -At 9:00AM
Register-ScheduledTask -TaskName "DividendRotationDaily" -Action $action -Trigger $trigger
```

---

## 故障排除

| 问题 | 解决方案 |
|------|---------|
| `ModuleNotFoundError: No module named 'pandas'` | `pip install -r requirements_dividend.txt` |
| `EODHD_API_TOKEN 未设置` | `$env:EODHD_API_TOKEN = "token"` |
| `筛选结果为空` | 降低 `--min-div-yield` 或 `--min-avg-vol` |
| `429 Rate Limited` | 脚本会自动重试，无需干预 |
| PDF/PNG 生成失败 | 检查磁盘空间，尝试去掉 `--emit-pdf --emit-png` |

---

## 下一步

1. **阅读详细文档**：`DIVIDEND_ROTATION_README.md`
2. **查看高级参数**：运行 `python dividend_rotation_v4_real_cli_plan.py --help`
3. **集成到 OMS**：使用 CSV 格式的计划表
4. **定期运行**：设置任务计划自动执行

---

## 更多帮助

- **API 文档**：https://eodhd.com/api
- **ETF 筛选**：https://eodhd.com/screener
- **分红日历**：https://eodhd.com/calendar/dividends

---

**祝你投资愉快！** 📈
