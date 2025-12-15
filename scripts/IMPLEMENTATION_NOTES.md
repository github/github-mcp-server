# 📦 V4 高频分红轮动策略 - 项目完成总结

## ✅ 已交付文件清单

### 核心程序
- ✅ **dividend_rotation_v4_real_cli_plan.py** (850 行)
  - 完整的量化投资工具
  - 包含回测、筛选、评分、计划生成、导出等全部功能
  - 内置重试机制和错误处理

### 依赖管理
- ✅ **requirements_dividend.txt**
  - 所有必需的 Python 包版本号
  - 一键安装：`pip install -r requirements_dividend.txt`

### 文档
- ✅ **DIVIDEND_ROTATION_README.md** (350 行)
  - 完整功能说明
  - 参数详解
  - 使用示例
  - 故障排除

- ✅ **QUICKSTART.md** (200 行)
  - 5 分钟快速开始指南
  - 常见场景示例
  - 实时监控脚本示例

- ✅ **INDEX.md** (300 行)
  - 项目总览
  - 文件导航
  - 快速参考

### 脚本示例
- ✅ **run_examples.bat**
  - Windows CMD 版本
  - 4 个不同的示例场景
  - 交互式菜单

- ✅ **run_examples.ps1**
  - PowerShell 版本（推荐）
  - 更好的错误处理
  - 彩色输出和进度显示

### 配置工具
- ✅ **config_presets.py** (280 行)
  - 5 种预设配置（保守/均衡/激进/高息/研究）
  - 命令自动生成
  - 预设对比分析

---

## 🎯 核心功能

### 1️⃣ 智能 ETF 筛选
```python
# 基于 EODHD API 的交易所级筛选
- 股息率范围限制
- 流动性（平均成交量）
- 交易所选择（US/HK/L 等）
```

### 2️⃣ 多维评分体系
```
综合得分 = 40% × 股息率 + 25% × 流动性 + 35% × 除权日期近度

支持自定义权重，快速切换策略
```

### 3️⃣ 精确日期计算
```
除权前 2 天买入 → [除权日] → 除权后 1 天卖出
↓
自动跳过周末、假期、数据缺失
↓
精确的交易日期匹配
```

### 4️⃣ 历史回测
```
- 完整模拟 24 个月交易
- 计算每笔交易的盈亏
- 生成权益曲线
- 统计胜率、收益率等指标
```

### 5️⃣ 未来计划表
```
- 未来 30-90 天的除权事件清单
- 自动计算买卖时点
- CSV 格式直接导入 OMS
```

### 6️⃣ 多格式导出
```
✓ Excel (.xlsx) - 3 张工作表
✓ PDF (.pdf) - 专业报告
✓ PNG (.png) - 性能图表
✓ CSV (.csv) - OMS 集成
```

---

## 📊 使用场景

### 场景 1：分析历史表现（5 分钟）
```bash
python dividend_rotation_v4_real_cli_plan.py \
  --start 2024-01-01 \
  --end 2025-11-11 \
  --initial-cash 200000 \
  --emit-xlsx --emit-pdf
```
**获得：** 完整回测报告 + 性能分析

### 场景 2：生成下周计划（3 分钟）
```bash
python dividend_rotation_v4_real_cli_plan.py \
  --topk 10 \
  --ex-lookahead 7 \
  --output-prefix NextWeek
```
**获得：** CSV 计划表 + 买卖时点

### 场景 3：对比策略效果（10 分钟）
```bash
# 运行保守和激进两个版本
python ... --min-div-yield 0.03 --output-prefix Conservative
python ... --min-div-yield 0.01 --topk 20 --output-prefix Aggressive
```
**获得：** 两个 Excel 文件，对比性能差异

### 场景 4：自动化日常执行
```powershell
# 使用 config_presets.py 生成脚本
python config_presets.py gen-ps1 balanced > daily_run.ps1

# 添加到 Windows 任务计划
Register-ScheduledTask -TaskName "DividendRotation" ...
```
**获得：** 每日自动执行的投资计划

