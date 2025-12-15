@echo off
REM Dividend Rotation Strategy Runner with Real EODHD Data
REM ========================================================

echo Setting up environment...
set EODHD_API_TOKEN=690d7cdc3013f4.57364117

echo.
echo Running V4 Dividend Rotation Strategy...
echo ========================================
echo.
echo Parameters:
echo - Start Date: 2023-11-01
echo - End Date: Today minus 1 day
echo - Initial Cash: $200,000
echo - Exchange: US
echo - Min Dividend Yield: 0.9%%
echo - Top K: 10 candidates
echo - Hold Pre: 2 days before ex-date
echo - Hold Post: 1 day after ex-date
echo - Ex-Dividend Lookahead: 90 days
echo.

python dividend_rotation_v4_real_cli_plan.py ^
  --start 2023-11-01 ^
  --initial-cash 200000 ^
  --exchange US ^
  --min-div-yield 0.009 ^
  --min-avg-vol 200000 ^
  --topk 10 ^
  --hold-pre 2 ^
  --hold-post 1 ^
  --ex-lookahead 90 ^
  --emit-xlsx ^
  --emit-pdf ^
  --emit-png

echo.
echo ========================================
echo Execution complete!
echo.
echo Generated files:
echo - Dividend_Rotation_Buy_Sell_Plan.xlsx
echo - Dividend_Rotation_Backtest_Report.pdf
echo - Dividend_Rotation_Performance_Chart.png
echo - Dividend_Rotation_Forward_Plan.csv
echo.
pause
