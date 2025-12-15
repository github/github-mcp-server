@echo off
REM Shanghai Gold Auto-Pilot System - Setup Script for Windows
REM Version: 1.0

echo ============================================================
echo Shanghai Gold Auto-Pilot System - Setup
echo ============================================================
echo.

REM Check Python installation
python --version >nul 2>&1
if errorlevel 1 (
    echo ERROR: Python not found!
    echo Please install Python 3.8 or higher from python.org
    pause
    exit /b 1
)

echo [1/4] Python found:
python --version
echo.

REM Install required packages
echo [2/4] Installing required packages...
pip install -r requirements_autopilot.txt
if errorlevel 1 (
    echo ERROR: Failed to install packages
    pause
    exit /b 1
)
echo.

REM Check for EODHD API key
echo [3/4] Checking for EODHD API key...
if "%EODHD_API_KEY%"=="" (
    echo WARNING: EODHD_API_KEY environment variable not set
    echo.
    echo Please set your API key:
    echo   set EODHD_API_KEY=your_api_key_here
    echo.
    echo Or add to System Environment Variables for persistence
) else (
    echo API key found: %EODHD_API_KEY:~0,10%...
)
echo.

REM Test the system
echo [4/4] Running test...
python auto_pilot_scheduler.py --test
if errorlevel 1 (
    echo ERROR: Test failed
    echo Please check the error messages above
    pause
    exit /b 1
)
echo.

echo ============================================================
echo Setup complete!
echo ============================================================
echo.
echo To start auto-pilot mode:
echo   python auto_pilot_scheduler.py
echo.
echo To run manual daily update:
echo   python daily_strategy_engine.py
echo.
echo For help:
echo   python auto_pilot_scheduler.py --help
echo.
pause
