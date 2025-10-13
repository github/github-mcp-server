#!/usr/bin/env node
const fs = require('fs');
const { Connection, PublicKey } = require('@solana/web3.js');

const manifest = JSON.parse(fs.readFileSync('DEPLOYMENT_MANIFEST.json', 'utf8'));
const connection = new Connection('https://api.mainnet-beta.solana.com', 'confirmed');

async function deployMultiProgram() {
  console.log('🚀 Multi-Program Deployment\n');
  console.log('Deployment ID:', manifest.deployment_id);
  console.log('New Master Controller:', manifest.new_master_controller);
  console.log('━'.repeat(60));

  const results = { verified: [], failed: [], total: 0 };

  // Verify Solana Programs
  console.log('\n📦 Verifying Solana Programs...');
  for (const [name, prog] of Object.entries(manifest.solana_programs.owned)) {
    results.total++;
    try {
      const info = await connection.getAccountInfo(new PublicKey(prog.address));
      if (info) {
        console.log(`✅ ${name}: ${prog.address}`);
        results.verified.push({ type: 'solana', name, address: prog.address });
      } else {
        console.log(`❌ ${name}: Not found`);
        results.failed.push({ type: 'solana', name, address: prog.address });
      }
    } catch (e) {
      console.log(`❌ ${name}: Error`);
      results.failed.push({ type: 'solana', name, error: e.message });
    }
  }

  // Verify Bot Army
  console.log('\n🤖 Verifying Bot Army...');
  for (const [name, bot] of Object.entries(manifest.bot_army)) {
    results.total += 2;
    try {
      const botBalance = await connection.getBalance(new PublicKey(bot.bot));
      const contractInfo = await connection.getAccountInfo(new PublicKey(bot.contract));
      console.log(`✅ ${name}`);
      console.log(`   Bot: ${bot.bot} (${(botBalance / 1e9).toFixed(4)} SOL)`);
      console.log(`   Contract: ${bot.contract}`);
      results.verified.push({ type: 'bot', name, bot: bot.bot, contract: bot.contract });
    } catch (e) {
      console.log(`❌ ${name}: Error`);
      results.failed.push({ type: 'bot', name, error: e.message });
    }
  }

  // Check Income Accounts
  console.log('\n💰 Checking Income Accounts...');
  let totalIncome = 0;
  for (const [name, acc] of Object.entries(manifest.income_accounts)) {
    results.total++;
    try {
      const balance = await connection.getBalance(new PublicKey(acc.address));
      const sol = balance / 1e9;
      totalIncome += sol;
      console.log(`✅ ${name}: ${sol.toFixed(6)} SOL`);
      results.verified.push({ type: 'income', name, balance: sol });
    } catch (e) {
      console.log(`❌ ${name}: Error`);
      results.failed.push({ type: 'income', name, error: e.message });
    }
  }

  console.log('\n' + '━'.repeat(60));
  console.log('\n📊 Deployment Summary:');
  console.log(`   ✅ Verified: ${results.verified.length}/${results.total}`);
  console.log(`   ❌ Failed: ${results.failed.length}/${results.total}`);
  console.log(`   💰 Total Income: ${totalIncome.toFixed(6)} SOL`);
  console.log(`   📈 Success Rate: ${((results.verified.length / results.total) * 100).toFixed(1)}%`);

  console.log('\n🎯 Next Steps:');
  manifest.deployment_steps.forEach((step, i) => console.log(`   ${step}`));

  if (results.failed.length === 0) {
    console.log('\n🎉 ALL SYSTEMS READY FOR DEPLOYMENT!');
  }

  return results;
}

deployMultiProgram().catch(console.error);
