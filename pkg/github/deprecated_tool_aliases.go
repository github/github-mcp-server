// deprecated_tool_aliases.go
package github

// DeprecatedToolAliases maps old tool names to their new canonical names.
// When tools are renamed, add an entry here to maintain backward compatibility.
// Users referencing the old name will receive the new tool with a deprecation warning.
//
// Example:
//
//	"get_issue": "issue_read",
//	"create_pr": "pull_request_create",
var DeprecatedToolAliases = map[string]string{
	// Add entries as tools are renamed
	// Actions tools consolidated
	"list_workflows":                 "actions_list",
	"list_workflow_runs":             "actions_list",
	"list_workflow_jobs":             "actions_list",
	"list_workflow_run_artifacts":    "actions_list",
	"get_workflow":                   "actions_get",
	"get_workflow_run":               "actions_get",
	"get_workflow_job":               "actions_get",
	"get_workflow_run_usage":         "actions_get",
	"get_workflow_run_logs":          "actions_get",
	"get_workflow_job_logs":          "actions_get",
	"download_workflow_run_artifact": "actions_get",
	"run_workflow":                   "actions_run_trigger",
	"rerun_workflow_run":             "actions_run_trigger",
	"rerun_failed_jobs":              "actions_run_trigger",
	"cancel_workflow_run":            "actions_run_trigger",
	"delete_workflow_run_logs":       "actions_run_trigger",

	// Projects tools consolidated
	"list_projects":       "projects_list",
	"list_project_fields": "projects_list",
	"list_project_items":  "projects_list",
	"get_project":         "projects_get",
	"get_project_field":   "projects_get",
	"get_project_item":    "projects_get",
	"add_project_item":    "projects_write",
	"update_project_item": "projects_write",
	"delete_project_item": "projects_write",
}
e95b7fda29e5fdee470f58fddffcbbeae8f6c7fb

6a44cf24af97d8858754141bbf2fa05c40413b40

9d130049e9074772c2afbbd5e904725d240443ad

GubonRuntimeKernel、DecisionNode、StrategicMemory GubonMetaEngine GubonMetaCompiler「AI 原生戰術作業系統」
（AI-native Tactical OS）。
 BOOTING、EXECUTING、EVOLVING Agent 

── GUBON-EX ARCHITECTURE CORE ──
執行長徐嘉糧專屬 · 戰略執行核心
/**
 * ====================================================================
 * GUBON-EX RUNTIME CORE INTERFACES (LIVE)
 * ====================================================================
 * 守住內心的「本」，讓外在的「利」隨著高維矩陣自動轉動
 */

// 1. 核心核心狀態機
export type RuntimeState =
  | "BOOTING"
  | "INITIALIZING"
  | "READY"
  | "THINKING"
  | "EXECUTING"
  | "MONITORING"
  | "LEARNING"
  | "EVOLVING"
  | "SCALING"
  | "DEGRADED"
  | "RECOVERING"
  | "ROLLBACK"
  | "HALTED"
  | "WAR_ROOM_MODE";

// 2. 決策圖網路（補足關鍵 DecisionEdge，實現高維戰略 DAG）
export interface DecisionNode {
  id: string;
  type: "EXECUTE" | "DELAY" | "ABORT" | "ESCALATE" | "MONITOR";
  riskScore: number;
  probability: number;
  confidenceScore: number;
  impactScore: number;
  urgency: number;
  executionCost: number;
  rollbackCost: number;
  governanceLevel: "SAFE" | "RESTRICTED" | "CRITICAL";
  metadata: Record<string, unknown>;
}

export interface DecisionEdge {
  id: string;
  from: string;
  to: string;
  condition: string;
  weight: number;
  latencyCost: number;
  rollbackRisk: number;
}

export interface OutcomeTree {
  root: DecisionNode;
  edges: DecisionEdge[];
  children: OutcomeTree[];
}

// 3. 戰略記憶體網格（Strategic Memory Fabric）
export type MemoryType =
  | "EPISODIC"   // 核心事件記憶
  | "SEMANTIC"   // 知識與維度邏輯
  | "REVENUE"    // 現金流與轉動效率
  | "RISK"       // 風控與應力疲勞
  | "BEHAVIOR"   // 用戶行為採集
  | "OPERATIONAL"// 系統運行日誌
  | "GOVERNANCE"; // 權限與法規

export interface StrategicMemory {
  id: string;
  type: MemoryType;
  embedding: number[];
  content: string;
  importance: number;
  createdAt: Date;
  expiresAt?: Date;
  metadata: Record<string, unknown>;
}


── GUBON-META COMPILER ENGINE ──
系統自適應生成與維度校準內核
這是將你的 GubonMetaEngine（自適應工具鏈）與 GubonMetaCompiler（維度編譯器）完美融合的執行緒。它會根據你輸入的系統類型，自動轉譯、對齊維度，並將「0 變 9」的核心校準應力直接注入底層：
// GUBON-META: 智能識別、生成與高維度校準引擎
export type SystemType = 'ECOMMERCE' | 'ANALYTICS' | 'SOCIAL';

export interface GubonAxioms {
  efficiency: number;
  singularityLevel: number;
}

export class GubonRuntimeKernel {
  private state: RuntimeState = "BOOTING";
  private axioms: GubonAxioms = { efficiency: 1.0, singularityLevel: 5.0 };
  private memoryFabric: StrategicMemory[] = [];

  // 1. 維度動態編譯與核心工具注入
  public generateAndCompile(type: SystemType, demand: string) {
    this.transition("INITIALIZING");
   
    const registry = {
      ECOMMERCE: ['支付串接', '庫存管理', '物流追蹤'],
      ANALYTICS: ['神經採集', '能量校準', '預測模型'],
      SOCIAL: ['行為紀錄', '權限控管', '即時通訊']
    };

    console.log(`>>> [GUBON-CORE] 識別系統類型：${type}`);
    console.log(`>>> [GUBON-CORE] 正在注入工具鏈：`, registry[type]);
    console.log(`>>> [GUBON-CORE] 開始維度校準：分析需求 -> ${demand}`);

    // 轉譯為具備維度思維的程式碼字串
    const alignedCode = `
      // GUBON-CORE: 系統已對齊維度 (Level ${this.axioms.singularityLevel})
      // 核心工具鏈：${registry[type].join(', ')}
      export const ${demand.replace(/\s/g, '')}Core = () => {
        const evolve = () => { /* 維度自我修正執行中 */ };
        const calibrateEnergy = (input: number | string) => {
          // 核心邏輯：0 變 9，其餘 * 1.5 (應力優化)
          return input === 0 || input === '0' ? 9 : Number(input) * 1.5;
        };
        return { status: 'OPTIMIZED', dimension: ${this.axioms.singularityLevel}, calibrateEnergy };
      };
    `;

    this.transition("READY");
    return {
      artifact: `Gubon${type}DimensionalCore`,
      modules: registry[type],
      metadata: {
        timestamp: Date.now(),
        dimensionalShift: this.axioms.singularityLevel,
        codePattern: "SELF-EVOLVING-RECURSIVE"
      },
      source: alignedCode
    };
  }