---

## 🚀 快速开始（3 步，5 分钟）

### 第 1 步：安装依赖
```bash
pip install -r requirements_dividend.txt
```

### 第 2 步：配置 API 密钥
```powershell
$env:EODHD_API_TOKEN = "690d7cdc3013f4.57364117"
```

### 第 3 步：运行分析
```bash
python dividend_rotation_v4_real_cli_plan.py --emit-xlsx
```

✅ **完成！** 已生成报告文件。

---

## 📁 文件导航

```
scripts/
├── 主程序
│   └── dividend_rotation_v4_real_cli_plan.py ........... 核心程序（850 行）
│
├── 配置
│   ├── requirements_dividend.txt ....................... 依赖清单
│   └── config_presets.py .............................. 预设配置工具（280 行）
│
├── 文档
│   ├── INDEX.md ...................................... 项目导航（★ 从这里开始）
│   ├── QUICKSTART.md .................................. 5 分钟快速开始
│   ├── DIVIDEND_ROTATION_README.md ..................... 完整使用手册
│   └── IMPLEMENTATION_NOTES.md ......................... 本文件
│
└── 脚本
    ├── run_examples.ps1 ............................... PowerShell 示例（推荐）
    └── run_examples.bat ............................... Windows CMD 示例

推荐阅读顺序：
1. INDEX.md ................... 项目概览
2. QUICKSTART.md .............. 立即开始
3. run_examples.ps1 ........... 运行示例
4. config_presets.py .......... 切换策略
5. DIVIDEND_ROTATION_README.md. 深入学习
```

---

## 🎓 学习路径

### 初级（30 分钟）
- [ ] 阅读 INDEX.md
- [ ] 阅读 QUICKSTART.md
- [ ] 安装依赖
- [ ] 运行第一个示例

### 中级（1-2 小时）
- [ ] 尝试 4 个示例场景
- [ ] 修改参数，观察结果变化
- [ ] 理解三维评分体系
- [ ] 查看 Excel 报告结构

### 高级（2-4 小时）
- [ ] 阅读完整的 README.md
- [ ] 学习 config_presets.py 的用法
- [ ] 创建自定义预设
- [ ] 集成到 OMS 系统

---

## 💡 关键技术点

### 1. API 集成
- EODHD REST API 调用
- 自动重试和指数退避
- Gzip 压缩数据处理
- 率限制管理

### 2. 时间序列处理
- 交易日历构建
- 周末/假期处理
- 日期偏移计算
- 价格数据填补

### 3. 数据分析
- Pandas DataFrame 操作
- NumPy 数值计算
- 归一化处理
- 统计指标计算

### 4. 报告生成
- ReportLab PDF 创建
- XlsxWriter Excel 写入
- Matplotlib 图表绘制
- CSV 导出

---

## 🔧 配置预设

5 种内置预设，一键切换：

| 预设 | 股息率 | 选股数 | 交易频率 | 目标收益 |
|------|--------|--------|---------|---------|
| **conservative** | 3%+ | 5 | 低 | 8-12% |
| **balanced** | 1.5%+ | 10 | 中 | 12-18% |
| **aggressive** | 0.9%+ | 20 | 高 | 18-30% |
| **high_yield** | 4.5%+ | 3 | 极低 | 6-10% |
| **quant_research** | 0.5%+ | 50 | 自定义 | 研究 |

**查看所有预设：**
```bash
python config_presets.py list
```

**生成命令：**
```bash
python config_presets.py show conservative
```

**对比预设：**
```bash
python config_presets.py compare conservative aggressive balanced
```

**生成脚本：**
```bash
python config_presets.py gen-ps1 balanced > run_balanced.ps1
```

---

## 📈 输出示例

### Excel 报告内容
```
Sheet1: Top_Candidates
  - 入选 ETF 清单
  - 股息率、流动性、评分排序

Sheet2: Buy_Sell_History
  - 历史交易明细（买入日、卖出日、价格、盈亏）
  - 分红现金统计
  - 交易统计（成交笔数、胜率）

Sheet3: Forward_Plan
  - 未来 30-90 天的除权日期
  - 计划买入日期
  - 计划卖出日期
  - 可直接导入 OMS
```

