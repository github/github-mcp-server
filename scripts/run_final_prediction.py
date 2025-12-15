#!/usr/bin/env python3
"""
FINAL DATA-DRIVEN PREDICTION
Combines real-time fundamentals with Bayesian Model Averaging
"""

import json
import subprocess
import sys

print("="*80)
print("FINAL DATA-DRIVEN PRICE PREDICTION")
print("Integrating Real-Time Fundamentals + Bayesian Model Averaging")
print("="*80)

# Load real-time market data
with open('real_time_market_data.json', 'r') as f:
    market_data = json.load(f)

print("\nREAL-TIME APPLE FUNDAMENTALS:")
apple_funds = market_data['data_fetched']['apple_fundamentals']
print(f"  P/E Ratio: {apple_funds['PE_Ratio']:.2f}")
print(f"  EPS: ${apple_funds['EPS']}")
print(f"  Analyst Rating: {apple_funds['Analyst_Rating']:.2f}/5")
print(f"  Target Price: ${apple_funds['Target_Price']:.2f}")
print(f"  Market Cap: ${apple_funds['Market_Cap']/1e12:.2f}T")

# Calculate upside to analyst target
current_apple_price = 276.97  # Will be updated by model
target_upside = ((apple_funds['Target_Price'] - current_apple_price) / current_apple_price) * 100
print(f"  Upside to Target: {target_upside:+.1f}%")

# Load ultimate predictions
with open('ultimate_predictions.json', 'r') as f:
    predictions = json.load(f)

print("\n" + "="*80)
print("FINAL PREDICTIONS (Bayesian Model Averaging)")
print("="*80)

for asset_name, result in predictions['results'].items():
    print(f"\n{asset_name}:")
    print(f"  Current Price: ${result['current_price']:.2f}")
    print(f"  5-Day Prediction: ${result['predictions'][4]:.2f}")
    print(f"  Expected Return: {result['expected_return']:+.2f}%")
    print(f"  95% Credible Interval: [${result['lower_95'][4]:.2f}, ${result['upper_95'][4]:.2f}]")
    print(f"  Probability of Loss: {result['prob_loss']:.1f}%")
    print(f"  Trading Signal: {result['signal']}")

    if asset_name == "Apple Inc. (AAPL)":
        # Adjust signal based on analyst target
        if target_upside > 2.0 and result['prob_loss'] < 55:
            print(f"\n  FUNDAMENTAL ADJUSTMENT:")
            print(f"    Analyst target suggests {target_upside:+.1f}% upside")
            print(f"    Combined with {100-result['prob_loss']:.1f}% win probability")
            print(f"    UPGRADED SIGNAL: BUY (from {result['signal']})")

print("\n" + "="*80)
print("RECOMMENDATION SUMMARY")
print("="*80)

gold_result = predictions['results']['Gold (XAU/USD)']
apple_result = predictions['results']['Apple Inc. (AAPL)']

print("\nGold (XAU/USD):")
if gold_result['prob_loss'] > 55:
    print("  Action: AVOID/SHORT")
    print(f"  Reason: {gold_result['prob_loss']:.1f}% loss probability")
elif gold_result['prob_loss'] < 45:
    print("  Action: BUY")
    print(f"  Reason: {100-gold_result['prob_loss']:.1f}% win probability")
else:
    print("  Action: HOLD/WAIT")
    print(f"  Reason: {gold_result['prob_loss']:.1f}% loss probability (neutral)")

print("\nApple (AAPL):")
apple_win_prob = 100 - apple_result['prob_loss']
if target_upside > 2.0:
    if apple_win_prob > 48:
        print("  Action: BUY")
        print(f"  Reason: {apple_win_prob:.1f}% model win probability + {target_upside:+.1f}% analyst upside")
        print(f"  Target: ${apple_funds['Target_Price']:.2f}")
    else:
        print("  Action: HOLD")
        print(f"  Reason: Fundamentals bullish but model uncertain ({apple_win_prob:.1f}% win)")
else:
    if apple_win_prob > 55:
        print("  Action: BUY")
        print(f"  Reason: {apple_win_prob:.1f}% win probability")
    else:
        print("  Action: HOLD")
        print(f"  Reason: {apple_win_prob:.1f}% win probability (neutral)")

print("\n" + "="*80)
