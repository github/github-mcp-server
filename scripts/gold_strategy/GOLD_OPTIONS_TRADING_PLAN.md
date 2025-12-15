# Gold Options Trading Strategy
## Shanghai Futures Exchange (SHFE) - November 2025 Outlook

---

## Executive Summary

This comprehensive trading plan provides a gold options strategy for the China futures market (SHFE), with price predictions based on historical data analysis, technical indicators, and macroeconomic factors. The analysis targets end of November 2025.

**Key Findings:**
- Current Gold Price: $4,085.14/oz (¥932.41/g CNY)
- Predicted Price (End Nov 2025): $4,140.00/oz (¥944.93/g CNY)
- Expected Return: **+1.34%**
- Market Outlook: **Moderately Bullish** with High Volatility

---

## Market Analysis

### Current Market Conditions

| Metric | Value |
|--------|-------|
| Gold Spot (USD/oz) | $4,085.14 |
| Gold Spot (CNY/g) | ¥932.41 |
| USD/CNY Rate | 7.0992 |
| 20-Day Volatility | 19.67% (HIGH) |
| Historical Average Vol | 14.29% |

### Historical Performance

#### November Returns (2010-2025)
| Year | Return |
|------|--------|
| 2022 | +7.34% |
| 2023 | +2.70% |
| 2024 | -2.98% |
| 2025 (YTD) | +2.06% |
| **Average** | **-1.12%** |

#### Recent Trends
| Period | Return |
|--------|--------|
| 1 Month | -0.89% |
| 3 Months | +12.36% |
| 6 Months | +22.75% |
| **1 Year** | **+52.99%** |

---

## Price Prediction Models

### Machine Learning Predictions (15-day forward)

| Model | Expected Return | Price Target (USD) |
|-------|-----------------|-------------------|
| Random Forest | -1.08% | $4,041.16 |
| Gradient Boosting | -0.48% | $4,065.56 |
| Linear Regression | -1.20% | $4,035.93 |

### Technical Analysis Signals

**Bullish Indicators:**
- Price above SMA 20 (¥4,037.44)
- Price above SMA 50 (¥4,018.75)
- Price above SMA 200 (¥3,515.04)
- MACD: +28.72 (above signal line)

**Neutral Indicators:**
- RSI: 54.81 (neutral zone 40-60)

**Summary:** Long-term uptrend intact, but short-term momentum showing mixed signals.

### Fundamental Drivers

| Factor | Current State | Impact on Gold |
|--------|--------------|----------------|
| US Real Interest Rates | Fed cutting cycle | **BULLISH** |
| USD Strength | Weakening trend | **BULLISH** |
| Geopolitical Risk | Elevated tensions | **BULLISH** |
| Central Bank Buying | Record purchases | **VERY BULLISH** |
| Inflation Expectations | Sticky ~2.95% | **NEUTRAL/BULLISH** |
| China Gold Demand | Strong retail + PBOC | **BULLISH** |

**Fundamental Score: 11/11 - STRONGLY BULLISH**

### Final Price Prediction

**95% Confidence Range:**
- Lower Bound: $3,950 (¥901.57/g)
- **Central Estimate: $4,140 (¥944.93/g)**
- Upper Bound: $4,300 (¥981.45/g)

---

## Options Trading Strategies

### Strategy 1: Bull Call Spread (PRIMARY RECOMMENDATION)

**Rationale:** Moderately bullish outlook with defined risk

**Structure:**
- BUY 1x Call @ Strike ¥914 (98% of spot)
- SELL 1x Call @ Strike ¥960 (103% of spot)

**Risk/Reward Profile:**
| Metric | Value |
|--------|-------|
| Net Debit | ¥20.92/g |
| Max Loss | ¥20.92/g |
| Max Profit | ¥25.08/g |
| Breakeven | ¥934.92/g |
| Risk/Reward | 1:1.20 |
| **Max ROI** | **119.9%** |

**P&L Scenarios:**
| Scenario | Price | P&L | Return |
|----------|-------|-----|--------|
| Bearish | ¥886 | -¥20.92 | -100% |
| Neutral | ¥932 | -¥2.50 | -12% |
| Slight Rally | ¥951 | +¥16.14 | +77% |
| Bullish | ¥979 | +¥25.08 | +120% |

---

### Strategy 2: Long Straddle (Volatility Play)

**Rationale:** Benefit from high volatility environment

**Structure:**
- BUY 1x Call @ ¥932 (ATM)
- BUY 1x Put @ ¥932 (ATM)

**Risk/Reward:**
| Metric | Value |
|--------|-------|
| Total Cost | ¥29.66/g |
| Max Loss | ¥29.66/g |
| Max Profit | Unlimited |
| Upper Breakeven | ¥961.66 (+3.14%) |
| Lower Breakeven | ¥902.34 (-3.22%) |

---

### Strategy 3: Protective Put (Hedging)