### PDF 报告内容
```
1. 执行摘要
   - 时间窗口
   - 初始资金、最终资金
   - 累计收益率、胜率

2. 候选 ETF 表格
   - Top 5 或 Top 10

3. 交易明细表
   - 所有历史交易记录

4. 未来计划表
   - 接下来的买卖计划
```

### 性能图表
```
X 轴：时间线
Y 轴：累计收益率 (%)

显示整个回测期的收益曲线
可观察趋势、回撤、波动性
```

---

## ⚙️ 参数自定义

所有参数都可通过命令行或环境变量覆盖：

```bash
# 命令行方式
python dividend_rotation_v4_real_cli_plan.py \
  --min-div-yield 0.025 \
  --topk 5 \
  --hold-pre 1 \
  --initial-cash 500000

# 环境变量方式
$env:MIN_DIVIDEND_YIELD = "0.025"
$env:TOP_K = "5"
$env:INITIAL_CASH = "500000"
```

---

## 🔐 安全注意事项

### API 密钥管理
```powershell
# ✓ 推荐做法
$env:EODHD_API_TOKEN = "your_token"

# ✗ 不要做的
# 不要在代码中硬编码
# 不要在 Git 中提交
# 不要在消息中分享
```

### 数据隐私
- 所有交易仅模拟，未实际执行
- 数据存储在本地
- 支持离线运行（使用缓存）

---

## 🐛 常见问题与解决

| 问题 | 解决方案 |
|------|---------|
| `ModuleNotFoundError` | `pip install -r requirements_dividend.txt` |
| API Token 未设置 | `$env:EODHD_API_TOKEN = "token"` |
| 筛选结果为空 | 降低 `--min-div-yield` 或 `--min-avg-vol` |
| 执行很慢 | 减少 `--topk` 或 `--hold-pre` 值 |
| 429 Rate Limited | 脚本自动重试（无需干预） |
| PDF/PNG 生成失败 | 检查磁盘空间，增加剩余存储 |

---

## 📞 获取帮助

### 文档资源
1. `INDEX.md` - 项目导航
2. `QUICKSTART.md` - 快速开始
3. `DIVIDEND_ROTATION_README.md` - 完整手册
4. `config_presets.py --help` - 预设工具帮助

### 脚本帮助
```bash
python dividend_rotation_v4_real_cli_plan.py --help
```

### API 文档
- https://eodhd.com/api
- https://eodhd.com/screener

---

## 🎯 后续扩展方向

### 短期改进
- [ ] 添加更多 ETF 市场（HK、CN、日本等）
- [ ] 支持股票直接交易
- [ ] 添加滑点和佣金计算
- [ ] 实时监控和告警

### 中期功能
- [ ] Web UI 界面
- [ ] 数据库存储历史记录
- [ ] 机器学习预测权重
- [ ] 实时 OMS 集成 API

### 长期愿景
- [ ] 完整的投资平台
- [ ] 多策略组合管理
- [ ] 风险分析工具
- [ ] 组织级部署

---

## 📝 版本控制

```
v4.0 (2025-11-12) ✅ 发布
├─ 完整的 CLI 工具
├─ 5 种配置预设
├─ 4 种输出格式
├─ 详细文档和示例
└─ OMS 集成就绪

v3.5 (之前)
├─ 基础回测功能
└─ 简单参数配置
```

---

## 📄 许可证

MIT License - 开源且可商业使用

---

## 🙏 致谢

感谢 EODHD 提供的数据 API 支持

---

## 📮 反馈与建议

发现 Bug 或有改进建议？
1. 检查日志输出
2. 查看文档 FAQ
3. 调整参数重试
4. 提交反馈

---

**项目完成日期：2025-11-12**
**所有功能已就绪，可投入使用！** ✨

**开始使用：打开 `INDEX.md` 或 `QUICKSTART.md`** 🚀
