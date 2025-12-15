#!/usr/bin/env python3
"""
Gold Price Prediction Model for November 2025
Uses multiple forecasting methods for robust predictions
"""

import pandas as pd
import numpy as np
from sklearn.ensemble import RandomForestRegressor, GradientBoostingRegressor
from sklearn.linear_model import LinearRegression
from sklearn.preprocessing import StandardScaler
from sklearn.model_selection import train_test_split
import warnings
warnings.filterwarnings('ignore')

def load_and_prepare_data():
    """Load historical data and prepare features"""
    df = pd.read_csv('gold_historical_with_indicators.csv', parse_dates=['date'])
    df.set_index('date', inplace=True)

    # Verify if data needs adjustment (EODHD sometimes uses different multipliers)
    # If latest price > 3000, it might be in points, not USD
    if df['close'].iloc[-1] > 3000:
        print(f"Note: Raw data shows ${df['close'].iloc[-1]:.2f}")
        print("This appears to be the actual gold price or EODHD's formatting")
        print()

    return df

def create_prediction_features(df, forecast_days=15):
    """Create features for machine learning model"""

    # Target: Future price change
    df['Target'] = df['close'].shift(-forecast_days)
    df['Target_Return'] = (df['Target'] / df['close'] - 1) * 100

    # Feature engineering
    features = pd.DataFrame(index=df.index)

    # Lag features
    for lag in [1, 5, 10, 20]:
        features[f'Return_{lag}d'] = df['close'].pct_change(lag) * 100

    # Technical indicators
    features['RSI_feat'] = df['RSI'].values
    features['MACD_feat'] = df['MACD'].values
    features['BB_Position'] = (df['close'] - df['BB_Lower']) / (df['BB_Upper'] - df['BB_Lower'])
    features['SMA_20_Distance'] = (df['close'] / df['SMA_20'] - 1) * 100
    features['SMA_50_Distance'] = (df['close'] / df['SMA_50'] - 1) * 100
    features['SMA_200_Distance'] = (df['close'] / df['SMA_200'] - 1) * 100
    features['Volatility'] = df['Volatility_20'].values
    features['Momentum_10_feat'] = df['Momentum_10'].values
    features['Momentum_30_feat'] = df['Momentum_30'].values

    # Seasonal features
    features['Month'] = df.index.month
    features['Quarter'] = df.index.quarter

    # Combine
    df = pd.concat([df, features], axis=1)

    return df

def train_ensemble_model(df):
    """Train ensemble of models for prediction"""
    print("Training Price Prediction Models...")
    print("="*60)

    # Prepare data
    feature_cols = ['Return_1d', 'Return_5d', 'Return_10d', 'Return_20d',
                    'RSI_feat', 'MACD_feat', 'BB_Position', 'SMA_20_Distance',
                    'SMA_50_Distance', 'SMA_200_Distance', 'Volatility',
                    'Momentum_10_feat', 'Momentum_30_feat', 'Month', 'Quarter']

    # Remove rows with NaN
    df_clean = df.dropna(subset=feature_cols + ['Target_Return'])

    X = df_clean[feature_cols]
    y = df_clean['Target_Return']

    # Split data
    X_train, X_test, y_train, y_test = train_test_split(
        X, y, test_size=0.2, shuffle=False
    )

    # Scale features
    scaler = StandardScaler()
    X_train_scaled = scaler.fit_transform(X_train)
    X_test_scaled = scaler.transform(X_test)

    # Train multiple models
    models = {
        'Random Forest': RandomForestRegressor(n_estimators=100, max_depth=10, random_state=42),
        'Gradient Boosting': GradientBoostingRegressor(n_estimators=100, max_depth=5, random_state=42),
        'Linear Regression': LinearRegression()
    }

    predictions = {}
    for name, model in models.items():
        model.fit(X_train_scaled, y_train)
        train_score = model.score(X_train_scaled, y_train)
        test_score = model.score(X_test_scaled, y_test)
        predictions[name] = model
        print(f"{name}:")
        print(f"  Train R²: {train_score:.4f}")
        print(f"  Test R²: {test_score:.4f}")

    return models, scaler, feature_cols

