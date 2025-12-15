#!/usr/bin/env python3
"""
Gold Options Trading Strategy for China Futures Market (SHFE)
Based on November 2025 Price Outlook
"""

import numpy as np
from scipy.stats import norm
import json
from datetime import datetime

class GoldOptionsStrategy:
    """Gold options strategy generator for SHFE market"""

    def __init__(self):
        # Current market conditions (from predictions)
        self.spot_price_usd = 4085.14  # USD per oz
        self.usd_cny_rate = 7.0992
        self.spot_price_cny = self.spot_price_usd * self.usd_cny_rate / 31.1035  # Convert to CNY/gram
        self.volatility = 0.1967  # 19.67% annualized
        self.risk_free_rate_cny = 0.02  # ~2% China risk-free rate
        self.days_to_expiry = 15  # End of November 2025
        self.time_to_expiry = self.days_to_expiry / 365

        # Price predictions
        self.predicted_price_usd = 4140.0  # Based on ML + fundamentals
        self.lower_bound_usd = 3950.0
        self.upper_bound_usd = 4300.0

        # Convert to SHFE format (CNY per gram)
        self.predicted_price_cny = self.predicted_price_usd * self.usd_cny_rate / 31.1035
        self.lower_bound_cny = self.lower_bound_usd * self.usd_cny_rate / 31.1035
        self.upper_bound_cny = self.upper_bound_usd * self.usd_cny_rate / 31.1035

    def black_scholes_call(self, S, K, T, r, sigma):
        """Calculate call option price using Black-Scholes"""
        if T <= 0:
            return max(S - K, 0)

        d1 = (np.log(S / K) + (r + 0.5 * sigma**2) * T) / (sigma * np.sqrt(T))
        d2 = d1 - sigma * np.sqrt(T)

        call_price = S * norm.cdf(d1) - K * np.exp(-r * T) * norm.cdf(d2)
        return call_price

    def black_scholes_put(self, S, K, T, r, sigma):
        """Calculate put option price using Black-Scholes"""
        if T <= 0:
            return max(K - S, 0)

        d1 = (np.log(S / K) + (r + 0.5 * sigma**2) * T) / (sigma * np.sqrt(T))
        d2 = d1 - sigma * np.sqrt(T)

        put_price = K * np.exp(-r * T) * norm.cdf(-d2) - S * norm.cdf(-d1)
        return put_price

    def calculate_greeks(self, S, K, T, r, sigma, option_type='call'):
        """Calculate option Greeks"""
        if T <= 0:
            return {'delta': 0, 'gamma': 0, 'theta': 0, 'vega': 0}

        d1 = (np.log(S / K) + (r + 0.5 * sigma**2) * T) / (sigma * np.sqrt(T))
        d2 = d1 - sigma * np.sqrt(T)

        if option_type == 'call':
            delta = norm.cdf(d1)
        else:
            delta = norm.cdf(d1) - 1

        gamma = norm.pdf(d1) / (S * sigma * np.sqrt(T))
        vega = S * norm.pdf(d1) * np.sqrt(T) / 100
        theta = (-(S * norm.pdf(d1) * sigma) / (2 * np.sqrt(T)) -
                 r * K * np.exp(-r * T) * norm.cdf(d2 if option_type == 'call' else -d2)) / 365

        return {
            'delta': delta,
            'gamma': gamma,
            'theta': theta,
            'vega': vega
        }

    def generate_strategy_report(self):
        """Generate comprehensive options trading strategy"""
        print("="*70)
        print("GOLD OPTIONS TRADING STRATEGY FOR SHFE")
        print("November 2025 Outlook")
        print("="*70)

        print(f"\n📊 MARKET CONDITIONS:")
        print(f"  Current Spot Price (USD/oz): ${self.spot_price_usd:.2f}")
        print(f"  Current Spot Price (CNY/g): ¥{self.spot_price_cny:.2f}")
        print(f"  USD/CNY Exchange Rate: {self.usd_cny_rate:.4f}")
        print(f"  Implied Volatility: {self.volatility*100:.2f}%")
        print(f"  Days to Expiry: {self.days_to_expiry}")

        print(f"\n🎯 PRICE OUTLOOK (End of November 2025):")
        print(f"  Predicted Price (USD): ${self.predicted_price_usd:.2f}")
        print(f"  Predicted Price (CNY/g): ¥{self.predicted_price_cny:.2f}")
        print(f"  Expected Return: {((self.predicted_price_usd/self.spot_price_usd)-1)*100:+.2f}%")
        print(f"  Range (95% confidence):")
        print(f"    Lower: ${self.lower_bound_usd:.2f} (¥{self.lower_bound_cny:.2f}/g)")
        print(f"    Upper: ${self.upper_bound_usd:.2f} (¥{self.upper_bound_cny:.2f}/g)")

        # Strategy 1: Bull Call Spread (Primary Strategy)
        print(f"\n" + "="*70)
        print("📈 STRATEGY 1: BULL CALL SPREAD (RECOMMENDED)")
        print("="*70)
        print("Rationale: Moderately bullish with capped risk")

        lower_strike_cny = round(self.spot_price_cny * 0.98, 0)
        upper_strike_cny = round(self.spot_price_cny * 1.03, 0)

        long_call_price = self.black_scholes_call(
            self.spot_price_cny, lower_strike_cny, self.time_to_expiry,
            self.risk_free_rate_cny, self.volatility
        )
        short_call_price = self.black_scholes_call(
            self.spot_price_cny, upper_strike_cny, self.time_to_expiry,
            self.risk_free_rate_cny, self.volatility
        )

        spread_cost = long_call_price - short_call_price
        max_profit = (upper_strike_cny - lower_strike_cny) - spread_cost
        max_loss = spread_cost
        breakeven = lower_strike_cny + spread_cost

        print(f"\nStructure:")
        print(f"  BUY 1x Call @ Strike ¥{lower_strike_cny:.0f} (98% of spot)")
        print(f"    Premium: ¥{long_call_price:.2f}/g")
        print(f"  SELL 1x Call @ Strike ¥{upper_strike_cny:.0f} (103% of spot)")
        print(f"    Premium: ¥{short_call_price:.2f}/g")

        print(f"\nRisk/Reward:")
        print(f"  Net Debit: ¥{spread_cost:.2f}/g")
        print(f"  Max Loss: ¥{max_loss:.2f}/g (if price <= ¥{lower_strike_cny:.0f})")
        print(f"  Max Profit: ¥{max_profit:.2f}/g (if price >= ¥{upper_strike_cny:.0f})")
        print(f"  Breakeven: ¥{breakeven:.2f}/g")
        print(f"  Risk/Reward Ratio: 1:{max_profit/max_loss:.2f}")
        print(f"  Max ROI: {(max_profit/spread_cost)*100:.1f}%")

        # Strategy 2: Long Straddle (High Volatility Play)
        print(f"\n" + "="*70)
        print("🔄 STRATEGY 2: LONG STRADDLE (VOLATILITY PLAY)")
        print("="*70)
        print("Rationale: Benefit from high volatility, direction neutral")

        atm_strike = round(self.spot_price_cny, 0)
        straddle_call = self.black_scholes_call(
            self.spot_price_cny, atm_strike, self.time_to_expiry,
            self.risk_free_rate_cny, self.volatility
        )
        straddle_put = self.black_scholes_put(
            self.spot_price_cny, atm_strike, self.time_to_expiry,
            self.risk_free_rate_cny, self.volatility
        )

        straddle_cost = straddle_call + straddle_put
        upper_breakeven = atm_strike + straddle_cost
        lower_breakeven = atm_strike - straddle_cost

        print(f"\nStructure:")
        print(f"  BUY 1x Call @ Strike ¥{atm_strike:.0f} (ATM)")
        print(f"    Premium: ¥{straddle_call:.2f}/g")
        print(f"  BUY 1x Put @ Strike ¥{atm_strike:.0f} (ATM)")
        print(f"    Premium: ¥{straddle_put:.2f}/g")

        print(f"\nRisk/Reward:")
        print(f"  Total Cost: ¥{straddle_cost:.2f}/g")
        print(f"  Max Loss: ¥{straddle_cost:.2f}/g (if price = ¥{atm_strike:.0f} at expiry)")
        print(f"  Max Profit: Unlimited")
        print(f"  Breakeven Points:")
        print(f"    Upper: ¥{upper_breakeven:.2f}/g (+{(upper_breakeven/self.spot_price_cny-1)*100:.2f}%)")
        print(f"    Lower: ¥{lower_breakeven:.2f}/g ({(lower_breakeven/self.spot_price_cny-1)*100:.2f}%)")

        # Strategy 3: Protective Put (Insurance)
        print(f"\n" + "="*70)
        print("🛡️ STRATEGY 3: PROTECTIVE PUT (HEDGING)")
        print("="*70)
        print("Rationale: Protect existing long gold position from downside")

        otm_put_strike = round(self.spot_price_cny * 0.95, 0)
        protective_put = self.black_scholes_put(
            self.spot_price_cny, otm_put_strike, self.time_to_expiry,
            self.risk_free_rate_cny, self.volatility
        )

        print(f"\nStructure:")
        print(f"  HOLD: Long Gold Position (1000g = 1 SHFE contract)")
        print(f"  BUY 1x Put @ Strike ¥{otm_put_strike:.0f} (95% of spot)")
        print(f"    Premium: ¥{protective_put:.2f}/g")

        print(f"\nRisk/Reward:")
        print(f"  Protection Cost: ¥{protective_put:.2f}/g")
        print(f"  Max Loss: ¥{(self.spot_price_cny - otm_put_strike + protective_put):.2f}/g")
        print(f"  Protection Level: {(otm_put_strike/self.spot_price_cny)*100:.1f}% of current price")
        print(f"  Breakeven: ¥{self.spot_price_cny + protective_put:.2f}/g")

        # Strategy 4: Iron Condor (Range-bound scenario)
        print(f"\n" + "="*70)
        print("🦅 STRATEGY 4: IRON CONDOR (RANGE-BOUND)")
        print("="*70)
        print("Rationale: Profit if gold stays within expected range")

        put_buy_strike = round(self.spot_price_cny * 0.93, 0)
        put_sell_strike = round(self.spot_price_cny * 0.96, 0)
        call_sell_strike = round(self.spot_price_cny * 1.04, 0)
        call_buy_strike = round(self.spot_price_cny * 1.07, 0)

        put_buy = self.black_scholes_put(
            self.spot_price_cny, put_buy_strike, self.time_to_expiry,
            self.risk_free_rate_cny, self.volatility
        )
        put_sell = self.black_scholes_put(
            self.spot_price_cny, put_sell_strike, self.time_to_expiry,
            self.risk_free_rate_cny, self.volatility
        )
        call_sell = self.black_scholes_call(
            self.spot_price_cny, call_sell_strike, self.time_to_expiry,
            self.risk_free_rate_cny, self.volatility
        )
        call_buy = self.black_scholes_call(
            self.spot_price_cny, call_buy_strike, self.time_to_expiry,
            self.risk_free_rate_cny, self.volatility
        )

        net_credit = (put_sell - put_buy) + (call_sell - call_buy)
        wing_width = put_sell_strike - put_buy_strike
        max_loss_ic = wing_width - net_credit

        print(f"\nStructure:")
        print(f"  BUY 1x Put @ ¥{put_buy_strike:.0f}")
        print(f"  SELL 1x Put @ ¥{put_sell_strike:.0f}")
        print(f"  SELL 1x Call @ ¥{call_sell_strike:.0f}")
        print(f"  BUY 1x Call @ ¥{call_buy_strike:.0f}")

        print(f"\nRisk/Reward:")
        print(f"  Net Credit: ¥{net_credit:.2f}/g")
        print(f"  Max Profit: ¥{net_credit:.2f}/g (if price stays between ¥{put_sell_strike:.0f}-¥{call_sell_strike:.0f})")
        print(f"  Max Loss: ¥{max_loss_ic:.2f}/g")
        print(f"  Profit Zone: ¥{put_sell_strike - net_credit:.2f} to ¥{call_sell_strike + net_credit:.2f}")

        # Recommendation Summary
        print(f"\n" + "="*70)
        print("💡 STRATEGY RECOMMENDATIONS")
        print("="*70)

        print("\n🥇 PRIMARY RECOMMENDATION: BULL CALL SPREAD")
        print("   - Best for: Moderately bullish outlook with defined risk")
        print("   - Position Size: 3-5% of portfolio")
        print("   - Entry: Market open, use limit orders")
        print("   - Exit: Hold to expiry or close at 70% max profit")

        print("\n🥈 ALTERNATIVE: LONG STRADDLE")
        print("   - Best for: High volatility environment")
        print("   - Position Size: 2-3% of portfolio")
        print("   - Entry: Before major economic events (Fed, PBOC)")
        print("   - Exit: Close when one leg shows 50%+ profit")

        print("\n⚠️ RISK MANAGEMENT:")
        print("   - Maximum portfolio allocation to gold options: 10%")
        print("   - Stop loss: Close position if loss exceeds 50% of premium paid")
        print("   - Monitor USD/CNY exchange rate for additional risk")
        print("   - Key events to watch: FOMC meeting, China PMI, US CPI")

        return {
            'bull_call_spread': {
                'lower_strike': lower_strike_cny,
                'upper_strike': upper_strike_cny,
                'cost': spread_cost,
                'max_profit': max_profit,
                'max_loss': max_loss,
                'breakeven': breakeven
            },
            'long_straddle': {
                'strike': atm_strike,
                'cost': straddle_cost,
                'upper_breakeven': upper_breakeven,
                'lower_breakeven': lower_breakeven
            }
        }

    def calculate_scenario_analysis(self):
        """Analyze different price scenarios"""
        print(f"\n" + "="*70)
        print("📊 SCENARIO ANALYSIS")
        print("="*70)

        scenarios = [
            ("Bearish", self.spot_price_cny * 0.95),
            ("Slight Decline", self.spot_price_cny * 0.98),
            ("Neutral", self.spot_price_cny),
            ("Slight Rally", self.spot_price_cny * 1.02),
            ("Bullish", self.spot_price_cny * 1.05),
            ("Strong Rally", self.spot_price_cny * 1.08),
        ]

        # Bull Call Spread Analysis
        lower_strike = round(self.spot_price_cny * 0.98, 0)
        upper_strike = round(self.spot_price_cny * 1.03, 0)
        long_call = self.black_scholes_call(
            self.spot_price_cny, lower_strike, self.time_to_expiry,
            self.risk_free_rate_cny, self.volatility
        )
        short_call = self.black_scholes_call(
            self.spot_price_cny, upper_strike, self.time_to_expiry,
            self.risk_free_rate_cny, self.volatility
        )
        spread_cost = long_call - short_call

        print(f"\nBull Call Spread P&L by Scenario:")
        print(f"{'Scenario':<20} {'Price':<15} {'P&L (CNY/g)':<15} {'Return %':<10}")
        print("-"*60)

        for name, price in scenarios:
            # At expiry payoff
            long_payoff = max(price - lower_strike, 0)
            short_payoff = -max(price - upper_strike, 0)
            total_pl = long_payoff + short_payoff - spread_cost
            roi = (total_pl / spread_cost) * 100

            print(f"{name:<20} ¥{price:.2f}    ¥{total_pl:+.2f}       {roi:+.1f}%")

        # Key Economic Events Impact
        print(f"\n" + "="*70)
        print("📅 KEY EVENTS & MARKET IMPACTS")
        print("="*70)

        events = [
            ("FOMC Meeting (Nov 2025)", "Rate cut expected - Bullish for gold"),
            ("China PMI Data", "Manufacturing health indicator"),
            ("US CPI Release", "Inflation data - Higher = Bullish"),
            ("US Employment Data", "Weak jobs = Fed dovish = Bullish"),
            ("China PBOC Policy", "Rate changes affect CNY/Gold"),
            ("Geopolitical Tensions", "Safe haven flows support gold"),
        ]

        for event, impact in events:
            print(f"  • {event}")
            print(f"    Impact: {impact}")
            print()

def main():
    print("="*70)
    print("GOLD OPTIONS TRADING STRATEGY BUILDER")
    print("Shanghai Futures Exchange (SHFE) - November 2025")
    print("="*70)

    strategy = GoldOptionsStrategy()

    # Generate strategy report
    results = strategy.generate_strategy_report()

    # Scenario analysis
    strategy.calculate_scenario_analysis()

    # Save strategy to file
    print(f"\n" + "="*70)
    print("💾 SAVING STRATEGY REPORT")
    print("="*70)

    with open('gold_options_strategy.json', 'w') as f:
        json.dump(results, f, indent=2, default=float)

    print("Strategy saved to gold_options_strategy.json")

    return results

if __name__ == "__main__":
    main()
