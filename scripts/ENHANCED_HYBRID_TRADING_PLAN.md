
# Enhanced Hybrid Trading Plan: "Macro-Filtered IBD & News Sentiment Model"

This document outlines a sophisticated, multi-layered trading strategy that integrates macroeconomic analysis, IBD-based technical scoring, and real-time news sentiment with the existing quantitative prediction engine. It is designed to make more informed, context-aware trading decisions.

## 1. Core Principles

- **Strategy Type:** A hybrid model combining quantitative time-series prediction, qualitative news analysis, and fundamental/technical scoring.
- **Decision Process:** A layered filtering system. A trade must pass all qualitative checks before the quantitative prediction is acted upon.
- **Asset Focus:** AAPL.US (Apple Inc.) and XAUUSD.FOREX (Gold).

---

## 2. Layer 1: Macroeconomic Health Check

This layer acts as the first and most important filter. We do not proceed if the macroeconomic environment is hostile to the asset class.

| Indicator                     | Source                | Latest Value (Nov 2025) | Implication for AAPL (Consumer Discretionary)                                    | Implication for XAUUSD (Gold)                                                    | Status (AAPL) | Status (XAUUSD) |
| ----------------------------- | --------------------- | ----------------------- | -------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- | :-----------: | :-------------: |
| **US Inflation Rate (CPI)**   | TradingEconomics      | 3.0% (Rising from 2.9%) | **Negative:** Rising inflation erodes consumer purchasing power for non-essential goods. | **Positive:** Gold is a traditional inflation hedge.                                |      🔴      |       🟢       |
| **US Interest Rate**          | TradingEconomics      | 3.75%-4.00% (Cut)       | **Positive:** Lower rates can boost growth stocks and increase borrowing/spending. | **Positive:** Lower rates decrease the opportunity cost of holding non-yielding gold. |      🟢      |       🟢       |
| **US Dollar Index (DXY)**     | TradingEconomics      | 99.6 (Slightly up)      | **Neutral:** A stable dollar has a mixed but generally minor impact.             | **Negative:** A stronger dollar typically pressures gold prices downwards.          |      🟡      |       🔴       |
| **US Consumer Confidence**    | TradingEconomics      | 51.0 (Falling)          | **Negative:** Low/falling confidence signals reduced consumer spending ahead.    | **Neutral:** Less direct impact, but can signal economic fear.                    |      🔴      |       🟡       |
| **US Retail Sales (MoM)**     | TradingEconomics      | +0.2% (Slowing)         | **Negative:** Slowing sales growth is a direct negative signal for retail products.  | **Neutral:** Not a primary driver for gold.                                      |      🔴      |       🟡       |
| **China Manufacturing PMI**   | TradingEconomics      | 50.6 (Slowing)          | **Negative:** China is a key market and supply chain hub for Apple. Slowdown hurts. | **Neutral/Positive:** A slowdown in China can increase global economic uncertainty. |      🔴      |       🟢       |
| **OVERALL MACRO SCORE**       |                       |                         |                                                                                  |                                                                                  |    **Hostile**    |    **Favorable**    |

**Rule:**
*   **For AAPL:** The overall macro score is **Hostile**. **Do not initiate new long positions.** Short positions may be considered if other layers align.
*   **For XAUUSD:** The overall macro score is **Favorable**. **Proceed to the next layer for both long and short considerations.**

---

## 3. Layer 2: IBD CAN SLIM Technical Scorecard (AAPL Only)

This layer applies the principles of Investor's Business Daily methodology to AAPL. Gold is not a stock and is therefore exempt from this analysis.