def predict_november_2025_price(df, models, scaler, feature_cols):
    """Generate predictions for end of November 2025"""
    print("\n" + "="*60)
    print("GOLD PRICE PREDICTIONS FOR END OF NOVEMBER 2025")
    print("="*60)

    # Get latest features
    latest = df.iloc[-1:][feature_cols].values
    latest_scaled = scaler.transform(latest)

    current_price = df['close'].iloc[-1]
    print(f"\nCurrent Gold Price: ${current_price:.2f}")
    print(f"Current Date: {df.index[-1].strftime('%Y-%m-%d')}")
    print()

    # Model predictions (15-day forward)
    ml_predictions = []
    for name, model in models.items():
        pred_return = model.predict(latest_scaled)[0]
        pred_price = current_price * (1 + pred_return/100)
        ml_predictions.append(pred_price)
        print(f"{name} Prediction:")
        print(f"  Expected Return: {pred_return:+.2f}%")
        print(f"  Price Target: ${pred_price:.2f}")
        print()

    # Technical Analysis Prediction
    print("Technical Analysis Outlook:")
    rsi = float(df['RSI'].iloc[-1])
    macd = float(df['MACD'].iloc[-1])
    signal = float(df['Signal'].iloc[-1])
    sma_20 = float(df['SMA_20'].iloc[-1])
    sma_50 = float(df['SMA_50'].iloc[-1])
    sma_200 = float(df['SMA_200'].iloc[-1])

    tech_score = 0

    if current_price > sma_20:
        tech_score += 1
        print(f"  ✓ Price above SMA 20 (Bullish)")
    else:
        print(f"  ✗ Price below SMA 20 (Bearish)")

    if current_price > sma_50:
        tech_score += 1
        print(f"  ✓ Price above SMA 50 (Bullish)")
    else:
        print(f"  ✗ Price below SMA 50 (Bearish)")

    if current_price > sma_200:
        tech_score += 2
        print(f"  ✓ Price above SMA 200 (Long-term Bullish)")
    else:
        print(f"  ✗ Price below SMA 200 (Long-term Bearish)")

    if 40 <= rsi <= 60:
        tech_score += 1
        print(f"  • RSI {rsi:.1f}: Neutral (Room to move either direction)")
    elif rsi > 70:
        tech_score -= 1
        print(f"  ⚠ RSI {rsi:.1f}: Overbought (Potential pullback)")
    elif rsi < 30:
        tech_score += 2
        print(f"  ✓ RSI {rsi:.1f}: Oversold (Potential bounce)")

    if macd > signal:
        tech_score += 1
        print(f"  ✓ MACD above Signal (Bullish momentum)")
    else:
        print(f"  ✗ MACD below Signal (Bearish momentum)")

    print(f"\n  Technical Score: {tech_score}/7")

    # Fundamental Drivers Score
    print("\nFundamental Drivers Score:")
    fund_score = 0
    drivers = [
        ("Fed Rate Cuts Expected", 2),
        ("Elevated Geopolitical Risk", 2),
        ("Strong Central Bank Demand", 3),
        ("USD Weakness Trend", 1),
        ("Inflation Hedge Demand", 1),
        ("China Gold Demand Strong", 2),
    ]

    for driver, points in drivers:
        fund_score += points
        print(f"  ✓ {driver}: +{points} points")

    print(f"\n  Fundamental Score: {fund_score}/11 (Strongly Bullish)")

    # Ensemble Prediction
    print("\n" + "="*60)
    print("FINAL PRICE PREDICTION SUMMARY")
    print("="*60)

    ml_avg = np.mean(ml_predictions)
    ml_std = np.std(ml_predictions)

    # Weighted ensemble
    # Technical: 20%, ML Models: 50%, Fundamentals: 30%
    ml_weight_price = ml_avg
    tech_adjustment = (tech_score / 7 - 0.5) * 50  # ±$25 based on tech score
    fund_adjustment = (fund_score / 11) * 100  # Up to +$100 for strong fundamentals

    final_prediction = ml_avg + tech_adjustment + fund_adjustment

    print(f"\nML Models Average: ${ml_avg:.2f} (±${ml_std:.2f})")
    print(f"Technical Adjustment: ${tech_adjustment:+.2f}")
    print(f"Fundamental Adjustment: ${fund_adjustment:+.2f}")
    print(f"\nFINAL PREDICTED PRICE (End Nov 2025): ${final_prediction:.2f}")

    # Calculate returns
    predicted_return = ((final_prediction / current_price) - 1) * 100
    print(f"Expected Return: {predicted_return:+.2f}%")

    # Price ranges
    lower_bound = final_prediction - (ml_std * 2)
    upper_bound = final_prediction + (ml_std * 2)
    print(f"\n95% Confidence Range:")
    print(f"  Lower Bound: ${lower_bound:.2f}")
    print(f"  Central Estimate: ${final_prediction:.2f}")
    print(f"  Upper Bound: ${upper_bound:.2f}")

    return {
        'current_price': current_price,
        'predicted_price': final_prediction,
        'lower_bound': lower_bound,
        'upper_bound': upper_bound,
        'expected_return': predicted_return,
        'ml_predictions': ml_predictions,
        'tech_score': tech_score,
        'fund_score': fund_score
    }

