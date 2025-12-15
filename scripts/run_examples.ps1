#!/usr/bin/env pwsh
<#
  V4 高频分红轮动策略 - PowerShell 示例脚本
  
  用法：
    .\run_examples.ps1
    
  前置条件：
    1. Python 已安装
    2. 环境变量 EODHD_API_TOKEN 已设置
    3. 依赖已安装：pip install -r requirements_dividend.txt
#>

param(
    [switch]$All,
    [int]$Example = 0
)

$ErrorActionPreference = "Stop"

# 颜色定义
$Colors = @{
    Success = [ConsoleColor]::Green
    Error   = [ConsoleColor]::Red
    Info    = [ConsoleColor]::Cyan
    Warn    = [ConsoleColor]::Yellow
}

function Write-Info { 
    Write-Host "[INFO] " -ForegroundColor $Colors.Info -NoNewline
    Write-Host $args
}

function Write-Success { 
    Write-Host "[SUCCESS] " -ForegroundColor $Colors.Success -NoNewline
    Write-Host $args
}

function Write-Error { 
    Write-Host "[ERROR] " -ForegroundColor $Colors.Error -NoNewline
    Write-Host $args
}

function Write-Warn { 
    Write-Host "[WARN] " -ForegroundColor $Colors.Warn -NoNewline
    Write-Host $args
}

function Check-Prerequisites {
    Write-Info "检查前置条件..."
    
    # 检查 Python
    try {
        $pythonVer = python --version 2>&1
        Write-Success "Python 已安装：$pythonVer"
    } catch {
        Write-Error "Python 未安装或不在 PATH 中"
        exit 1
    }
    
    # 检查 API 密钥
    if ([string]::IsNullOrWhiteSpace($env:EODHD_API_TOKEN)) {
        Write-Error "环境变量 EODHD_API_TOKEN 未设置"
        Write-Warn "请运行：`$env:EODHD_API_TOKEN = 'your_token'"
        exit 1
    }
    Write-Success "API Token 已配置"
}

function Run-Example {
    param(
        [int]$ExampleNum,
        [string]$Title,
        [string]$Description,
        [hashtable]$Params
    )
    
    Write-Host "`n" + ("=" * 60)
    Write-Host "[EXAMPLE $ExampleNum] $Title" -ForegroundColor $Colors.Info
    Write-Host ("=" * 60)
    Write-Host "$Description`n"
    
    # 构建命令
    $cmd = @("dividend_rotation_v4_real_cli_plan.py")
    $Params.GetEnumerator() | ForEach-Object {
        $cmd += "--$($_.Key)"
        $cmd += $_.Value
    }
    
    Write-Info "执行：python $($cmd -join ' ')`n"
    Write-Host "按 Enter 开始..."
    Read-Host | Out-Null
    
    try {
        & python @cmd
        Write-Success "示例 $ExampleNum 完成"
        return $true
    } catch {
        Write-Error "示例 $ExampleNum 执行失败：$($_.Exception.Message)"
        return $false
    }
}

# ==================== 主程序 ====================
Write-Host "`n"
Write-Host ("=" * 60)
Write-Host "V4 高频分红轮动策略 - PowerShell 示例脚本" -ForegroundColor $Colors.Info
Write-Host ("=" * 60) + "`n"

Check-Prerequisites

