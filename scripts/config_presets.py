#!/usr/bin/env python3
# -*- coding: utf-8 -*-

"""
V4 分红轮动策略 - 配置预设

本文件包含不同投资风格的预设配置。
复制相关配置并传递给主脚本，实现快速切换策略。
"""

# ==========================================
# 预设配置字典
# ==========================================

PRESETS = {
    # ================== 1. 保守型策略 ==================
    "conservative": {
        "name": "保守型 - 高息稳定",
        "description": "追求稳定分红收入，低交易频率",
        "params": {
            "min_div_yield": 0.03,      # 最低 3% 股息率
            "min_avg_vol": 500000,      # 较高流动性要求
            "topk": 5,                  # 精选 5 只
            "hold_pre": 1,              # 除权前 1 天买入
            "hold_post": 2,             # 除权后 2 天卖出
            "alloc_per_event": 0.25,    # 较低单笔风险
            "wY": 0.6,                  # 股息率权重 60%
            "wL": 0.2,                  # 流动性权重 20%
            "wS": 0.2,                  # 近度权重 20%
            "initial_cash": 200000,
            "ex_lookahead": 90,
        },
        "target": {
            "annual_return": "8-12%",
            "max_drawdown": "-10%",
            "sharpe_ratio": ">1.0",
            "win_rate": ">55%"
        }
    },

    # ================== 2. 均衡型策略 ==================
    "balanced": {
        "name": "均衡型 - 收益风险兼备",
        "description": "在收入和增长间寻找平衡，中等交易频率",
        "params": {
            "min_div_yield": 0.015,     # 1.5% 股息率
            "min_avg_vol": 300000,      # 中等流动性
            "topk": 10,                 # 选择 10 只
            "hold_pre": 2,              # 除权前 2 天买入
            "hold_post": 1,             # 除权后 1 天卖出
            "alloc_per_event": 0.33,    # 中等单笔风险
            "wY": 0.4,                  # 股息率权重 40%
            "wL": 0.25,                 # 流动性权重 25%
            "wS": 0.35,                 # 近度权重 35%
            "initial_cash": 300000,
            "ex_lookahead": 90,
        },
        "target": {
            "annual_return": "12-18%",
            "max_drawdown": "-15%",
            "sharpe_ratio": ">1.2",
            "win_rate": ">50%"
        }
    },

    # ================== 3. 激进型策略 ==================
    "aggressive": {
        "name": "激进型 - 高频轮动",
        "description": "高频轮动，除权日期近度优先，追求最大收益",
        "params": {
            "min_div_yield": 0.009,     # 最低 0.9% 股息率
            "min_avg_vol": 100000,      # 较低流动性要求
            "topk": 20,                 # 选择 20 只
            "hold_pre": 3,              # 除权前 3 天买入
            "hold_post": 0,             # 除权当日卖出
            "alloc_per_event": 0.50,    # 较高单笔风险
            "wY": 0.2,                  # 股息率权重 20%
            "wL": 0.2,                  # 流动性权重 20%
            "wS": 0.6,                  # 近度权重 60%（优先）
            "initial_cash": 500000,
            "ex_lookahead": 30,         # 短期计划
        },
        "target": {
            "annual_return": "18-30%",
            "max_drawdown": "-25%",
            "sharpe_ratio": ">0.8",
            "win_rate": ">48%"
        }
    },

    # ================== 4. 高息专注型 ==================
    "high_yield": {
        "name": "高息专注 - 闭眼收息",
        "description": "只选择高息 ETF，定期收息，最少交易",
        "params": {
            "min_div_yield": 0.045,     # 最低 4.5% 股息率
            "min_avg_vol": 1000000,     # 极高流动性要求
            "topk": 3,                  # 只选 3 只精品
            "hold_pre": 1,              # 除权前 1 天
            "hold_post": 1,             # 除权后 1 天
            "alloc_per_event": 0.15,    # 极低风险
            "wY": 0.8,                  # 股息率权重 80%
            "wL": 0.15,                 # 流动性权重 15%
            "wS": 0.05,                 # 近度权重 5%
            "initial_cash": 100000,
            "ex_lookahead": 180,        # 长期规划
        },
        "target": {
            "annual_return": "6-10%",
            "max_drawdown": "-5%",
            "sharpe_ratio": ">1.5",
            "win_rate": ">65%"
        }
    },

    # ================== 5. 量化研究型 ==================
    "quant_research": {
        "name": "量化研究 - 全面回测",
        "description": "完整数据收集和分析，适合学术研究和参数优化",
        "params": {
            "min_div_yield": 0.005,     # 极低筛选条件
            "min_avg_vol": 50000,       # 包含小盘 ETF
            "topk": 50,                 # 大样本集
            "hold_pre": 5,              # 长期持仓窗口
            "hold_post": 5,
            "alloc_per_event": 0.10,    # 小额多笔
            "wY": 0.33,                 # 均衡权重
            "wL": 0.33,
            "wS": 0.34,
            "initial_cash": 1000000,
            "ex_lookahead": 365,        # 年度规划
        },
        "target": {
            "annual_return": "不限",
            "max_drawdown": "不限",
            "sharpe_ratio": "用于优化",
            "win_rate": "统计分析"
        }
    }
}

# ==========================================
# 命令生成函数
# ==========================================

