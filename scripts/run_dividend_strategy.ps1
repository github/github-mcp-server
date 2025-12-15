# Dividend Rotation Strategy Runner with Real EODHD Data
# ========================================================

Write-Host "Setting up environment..." -ForegroundColor Cyan
$env:EODHD_API_TOKEN = "690d7cdc3013f4.57364117"

Write-Host "`nRunning V4 Dividend Rotation Strategy..." -ForegroundColor Green
Write-Host "========================================"

Write-Host "`nParameters:" -ForegroundColor Yellow
Write-Host "- Start Date: 2023-11-01"
Write-Host "- End Date: Today minus 1 day"
Write-Host "- Initial Cash: `$200,000"
Write-Host "- Exchange: US"
Write-Host "- Min Dividend Yield: 0.9%"
Write-Host "- Top K: 10 candidates"
Write-Host "- Hold Pre: 2 days before ex-date"
Write-Host "- Hold Post: 1 day after ex-date"
Write-Host "- Ex-Dividend Lookahead: 90 days"
Write-Host ""

python dividend_rotation_v4_real_cli_plan.py `
  --start 2023-11-01 `
  --initial-cash 200000 `
  --exchange US `
  --min-div-yield 0.009 `
  --min-avg-vol 200000 `
  --topk 10 `
  --hold-pre 2 `
  --hold-post 1 `
  --ex-lookahead 7

Write-Host "`n========================================"
Write-Host "Execution complete!" -ForegroundColor Green
Write-Host "`nGenerated files:" -ForegroundColor Cyan
Write-Host "- Dividend_Rotation_Buy_Sell_Plan.xlsx"
Write-Host "- Dividend_Rotation_Backtest_Report.pdf"
Write-Host "- Dividend_Rotation_Performance_Chart.png"
Write-Host "- Dividend_Rotation_Forward_Plan.csv"