| CAN SLIM Factor       | Analysis                                                                                                 | Data Source              | Score (0-2) |
| --------------------- | -------------------------------------------------------------------------------------------------------- | ------------------------ | :---------: |
| **C** - Current Earnings  | *Requires fundamental data feed.* (Placeholder: Assume neutral until feed is connected).               | Financial Data API       |      1      |
| **A** - Annual Growth     | *Requires fundamental data feed.* (Placeholder: Assume neutral until feed is connected).               | Financial Data API       |      1      |
| **N** - New.../New High | **New Products:** M5 iPad/MacBook, Vision Pro are new. **New Highs:** Must check price vs 52-week high.      | News / Price Data        |      2      |
| **S** - Supply & Demand | Technical Check: Is price moving up on high volume? (Requires volume data analysis).                       | Price/Volume Feed        |      1      |
| **L** - Leader/Laggard    | News indicates AAPL is set to surpass Samsung, making it a clear market leader.                          | News / Market Data       |      2      |
| **I** - Institutional   | *Requires institutional ownership data.* (Placeholder: Assume neutral).                                  | Financial Data API       |      1      |
| **M** - Market Direction  | SPY vs SMA50 (as defined in previous plan). Currently in a **Bearish Regime** (`SPY < SMA50`). | Price Data               |      0      |
| **TOTAL IBD SCORE**       |                                                                                                          |                          | **8 / 14**  |

**Rule:**
*   A score **below 7** is weak.
*   A score of **7-10** is decent.
*   A score **above 10** is strong.

With a score of **8/14**, AAPL shows some positive signs (Leadership, New Products) but is held back by the overall weak market direction. This is a **Neutral** signal. It does not provide a strong case for a long position and slightly supports a short one given the weak market (`M` score is 0).

---

## 4. Layer 3: News Sentiment Analysis

This layer assesses the immediate "mood" from recent headlines.

| Asset    | Recent Headlines                                                                                                        | Sentiment Score (-2 to +2) |
| -------- | ----------------------------------------------------------------------------------------------------------------------- | :------------------------: |
| **AAPL** | - Strong China sales (+) <br>- Set to beat Samsung (+) <br>- Potential large fine (-) <br>- Lawsuit (-) <br>- Job cuts (-) |             -1             |
| **XAUUSD** | - Fed rate cut expectations (+) <br>- Softer inflation (+) <br>- Geopolitical tension (+) <br>- Holding near highs (+)         |             +2             |

**Rule:**
*   **-2 (Strongly Negative):** Avoid long trades.
*   **-1 (Negative):** Reduce position size for long trades.
*   **0 (Neutral):** No impact.
*   **+1 (Positive):** Standard position size.
*   **+2 (Strongly Positive):** Consider a slightly larger position size.

---

## 5. Layer 4: Final Trade Execution

This is the final step where we combine all layers to make a decision.

### For AAPL.US:

1.  **Macro-Economic Check:** **Hostile (FAIL)**. Do not initiate new long positions.
2.  **IBD Scorecard:** **8/14 (Neutral)**. Does not support a long position.
3.  **News Sentiment:** **-1 (Negative)**.
4.  **Quantitative Prediction:** Latest prediction shows a minor positive change of `+0.02%`.

**Conclusion for AAPL:** **NO TRADE.** The macro environment is poor, the IBD score is mediocre, and news sentiment is negative. Even if the quantitative model were strongly positive, the qualitative filters would overrule it. We do not risk capital on a long trade here. A short trade could be considered, but the quantitative signal does not support it.

### For XAUUSD.FOREX:

1.  **Macro-Economic Check:** **Favorable (PASS)**.
2.  **IBD Scorecard:** Not applicable.
3.  **News Sentiment:** **+2 (Strongly Positive)**.
4.  **Quantitative Prediction:** Latest prediction shows a minor negative change of `-0.12%`.
5.  **Risk Management Rules (from previous plan):**
    *   **Entry Signal:** A trade requires a `change_pct` signal stronger than +/- 0.5%. The current prediction of -0.12% is **too weak to act on.**
    *   **Volatility:** `VIX` is assumed to be < 25 (per previous plan).
    *   **Stop/Profit:** Use `lower_95` and `upper_95` for stop-loss placement.

**Conclusion for XAUUSD:** **NO TRADE, BUT STRONG BULLISH BIAS.** The macro and news sentiment is strongly bullish. We are simply waiting for a quantitative entry signal. The system should now actively monitor the predictions for Gold, waiting for a prediction with `change_pct > +0.5%`. When that signal appears, it is pre-approved by the qualitative layers to be executed immediately with a slightly larger position size due to the high conviction from the sentiment score.

This layered approach prevents the autonomous engine from trading in hostile environments and aligns its actions with a deeper, more comprehensive understanding of the market.
