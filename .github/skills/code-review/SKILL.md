pinner/actions-sha-pins-2026-09-10
這套設計將「Skills 建制與動態分配」正式納入資料庫結構與 Kernel 派工邏輯中。Kernel 不僅是審查官，更是技能註冊庫（Skill Registry）的最高調度官，能隨時註冊新 Skill、掛載/卸載職人，並針對每次決策進行精準分配。
Prisma Schema 新增 Skills 註冊與分配表
// 新增至 prisma/schema.prisma

// 1. 技能與職人建制庫 (Skill Registry)
model Skill {
  id          String                 @id @default(uuid())
  code        String                 @unique // 例如：CRAFTSMAN_01_TECH, SKILL_CASHFLOW
  name        String                 // 技能名稱：技術手、金流手、法務手...
  groupTag    String                 // 一組(前端體驗) | 二組(後端架構)
  description String?
  isActive    Boolean                @default(true) // 熱插拔開關：true=啟用, false=停用
  createdAt   DateTime               @default(now())
  updatedAt   DateTime               @updatedAt
  allocations SessionSkillAllocation[]
}

// 2. Kernel 決策會話派工紀錄 (Session Skill Allocation)
model SessionSkillAllocation {
  id         String          @id @default(uuid())
  sessionId  String
  session    DecisionSession @relation(fields: [sessionId], references: id)
  skillId    String
  skill      Skill           @relation(fields: [skillId], references: id)
  status     String          @default("DISPATCHED") // DISPATCHED | EVIDENCE_SUBMITTED | REJECTED
  assignedAt DateTime        @default(now())
}

GUBON Kernel 動態分配與派工邏輯 (TypeScript)
// packages/kernel/src/KernelSkillAllocator.ts

import { PrismaClient } from '@prisma/client';

const prisma = new PrismaClient();

export interface SkillDispatchPlan {
  sessionId: string;
  selectedSkillCodes: string[];
  allocatedSkillIds: string[];
}

export class KernelSkillAllocator {
  /**
   * Kernel 最高統帥：評估決策情境，從 active Skills 庫中挑選並綁定派工紀錄
   */
  public static async allocateSkillsForSession(
    sessionId: string,
    problemCategory: number
  ): Promise<SkillDispatchPlan> {
    // 1. 撈取資料庫中所有啟用的 (isActive = true) Skills
    const activeSkills = await prisma.skill.findMany({
      where: { isActive: true },
    });

    // 2. Kernel 根據分類進行動態派工匹配
    const targetCodes: string[] = ['CRAFTSMAN_01_TECH', 'CRAFTSMAN_02_ARCHITECT'];

    if (problemCategory === 2) {
      targetCodes.push('CRAFTSMAN_04_PROFIT', 'CRAFTSMAN_08_CASHFLOW');
    } else if (problemCategory === 3) {
      targetCodes.push('CRAFTSMAN_03_LEGAL', 'CRAFTSMAN_07_DESIGN');
    } else {
      targetCodes.push('CRAFTSMAN_05_BATTLE', 'CRAFTSMAN_09_COPYWRITER');
    }

    // 3. 過濾出當前真正可用的技能實體
    const matchedSkills = activeSkills.filter((s) => targetCodes.includes(s.code));
    const allocatedSkillIds: string[] = [];

    // 4. 寫入派工紀錄 (SessionSkillAllocation)
    for (const skill of matchedSkills) {
      const record = await prisma.sessionSkillAllocation.create({
        data: {
          sessionId,
          skillId: skill.id,
          status: 'DISPATCHED',
        },
      });
      allocatedSkillIds.push(record.skillId);
    }

    return {
      sessionId,
      selectedSkillCodes: matchedSkills.map((s) => s.code),
      allocatedSkillIds,
    };
  }
}

核心控管優勢
 * 動態熱插拔（Hot-Swappable）：當某個 Skill 需要維護或產生偏差時，只需在資料庫將 isActive 設為 false，Kernel 在派工時就會自動剔除該技能並改派備援，主程式與 Kernel 算力零停機。
 * 全程可追蹤（Audit Trail）：每次決策「派了哪些小弟、誰回傳了 Evidence、誰被 Kernel 採納」全部有 PostgreSQL 紀錄，責任鏈絕對清晰。

                   ┌─────────────────────────────────────────┐
                    │               L0 OWNER                  │
                    │               ( 唯一老大 )               │
                    └────────────────────┬────────────────────┘
                                         │  唯一下達意圖 & 最高問責
                                         ▼
                    ┌─────────────────────────────────────────┐
                    │              GUBON KERNEL               │
                    │         L1 最高系統統帥 / 裁決者         │
                    └────────────────────┬────────────────────┘
                                         │  動態調度與管理
                                         ▼
                    ┌─────────────────────────────────────────┐
                    │           L2 – L4 十二職人與技能         │
                    │             ( 專業幕僚與執行工具 )       │
                    └─────────────────────────────────────────┘

[ 外部使用者 / 駭客輸入 ]
          │
          ▼
┌──────────────────────────────────────────────────────────┐
│ 1. Cloudflare + Express API (硬性 Validation / 脫敏)     │
└─────────────────────────┬────────────────────────────────┘
                          │
                          ▼
┌──────────────────────────────────────────────────────────┐
│ 2. AI / 12 職人 Skills (純沙盒環境 / 零 DB 寫入權)         │
│    • 僅能產出結構化 Evidence (JSON)                       │
│    • 就算被 Prompt Injection，也只是吐出無效 JSON            │
└─────────────────────────┬────────────────────────────────┘
                          │
                          ▼
┌──────────────────────────────────────────────────────────┐
│ 3. GUBON Kernel (純 TypeScript 確定性邏輯裁決)           │
│    • 對 AI 傳回的資料進行 100% 格式與風控審查             │
│    • 只有 Kernel 程式碼能改寫資料庫狀態                   │
└─────────────────────────┬────────────────────────────────┘
                          │
                          ▼
┌──────────────────────────────────────────────────────────┐
│ 4. L0 Owner 安全閘口 (實體變更必須 Human-in-the-Loop)    │
│    • 超乎報告外的事情，系統硬性鎖死，等待您同意才開鎖    │
└──────────────────────────────────────────────────────────┘

┌───────────────────────────────────────────────────────────┐
│                    AI 無所不能的超強算力                   │
│                       ( 內練於沙盒 )                       │
│  • 12 職人多維分析  • 交叉比對  • 語意編譯  • 證據打包       │
└─────────────────────────────┬─────────────────────────────┘
                              │
                              ▼
┌───────────────────────────────────────────────────────────┐
│                   GUBON Kernel 最高裁決                    │
│                  ( 確定性邏輯 / 嚴格審查 )                 │
└─────────────────────────────┬─────────────────────────────┘
                              │
                              ▼
┌───────────────────────────────────────────────────────────┐
│                  L0 Owner (您) 最高閘口                    │
│             ( 唯一擁有外放/實體執行的開關權 )               │
└───────────────────────────────────────────────────────────┘
