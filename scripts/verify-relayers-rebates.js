#!/usr/bin/env node
const { Connection, PublicKey } = require('@solana/web3.js');

const RELAYERS = {
  helius: {
    url: process.env.HELIUS_API_KEY ? `https://mainnet.helius-rpc.com/?api-key=${process.env.HELIUS_API_KEY}` : null,
    submit: 'https://api.helius.xyz/v0/transactions/submit',
    feePayer: 'HeLiuSrpc1111111111111111111111111111111111'
  },
  quicknode: {
    url: process.env.QUICKNODE_ENDPOINT || null
  }
};

const REBATE_ACCOUNTS = [
  'FVhQ3QHvXudWSdGix2sdcG47YmrmUxRhf3KCBmiKfekf',
  'CvQZZ23qYDWF2RUpxYJ8y9K4skmuvYEEjH7fK58jtipQ',
  '7ZyDFzet6sKgZLN4D89JLfo7chu2n7nYdkFt5RCFk8Sf'
];

async function verifyRelayersAndRebates() {
  console.log('🔍 Verifying Relayers & Rebates\n');
  console.log('━'.repeat(60));

  // Check Relayers
  console.log('\n🚀 Relayer Status:');
  
  for (const [name, config] of Object.entries(RELAYERS)) {
    if (config.url) {
      try {
        const response = await fetch(config.url, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'getHealth' })
        });
        const data = await response.json();
        console.log(`✅ ${name}: ${data.result || 'OK'}`);
      } catch (e) {
        console.log(`❌ ${name}: ${e.message}`);
      }
    } else {
      console.log(`⚠️  ${name}: No API key configured`);
    }
  }

  // Check Rebates
  console.log('\n💰 Rebate Earnings:');
  const connection = new Connection('https://api.mainnet-beta.solana.com', 'confirmed');
  let totalRebates = 0;

  for (const addr of REBATE_ACCOUNTS) {
    try {
      const balance = await connection.getBalance(new PublicKey(addr));
      const sol = balance / 1e9;
      totalRebates += sol;
      console.log(`✅ ${addr.slice(0, 8)}...: ${sol.toFixed(6)} SOL`);
    } catch (e) {
      console.log(`❌ ${addr.slice(0, 8)}...: Error`);
    }
  }

  console.log('\n━'.repeat(60));
  console.log(`\n📊 Total Rebates: ${totalRebates.toFixed(6)} SOL`);
  console.log(`💵 USD Value: $${(totalRebates * 200).toFixed(2)} (@$200/SOL)`);

  return { relayers: RELAYERS, totalRebates };
}

verifyRelayersAndRebates().catch(console.error);
