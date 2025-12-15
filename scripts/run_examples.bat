@echo off
REM V4 高频分红轮动策略 - 示例脚本集合
REM 请确保已设置环境变量：set EODHD_API_TOKEN=your_token

setlocal enabledelayedexpansion

REM 颜色输出（可选）
cls

echo.
echo ========================================
echo V4 高频分红轮动策略 示例脚本
echo ========================================
echo.

REM 检查 Python
python --version >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Python 未安装或不在 PATH 中
    exit /b 1
)

REM 检查 API 密钥
if "%EODHD_API_TOKEN%"=="" (
    echo [ERROR] 环境变量 EODHD_API_TOKEN 未设置
    echo 请运行：set EODHD_API_TOKEN=your_token
    exit /b 1
)

echo [OK] Python 已安装
echo [OK] API Token 已配置
echo.

REM ======================================
REM 示例 1：保守策略（默认参数）
REM ======================================
echo.
echo [EXAMPLE 1] 保守策略 - 高息 ETF，低交易频率
echo 命令：
echo python dividend_rotation_v4_real_cli_plan.py ^
echo   --start 2023-11-01 ^
echo   --end 2025-11-11 ^
echo   --initial-cash 200000 ^
echo   --min-div-yield 0.025 ^
echo   --min-avg-vol 500000 ^
echo   --topk 5 ^
echo   --hold-pre 1 --hold-post 2 ^
echo   --wY 0.6 --wL 0.2 --wS 0.2 ^
echo   --output-prefix Conservative_Strategy ^
echo   --emit-xlsx --emit-pdf --emit-png
echo.
pause
python dividend_rotation_v4_real_cli_plan.py ^
  --start 2023-11-01 ^
  --end 2025-11-11 ^
  --initial-cash 200000 ^
  --min-div-yield 0.025 ^
  --min-avg-vol 500000 ^
  --topk 5 ^
  --hold-pre 1 --hold-post 2 ^
  --wY 0.6 --wL 0.2 --wS 0.2 ^
  --output-prefix Conservative_Strategy ^
  --emit-xlsx --emit-pdf --emit-png

if errorlevel 1 (
    echo [FAILED] 示例 1 执行失败
) else (
    echo [SUCCESS] 示例 1 完成
    echo 输出文件：
    echo   - Conservative_Strategy_Buy_Sell_Plan.xlsx
    echo   - Conservative_Strategy_Backtest_Report.pdf
    echo   - Conservative_Strategy_Performance_Chart.png
)

REM ======================================
REM 示例 2：激进策略（高频、近度优先）
REM ======================================
echo.
echo [EXAMPLE 2] 激进策略 - 高频轮动，近度优先
echo 命令：
echo python dividend_rotation_v4_real_cli_plan.py ^
echo   --initial-cash 500000 ^
echo   --min-div-yield 0.009 ^
echo   --min-avg-vol 100000 ^
echo   --topk 20 ^
echo   --hold-pre 3 --hold-post 0 ^
echo   --wY 0.2 --wL 0.2 --wS 0.6 ^
echo   --ex-lookahead 30 ^
echo   --output-prefix Aggressive_Strategy ^
echo   --emit-xlsx --emit-pdf --emit-png
echo.
pause
python dividend_rotation_v4_real_cli_plan.py ^
  --initial-cash 500000 ^
  --min-div-yield 0.009 ^
  --min-avg-vol 100000 ^
  --topk 20 ^
  --hold-pre 3 --hold-post 0 ^
  --wY 0.2 --wL 0.2 --wS 0.6 ^
  --ex-lookahead 30 ^
  --output-prefix Aggressive_Strategy ^
  --emit-xlsx --emit-pdf --emit-png

if errorlevel 1 (
    echo [FAILED] 示例 2 执行失败
) else (
    echo [SUCCESS] 示例 2 完成
    echo 输出文件：
    echo   - Aggressive_Strategy_Buy_Sell_Plan.xlsx
    echo   - Aggressive_Strategy_Backtest_Report.pdf
    echo   - Aggressive_Strategy_Performance_Chart.png
)

REM ======================================
REM 示例 3：完整分析（所有导出）
REM ======================================
echo.
echo [EXAMPLE 3] 完整分析 - 2023 年全年数据
echo 命令：
echo python dividend_rotation_v4_real_cli_plan.py ^
echo   --start 2023-01-01 ^
echo   --end 2023-12-31 ^
echo   --initial-cash 1000000 ^
echo   --topk 15 ^
echo   --output-prefix Full_2023_Analysis ^
echo   --emit-xlsx --emit-pdf --emit-png
echo.
pause
python dividend_rotation_v4_real_cli_plan.py ^
  --start 2023-01-01 ^
  --end 2023-12-31 ^
  --initial-cash 1000000 ^
  --topk 15 ^
  --output-prefix Full_2023_Analysis ^
  --emit-xlsx --emit-pdf --emit-png

if errorlevel 1 (
    echo [FAILED] 示例 3 执行失败
) else (
    echo [SUCCESS] 示例 3 完成
    echo 输出文件：
    echo   - Full_2023_Analysis_Buy_Sell_Plan.xlsx
    echo   - Full_2023_Analysis_Backtest_Report.pdf
    echo   - Full_2023_Analysis_Performance_Chart.png
)

REM ======================================
REM 示例 4：仅生成未来计划（无回测）
REM ======================================
echo.
echo [EXAMPLE 4] 仅生成未来计划（下周执行）
echo 命令：
echo python dividend_rotation_v4_real_cli_plan.py ^
echo   --start 2025-08-01 ^
echo   --end 2025-11-11 ^
echo   --initial-cash 100000 ^
echo   --topk 10 ^
echo   --ex-lookahead 7 ^
echo   --output-prefix Weekly_Plan
echo.
pause
python dividend_rotation_v4_real_cli_plan.py ^
  --start 2025-08-01 ^
  --end 2025-11-11 ^
  --initial-cash 100000 ^
  --topk 10 ^
  --ex-lookahead 7 ^
  --output-prefix Weekly_Plan

if errorlevel 1 (
    echo [FAILED] 示例 4 执行失败
) else (
    echo [SUCCESS] 示例 4 完成
    echo 输出文件：
    echo   - Weekly_Plan_Forward_Plan.csv （可直接导入 OMS）
)

REM ======================================
REM 完成
REM ======================================
echo.
echo ========================================
echo 所有示例已完成
echo ========================================
echo.
echo 输出文件位置：当前工作目录
echo 使用 Excel 打开 .xlsx 文件查看详细数据
echo 使用 PDF 查看器打开 .pdf 文件查看报告
echo.
pause

endlocal
