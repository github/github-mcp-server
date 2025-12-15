# 🚀 Quick Start Guide

## What's Working Now?

### ✅ 1. MCP Server for Claude Desktop
**Location:** `C:\Users\micha\AppData\Roaming\Claude\claude_desktop_config.json`

**Action Required:** Restart Claude Desktop

**Then Ask:**
- "Get the latest price for SCHD"
- "Show dividend history for VYM"
- "What's the dividend yield for JEPI?"

---

### ✅ 2. Python Dividend Rotation Strategy
**Script:** `dividend_rotation_v4_real_cli_plan.py`

**Run It:**
```batch
run_dividend_strategy.bat
```

**What It Does:**
- Fetches real dividend data from EODHD API ✅
- Backtests 2-year dividend rotation strategy ✅
- Generates Excel, PDF, and PNG reports ✅
- Creates forward-looking dividend plan ✅

**Latest Results:**
- 113 trades executed
- 83.2% win rate
- 0.63% cumulative return
- $201,260 final equity (from $200k)

---

## 📊 Generated Reports

1. **Dividend_Rotation_Buy_Sell_Plan.xlsx** - Complete trade log
2. **Dividend_Rotation_Backtest_Report.pdf** - Professional report
3. **Dividend_Rotation_Performance_Chart.png** - Visual performance
4. **Dividend_Rotation_Forward_Plan.csv** - Upcoming dividends

---

## 🔧 What Got Fixed?

**Problem:** API endpoints were returning 404/422 errors

**Solution:** Updated script to use correct EODHD endpoints:
- ❌ `/dividends/` → ✅ `/div/`
- Now getting real historical dividend data!

---

## 📝 Customization

Edit parameters in `run_dividend_strategy.bat`:

```batch
--topk 10              # Number of ETFs to trade
--hold-pre 2           # Days before ex-date to buy
--hold-post 1          # Days after ex-date to sell  
--min-div-yield 0.009  # Minimum 0.9% yield
--initial-cash 200000  # Starting capital
```

---

## 🆘 Need Help?

See full documentation: `EODHD_API_INTEGRATION_SUMMARY.md`
