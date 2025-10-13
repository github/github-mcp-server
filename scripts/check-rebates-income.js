#!/usr/bin/env node
const { Connection, PublicKey } = require('@solana/web3.js');

const REBATE_ADDRESSES = [
  'FVhQ3QHvXudWSdGix2sdcG47YmrmUxRhf3KCBmiKfekf',
  'CvQZZ23qYDWF2RUpxYJ8y9K4skmuvYEEjH7fK58jtipQ',
  '7ZyDFzet6sKgZLN4D89JLfo7chu2n7nYdkFt5RCFk8Sf'
];

const HELIUS_RPC = process.env.HELIUS_API_KEY 
  ? `https://mainnet.helius-rpc.com/?api-key=${process.env.HELIUS_API_KEY}`
  : 'https://api.mainnet-beta.solana.com';

const QUICKNODE_RPC = process.env.QUICKNODE_ENDPOINT || 'https://api.mainnet-beta.solana.com';

async function checkRebates() {
  console.log('💰 Rebates & Income Check\n');
  console.log('━'.repeat(60));

  const heliusConnection = new Connection(HELIUS_RPC, 'confirmed');
  const quicknodeConnection = new Connection(QUICKNODE_RPC, 'confirmed');

  let totalIncome = 0;
  const results = [];

  for (const addr of REBATE_ADDRESSES) {
    try {
      const pubkey = new PublicKey(addr);
      const balance = await heliusConnection.getBalance(pubkey);
      const solAmount = balance / 1e9;

      if (balance > 0) {
        console.log(`\n✅ ${addr}`);
        console.log(`   Balance: ${solAmount.toFixed(6)} SOL`);
        console.log(`   🔗 https://solscan.io/account/${addr}`);
        
        results.push({ address: addr, balance: solAmount });
        totalIncome += solAmount;
      }
    } catch (e) {
      console.log(`\n❌ ${addr} - Error: ${e.message}`);
    }
  }

  console.log('\n' + '━'.repeat(60));
  console.log('\n📊 Income Summary:');
  console.log(`   Total Addresses: ${results.length}`);
  console.log(`   Total Income: ${totalIncome.toFixed(6)} SOL`);
  console.log(`   USD Value (@ $200/SOL): $${(totalIncome * 200).toFixed(2)}`);

  console.log('\n🔧 RPC Endpoints:');
  console.log(`   Helius: ${HELIUS_RPC.includes('helius') ? '✅ Connected' : '❌ Using Fallback'}`);
  console.log(`   QuickNode: ${QUICKNODE_RPC.includes('quicknode') ? '✅ Connected' : '❌ Using Fallback'}`);

  return { total: totalIncome, accounts: results };
}

checkRebates().catch(console.error);