# 示例列表
$Examples = @(
    @{
        Num         = 1
        Title       = "保守策略"
        Description = "高息 ETF，低交易频率，风险小"
        Params      = @{
            'start'           = '2023-11-01'
            'end'             = '2025-11-11'
            'initial-cash'    = '200000'
            'min-div-yield'   = '0.025'
            'min-avg-vol'     = '500000'
            'topk'            = '5'
            'hold-pre'        = '1'
            'hold-post'       = '2'
            'wY'              = '0.6'
            'wL'              = '0.2'
            'wS'              = '0.2'
            'output-prefix'   = 'Conservative_Strategy'
            'emit-xlsx'       = ''
            'emit-pdf'        = ''
            'emit-png'        = ''
        }
    },
    @{
        Num         = 2
        Title       = "激进策略"
        Description = "高频轮动，除权日期近度优先"
        Params      = @{
            'initial-cash'    = '500000'
            'min-div-yield'   = '0.009'
            'min-avg-vol'     = '100000'
            'topk'            = '20'
            'hold-pre'        = '3'
            'hold-post'       = '0'
            'wY'              = '0.2'
            'wL'              = '0.2'
            'wS'              = '0.6'
            'ex-lookahead'    = '30'
            'output-prefix'   = 'Aggressive_Strategy'
            'emit-xlsx'       = ''
            'emit-pdf'        = ''
            'emit-png'        = ''
        }
    },
    @{
        Num         = 3
        Title       = "完整分析"
        Description = "2023 年全年数据，生成完整报告"
        Params      = @{
            'start'           = '2023-01-01'
            'end'             = '2023-12-31'
            'initial-cash'    = '1000000'
            'topk'            = '15'
            'output-prefix'   = 'Full_2023_Analysis'
            'emit-xlsx'       = ''
            'emit-pdf'        = ''
            'emit-png'        = ''
        }
    },
    @{
        Num         = 4
        Title       = "未来计划（OMS 集成）"
        Description = "仅生成下周的买卖计划，可直接导入 OMS"
        Params      = @{
            'start'           = '2025-08-01'
            'end'             = '2025-11-11'
            'initial-cash'    = '100000'
            'topk'            = '10'
            'ex-lookahead'    = '7'
            'output-prefix'   = 'Weekly_Plan'
        }
    }
)

if ($All) {
    # 运行所有示例
    $results = @()
    foreach ($ex in $Examples) {
        $success = Run-Example -ExampleNum $ex.Num -Title $ex.Title `
                              -Description $ex.Description -Params $ex.Params
        $results += @{ Example = $ex.Num; Success = $success }
    }
    
    # 总结
    Write-Host "`n" + ("=" * 60)
    Write-Host "执行完成 - 汇总" -ForegroundColor $Colors.Info
    Write-Host ("=" * 60)
    foreach ($r in $results) {
        if ($r.Success) {
            Write-Success "示例 $($r.Example) 成功"
        } else {
            Write-Error "示例 $($r.Example) 失败"
        }
    }
} elseif ($Example -gt 0 -and $Example -le $Examples.Count) {
    # 运行指定示例
    $ex = $Examples[$Example - 1]
    Run-Example -ExampleNum $ex.Num -Title $ex.Title `
                -Description $ex.Description -Params $ex.Params
} else {
    # 交互式菜单
    Write-Host "请选择要运行的示例：" -ForegroundColor $Colors.Info
    Write-Host ""
    foreach ($ex in $Examples) {
        Write-Host "  [$($ex.Num)] $($ex.Title)"
        Write-Host "      $($ex.Description)`n"
    }
    Write-Host "  [0] 运行所有示例"
    Write-Host "  [Q] 退出`n"
    
    $choice = Read-Host "请选择 (0-$($Examples.Count), Q)"
    
    if ($choice -eq 'Q' -or $choice -eq 'q') {
        exit 0
    } elseif ($choice -eq '0') {
        $All = $true
        # 递归调用以运行全部
        & $MyInvocation.MyCommand.Path -All
    } elseif ([int]::TryParse($choice, [ref]$Example) -and $Example -le $Examples.Count) {
        $ex = $Examples[$Example - 1]
        Run-Example -ExampleNum $ex.Num -Title $ex.Title `
                    -Description $ex.Description -Params $ex.Params
    } else {
        Write-Error "无效选择"
        exit 1
    }
}

Write-Host "`n"
Write-Host "输出文件位置：当前工作目录" -ForegroundColor $Colors.Info
Write-Host "  - 使用 Excel 打开 .xlsx 文件查看详细数据"
Write-Host "  - 使用 PDF 查看器打开 .pdf 文件查看报告"
Write-Host "  - 使用图片查看器查看 .png 图表`n"