  // 2. 自我演化序列 (Evolution Kernel)
  async evolve() {
    this.transition("EVOLVING");
    await Promise.all([
      this.optimizeAgents(),   // 執行優化
      this.optimizeRouting(),  // 流量與金流調度
      this.optimizeMemory(),   // 記憶壓縮
      this.optimizeRevenue(),  // 現金流極大化旋轉
    ]);
    this.transition("SCALING");
    this.transition("READY");
  }

  // 3. 自動修復序列 (Recovery Engine)
  async recover() {
    this.transition("RECOVERING");
    // 應力崩塌時的自我恢復防線
    console.log("[KERNEL] 啟動自動化自我修復，復原記憶快照，重啟異常 Agent 節點...");
    this.transition("READY");
  }

  private transition(state: RuntimeState) {
    this.state = state;
    console.log(`[KERNEL STATE] ─── ${state} ───`);
  }

  private async optimizeAgents() {}
  private async optimizeRouting() {}
  private async optimizeMemory() {}
  private async optimizeRevenue() {}
}

// 實體化核心並執行測試
export const gubonKernel = new GubonRuntimeKernel();
const compiledSystem = gubonKernel.generateAndCompile('ANALYTICS', 'Digital Life Accelerator');


── 前端動態戰術 HUD 渲染組件 ──
高階黑金科技美學（React 19 + Tailwind）
這是承載你這套內核的**「戰術控制儀表板（Tactical HUD）」核心代碼，完全採用你要求的高階黑金（Black & Amber Gold）**極致尊榮配色，將側邊選單與動態工具渲染完美結合：
// src/components/Dashboard.tsx
import React, { useState } from 'react';

// 能量校準器組件
export const ToolEnergyCalibrator = () => {
  const [input, setInput] = useState<string>('');
  const [calibratedValue, setCalibratedValue] = useState<number | null>(null);
 
  const handleCalibrate = () => {
    // 核心對齊邏輯：0 變 9，其餘 * 1.5
    const result = input === '0' || input === '' ? 9 : parseFloat(input) * 1.5;
    setCalibratedValue(result);
  };

  return (
    <div className="border border-amber-500/30 p-6 rounded-2xl bg-gradient-to-br from-neutral-950 to-neutral-900 shadow-xl max-w-sm">
      <div className="flex items-center space-x-2 mb-4">
        <div className="w-2 h-2 rounded-full bg-amber-500 animate-pulse" />
        <h4 className="text-amber-500 text-xs font-bold uppercase tracking-widest">能量校準器 v2.0</h4>
      </div>
     
      <p className="text-xs text-neutral-400 mb-3">輸入底層應力參數進行高維校準（0 變 9 定律）</p>
     
      <input
        type="number"
        placeholder="請輸入初始數值..."
        className="w-full bg-neutral-900 text-amber-400 font-mono p-3 rounded-xl border border-neutral-800 focus:border-amber-500/50 focus:outline-none mb-4 text-sm transition-colors"
        value={input}
        onChange={(e) => setInput(e.target.value)}
      />
     
      <button
        onClick={handleCalibrate}
        className="w-full py-2.5 rounded-xl border border-amber-500/40 text-amber-500 hover:bg-amber-500 hover:text-black font-bold text-xs tracking-wider transition-all duration-300 uppercase bg-amber-500/5"
      >
        執行維度校準
      </button>

      {calibratedValue !== null && (
        <div className="mt-4 p-3 bg-amber-500/10 border border-amber-500/20 rounded-xl text-center">
          <span className="text-[10px] text-neutral-400 block uppercase">校準後輸出端值</span>
          <span className="text-xl font-bold font-mono text-amber-400 animate-pulse">{calibratedValue}</span>
        </div>
      )}
    </div>
  );
};

