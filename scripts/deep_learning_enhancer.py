#!/usr/bin/env python3
"""
Deep Learning Enhancement Module for Gold Price Prediction
Uses LSTM neural network to enhance ARIMA predictions with pattern recognition
"""

import numpy as np
import pandas as pd
import json
import os
from datetime import datetime, timedelta
from typing import Dict, List, Tuple, Optional
import logging

# TensorFlow/Keras imports (with fallback if not installed)
try:
    import tensorflow as tf
    from tensorflow import keras
    from tensorflow.keras import layers
    from tensorflow.keras.models import Sequential, load_model
    from tensorflow.keras.layers import LSTM, Dense, Dropout
    from tensorflow.keras.callbacks import EarlyStopping, ModelCheckpoint
    from sklearn.preprocessing import MinMaxScaler
    DEEP_LEARNING_AVAILABLE = True
except ImportError:
    DEEP_LEARNING_AVAILABLE = False
    logger = logging.getLogger(__name__)
    logger.warning("TensorFlow not installed. Deep learning features disabled.")
    logger.warning("Install with: pip install tensorflow scikit-learn")

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


class GoldPriceLSTM:
    """
    LSTM-based deep learning model for gold price prediction
    Enhances ARIMA predictions with pattern recognition
    """

    def __init__(self, model_dir: str = None, sequence_length: int = 30):
        """
        Initialize LSTM model

        Args:
            model_dir: Directory to save/load models
            sequence_length: Number of historical days to use for prediction
        """
        if not DEEP_LEARNING_AVAILABLE:
            raise RuntimeError("TensorFlow not installed. Cannot use deep learning features.")

        self.model_dir = model_dir or os.path.dirname(__file__)
        self.sequence_length = sequence_length
        self.model_path = os.path.join(self.model_dir, 'gold_lstm_model.h5')
        self.scaler_path = os.path.join(self.model_dir, 'gold_scaler.json')

        self.model = None
        self.scaler = MinMaxScaler(feature_range=(0, 1))
        self.feature_names = ['price', 'dxy', 'vix', 'treasury', 'volume']

        # Try to load existing model
        self.load_model()

    def create_model(self, n_features: int = 5) -> Sequential:
        """
        Create LSTM neural network architecture

        Args:
            n_features: Number of input features

        Returns:
            Compiled Keras Sequential model
        """
        model = Sequential([
            # First LSTM layer with dropout
            LSTM(units=50, return_sequences=True, input_shape=(self.sequence_length, n_features)),
            Dropout(0.2),

            # Second LSTM layer
            LSTM(units=50, return_sequences=True),
            Dropout(0.2),

            # Third LSTM layer
            LSTM(units=50),
            Dropout(0.2),

            # Output layer
            Dense(units=1)  # Predicting single value (price)
        ])

        # Compile model
        model.compile(
            optimizer='adam',
            loss='mean_squared_error',
            metrics=['mae', 'mse']
        )

        logger.info("Created LSTM model with 3 layers, 50 units each")
        return model

    def prepare_sequences(self, data: pd.DataFrame, feature_cols: List[str]) -> Tuple[np.ndarray, np.ndarray]:
        """
        Prepare time series sequences for LSTM training

        Args:
            data: DataFrame with price and feature data
            feature_cols: List of column names to use as features

        Returns:
            Tuple of (X, y) arrays for training
        """
        # Scale the data
        scaled_data = self.scaler.fit_transform(data[feature_cols].values)

        X, y = [], []

        # Create sequences
        for i in range(self.sequence_length, len(scaled_data)):
            X.append(scaled_data[i-self.sequence_length:i])
            y.append(scaled_data[i, 0])  # First column is price

        return np.array(X), np.array(y)

    def train(self, historical_data: pd.DataFrame, epochs: int = 50, batch_size: int = 32,
              validation_split: float = 0.2) -> Dict:
        """
        Train LSTM model on historical gold price data

        Args:
            historical_data: DataFrame with columns: date, price, dxy, vix, treasury, volume
            epochs: Number of training epochs
            batch_size: Training batch size
            validation_split: Fraction of data to use for validation

        Returns:
            Dict with training history and metrics
        """
        logger.info(f"Training LSTM model on {len(historical_data)} data points...")

        # Prepare sequences
        feature_cols = [col for col in self.feature_names if col in historical_data.columns]
        X, y = self.prepare_sequences(historical_data, feature_cols)

        logger.info(f"Created {len(X)} sequences of length {self.sequence_length}")

        # Create model if not exists
        if self.model is None:
            self.model = self.create_model(n_features=len(feature_cols))

        # Setup callbacks
        callbacks = [
            EarlyStopping(monitor='val_loss', patience=10, restore_best_weights=True),
            ModelCheckpoint(self.model_path, save_best_only=True, monitor='val_loss')
        ]

        # Train model
        history = self.model.fit(
            X, y,
            epochs=epochs,
            batch_size=batch_size,
            validation_split=validation_split,
            callbacks=callbacks,
            verbose=1
        )

        # Save scaler parameters
        scaler_params = {
            'min': self.scaler.min_.tolist(),
            'scale': self.scaler.scale_.tolist(),
            'data_min': self.scaler.data_min_.tolist(),
            'data_max': self.scaler.data_max_.tolist(),
            'feature_names': feature_cols
        }

        with open(self.scaler_path, 'w') as f:
            json.dump(scaler_params, f, indent=2)

        logger.info(f"Model saved to: {self.model_path}")
        logger.info(f"Scaler saved to: {self.scaler_path}")

        # Return training metrics
        return {
            'final_loss': float(history.history['loss'][-1]),
            'final_val_loss': float(history.history['val_loss'][-1]),
            'final_mae': float(history.history['mae'][-1]),
            'epochs_trained': len(history.history['loss']),
            'training_samples': len(X)
        }

    def predict(self, recent_data: pd.DataFrame, n_days: int = 1) -> np.ndarray:
        """
        Predict future gold prices

        Args:
            recent_data: DataFrame with recent price and feature data (last N days)
            n_days: Number of days to predict ahead

        Returns:
            Array of predicted prices
        """
        if self.model is None:
            raise RuntimeError("Model not trained or loaded")

        # Use last sequence_length days
        if len(recent_data) < self.sequence_length:
            raise ValueError(f"Need at least {self.sequence_length} days of data")

        feature_cols = [col for col in self.feature_names if col in recent_data.columns]

        # Scale recent data
        scaled_recent = self.scaler.transform(recent_data[feature_cols].tail(self.sequence_length).values)

        predictions = []

        for _ in range(n_days):
            # Prepare input sequence
            X_input = scaled_recent[-self.sequence_length:].reshape(1, self.sequence_length, len(feature_cols))

            # Predict next value
            scaled_pred = self.model.predict(X_input, verbose=0)

            # Inverse transform to get actual price
            # Create full feature array with prediction as first column
            full_pred = np.zeros((1, len(feature_cols)))
            full_pred[0, 0] = scaled_pred[0, 0]

            # Inverse transform
            actual_pred = self.scaler.inverse_transform(full_pred)[0, 0]
            predictions.append(actual_pred)

            # Update sequence for next prediction (append prediction)
            new_row = np.array([[scaled_pred[0, 0]] + [0] * (len(feature_cols) - 1)])
            scaled_recent = np.vstack([scaled_recent[1:], new_row])

        return np.array(predictions)

    def load_model(self) -> bool:
        """Load saved model and scaler"""
        if os.path.exists(self.model_path) and os.path.exists(self.scaler_path):
            try:
                self.model = load_model(self.model_path)

                with open(self.scaler_path, 'r') as f:
                    scaler_params = json.load(f)

                self.scaler.min_ = np.array(scaler_params['min'])
                self.scaler.scale_ = np.array(scaler_params['scale'])
                self.scaler.data_min_ = np.array(scaler_params['data_min'])
                self.scaler.data_max_ = np.array(scaler_params['data_max'])
                self.feature_names = scaler_params.get('feature_names', self.feature_names)

                logger.info(f"Loaded model from: {self.model_path}")
                return True

            except Exception as e:
                logger.warning(f"Error loading model: {e}")
                return False

        return False

    def evaluate(self, test_data: pd.DataFrame) -> Dict:
        """
        Evaluate model on test data

        Args:
            test_data: DataFrame with price and feature data

        Returns:
            Dict with evaluation metrics
        """
        if self.model is None:
            raise RuntimeError("Model not trained or loaded")

        feature_cols = [col for col in self.feature_names if col in test_data.columns]
        X_test, y_test = self.prepare_sequences(test_data, feature_cols)

        # Evaluate
        loss, mae, mse = self.model.evaluate(X_test, y_test, verbose=0)

        # Calculate additional metrics
        predictions = self.model.predict(X_test, verbose=0)

        # Inverse transform for actual comparison
        rmse = np.sqrt(mse)

        return {
            'loss': float(loss),
            'mae': float(mae),
            'mse': float(mse),
            'rmse': float(rmse),
            'test_samples': len(X_test)
        }