**For existing long gold positions**

**Structure:**
- HOLD: Long Gold (1000g = 1 SHFE contract)
- BUY 1x Put @ ¥886 (95% of spot)

**Cost of Protection:** ¥1.65/g
**Maximum Downside:** ¥48.06/g

---

### Strategy 4: Iron Condor (Range-Bound)

**Rationale:** Profit if gold stays within range

**Structure:**
- BUY Put @ ¥867
- SELL Put @ ¥895
- SELL Call @ ¥970
- BUY Call @ ¥998

**Risk/Reward:**
| Metric | Value |
|--------|-------|
| Net Credit | ¥4.92/g |
| Max Profit | ¥4.92/g |
| Max Loss | ¥23.08/g |
| Profit Zone | ¥890 - ¥975 |

---

## Trade Execution Plan

### Entry Criteria
1. Wait for price to test SMA 20 support (¥4,037)
2. RSI between 40-60 (neutral, room to move)
3. No major economic event within 24 hours
4. Use limit orders at mid-price or better

### Position Sizing
- **Primary Strategy (Bull Call Spread):** 3-5% of portfolio
- **Alternative (Straddle):** 2-3% of portfolio
- **Maximum Gold Options Allocation:** 10% of total portfolio

### Exit Strategy

**Bull Call Spread:**
- Target: Close at 70% of max profit (¥17.56/g gain)
- Time stop: Close 3 days before expiry if in profit
- Stop loss: Close if loss exceeds ¥10.46/g (50% of premium)

**Long Straddle:**
- Close when one leg shows 50%+ profit
- Close before major event if volatility spike achieved
- Time decay accelerates in last 5 days - be cautious

---

## Risk Management Framework

### Market Risks

1. **Direction Risk**
   - ML models suggest slight negative return (-0.5% to -1.2%)
   - Fundamental support strong but November historically weak

2. **Volatility Risk**
   - Current vol 38% above average
   - High vol favors straddles but increases spread costs

3. **Currency Risk**
   - USD/CNY at 7.0992
   - Yuan depreciation would increase CNY gold prices

4. **Liquidity Risk**
   - SHFE options less liquid than COMEX
   - Use limit orders, avoid large positions

### Risk Mitigation

- **Diversification:** Don't exceed 10% allocation to gold options
- **Position sizing:** Scale into positions (50% initial, 50% on pullback)
- **Hedging:** Consider protective puts for long positions
- **Stop losses:** Pre-defined exit points before entry
- **Event monitoring:** Close or reduce before major announcements

---

## Key Economic Events - November 2025

| Date | Event | Expected Impact |
|------|-------|-----------------|
| Early Nov | FOMC Meeting | Rate cut expected - Bullish |
| Mid Nov | China PMI | Manufacturing health |
| Mid Nov | US CPI | Inflation data - Higher = Bullish |
| Late Nov | US Employment | Weak jobs = Fed dovish = Bullish |
| Ongoing | China PBOC | Rate changes affect CNY/Gold |
| Ongoing | Geopolitical | Safe haven flows |

**Critical:** Fed Balance Sheet report on Nov 28

---

## SHFE Gold Contract Specifications

- **Contract Size:** 1000 grams per lot
- **Price Quote:** Yuan (RMB) per gram
- **Min Price Change:** 0.02 Yuan/gram
- **Delivery Unit:** 3000 grams (fine weight)
- **Gold Purity:** Minimum 999.5 fine
- **Listed Contracts:** Recent 3 months + bi-monthly for 13 months

---

## Implementation Checklist

- [ ] Review current portfolio allocation
- [ ] Verify SHFE account access and margin requirements
- [ ] Set up price alerts at key levels (¥914, ¥932, ¥960)
- [ ] Monitor USD/CNY exchange rate daily
- [ ] Subscribe to economic calendar for Nov 2025
- [ ] Set stop-loss orders immediately after entry
- [ ] Daily review of Greeks and position delta

---

## Disclaimer

This trading plan is based on historical data analysis and mathematical models. Past performance does not guarantee future results. Options trading involves significant risk and may not be suitable for all investors. Consult with a qualified financial advisor before implementing any trading strategy. The author assumes no responsibility for trading losses.

---

## Files Generated

1. `gold_data_collector.py` - Data collection from EODHD API
2. `gold_price_predictor.py` - ML-based price prediction models
3. `options_strategy.py` - Options strategy calculator
4. `gold_historical_with_indicators.csv` - 15 years of historical data
5. `usd_cny_exchange.csv` - Exchange rate data
6. `us_macro_indicators.json` - US economic indicators
7. `china_macro_indicators.json` - China economic indicators
8. `november_2025_events.json` - Economic calendar
9. `gold_options_strategy.json` - Strategy parameters

---

**Generated:** November 16, 2025
**Data Source:** EODHD API, Shanghai Futures Exchange
**Analysis Period:** 2010-2025
