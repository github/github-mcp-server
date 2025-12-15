# US vs China Dividend Rotation Strategy Comparison

## 📊 Strategy Comparison Matrix

| Aspect | US Strategy | China Strategy | Recommendation |
|--------|------------|----------------|-----------------|
| **API Data Source** | EODHD | TuShare | Use both for diversification |
| **Target Assets** | 8 ETFs | 11 Stocks/ETFs | China has more opportunities |
| **Annual Yields** | 7.2 - 8.9% | 1.8 - 5.8% | US higher but China more frequent |
| **Typical Events/Month** | 2-3 | 4-6 | China offers more cycles |
| **Expected Return/Cycle** | 0.5 - 1.5% | 0.3 - 0.8% | US higher but both good |
| **Hold Period** | 5-6 days | 3-5 days | China faster turnaround |
| **Currency** | USD | CNY + HKD | Diversify both |
| **Settlement** | T+1 | T+1 (A) / T+2 (H) | China requires more planning |
| **Tax Treatment** | 0% (ETF distributions) | 10-20% (dividend tax) | US more tax efficient |
| **Broker Setup** | US brokers only | Chinese brokers + 港股通 | Separate but straightforward |
| **Complexity Level** | Easy | Medium | China needs holiday calendar |
| **Capital Required** | $5,000+ | ¥50,000+ | Similar in USD terms |
| **Risk Level** | Low-Medium | Medium | Similar risk profiles |

---

## 🎯 Which Strategy for Your Situation?

### Choose US Strategy If:
```
✓ You want simplicity and easy execution
✓ You have USD available and prefer US markets
✓ You want higher yields (7-9% annually)
✓ You prefer 1-2 trades per month
✓ You want 0% dividend tax (ETF distributions)
✓ You trade during US market hours

→ Good for: Passive investors, US-based traders
```

### Choose China Strategy If:
```
✓ You have CNY/RMB funds
✓ You want maximum frequency (4-6 trades/month)
✓ You enjoy active trading and daily monitoring
✓ You understand China dividend tax rules
✓ You can handle 3-5 day cycles consistently
✓ You want portfolio diversification into China

→ Good for: Active traders, China-focused investors
```

### Choose BOTH Strategies If:
```
✓ You have both USD and CNY available
✓ You want maximum diversification (2 markets)
✓ You can manage separate accounts
✓ You enjoy active trading on both sides
✓ You want 10+ trading opportunities per month

→ Best for: Serious traders, balanced approach
→ Expected monthly return: 2-5% (combined)
→ Work needed: 15-20 mins/day
```

---

## 💵 Capital Allocation Strategy

### Conservative Portfolio (Small Capital)
```
Total: $5,000 / ¥50,000

Option A (US Only):
  └─ EODHD ETFs: 100% = $5,000
  └─ Monthly trades: 2-3
  └─ Expected return: 1-1.5%/month = $50-75

Option B (China Only):
  └─ A-Shares: 60% = ¥30,000
  └─ H-Shares: 40% = ¥20,000
  └─ Monthly trades: 4-5
  └─ Expected return: 1.5-2%/month = ¥750-1,000

Option C (Balanced):
  └─ US ETFs: 30% = $1,500
  └─ China A-Shares: 50% = ¥40,000
  └─ China H-Shares: 20% = ¥10,000
  └─ Monthly trades: 5-6
  └─ Expected return: 1.5-2%/month (blended)
```

### Moderate Portfolio
```
Total: $50,000 / ¥500,000

Balanced Allocation:
  └─ US ETFs: 40% = $20,000 → 3-4 trades/month
  └─ China A-Shares: 40% = ¥200,000 → 4-5 trades/month
  └─ China H-Shares: 20% = ¥100,000 → 2-3 trades/month
  
Monthly Income Target:
  └─ US: $100-300
  └─ China: ¥1,000-2,000
  └─ Total: $100-300 + ¥1,000-2,000 (blended)
  
Expected Return: 2-3% monthly = $1,000-1,500
```

### Aggressive Portfolio
```
Total: $100,000+ / ¥1,000,000+

Diversified Allocation:
  └─ US ETFs: 30% = $30,000 → 4-5 trades/month
  └─ China A-Shares: 50% = ¥500,000 → 8-10 trades/month
  └─ China H-Shares: 20% = ¥200,000 → 3-4 trades/month
  
Monthly Income Target:
  └─ US: $300-500
  └─ China: ¥3,000-5,000
  └─ Combined: 3-5% monthly
  
Expected Return: $300-500 + ¥3,000-5,000/month
Annual Projection: $3,600-6,000 + ¥36,000-60,000
```

---

## 🔄 Daily/Weekly Workflow Comparison

### US Strategy - Daily Workflow
```
Morning (09:00 EST):
  1. Check EODHD for any dividend announcements
  2. Review current positions
  3. Execute any buys (T-2 days before ex-date)
  
Midday (Passive):
  - Monitor during US market hours (13:30-20:00 Beijing time)
  - Check for any sharp price movements
  
Afternoon (14:30 EST):
  1. Prepare any sells for tomorrow (T+1 after ex-date)
  2. Log trades in spreadsheet
  3. Review next week's schedule

Time commitment: 30 minutes/day
Trading frequency: 2-3x per month
```