class EnsemblePredictor:
    """
    Combines ARIMA and LSTM predictions for improved accuracy
    """

    def __init__(self, lstm_model: GoldPriceLSTM, arima_weight: float = 0.6, lstm_weight: float = 0.4):
        """
        Initialize ensemble predictor

        Args:
            lstm_model: Trained LSTM model
            arima_weight: Weight for ARIMA predictions (0-1)
            lstm_weight: Weight for LSTM predictions (0-1)
        """
        self.lstm_model = lstm_model
        self.arima_weight = arima_weight
        self.lstm_weight = lstm_weight

        # Normalize weights
        total = arima_weight + lstm_weight
        self.arima_weight /= total
        self.lstm_weight /= total

    def predict(self, arima_prediction: float, recent_data: pd.DataFrame) -> Dict:
        """
        Combine ARIMA and LSTM predictions

        Args:
            arima_prediction: Price prediction from ARIMA model
            recent_data: Recent historical data for LSTM

        Returns:
            Dict with ensemble prediction and confidence metrics
        """
        # Get LSTM prediction
        try:
            lstm_predictions = self.lstm_model.predict(recent_data, n_days=1)
            lstm_prediction = lstm_predictions[0]
        except Exception as e:
            logger.warning(f"LSTM prediction failed: {e}, using ARIMA only")
            return {
                'ensemble_prediction': arima_prediction,
                'arima_prediction': arima_prediction,
                'lstm_prediction': None,
                'confidence': 'MEDIUM',
                'method': 'ARIMA_ONLY'
            }

        # Calculate ensemble prediction
        ensemble_prediction = (
            self.arima_weight * arima_prediction +
            self.lstm_weight * lstm_prediction
        )

        # Calculate agreement (how close are the two models?)
        agreement = 1 - abs(arima_prediction - lstm_prediction) / max(arima_prediction, lstm_prediction)

        # Determine confidence based on agreement
        if agreement > 0.95:
            confidence = 'HIGH'
        elif agreement > 0.90:
            confidence = 'MEDIUM'
        else:
            confidence = 'LOW'

        return {
            'ensemble_prediction': ensemble_prediction,
            'arima_prediction': arima_prediction,
            'lstm_prediction': lstm_prediction,
            'agreement': agreement,
            'confidence': confidence,
            'method': 'ENSEMBLE',
            'weights': {
                'arima': self.arima_weight,
                'lstm': self.lstm_weight
            }
        }


