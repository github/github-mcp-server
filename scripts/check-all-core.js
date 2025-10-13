#!/usr/bin/env node
const { Connection, PublicKey } = require('@solana/web3.js');

const CORE_ADDRESSES = {
  programs: [
    { name: 'Jupiter Aggregator v6', address: 'JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4' },
    { name: 'Jupiter Program Data', address: '4Ec7ZxZS6Sbdg5UGSLHbAnM7GQHp2eFd4KYWRexAipQT' },
    { name: 'Squads V3', address: 'SMPLecH534NA9acpos4G6x7uf3LWbCAwZQE9e8ZekMu' }
  ],
  authorities: [
    { name: 'Current Authority', address: 'CvQZZ23qYDWF2RUpxYJ8y9K4skmuvYEEjH7fK58jtipQ' },
    { name: 'New Master Controller', address: 'GLzZk1sczzW6fM4uPFeQCtTZQaf8H5VaBt99tUMbJAAW' },
    { name: 'Multisig Account', address: '7ZyDFzet6sKgZLN4D89JLfo7chu2n7nYdkFt5RCFk8Sf' }
  ],
  income: [
    { name: 'Rebate Account 1', address: 'FVhQ3QHvXudWSdGix2sdcG47YmrmUxRhf3KCBmiKfekf' }
  ],
  multisigMembers: [
    '2MgqMXdwSf3bRZ6S8uKJSffZAaoZBhD2mjst3phJXE7p',
    '89FnbsKH8n6FXCghGUijxh3snqx3e6VXJ7q1fQAHWkQQ',
    'BYidGfUnfoQtqi4nHiuo57Fjreizbej6hawJLnbwJmYr',
    'CHRDWWqUs6LyeeoD7pJb3iRfnvYeMfwMUtf2N7zWk7uh',
    'Dg5NLa5JuwfRMkuwZEguD9RpVrcQD3536GxogUv7pLNV',
    'EhJqf1p39c8NnH5iuZAJyw778LQua1AhZWxarT5SF8sT',
    'GGG2JyBtwbPAsYwUQED8GBbj9UMi7NQa3uwN3DmyGNtz'
  ]
};

async function checkAllCore() {
  const connection = new Connection('https://api.mainnet-beta.solana.com', 'confirmed');
  
  console.log('🔍 Core System Check\n');
  console.log('━'.repeat(60));

  const results = { working: 0, failed: 0, total: 0 };

  // Check Programs
  console.log('\n📦 Programs:');
  for (const prog of CORE_ADDRESSES.programs) {
    results.total++;
    try {
      const info = await connection.getAccountInfo(new PublicKey(prog.address));
      if (info) {
        console.log(`✅ ${prog.name}`);
        console.log(`   ${prog.address}`);
        console.log(`   Executable: ${info.executable}`);
        results.working++;
      } else {
        console.log(`❌ ${prog.name} - Not found`);
        results.failed++;
      }
    } catch (e) {
      console.log(`❌ ${prog.name} - Error: ${e.message}`);
      results.failed++;
    }
  }

  // Check Authorities
  console.log('\n🔐 Authorities:');
  for (const auth of CORE_ADDRESSES.authorities) {
    results.total++;
    try {
      const balance = await connection.getBalance(new PublicKey(auth.address));
      console.log(`✅ ${auth.name}`);
      console.log(`   ${auth.address}`);
      console.log(`   Balance: ${(balance / 1e9).toFixed(6)} SOL`);
      results.working++;
    } catch (e) {
      console.log(`❌ ${auth.name} - Error: ${e.message}`);
      results.failed++;
    }
  }

  // Check Income Accounts
  console.log('\n💰 Income Accounts:');
  for (const inc of CORE_ADDRESSES.income) {
    results.total++;
    try {
      const balance = await connection.getBalance(new PublicKey(inc.address));
      console.log(`✅ ${inc.name}`);
      console.log(`   ${inc.address}`);
      console.log(`   Balance: ${(balance / 1e9).toFixed(6)} SOL`);
      results.working++;
    } catch (e) {
      console.log(`❌ ${inc.name} - Error: ${e.message}`);
      results.failed++;
    }
  }

  // Check Multisig Members
  console.log('\n👥 Multisig Members (7):');
  for (let i = 0; i < CORE_ADDRESSES.multisigMembers.length; i++) {
    results.total++;
    try {
      const balance = await connection.getBalance(new PublicKey(CORE_ADDRESSES.multisigMembers[i]));
      console.log(`✅ Member ${i + 1}: ${(balance / 1e9).toFixed(4)} SOL`);
      results.working++;
    } catch (e) {
      console.log(`❌ Member ${i + 1} - Error`);
      results.failed++;
    }
  }

  console.log('\n' + '━'.repeat(60));
  console.log('\n📊 System Status:');
  console.log(`   ✅ Working: ${results.working}/${results.total}`);
  console.log(`   ❌ Failed: ${results.failed}/${results.total}`);
  console.log(`   📈 Success Rate: ${((results.working / results.total) * 100).toFixed(1)}%`);

  if (results.failed === 0) {
    console.log('\n🎉 ALL CORE SYSTEMS OPERATIONAL!');
  } else {
    console.log('\n⚠️  Some systems need attention');
  }

  return results;
}

checkAllCore().catch(console.error);
