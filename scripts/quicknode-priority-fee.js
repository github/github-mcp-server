#!/usr/bin/env node
const { Connection } = require('@solana/web3.js');

const QUICKNODE_RPC = process.env.QUICKNODE_ENDPOINT || 'https://api.mainnet-beta.solana.com';

async function getQuickNodePriorityFee() {
  try {
    const response = await fetch(QUICKNODE_RPC, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        jsonrpc: '2.0',
        id: 1,
        method: 'qn_estimatePriorityFees',
        params: {
          last_n_blocks: 100,
          account: 'JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4'
        }
      })
    });
    
    const data = await response.json();
    const fees = data.result?.per_compute_unit || {};
    
    console.log('⚡ QuickNode Priority Fees\n');
    console.log('Low:', fees.low || 0, 'micro-lamports');
    console.log('Medium:', fees.medium || 0, 'micro-lamports');
    console.log('High:', fees.high || 0, 'micro-lamports');
    console.log('\n✅ Recommended: Use LOW (0) for zero-cost transfer');
    
    return fees.low || 0;
  } catch (error) {
    console.log('ℹ️  Using fallback: 0 priority fee');
    return 0;
  }
}

getQuickNodePriorityFee();