def generate_command(preset_name, additional_params=None):
    """
    生成完整的 CLI 命令
    
    Args:
        preset_name: 预设名称 (conservative/balanced/aggressive/high_yield/quant_research)
        additional_params: 额外参数字典（覆盖预设）
    
    Returns:
        完整的 CLI 命令字符串
    """
    if preset_name not in PRESETS:
        raise ValueError(f"未知预设：{preset_name}")
    
    preset = PRESETS[preset_name]
    params = preset["params"].copy()
    
    # 覆盖额外参数
    if additional_params:
        params.update(additional_params)
    
    # 构建命令
    cmd = ["python dividend_rotation_v4_real_cli_plan.py"]
    
    for key, value in params.items():
        param_name = key.replace("_", "-")
        cmd.append(f"--{param_name}")
        cmd.append(str(value))
    
    return " ^\n  ".join(cmd)


# ==========================================
# 打印帮助信息
# ==========================================

def print_presets():
    """打印所有可用预设"""
    print("\n" + "="*60)
    print("V4 分红轮动策略 - 配置预设")
    print("="*60 + "\n")
    
    for key, preset in PRESETS.items():
        print(f"【{key.upper()}】{preset['name']}")
        print(f"  描述：{preset['description']}")
        print(f"  目标：")
        for metric, target in preset['target'].items():
            print(f"    - {metric}: {target}")
        print()


def show_command(preset_name):
    """显示特定预设的完整命令"""
    if preset_name not in PRESETS:
        print(f"错误：未知预设 {preset_name}")
        return
    
    preset = PRESETS[preset_name]
    cmd = generate_command(preset_name)
    
    print(f"\n预设：{preset['name']}")
    print(f"描述：{preset['description']}\n")
    print("完整命令：\n")
    print(cmd)
    print("\n")


# ==========================================
# PowerShell 脚本生成函数
# ==========================================

def generate_ps1_script(preset_name, output_prefix=None):
    """
    生成可直接运行的 PowerShell 脚本
    
    Args:
        preset_name: 预设名称
        output_prefix: 输出前缀（默认使用预设名）
    
    Returns:
        PowerShell 脚本字符串
    """
    if preset_name not in PRESETS:
        raise ValueError(f"未知预设：{preset_name}")
    
    preset = PRESETS[preset_name]
    params = preset["params"].copy()
    
    if not output_prefix:
        output_prefix = preset_name
    
    params["output_prefix"] = output_prefix
    params["emit_xlsx"] = ""
    params["emit_pdf"] = ""
    params["emit_png"] = ""
    
    script = f"""#!/usr/bin/env pwsh
<#
  策略预设：{preset['name']}
  描述：{preset['description']}
#>

Write-Host "开始执行：{preset['name']}" -ForegroundColor Green
Write-Host ""

$params = @(
"""
    
    for key, value in params.items():
        if value == "":
            script += f'    "--{key.replace("_", "-")}",\n'
        else:
            script += f'    "--{key.replace("_", "-")}", "{value}",\n'
    
    script += """
)

& python dividend_rotation_v4_real_cli_plan.py @params

Write-Host ""
Write-Host "执行完成！" -ForegroundColor Green
"""
    
    return script


# ==========================================
# 对比分析函数
# ==========================================

def compare_presets(presets_list):
    """
    对比多个预设的差异
    
    Args:
        presets_list: 预设名称列表
    """
    print("\n" + "="*100)
    print("预设对比分析")
    print("="*100 + "\n")
    
    # 提取所有唯一的参数键
    all_keys = set()
    preset_data = {}
    
    for preset_name in presets_list:
        if preset_name not in PRESETS:
            print(f"警告：未知预设 {preset_name}")
            continue
        
        preset = PRESETS[preset_name]
        preset_data[preset_name] = preset
        all_keys.update(preset["params"].keys())
    
    # 构建对比表
    print(f"{'参数':<25}", end="")
    for preset_name in presets_list:
        print(f"{preset_name:<15}", end="")
    print("\n" + "-"*100)
    
    for key in sorted(all_keys):
        print(f"{key:<25}", end="")
        for preset_name in presets_list:
            if preset_name in preset_data:
                value = preset_data[preset_name]["params"].get(key, "-")
                print(f"{str(value):<15}", end="")
            else:
                print(f"{'N/A':<15}", end="")
        print()
    
    print("\n")


# ==========================================
# CLI 入口
# ==========================================

if __name__ == "__main__":
    import sys
    
    if len(sys.argv) < 2:
        print_presets()
        print("\n使用方法：")
        print("  python config_presets.py [command] [preset]")
        print("\n可用命令：")
        print("  list           - 列出所有预设")
        print("  show <preset>  - 显示特定预设的完整命令")
        print("  compare <p1> <p2> ... - 对比多个预设")
        print("  gen-ps1 <preset> - 生成 PowerShell 脚本")
        print("\n示例：")
        print("  python config_presets.py show conservative")
        print("  python config_presets.py compare conservative aggressive")
        print("  python config_presets.py gen-ps1 balanced > run_balanced.ps1")
    
    elif sys.argv[1] == "list":
        print_presets()
    
    elif sys.argv[1] == "show" and len(sys.argv) >= 3:
        show_command(sys.argv[2])
    
    elif sys.argv[1] == "compare":
        compare_presets(sys.argv[2:])
    
    elif sys.argv[1] == "gen-ps1" and len(sys.argv) >= 3:
        preset_name = sys.argv[2]
        output_prefix = sys.argv[3] if len(sys.argv) >= 4 else None
        script = generate_ps1_script(preset_name, output_prefix)
        print(script)
    
    else:
        print("未知命令。运行 'python config_presets.py' 查看帮助。")
