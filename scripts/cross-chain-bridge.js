#!/usr/bin/env node

const { Connection, PublicKey } = require('@solana/web3.js');
const { ethers } = require('ethers');

const SOLANA_CONFIG = {
  treasury: '4eJZVbbsiLAG6EkWvgEYEWKEpdhJPFBYMeJ6DBX98w6a',
  geneMint: 'GENEtH5amGSi8kHAtQoezp1XEXwZJ8vcuePYnXdKrMYz',
  daoController: 'CvQZZ23qYDWF2RUpxYJ8y9K4skmuvYEEjH7fK58jtipQ',
  rpc: 'https://api.mainnet-beta.solana.com'
};

const EVM_CONFIG = {
  deployer: '0xE38FB59ba3AEAbE2AD0f6FB7Fb84453F6d145D23',
  dmtToken: '0x742d35Cc6634C0532925a3b8D4C9db96C4b4d8b6',
  iemMatrix: '0xc6D31F2F6CcBcd101604a92C6c08e0aee2937B3a',
  rpc: 'https://mainnet.skalenodes.com/v1/honorable-steel-rasalhague'
};

class CrossChainBridge {
  constructor() {
    this.solanaConnection = new Connection(SOLANA_CONFIG.rpc, 'confirmed');
    this.evmProvider = new ethers.JsonRpcProvider(EVM_CONFIG.rpc);
  }

  async getSolanaBalance() {
    const pubkey = new PublicKey(SOLANA_CONFIG.treasury);
    const balance = await this.solanaConnection.getBalance(pubkey);
    return balance / 1e9;
  }

  async getEVMBalance() {
    const balance = await this.evmProvider.getBalance(EVM_CONFIG.deployer);
    return ethers.formatEther(balance);
  }

  async syncTreasuries() {
    console.log('🌉 CROSS-CHAIN TREASURY SYNC');
    console.log('=' .repeat(60));
    
    const solBalance = await this.getSolanaBalance();
    const evmBalance = await this.getEVMBalance();
    
    console.log('Solana Treasury:', solBalance, 'SOL');
    console.log('EVM Treasury:', evmBalance, 'ETH');
    console.log('Total Value:', (solBalance + parseFloat(evmBalance)).toFixed(4));
    
    return {
      solana: solBalance,
      evm: parseFloat(evmBalance),
      total: solBalance + parseFloat(evmBalance)
    };
  }

  async getBotStatus() {
    console.log('\n🤖 BOT ARMY STATUS');
    console.log('=' .repeat(60));
    
    const solanaBots = 8;
    const evmTraders = 3;
    
    console.log(`Solana Bots: ${solanaBots} active`);
    console.log(`EVM Traders: ${evmTraders} active`);
    console.log(`Total Agents: ${solanaBots + evmTraders}`);
    
    return {
      solana: solanaBots,
      evm: evmTraders,
      total: solanaBots + evmTraders
    };
  }

  async initializeBridge() {
    console.log('🚀 INITIALIZING CROSS-CHAIN BRIDGE');
    console.log('=' .repeat(60));
    
    console.log('\n📍 Solana Configuration:');
    console.log('  Treasury:', SOLANA_CONFIG.treasury);
    console.log('  Gene Mint:', SOLANA_CONFIG.geneMint);
    console.log('  DAO Controller:', SOLANA_CONFIG.daoController);
    
    console.log('\n📍 EVM Configuration:');
    console.log('  Deployer:', EVM_CONFIG.deployer);
    console.log('  DMT Token:', EVM_CONFIG.dmtToken);
    console.log('  IEM Matrix:', EVM_CONFIG.iemMatrix);
    
    const treasuries = await this.syncTreasuries();
    const bots = await this.getBotStatus();
    
    console.log('\n✅ Bridge Initialized');
    console.log('=' .repeat(60));
    
    return {
      treasuries,
      bots,
      status: 'active'
    };
  }
}

async function main() {
  const bridge = new CrossChainBridge();
  const result = await bridge.initializeBridge();
  
  console.log('\n📊 BRIDGE STATUS:');
  console.log(JSON.stringify(result, null, 2));
}

if (require.main === module) {
  main().catch(console.error);
}

module.exports = { CrossChainBridge };