def create_sample_training_data(n_days: int = 365) -> pd.DataFrame:
    """
    Create sample historical data for testing
    In production, this would load actual historical gold prices
    """
    dates = pd.date_range(end=datetime.now(), periods=n_days, freq='D')

    # Simulate gold price with trend and noise
    base_price = 950
    trend = np.linspace(0, 50, n_days)
    noise = np.random.normal(0, 10, n_days)
    price = base_price + trend + noise

    # Simulate correlated features
    dxy = 100 - (price - base_price) * 0.02 + np.random.normal(0, 0.5, n_days)
    vix = 15 + np.random.normal(0, 2, n_days)
    treasury = 4.0 + np.random.normal(0, 0.2, n_days)
    volume = np.random.randint(10000, 50000, n_days)

    df = pd.DataFrame({
        'date': dates,
        'price': price,
        'dxy': dxy,
        'vix': vix,
        'treasury': treasury,
        'volume': volume
    })

    return df


def main():
    """Test the deep learning module"""

    if not DEEP_LEARNING_AVAILABLE:
        print("❌ TensorFlow not installed. Please install with:")
        print("   pip install tensorflow scikit-learn pandas numpy")
        return

    print("=" * 60)
    print("Deep Learning Enhancement Module - Test")
    print("=" * 60)

    # Create sample data
    print("\n[1/4] Creating sample training data...")
    data = create_sample_training_data(n_days=365)
    print(f"Generated {len(data)} days of data")

    # Split into train/test
    split_idx = int(len(data) * 0.8)
    train_data = data[:split_idx]
    test_data = data[split_idx:]

    # Initialize and train LSTM
    print("\n[2/4] Training LSTM model...")
    lstm = GoldPriceLSTM(sequence_length=30)

    metrics = lstm.train(train_data, epochs=20, batch_size=16)
    print(f"Training complete: Loss={metrics['final_loss']:.4f}, MAE={metrics['final_mae']:.4f}")

    # Evaluate on test set
    print("\n[3/4] Evaluating model...")
    eval_metrics = lstm.evaluate(test_data)
    print(f"Test RMSE: {eval_metrics['rmse']:.2f}")
    print(f"Test MAE: {eval_metrics['mae']:.2f}")

    # Make prediction
    print("\n[4/4] Making ensemble prediction...")
    recent_data = data.tail(30)
    current_price = recent_data['price'].iloc[-1]

    # Simulate ARIMA prediction (in production, this comes from production_prediction_engine.py)
    arima_prediction = current_price * 1.005  # +0.5%

    # Create ensemble predictor
    ensemble = EnsemblePredictor(lstm, arima_weight=0.6, lstm_weight=0.4)
    result = ensemble.predict(arima_prediction, recent_data)

    print(f"\nCurrent Price: CNY {current_price:.2f}")
    print(f"ARIMA Prediction: CNY {result['arima_prediction']:.2f} ({((result['arima_prediction']/current_price-1)*100):+.2f}%)")
    print(f"LSTM Prediction: CNY {result['lstm_prediction']:.2f} ({((result['lstm_prediction']/current_price-1)*100):+.2f}%)")
    print(f"Ensemble Prediction: CNY {result['ensemble_prediction']:.2f} ({((result['ensemble_prediction']/current_price-1)*100):+.2f}%)")
    print(f"Agreement: {result['agreement']*100:.1f}%")
    print(f"Confidence: {result['confidence']}")

    print("\n" + "=" * 60)
    print("✅ Deep learning module test complete!")
    print("=" * 60)


if __name__ == '__main__':
    main()
