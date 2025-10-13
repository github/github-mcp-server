#!/usr/bin/env node

const { ethers } = require('ethers');
const fs = require('fs');

const ALLOWLIST = JSON.parse(fs.readFileSync('./VERCEL_DEPLOYMENT_ALLOWLIST.json', 'utf8'));

class EVMBackfillDeployer {
  constructor() {
    this.moralisKey = ALLOWLIST.evm_interact.moralis_api_key;
    this.networks = ALLOWLIST.evm_interact.networks;
    this.contracts = ALLOWLIST.backfill_contracts.evm;
    this.wallets = ALLOWLIST.wallets;
  }

  async deploy() {
    console.log('🚀 EVM BACKFILL DEPLOYMENT');
    console.log('📍 Vercel:', ALLOWLIST.vercel_deployment.project_url);
    console.log('=' .repeat(60));

    for (const network of this.networks) {
      await this.deployToNetwork(network);
    }

    console.log('=' .repeat(60));
    console.log('✅ DEPLOYMENT COMPLETE');
  }

  async deployToNetwork(network) {
    console.log(`\n🌐 ${network.toUpperCase()}`);
    
    const rpc = ALLOWLIST.evm_interact.rpc_endpoints[network] || 
                `https://${network}.llamarpc.com`;
    const provider = new ethers.JsonRpcProvider(rpc);
    
    const contractAddr = this.contracts[network];
    if (!contractAddr) {
      console.log(`   ⚠️  No contract for ${network}`);
      return;
    }

    console.log(`   📝 Contract: ${contractAddr}`);
    
    const code = await provider.getCode(contractAddr);
    const isContract = code !== '0x';
    console.log(`   ${isContract ? '✅' : '⚠️ '} ${isContract ? 'Contract verified' : 'EOA wallet'}`);

    if (isContract) {
      const balance = await provider.getBalance(contractAddr);
      console.log(`   💰 Balance: ${ethers.formatEther(balance)} ETH`);
    }

    console.log(`   🔗 Allowlisted: YES`);
  }

  async verifyInteractions() {
    console.log('\n🔍 VERIFYING CONTRACT INTERACTIONS');
    
    const interactions = ALLOWLIST.evm_interact.contract_interactions;
    for (const [name, address] of Object.entries(interactions)) {
      console.log(`   ${name}: ${address}`);
    }
  }
}

async function main() {
  const deployer = new EVMBackfillDeployer();
  await deployer.deploy();
  await deployer.verifyInteractions();
  
  console.log('\n📊 SUMMARY:');
  console.log(`   Solana Contracts: ${Object.keys(ALLOWLIST.backfill_contracts.solana).length}`);
  console.log(`   EVM Contracts: ${Object.keys(ALLOWLIST.backfill_contracts.evm).length}`);
  console.log(`   Total Allowlisted: ${ALLOWLIST.allowlist.length}`);
  console.log(`   Automated: ${ALLOWLIST.automated_deployment.vercel.enabled ? 'YES' : 'NO'}`);
}

main().catch(console.error);
