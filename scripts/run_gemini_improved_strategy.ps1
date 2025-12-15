# Set the EODHD API Token for this script execution
$env:EODHD_API_TOKEN = "690d7cdc3013f4.57364117"

# Execute the backtesting script with Gemini's improved parameters
python .\dividend_rotation_v4_real_cli_plan.py `
  --start 2023-11-01 `
  --end 2025-11-11 `
  --initial-cash 200000 `
  --min-div-yield 0.015 `
  --min-avg-vol 300000 `
  --topk 8 `
  --hold-pre 2 `
  --hold-post 2 `
  --wY 0.5 `
  --wL 0.3 `
  --wS 0.2 `
  --output-prefix Gemini_Improved_Strategy `
  --emit-xlsx `
  --emit-pdf `
  --emit-png

# Indicate completion
Write-Host "Improved backtest complete. Check the 'Gemini_Improved_Strategy' files."