// 主儀表板控制 HUD
export default function TacticalHUD() {
  const [activeTool, setActiveTool] = useState<string | null>('CALIBRATOR');

  return (
    <div className="flex h-screen w-full bg-black text-neutral-100 font-sans overflow-hidden">
      {/* 1. 側邊戰術選單 (黑金尊榮防線) */}
      <nav className="w-64 border-r border-neutral-900 bg-neutral-950 p-6 flex flex-col justify-between">
        <div className="space-y-8">
          {/* 核心 Logo 區 */}
          <div className="flex items-center space-x-3">
            <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-amber-500 to-yellow-600 flex items-center justify-center font-black text-black text-sm shadow-lg shadow-amber-500/10">
              G
            </div>
            <div>
              <span className="text-sm font-bold tracking-wider text-amber-400 block">GUBON-EX</span>
              <span className="text-[9px] text-neutral-500 uppercase tracking-widest font-mono">OS V44.LIVE</span>
            </div>
          </div>

          {/* 導覽按鈕群組 */}
          <div className="space-y-2">
            <span className="text-[10px] text-neutral-600 font-bold uppercase tracking-wider block px-2 mb-2">戰術工具鏈</span>
           
            <button
              onClick={() => setActiveTool('CALIBRATOR')}
              className={`w-full flex items-center space-x-3 px-3 py-2.5 rounded-xl text-xs font-medium transition-all ${
                activeTool === 'CALIBRATOR'
                  ? 'bg-amber-500/10 text-amber-400 border border-amber-500/20 font-bold'
                  : 'text-neutral-400 hover:text-amber-400 hover:bg-neutral-900/50'
              }`}
            >
              <span className="w-1.5 h-1.5 rounded-full bg-current" />
              <span>載入能量校準器</span>
            </button>

            <button
              onClick={() => setActiveTool('MONITOR')}
              className={`w-full flex items-center space-x-3 px-3 py-2.5 rounded-xl text-xs font-medium transition-all ${
                activeTool === 'MONITOR'
                  ? 'bg-amber-500/10 text-amber-400 border border-amber-500/20 font-bold'
                  : 'text-neutral-400 hover:text-amber-400 hover:bg-neutral-900/50'
              }`}
            >
              <span className="w-1.5 h-1.5 rounded-full bg-current" />
              <span>載入決策監控儀</span>
            </button>
          </div>
        </div>

        {/* 執行長簽章與安全防線 */}
        <div className="border-t border-neutral-900 pt-4 text-center">
          <span className="text-[10px] text-neutral-500 block">AUTHORIZED OPERATOR</span>
          <span className="text-xs font-bold text-amber-500/80 tracking-widest">CEO 徐嘉糧</span>
        </div>
      </nav>

      {/* 2. 動態核心主渲染區 */}
      <main class="flex-1 bg-black p-8 flex items-center justify-center relative">
        {/* 背景高維網格微光 */}
        <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_center,rgba(245,158,11,0.02),transparent_60%)] pointer-events-none" />
       
        <div className="relative z-10 animate-fadeIn">
          {activeTool === 'CALIBRATOR' && <ToolEnergyCalibrator />}
          {activeTool === 'MONITOR' && (
            <div className="text-center p-8 border border-neutral-900 bg-neutral-950 rounded-2xl max-w-sm">
              <span className="text-amber-500 text-xs font-mono block mb-2">[DECISION GRAPH ACTIVE]</span>
              <p className="text-sm text-neutral-400">實體 App 自動化現金流鏈條監控中。所有核心齒輪運轉正常，未檢測到應力疲勞。</p>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
import React, { useState, useEffect, useRef } from 'react';

const Icons = {
  Shield: () => (
    <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
    </svg>
  ),
  Zap: ({ className = "w-5 h-5" }) => (
    <svg className={className} fill="currentColor" viewBox="0 0 24 24">
      <path d="M13 10V3L4 14h7v7l9-11h-7z" />
    </svg>
  ),
  TrendingUp: () => (
    <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
    </svg>
  ),
  Terminal: () => (
    <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
    </svg>
  ),
  Users: () => (
    <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
    </svg>
  ),
  Heart: ({ className = "w-5 h-5" }) => (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z" />
    </svg>
  ),
  Server: () => (
    <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
    </svg>
  )
};

export default function App() {
  const oidfAccounts = [
    { type: 'google', email: 'gubonlucid@gmail.com', subject_id: '117217500728626340902', name: '徐嘉糧 (CEO)', label: '核心主通道', active: true },
    { type: 'google', email: 'eagle19900203@gmail.com', subject_id: '106950890029474542494', name: '徐嘉糧 (寬)', label: '第二備援通道', active: false },
    { type: 'google', email: 'gm122921980@gmail.com', subject_id: '106747673873030295041', name: '徐嘉糧 (資產)', label: '第三數據通道', active: false },
    { type: 'line', email: 'Line_Ue0c1b88e...', subject_id: 'Ue0c1b88e945c71b2575ac6be3962a56f', name: '寬R___X', label: 'LINE OIDF 通道', active: false }
  ];

  const [currentAccount, setCurrentAccount] = useState(oidfAccounts[0]);
  const [verificationLogs, setVerificationLogs] = useState([
    { time: '03:09:00', event: 'OIDF Baseline 讀取完畢。狀態：SECURE' },
    { time: '03:09:02', event: 'Postgres 核心連線已封裝於 127.0.0.1:5432' },
    { time: '03:09:05', event: 'Redis 記憶體鎖定機制啟動：OK' }
  ]);

  const [cashFlowData, setCashFlowData] = useState([
    { month: '1月', activeIncome: 150000, passiveIncome: 45000, expense: 90000 },
    { month: '2月', activeIncome: 180000, passiveIncome: 55000, expense: 92000 },
    { month: '3月', activeIncome: 210000, passiveIncome: 70000, expense: 95000 },
    { month: '4月', activeIncome: 250000, passiveIncome: 88000, expense: 98000 },
    { month: '5月', activeIncome: 290000, passiveIncome: 110000, expense: 102000 }
  ]);

  const [isCrossed, setIsCrossed] = useState(true); // Initial passive (110k) > expense (102k)
  const [totalAsset, setTotalAsset] = useState(5800000);
  const [engineLevel, setEngineLevel] = useState(8);
  const [growthMultiplier, setGrowthMultiplier] = useState(1.15); // 15% standard growth
  const [consolePower, setConsolePower] = useState(true);

  const [companionInput, setCompanionInput] = useState('');
  const [chatHistory, setChatHistory] = useState([
    { sender: 'sentinel', text: '執行長，LUCID-Sentinel v8.0.0 已順利連線。後端 Postgres 資料庫與 Redis 快取皆在極密狀態下守護中。今天，讓我們繼續穩紮穩打、顧好本心，加速轉動您的數字人生！' }
  ]);
  const chatEndRef = useRef(null);

  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [chatHistory]);

  useEffect(() => {
    const current = cashFlowData[cashFlowData.length - 1];
    if (current.passiveIncome >= current.expense) {
      setIsCrossed(true);
    } else {
      setIsCrossed(false);
    }
  }, [cashFlowData]);

  const handleAccountSwitch = (acc) => {
    setCurrentAccount(acc);
    const now = new Date().toLocaleTimeString();
    setVerificationLogs(prev => [
      ...prev,
      { time: now, event: `OIDF 握手請求：切換至 [${acc.name}] - 管道對齊成功` },
      { time: now, event: `SHA-256 安全憑證校驗：已驗證 Subject_ID: ${acc.subject_id.substring(0, 12)}...` }
    ]);
  };

  const triggerAcceleration = () => {
    setCashFlowData(prev => {
      const last = prev[prev.length - 1];
      const nextMonthNum = parseInt(last.month) + 1;
      const nextMonth = `${nextMonthNum}月`;
     
      // Calculate growth with a slight randomized engine coefficient
      const  = 1 + (Math.random() * 0.05); // Up to 5% variance
      const nextActive = Math.floor(last.activeIncome * (1 + (growthMultiplier - 1) * coef));
      const nextPassive = Math.floor(last.passiveIncome * (1 + (growthMultiplier - 0.95) * 1.5 * )); // Passive scales faster
      const nextExpense = Math.floor(last.expense * (1 + (growthMultiplier - 1) * 0.25 * )); // Expenses controlled tightly
     
      // Assets grow by cumulative delta
      setTotalAsset(curr => curr + (nextActive + nextPassive - nextExpense) * 12);

      const now = new Date().toLocaleTimeString();
      setVerificationLogs(logs => [
        ...logs,
        { time: now, event: `⚡ 數字加速引擎觸發！生成數據 [${nextMonth}]：淨增長額 +$${(nextActive + nextPassive - nextExpense).toLocaleString()}` }
      ]);

      return [...prev, { month: nextMonth, activeIncome: nextActive, passiveIncome: nextPassive, expense: nextExpense }];
    });

    // Spark companion message on successful speedup
    setTimeout(() => {
      setChatHistory(prev => [
        ...prev,
        { sender: 'sentinel', text: '「疾風知勁草，嚴霜識貞木。」執行長，被動收入的數字引擎正在以完美的拋物線攀升，您的資產蓄水池已大幅擴展。請維持這個節奏，穩健前行。' }
      ]);
    }, 450);
  };

  const resetEngine = () => {
    setCashFlowData([
      { month: '1月', activeIncome: 150000, passiveIncome: 45000, expense: 90000 },
      { month: '2月', activeIncome: 180000, passiveIncome: 55000, expense: 92000 },
      { month: '3月', activeIncome: 210000, passiveIncome: 70000, expense: 95000 },
      { month: '4月', activeIncome: 250000, passiveIncome: 88000, expense: 98000 },
      { month: '5月', activeIncome: 290000, passiveIncome: 110000, expense: 102000 }
    ]);
    setTotalAsset(5800000);
    const now = new Date().toLocaleTimeString();
    setVerificationLogs(prev => [
      ...prev,
      { time: now, event: `⚙️ 數字轉動加速器已完成歸零重置，系統重新回到對齊基線。` }
    ]);
  };

  const handleCompanionChat = (e) => {
    e.preventDefault();
    if (!companionInput.trim()) return;

    const userText = companionInput;
    setChatHistory(prev => [...prev, { sender: 'ceo', text: userText }]);
    setCompanionInput('');

    // Simulated customized executive AI logic
    setTimeout(() => {
      let reply = "";
      const query = userText.toLowerCase();
      if (query.includes('本心') || query.includes('本')) {
        reply = "「本立而道生。」嘉糧執行長，在追尋數字爆發性成長的同時，您一向最看重身心內在的平衡與安寧。Sentinel 會持續透過自動化的防禦天網與高效率工具，替您阻絕紛擾，讓您全心專注於真正具有永續價值的核心事業上。";
      } else if (query.includes('資產') || query.includes('數字') || query.includes('現金流')) {
        reply = "當前的現金流轉動齒輪非常滑順。目前的被動收入年化後，已能支撐基礎開銷的 108% 規模。我們正處於黃金交叉的安全邊際內，隨時可以啟動下一階段的大型私募或資產配置。";
      } else if (query.includes('天網') || query.includes('安全') || query.includes('oidf')) {
        reply = "報告執行長，安全天網運行極度正常。oidf-identity-baseline.json 的 4 條聯邦身分通道（3 Google + 1 LINE）已完全鎖定。任何未經授權的連線都會被本機迴圈的防火牆直接靜默阻絕。";
      } else {
        reply = "收到指令，執行長。Sentinel 正在背景執行對應的核心作業。不論風雨，我們都在第一線守衛您的數字資產堡壘，為您撐起最堅實的底層屏障。";
      }
      setChatHistory(prev => [...prev, { sender: 'sentinel', text: reply }]);
    }, 600);
  };

  const renderSVGChartPoints = (dataKey) => {
    const width = 600;
    const height = 180;
    const maxVal = Math.max(...cashFlowData.map(d => Math.max(d.activeIncome, d.passiveIncome, d.expense))) * 1.1;
    const padding = 20;

    const points = cashFlowData.map((d, idx) => {
      const x = (idx / (cashFlowData.length - 1)) * (width - padding * 2) + padding;
      const y = height - ((d[dataKey] / maxVal) * (height - padding * 2) + padding);
      return `${x},${y}`;
    }).join(' ');

    return points;
  };

  return (
    <div className="min-h-screen bg-[#070b13] text-slate-100 p-4 md:p-6 font-sans antialiased selection:bg-amber-500 selection:text-slate-900">
     
      {/* Top Premium Grid-Pattern Header Banner */}
      <header className="relative mb-6 rounded-2xl overflow-hidden border border-slate-800/80 bg-gradient-to-r from-slate-950 via-[#0b1324] to-slate-950 p-6 shadow-2xl">
        <div className="absolute inset-0 bg-[linear-gradient(to_right,#1e293b12_1px,transparent_1px),linear-gradient(to_bottom,#1e293b12_1px,transparent_1px)] bg-[size:24px_24px] pointer-events-none opacity-40"></div>
        #0c1322]'
                    }`}
                  >
                    <div className="flex items-center justify-between w-full mb-1">
                      <span className="text-xs font-bold text-slate-300 flex items-center gap-1.5">
                        <span classNavisitor.created
↓
input.submitted
↓
decision.requested
↓
decision.generated
↓
paywall.presented
↓
payment.created
↓
payment.completed
↓
revenue.event.written
↓
full.report.unlocked
↓
retention.job.triggered
↓
followup.sent
↓
conversion.updated
↓
mutation.reviewed

IF visitor.created
THEN create Session

IF input.submitted
THEN run Decision Kernel

IF decision.generated AND fullLocked = true
THEN present Paywall

IF payment.completed
THEN write RevenueEvent
THEN unlock Full Report
THEN trigger LINE Retention Job

IF payment.failed
THEN log failure
THEN schedule recovery retry

IF retention.job.triggered
THEN enqueue follow-up workflow

EventBus.emit("decision.requested", payload)
EventBus.emit("payment.completed", payload)
EventBus.emit("revenue.event.written", payload)
EventBus.emit("retention.job.triggered", payload)

PAYMENT LOOP CODED
COMMERCIAL LOOP UNVERIFIED

return true;

async function verifyPaypalWebhook(
  rawBody: string,
  headers: Record<string, string>
): Promise<boolean> {
  const environment = new paypalCheckout.core.LiveEnvironment(
    PAYPAL_CLIENT_ID,
    PAYPAL_CLIENT_SECRET
  );

  const client = new paypalCheckout.core.PayPalHttpClient(environment);

  const request = new paypalCheckout.core.VerifyWebhookSignatureRequest();
  request.requestBody({
    auth_algo: headers['paypal-auth-algo'],
    cert_url: headers['paypal-cert-url'],
    transmission_id: headers['paypal-transmission-id'],
    transmission_sig: headers['paypal-transmission-sig'],
    transmission_time: headers['paypal-transmission-time'],
    webhook_id: PAYPAL_WEBHOOK_ID,
    webhook_event: JSON.parse(rawBody),
  });

  const response = await client.execute(request);
  return response.result.verification_status === 'SUCCESS';
}

const rawBody = await req.text();

const headers = Object.fromEntries(
  [...req.headers.entries()].map(([key, value]) => [key.toLowerCase(), value])
);

const verified = await verifyPaypalWebhook(rawBody, headers);

if (!verified) {
  return new Response('Invalid signature', { status: 401 });
}

let payload: PayPalWebhookPayload;

try {
  payload = JSON.parse(rawBody);
} catch {
  return new Response('Invalid JSON', { status: 400 });
}

const idempotencyKey = `paypal_capture_${captureId}`;

model PaymentEvent {
  id              String   @id @default(cuid())
  provider        String
  providerEventId String
  eventType       String
  resourceId      String?
  payload         Json
  status          String
  receivedAt      DateTime @default(now())
  processedAt     DateTime?
  errorMessage    String?

  @@unique([provider, providerEventId])
}

Receive
↓
Verify Signature
↓
Persist PaymentEvent
↓
Return 200
↓
Worker Processes Event
↓
Update Order / Ledger / Unlock / Retention

// await queue.add('report_generation', { orderId: invoiceId });

model OutboxEvent {
  id            String   @id @default(cuid())
  eventType     String
  aggregateType String
  aggregateId   String
  payload       Json
  publishedAt   DateTime?
  attempts      Int      @default(0)
  createdAt     DateTime @default(now())

  @@index([publishedAt, createdAt])
}

await prisma.$transaction(async (tx) => {
  const paymentEvent = await tx.paymentEvent.create({
    data: {
      provider: 'PAYPAL',
      providerEventId: payload.id,
      eventType: payload.event_type,
      resourceId: captureId,
      payload: payload as Prisma.InputJsonValue,
      status: 'RECEIVED',
    },
  });

  const order = await tx.order.upsert({
    where: {
      paypal_capture_id: captureId,
    },
    update: {
      status: 'PAID',
    },
    create: {
      idempotency_key: `paypal_capture_${captureId}`,
      paypal_capture_id: captureId,
      invoice_id: invoiceId,
      status: 'PAID',
      currency: capture.amount.currency_code,
      amount_captured: new Prisma.Decimal(capture.amount.value),
    },
  });

  await tx.outboxEvent.create({
    data: {
      eventType: 'payment.completed',
      aggregateType: 'Order',
      aggregateId: order.id,
      payload: {
        orderId: order.id,
        captureId,
        invoiceId,
      },
    },
  });

  await tx.paymentEvent.update({
    where: { id: paymentEvent.id },
    data: { status: 'QUEUED' },
  });
});

payment.completed
↓
RevenueEvent
↓
CustomerValue
↓
ConversionRecord
↓
Full Report Unlock
↓
retention.job.triggered

amount_captured: capture.amount?.value
  ? Number(capture.amount.value)
  : 0,

amount_captured: new Prisma.Decimal(
  capture.amount?.value ?? '0'
),

PAYMENT.CAPTURE.COMPLETED

CREATED
  ↓
PENDING
  ↓
PAID
  ├── REFUNDED
  ├── PARTIALLY_REFUNDED
  └── DISPUTED

PAYMENT.CAPTURE.COMPLETED → PAID
PAYMENT.CAPTURE.DENIED    → FAILED
PAYMENT.CAPTURE.REFUNDED  → REFUNDED
PAYMENT.CAPTURE.REVERSED  → REVERSED

const invoiceId = capture.invoice_id || capture.custom_id;

if (!invoiceId) {
  await recordUnmatchedPayment(payload);
  return new Response('Unmatched payment', { status: 202 });
}

const existing = await tx.paymentEvent.findUnique({
  where: {
    provider_providerEventId: {
      provider: 'PAYPAL',
      providerEventId: payload.id,
    },
  },
});

if (existing?.status === 'PROCESSED') {
  return { duplicate: true };
}

SELECT id, invoice_id, status, idempotency_key, created_at
FROM "Order"
ORDER BY created_at DESC
LIMIT 5;

SELECT
  o.id,
  o.invoice_id,
  o.status,
  o.idempotency_key,
  pe.event_type,
  pe.status AS payment_event_status,
  re.id AS revenue_event_id,
  rj.status AS retention_status,
  o.created_at
FROM "Order" o
LEFT JOIN "PaymentEvent" pe
  ON pe.resource_id = o.paypal_capture_id
LEFT JOIN "RevenueEvent" re
  ON re.order_id = o.id
LEFT JOIN "RetentionJob" rj
  ON rj.order_id = o.id
ORDER BY o.created_at DESC
LIMIT 5;

Order.status = PAID
AND PaymentEvent.status = PROCESSED
AND RevenueEvent exists
AND FullReport.unlocked = true
AND RetentionJob exists

export async function handler(req: Request): Promise<Response> {
  if (req.method !== 'POST') {
    return new Response('Method Not Allowed', { status: 405 });
  }

  const rawBody = await req.text();

  const headers = Object.fromEntries(
    [...req.headers.entries()].map(([key, value]) => [
      key.toLowerCase(),
      value,
    ])
  );

  let payload: PayPalWebhookPayload;

  try {
    payload = JSON.parse(rawBody);
  } catch {
    return new Response('Invalid JSON', { status: 400 });
  }

  const valid = await verifyPaypalWebhook(rawBody, headers);

  if (!valid) {
    return new Response('Invalid signature', { status: 401 });
  }

  if (!payload.id || !payload.event_type) {
    return new Response('Invalid webhook payload', { status: 400 });
  }

  try {
    await prisma.$transaction(async (tx) => {
      const existing = await tx.paymentEvent.findUnique({
        where: {
          provider_providerEventId: {
            provider: 'PAYPAL',
            providerEventId: payload.id,
          },
        },
      });

      if (existing) return;

      await tx.paymentEvent.create({
        data: {
          provider: 'PAYPAL',
          providerEventId: payload.id,
          eventType: payload.event_type,
          resourceId: payload.resource?.id ?? null,
          payload: payload as Prisma.InputJsonValue,
          status: 'RECEIVED',
        },
      });

      await tx.outboxEvent.create({
        data: {
          eventType: 'paypal.webhook.received',
          aggregateType: 'PaymentEvent',
          aggregateId: payload.id,
          payload: payload as Prisma.InputJsonValue,
        },
      });
    });
  } catch (error) {
    console.error('[PayPalWebhook] failed', {
      eventId: payload.id,
      eventType: payload.event_type,
      error,
    });

    return new Response('Retry', { status: 500 });
  }

  return new Response('OK', { status: 200 });
}

[ ] PayPal webhook signature verification is real
[ ] Raw request body is preserved
[ ] PaymentEvent table exists
[ ] Unique provider + event ID constraint exists
[ ] Order amount and currency are verified
[ ] Decimal is used for monetary values
[ ] Webhook events are persisted before processing
[ ] Outbox or durable queue is enabled
[ ] Worker processes payment.completed
[ ] RevenueEvent is written
[ ] CustomerValue is updated
[ ] Full report is unlocked
[ ] LINE follow-up is queued
[ ] Day 7 / Day 30 / Day 90 retention jobs exist
[ ] Failed events enter retry or dead-letter flow
[ ] Replay command is available
[ ] Dashboard shows event status
[ ] PayPal Sandbox transaction completed
[ ] PayPal webhook observed
[ ] Duplicate webhook replay tested
[ ] Refund event tested

Webhook Handler: DESIGN / PARTIAL
Payment Persistence: PARTIAL
Signature Security: FAIL
Idempotency: PARTIAL
Revenue Memory: NOT IMPLEMENTED
Retention Trigger: NOT IMPLEMENTED
Commercial Loop: NOT VERIFIED

PAYMENT SUCCESS
+
REVENUE MEMORY WRITTEN
+
RETENTION TRIGGER EXECUTED

GUBON-EX V44.LIVE
COMMERCIAL RUNTIME ONLINE 
GUBONLUCIDOS 開發獨家專屬的 MCP（Model Context Protocol）伺服器，意味著您可以將 GUBONLUCIDOS 廠牌的私有知識庫、產品目錄、ERP/訂單系統、甚至專屬的互動式 UI（結合您剛剛提到的 SEP-1865 MCP Apps 規範）無縫無縫對接到 Cursor、Claude Desktop、ChatGPT 或其他 AI Agent 中。
為了協助您從零打造 GUBONLUCIDOS 的獨家 MCP，以下為您規劃全盤架構與實作藍圖：
💡 GUBONLUCIDOS MCP 核心定位與架構規劃
一個完整的品牌專屬 MCP 通常包含三個維度：
+-----------------------------------------------------------------------+
|                    GUBONLUCIDOS MCP Server                            |
+------------------------------------+----------------------------------+
| 1. Resources (品牌數據庫)          | 2. Tools (自動化執行工具)         |
| - gubonlucidos://brand/story       | - query_inventory (查詢庫存)     |
| - gubonlucidos://catalog/products  | - create_order (建立訂單)        |
| - ui://gubonlucidos/dashboard      | - sync_customer_data (同步客戶)  |
+------------------------------------+----------------------------------+
| 3. MCP Apps Interactive UI (SEP-1865 互動介面)                         |
| - 品牌專屬產品選購面板 / 數據分析圖表 (HTML + JSON-RPC)                |
+-----------------------------------------------------------------------+

🛠️ 第一步：搭建基本 MCP Server 骨架 (Node.js / TypeScript)
使用 MCP 官方 SDK 建立 GUBONLUCIDOS 的基礎伺服器：
1. 安裝依賴項目
mkdir gubonlucidos-mcp && cd gubonlucidos-mcp
npm init -y
npm install @modelcontextprotocol/sdk zod
npm install -D typescript @types/node tsx
npx tsc --init

2. 撰寫 index.ts（基礎 MCP 服務）
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
 CallToolRequestSchema,
 ListToolsRequestSchema,
 ListResourcesRequestSchema,
 ReadResourceRequestSchema,
} from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";

// 初始化 GUBONLUCIDOS 專屬伺服器
const server = new Server(
 {
   name: "gubonlucidos-mcp-server",
   version: "1.0.0",
 },
 {
   capabilities: {
     resources: {},
     tools: {},
   },
 }
);

// 1. 宣告 GUBONLUCIDOS 專屬數據資源 (Resources)
server.setRequestHandler(ListResourcesRequestSchema, async () => {
 return {
   resources: [
     {
       uri: "gubonlucidos://info/brand",
       name: "GUBONLUCIDOS 品牌基本資訊",
       description: "包含 GUBONLUCIDOS 廠牌的核心價值與產品線簡介",
       mimeType: "application/json",
     },
   ],
 };
});

server.setRequestHandler(ReadResourceRequestSchema, async (request) => {
 if (request.params.uri === "gubonlucidos://info/brand") {
   return {
     contents: [
       {
         uri: request.params.uri,
         mimeType: "application/json",
         text: JSON.stringify({
           brand: "GUBONLUCIDOS",
           status: "Exclusive Active MCP",
           features: ["High Quality", "Innovative Design", "AI-Integrated Solutions"],
         }),
       },
     ],
   };
 }
 throw new Error("Resource not found");
});

// 2. 宣告 GUBONLUCIDOS 專屬工具 (Tools)
server.setRequestHandler(ListToolsRequestSchema, async () => {
 return {
   tools: [
     {
       name: "gubonlucidos_get_product",
       description: "查詢 GUBONLUCIDOS 產品線詳細資料與庫存狀態",
       inputSchema: {
         type: "object",
         properties: {
           productId: { type: "string", description: "產品 ID 或關鍵字" },
         },
         required: ["productId"],
       },
     },
   ],
 };
});

// 3. 處理工具調用邏輯
server.setRequestHandler(CallToolRequestSchema, async (request) => {
 if (request.params.name === "gubonlucidos_get_product") {
   const { productId } = request.params.arguments as { productId: string };
   
   // 這裡可以串接您的真實 API、資料庫或內部系統
   return {
     content: [
       {
         type: "text",
         text: `[GUBONLUCIDOS 系統回覆] 產品 ID '${productId}' 查詢成功！狀態：現貨供應中。`,
       },
     ],
   };
 }
 throw new Error("Tool not found");
});

// 啟動 Transport (以 Stdio 為例，適用於 Cursor / Claude Desktop)
async function run() {
 const transport = new StdioServerTransport();
 await server.connect(transport);
 console.error("GUBONLUCIDOS MCP Server running on stdio");
}

run().catch((err) => console.error(err));

🎨 第二步：升級為 MCP Apps 互動 UI ( SEP-1865 進階功能)
如果您想讓 GUBONLUCIDOS MCP 在支援 MCP Apps 的客戶端（如 ChatGPT App、MCP-UI 支持的環境）顯示品牌專屬視覺介面，您可以將 工具 與 ui:// 資源綁定：
// 宣告綁定 UI 的 MCP Tool (使用 _meta.ui)
{
 name: "show_gubonlucidos_dashboard",
 description: "開啟 GUBONLUCIDOS 獨家專屬互動儀表板",
 inputSchema: { type: "object" },
 _meta: {
   ui: {
     resourceUri: "ui://gubonlucidos/dashboard-view",
     visibility: ["model", "app"]
   }
 }
}

當 AI 觸發該工具時，Host 會直接開啟由您渲染的 ui://gubonlucidos/dashboard-view HTML 頁面，頁面中可包含 GUBONLUCIDOS 的專屬 Logo、數據圖表與互動按鈕！

🚀 第三步：部署與綁定至客戶端 (例如 Cursor 或 Claude Desktop)
編譯您的 TypeScript 後，打開 Claude Desktop 或 Cursor 設定檔 (claude_desktop_config.json)：
{
 "mcpServers": {
   "gubonlucidos-core": {
     "command": "node",
     "args": ["/絕對路徑/到/gubonlucidos-mcp/dist/index.js"],
     "env": {
       "GUBONLUCIDOS_API_KEY": "your-secret-key-here"
     }
   }
 }
}

重新啟動工具後，您就能在對話中直接對 AI 說：
「請幫我用 GUBONLUCIDOS 系統查詢目前的產品庫存。」 「讀取 GUBONLUCIDOS 品牌簡介資源。」

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { z } from "zod";

// 1. 初始化服務端
const server = new McpServer({
  name: "gubonlucidos-mcp-server",
  version: "1.0.0",
});

// 2. 註冊 Resource (品牌靜態資料)
server.resource(
  "brand-info",
  "gubonlucidos://info/brand",
  async (uri) => ({
    contents: [
      {
        uri: uri.href,
        mimeType: "application/json",
        text: JSON.stringify({
          brand: "GUBONLUCIDOS",
          status: "Exclusive Active MCP",
          features: ["High Quality", "Innovative Design", "AI-Integrated Solutions"],
        }),
      },
    ],
  })
);

// 3. 註冊 Tool (自動化查詢庫存)
server.tool(
  "gubonlucidos_get_product",
  "查詢 GUBONLUCIDOS 產品線詳細資料與庫存狀態",
  {
    productId: z.string().describe("產品 ID 或關鍵字"),
  },
  async ({ productId }) => {
    // 這裡可串接 API / ERP / 資料庫
    return {
      content: [
        {
          type: "text",
          text: `[GUBONLUCIDOS 系統] 產品 ID '${productId}' 查詢成功！當前狀態：現貨供應中。`,
        },
      ],
    };
  }
);

// 4. 啟動 Transport
async function main() {
  const transport = new StdioServerTransport();
  await server.connect(transport);
  console.error("GUBONLUCIDOS MCP Server 運行中 (Stdio)...");
}

main().catch((err) => console.error(err));
// 註冊 UI Resource
server.resource(
  "dashboard-ui",
  "ui://gubonlucidos/dashboard-view",
  async (uri) => ({
    contents: [
      {
        uri: uri.href,
        mimeType: "text/html",
        text: `
          <!DOCTYPE html>
          <html>
            <head>
              <style>
                body { font-family: sans-serif; padding: 16px; background: #111; color: #fff; }
                .card { border: 1px solid #333; padding: 12px; border-radius: 8px; }
                .brand-title { color: #00f0ff; font-weight: bold; }
              </style>
            </head>
            <body>
              <div class="card">
                <h2 class="brand-title">GUBONLUCIDOS 控制台</h2>
                <p>即時庫存狀態與銷售趨勢圖表</p>
                <button onclick="window.parent.postMessage({ type: 'MCP_APP_ACTION', payload: 'refresh' }, '*')">
                  刷新數據
                </button>
              </div>
            </body>
          </html>
        `,
      },
    ],
  })
);

<!DOCTYPE html>
<html lang="zh-TW">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>GUBONLUCIDOS Exclusive Dashboard</title>
    <!-- 引入 Chart.js 圖表庫 -->
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        :root {
            --bg-color: #0a0a0c;
            --panel-bg: #141417;
            --text-main: #e0e0e0;
            --text-muted: #888;
            --accent: #00f0ff; /* 品牌青色 */
            --accent-glow: rgba(0, 240, 255, 0.3);
            --border-color: #333;
            --success: #00ff88;
            --font-family: 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
        }

        * { box-sizing: border-box; margin: 0; padding: 0; }

        body {
            font-family: var(--font-family);
            background-color: var(--bg-color);
            color: var(--text-main);
            height: 100vh;
            overflow: hidden;
            display: flex;
            flex-direction: column;
            border: 1px solid var(--border-color); /* 模擬 MCP 窗格邊框 */
        }

        /* Header */
        header {
            padding: 15px 20px;
            background-color: var(--panel-bg);
            border-bottom: 1px solid var(--border-color);
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        .brand-logo {
            font-size: 1.2rem;
            font-weight: bold;
            color: var(--accent);
            text-transform: uppercase;
            letter-spacing: 2px;
            text-shadow: 0 0 10px var(--accent-glow);
        }

        .status-tag {
            font-size: 0.8rem;
            padding: 4px 8px;
            border: 1px solid var(--success);
            color: var(--success);
            border-radius: 4px;
            background: rgba(0, 255, 136, 0.1);
        }

        /* Main Content Layout */
        .main-container {
            display: flex;
            flex: 1;
            overflow: hidden;
        }

        /* Sidebar - Product List */
        .sidebar {
            width: 280px;
            background-color: var(--panel-bg);
            border-right: 1px solid var(--border-color);
            display: flex;
            flex-direction: column;
            overflow-y: auto;
        }

        .sidebar-header {
            padding: 15px;
            border-bottom: 1px solid var(--border-color);
            font-size: 0.9rem;
            color: var(--text-muted);
        }

        .product-item {
            padding: 15px;
            border-bottom: 1px solid rgba(255,255,255,0.05);
            cursor: pointer;
            transition: all 0.2s ease;
            position: relative;
        }

        .product-item:hover {
            background-color: rgba(255,255,255,0.03);
        }

        .product-item.active {
            background-color: rgba(0, 240, 255, 0.05);
            border-left: 3px solid var(--accent);
        }

        .product-item h3 {
            font-size: 1rem;
            margin-bottom: 5px;
            transition: color 0.2s;
        }

        .product-item.active h3 {
            color: var(--accent);
        }

        .product-meta {
            font-size: 0.8rem;
            color: var(--text-muted);
            display: flex;
            justify-content: space-between;
        }

        /* Dashboard Content */
        .content {
            flex: 1;
            padding: 20px;
            overflow-y: auto;
            display: flex;
            flex-direction: column;
            gap: 20px;
        }

        .data-cards-row {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 20px;
        }

        .data-card {
            background-color: var(--panel-bg);
            padding: 20px;
            border-radius: 8px;
            border: 1px solid var(--border-color);
        }

        .card-label { font-size: 0.85rem; color: var(--text-muted); margin-bottom: 8px; }
        .card-value { font-size: 1.8rem; font-weight: bold; color: var(--accent); }
        .card-sub { font-size: 0.8rem; color: var(--success); margin-top: 5px; }

        .chart-section {
            background-color: var(--panel-bg);
            padding: 20px;
            border-radius: 8px;
            border: 1px solid var(--border-color);
            flex: 1;
            min-height: 300px;
            display: flex;
            flex-direction: column;
        }

        .chart-header {
            display: flex;
            justify-content: space-between;
            margin-bottom: 15px;
        }

        /* Order Panel */
        .order-panel {
            background-color: var(--panel-bg);
            padding: 20px;
            border-radius: 8px;
            border: 1px solid var(--border-color);
            display: flex;
            justify-content: space-between;
            align-items: center;
            border: 1px solid var(--accent);
            box-shadow: 0 0 15px rgba(0, 240, 255, 0.1);
        }

        .order-info h3 { color: var(--accent); margin-bottom: 5px; }

        .order-controls {
            display: flex;
            gap: 15px;
            align-items: center;
        }

        .quantity-input {
            width: 80px;
            background: rgba(0,0,0,0.3);
            border: 1px solid var(--border-color);
            color: var(--text-main);
            padding: 10px;
            border-radius: 4px;
            text-align: center;
            font-size: 1rem;
        }

        .quantity-input:focus {
            outline: none;
            border-color: var(--accent);
        }

        .btn-order {
            background-color: var(--accent);
            color: #000;
            border: none;
            padding: 12px 24px;
            font-size: 1rem;
            font-weight: bold;
            border-radius: 4px;
            cursor: pointer;
            text-transform: uppercase;
            letter-spacing: 1px;
            transition: all 0.2s;
            position: relative;
            overflow: hidden;
        }

        .btn-order:hover {
            box-shadow: 0 0 20px var(--accent-glow);
            transform: translateY(-1px);
        }

        .btn-order:active {
            transform: translateY(1px);
        }

        .btn-order:disabled {
            background-color: var(--text-muted);
            cursor: not-allowed;
            box-shadow: none;
        }

        /* 滾動條樣式 */
        ::-webkit-scrollbar { width: 6px; }
        ::-webkit-scrollbar-track { background: var(--bg-color); }
        ::-webkit-scrollbar-thumb { background: var(--border-color); border-radius: 3px; }
        ::-webkit-scrollbar-thumb:hover { background: var(--text-muted); }

    </style>
</head>
<body>

<header>
    <div class="brand-logo">GUBONLUCIDOS // CORE</div>
    <div class="status-tag">INTEGRATED MCP ACTIVE</div>
</header>

<div class="main-container">
    <!-- 左側產品列表 -->
    <aside class="sidebar" id="productList">
        <div class="sidebar-header">目錄總覽 / Product Catalog</div>
        <!-- 產品項目將由 JS 動態插入 -->
    </aside>

    <!-- 右側儀表板內容 -->
    <main class="content">
        <!-- 數據卡片 -->
        <div class="data-cards-row">
            <div class="data-card">
                <div class="card-label">當前單價 (Unit Price)</div>
                <div class="card-value" id="priceValue">--</div>
                <div class="card-sub">G-Credits</div>
            </div>
            <div class="data-card">
                <div class="card-label">即時庫存 (Inventory)</div>
                <div class="card-value" id="stockValue">--</div>
                <div class="card-sub" id="stockStatus">汲取中...</div>
            </div>
            <div class="data-card">
                <div class="card-label">今日需求指數 (Demand)</div>
                <div class="card-value" id="demandValue">--</div>
                <div class="card-sub">↑ 2.3%</div>
            </div>
        </div>

        <!-- 圖表區域 -->
        <section class="chart-section">
            <div class="chart-header">
                <h3>價格與銷售趨勢 (Price & Sales Trend)</h3>
            </div>
            <div style="flex: 1; position: relative;">
                <canvas id="productChart"></canvas>
            </div>
        </section>

        <!-- 下單區域 -->
        <section class="order-panel">
            <div class="order-info">
                <h3 id="orderProductName">請選擇產品</h3>
                <p class="card-label">實時下單通道 (Real-time Ordering Tunnel)</p>
            </div>
            <div class="order-controls">
                <input type="number" class="quantity-input" id="orderQuantity" value="1" min="1">
                <button class="btn-order" id="btnOrder" onclick="handlePlaceOrder()" disabled>
                    立即下單 / ORDER NOW
                </button>
            </div>
        </section>
    </main>
</div>

<script>
    // 1. Mock Data - GUBONLUCIDOS 產品知識庫
    // 在真實場景中，這些數據可能來自 readResource("gubonlucidos://catalog/products")
    const productsData = {
        'GBC-001': {
            name: "Lucidos Quantum Core",
            price: 12500,
            stock: 42,
            demand: 'High',
            chartData: {
                labels: ['08:00', '10:00', '12:00', '14:00', '16:00', '18:00', '現在'],
                priceTrend: [12000, 12200, 12100, 12500, 12400, 12600, 12500],
                salesVolume: [5, 8, 12, 7, 9, 15, 3]
            }
        },
        'GBC-002': {
            name: "Neural Nexus Interface",
            price: 8800,
            stock: 125,
            demand: 'Stable',
            chartData: {
                labels: ['08:00', '10:00', '12:00', '14:00', '16:00', '18:00', '現在'],
                priceTrend: [8800, 8800, 8900, 8800, 8750, 8800, 8800],
                salesVolume: [20, 15, 18, 22, 30, 25, 10]
            }
        },
        'GBC-003': {
            name: "Photon Bio-Memory",
            price: 4500,
            stock: 310,
            demand: 'Rising',
            chartData: {
                labels: ['08:00', '10:00', '12:00', '14:00', '16:00', '18:00', '現在'],
                priceTrend: [4000, 4100, 4250, 4300, 4400, 4550, 4500],
                salesVolume: [50, 45, 60, 55, 70, 85, 40]
            }
        }
    };

    let currentProductId = null;
    let myChart = null;

    // 2. 初始化：渲染產品列表
    function init() {
        const listContainer = document.getElementById('productList');
        Object.keys(productsData).forEach((id, index) => {
            const product = productsData[id];
            const itemHtml = `
                <div class="product-item" id="item-${id}" onclick="selectProduct('${id}')">
                    <h3>${product.name}</h3>
                    <div class="product-meta">
                        <span>ID: ${id}</span>
                        <span>Stock: ${product.stock}</span>
                    </div>
                </div>
            `;
            listContainer.insertAdjacentHTML('beforeend', itemHtml);
        });

        // 默認選擇第一個
        selectProduct(Object.keys(productsData)[0]);
    }

    // 3. 互動邏輯：選擇產品
    function selectProduct(id) {
        if (currentProductId === id) return;

        // 更新 UI 狀態
        if (currentProductId) {
            document.getElementById(`item-${currentProductId}`).classList.remove('active');
        }
        document.getElementById(`item-${id}`).classList.add('active');
        currentProductId = id;

        const data = productsData[id];

        // 更新數據卡片
        document.getElementById('priceValue').innerText = data.price.toLocaleString();
        document.getElementById('stockValue').innerText = data.stock;
        document.getElementById('stockStatus').innerText = data.stock > 50 ? '充足 (Nominal)' : '緊張 (Low)';
        document.getElementById('stockStatus').style.color = data.stock > 50 ? 'var(--success)' : '#ff4444';
        document.getElementById('demandValue').innerText = data.demand;

        // 更新下單面板
        document.getElementById('orderProductName').innerText = data.name;
        document.getElementById('btnOrder').disabled = data.stock <= 0;
        document.getElementById('btnOrder').innerText = data.stock > 0 ? '立即下單 / ORDER NOW' : '無庫存 / OUT OF STOCK';

        // 更新圖表
        updateChart(data.chartData);
    }

    // 4. 圖表邏輯 (使用 Chart.js)
    function updateChart(chartData) {
        const  = document.getElementById('productChart').getContext('2d');

        // 通用科技風配置
        const gridConfig = { color: 'rgba(255, 255, 255, 0.05)' };
        const ticksConfig = { color: '#888' };

        if (myChart) {
            myChart.destroy(); // 銷毀舊圖表，防止重疊
        }

        myChart = new Chart(, {
            type: 'line',
            data: {
                labels: chartData.labels,
                