### China Strategy - Daily Workflow
```
Morning (09:00 Beijing):
  1. Run: python dividend_rotation_china_v1.py
  2. Review 60-day forward plan
  3. Execute any buys (T-2 before ex-date)
  4. Check for cancellations/cuts
  
During Day:
  - Monitor 09:30-11:30 + 13:00-15:00 (trading hours)
  - Watch for unexpected gaps
  
Afternoon (15:00):
  1. Confirm all positions secured
  2. Prepare sells for tomorrow
  3. Update trading log
  4. Record dividend received

Time commitment: 45 minutes/day
Trading frequency: 4-6x per month
```

### Combined (Both Strategies) - Daily Workflow
```
Early Morning (08:00):
  1. Check overnight news (US & China markets)
  2. Run both scripts:
     - python dividend_rotation_v4_real_cli_plan.py
     - python dividend_rotation_china_v1.py
  3. Review combined opportunities

Morning (09:00-11:00):
  1. Execute buys for both markets (3-4 trades typical)
  2. Monitor execution quality
  
Afternoon (14:00-15:30):
  1. US market opens → monitor
  2. China market closes → finalize sells
  3. Update combined ledger
  
Evening (18:00):
  1. Weekly planning if needed
  2. Performance review

Time commitment: 1-1.5 hours/day
Trading frequency: 6-10x per month
Monthly expected return: 2-4%
```

---

## 📈 Expected Returns Comparison

### Scenario 1: Conservative (¥50,000 Capital)

**US Only:**
```
Capital: ¥50,000 ($6,700)
Monthly trades: 3
Return/trade: 0.7%
Monthly return: 2.1% = ¥1,050
Annual return: 25.2% = ¥12,600
```

**China Only:**
```
Capital: ¥50,000
Monthly trades: 5
Return/trade: 0.5%
Monthly return: 2.5% = ¥1,250
Annual return: 30% = ¥15,000
```

**Both (50/50):**
```
Capital: ¥50,000 (¥25,000 each)
US return: ¥1,050 × 0.5 = ¥525
China return: ¥1,250 × 0.5 = ¥625
Monthly total: ¥1,150
Annual total: ¥13,800
```

### Scenario 2: Aggressive (¥500,000 Capital)

**US Only:**
```
Capital: ¥500,000 ($67,000)
Monthly trades: 5
Return/trade: 0.8%
Monthly return: 4% = ¥20,000
Annual return: 48% = ¥240,000
```

**China Only:**
```
Capital: ¥500,000
Monthly trades: 8
Return/trade: 0.6%
Monthly return: 4.8% = ¥24,000
Annual return: 57.6% = ¥288,000
```

**Both (40/60):**
```
Capital: ¥500,000 (¥200k US, ¥300k China)
US return: ¥20,000 × 0.4 = ¥8,000
China return: ¥24,000 × 0.6 = ¥14,400
Monthly total: ¥22,400
Annual total: ¥268,800 (53.76% annual)
```

---

## ⚙️ Technical Integration

### Option 1: Separate Tracking
```
File: US_Trading_Log.xlsx
├─ Column A: Date
├─ Column B: ETF Ticker
├─ Column C: Buy Price
├─ Column D: Sell Price
├─ Column E: Dividend
├─ Column F: P&L
└─ Column G: Return %

File: China_Trading_Log.xlsx
├─ Same structure as US
├─ Support both CNY and HKD
└─ Track tax implications
```

### Option 2: Consolidated Tracking
```
File: Combined_Performance.xlsx

Sheet 1: US Trades
├─ Current month's US activity
└─ YTD summary

Sheet 2: China Trades
├─ Current month's China activity
└─ YTD summary (CNY converted to USD)

Sheet 3: Combined Dashboard
├─ Total capital deployed
├─ Monthly return (blended)
├─ Win rate by market
└─ Risk metrics

Sheet 4: Future Calendar
├─ Next 60 days (US + China merged)
└─ Opportunity summary
```

### Option 3: Automated Tracking
```
Script: combine_results.py

Input:
  - FORWARD_PLAN_60DAY.md (US strategy output)
  - China_Dividend_60Day_Plan.md (China strategy output)

Output:
  - Combined_60Day_Opportunities.md
    └─ Sorted by date
    └─ Shows alternating US/China trades
    └─ Highlights conflicts (if any)
    └─ Calculates optimal deployment

Usage:
  python combine_results.py --output Combined_Plan.md
```

---

## 🎓 Learning Path Recommendations