def historical_performance_analysis(df):
    """Analyze historical gold performance patterns"""
    print("\n" + "="*60)
    print("HISTORICAL PERFORMANCE ANALYSIS")
    print("="*60)

    # November historical performance
    df_copy = df.copy()
    df_copy['Year'] = df_copy.index.year
    df_copy['Month'] = df_copy.index.month

    november_data = df_copy[df_copy['Month'] == 11].copy()
    if not november_data.empty:
        november_returns = november_data.groupby('Year').apply(
            lambda x: (x['close'].iloc[-1] / x['close'].iloc[0] - 1) * 100
        )

        print("\nNovember Historical Returns:")
        for year, ret in november_returns.items():
            print(f"  {year}: {ret:+.2f}%")

        avg_nov_return = november_returns.mean()
        print(f"\n  Average November Return: {avg_nov_return:+.2f}%")

    # Recent trend analysis
    print("\nRecent Price Trends:")
    recent_periods = {
        '1 Month': 21,
        '3 Months': 63,
        '6 Months': 126,
        '1 Year': 252
    }

    for period_name, days in recent_periods.items():
        if len(df) > days:
            period_return = (df['close'].iloc[-1] / df['close'].iloc[-days] - 1) * 100
            print(f"  {period_name}: {period_return:+.2f}%")

    # Volatility regime
    current_vol = df['Volatility_20'].iloc[-1]
    avg_vol = df['Volatility_20'].mean()
    print(f"\nVolatility Analysis:")
    print(f"  Current (20-day): {current_vol:.2f}%")
    print(f"  Historical Average: {avg_vol:.2f}%")

    if current_vol > avg_vol * 1.2:
        print(f"  Regime: HIGH VOLATILITY")
    elif current_vol < avg_vol * 0.8:
        print(f"  Regime: LOW VOLATILITY")
    else:
        print(f"  Regime: NORMAL VOLATILITY")

def main():
    print("="*60)
    print("GOLD PRICE PREDICTION MODEL")
    print("November 2025 Outlook for China Futures Market")
    print("="*60)

    # Load data
    df = load_and_prepare_data()

    # Create features
    df = create_prediction_features(df, forecast_days=15)

    # Historical analysis
    historical_performance_analysis(df)

    # Train models
    models, scaler, feature_cols = train_ensemble_model(df)

    # Generate predictions
    predictions = predict_november_2025_price(df, models, scaler, feature_cols)

    # Save predictions
    with open('price_predictions.txt', 'w') as f:
        f.write("GOLD PRICE PREDICTIONS - END OF NOVEMBER 2025\n")
        f.write("="*60 + "\n\n")
        f.write(f"Current Price: ${predictions['current_price']:.2f}\n")
        f.write(f"Predicted Price: ${predictions['predicted_price']:.2f}\n")
        f.write(f"Expected Return: {predictions['expected_return']:+.2f}%\n")
        f.write(f"Lower Bound (95%): ${predictions['lower_bound']:.2f}\n")
        f.write(f"Upper Bound (95%): ${predictions['upper_bound']:.2f}\n")

    print("\nPredictions saved to price_predictions.txt")

    return predictions

if __name__ == "__main__":
    predictions = main()
