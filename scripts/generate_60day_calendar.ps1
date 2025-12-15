# Set the EODHD API Token for this script execution
$env:EODHD_API_TOKEN = "690d7cdc3013f4.57364117"

# Execute the backtesting script with a 60-day lookahead for the forward plan
# Use PowerShell-friendly invocation. We pass arguments as an array to avoid line-continuation issues.
$pythonArgs = @(
  '.\dividend_rotation_v4_real_cli_plan.py'
  '--start', '2025-09-10'
  '--end', '2025-11-11'
  '--initial-cash', '200000'
  '--min-div-yield', '0.015'
  '--min-avg-vol', '300000'
  '--topk', '15'
  '--hold-pre', '2'
  '--hold-post', '2'
  '--wY', '0.5'
  '--wL', '0.3'
  '--wS', '0.2'
  '--ex-lookahead', '60'
  '--output-prefix', 'Forward_Calendar_60_Day'
)

# Use the full 'python' executable from PATH; invokes python with the array of args
Write-Host "Running: python $($pythonArgs -join ' ')" -ForegroundColor Yellow
& python @pythonArgs

# Indicate completion
if ($LASTEXITCODE -eq 0) {
  Write-Host "60-day forward calendar generation complete." -ForegroundColor Green
} else {
  Write-Host "Script exited with code $LASTEXITCODE" -ForegroundColor Red
}