### Week 1-2: Foundation
```
□ US Strategy:
  └─ Read: FORWARD_PLAN_60DAY.md
  └─ Run: dividend_rotation_v4_real_cli_plan.py
  └─ Execute: 1 test trade (minimum position)
  └─ Monitor: 3-5 days hold period
  
□ China Strategy:
  └─ Read: CHINA_STRATEGY_GUIDE.md
  └─ Run: dividend_rotation_china_v1.py
  └─ Execute: 1 test trade (minimum position)
  └─ Monitor: 3-5 days hold period
```

### Week 3-4: Consolidation
```
□ Execute 3-5 full cycles in parallel
□ Track performance daily
□ Keep detailed notes on:
  └─ What worked
  └─ What surprised you
  └─ Risk events encountered
  └─ Tax implications realized
```

### Month 2+: Optimization
```
□ Increase position size based on confidence
□ Optimize allocation (40/60 vs 30/70, etc.)
□ Implement automated tracking
□ Plan annual strategy review
```

---

## ⚠️ Risk Management Across Both Strategies

### Position Sizing Rule
```
Total exposure = max(portfolio value × 3)

Example with ¥500,000 portfolio:
  Maximum simultaneous exposure: ¥1,500,000
  
  At ¥500k capital:
  └─ Can run 5-6 trades simultaneously
  └─ Each ¥100,000 on average
  └─ Rotates every 3-5 days
```

### Diversification Requirements
```
Across ALL positions (US + China):
  └─ No single position > 10% of capital
  └─ No single sector > 20% of capital
  └─ Technology + Finance max 50%
  └─ Energy + Consumer max 30%

Example:
  Capital: ¥500,000
  Max per position: ¥50,000
  Max per sector: ¥100,000
```

### Stop-Loss Rules
```
Hard stops (exit immediately):
  └─ Position down -2% = EXIT
  └─ Dividend cut announced = EXIT
  └─ Major fraud/scandal = EXIT
  └─ Sector-wide suspension = EXIT

Soft stops (monitor closely):
  └─ Position down -1% to -2% = WATCH
  └─ Competitor bad news = WATCH
  └─ Regulatory announcement = WATCH
```

---

## 📊 Monthly Performance Dashboard

```
╔════════════════════════════════════════════════╗
║       MONTHLY DIVIDEND ROTATION REPORT         ║
║              November 2025                      ║
╠════════════════════════════════════════════════╣
║                                                 ║
║  US Strategy (EODHD):                          ║
║  ├─ Trades Completed: 3                        ║
║  ├─ Total Return: 2.1% = $1,050                ║
║  ├─ Win Rate: 100% (3/3)                       ║
║  ├─ Best Trade: +1.5% (JEPI)                   ║
║  └─ Average Hold: 5.2 days                     ║
║                                                 ║
║  China Strategy (TuShare):                     ║
║  ├─ Trades Completed: 5                        ║
║  ├─ Total Return: 2.5% = ¥1,250                ║
║  ├─ Win Rate: 100% (5/5)                       ║
║  ├─ Best Trade: +0.8% (601988)                 ║
║  └─ Average Hold: 3.8 days                     ║
║                                                 ║
║  Combined Performance:                         ║
║  ├─ Total Capital Deployed: ¥500,000           ║
║  ├─ Total Trades: 8                            ║
║  ├─ Blended Return: 2.3%                       ║
║  ├─ Win Rate: 100%                             ║
║  └─ Annualized (if consistent): 27.6%          ║
║                                                 ║
╚════════════════════════════════════════════════╝
```

---

## 🎯 Recommended Starting Strategy

### For New Traders:
```
Month 1: US Only
  └─ Reason: Simpler to learn, higher yields
  └─ Execute: 2-3 trades
  └─ Goal: Build confidence

Month 2: Add China
  └─ Reason: Once comfortable, expand to more opportunities
  └─ Execute: 3-4 China trades
  └─ Goal: Compare markets and execution

Month 3+: Balanced Portfolio
  └─ Run both in parallel
  └─ Allocate capital based on results
  └─ Target: 2-3% monthly blended return
```

### For Experienced Traders:
```
Month 1: Deploy Both Immediately
  └─ US: 40% of capital
  └─ China: 60% of capital
  └─ Execute: 6-8 trades total/month
  └─ Target: 2-4% monthly

Month 2+: Optimization
  └─ Adjust allocation based on market conditions
  └─ Increase frequency as confidence grows
  └─ Consider leverage only if proven profitable
```

---

## Summary

| Metric | US Strategy | China Strategy | Combined |
|--------|------------|----------------|----------|
| **Ease of Entry** | Easy | Medium | Recommended |
| **Monthly Opportunities** | 2-3 | 4-6 | 8-10 |
| **Expected Return/Month** | 1.5-2% | 2-3% | 2-3% |
| **Tax Efficiency** | High | Medium | Medium |
| **Daily Time Required** | 30 mins | 45 mins | 1 hour |
| **Capital Flexibility** | High | Medium | High |
| **Best For** | Passive investors | Active traders | Balanced approach |

**The best strategy depends on your available capital, time commitment, and risk tolerance. Start with whichever feels natural, then expand to both for maximum returns.**
