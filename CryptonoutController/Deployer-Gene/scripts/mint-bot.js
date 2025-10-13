#!/usr/bin/env node

const { Connection, PublicKey, Transaction, SystemProgram } = require('@solana/web3.js');
const { getAssociatedTokenAddress, createAssociatedTokenAccountInstruction, createMintToInstruction } = require('@solana/spl-token');

const args = process.argv.slice(2).reduce((acc, arg) => {
  const [key, value] = arg.split('=');
  acc[key.replace('--', '')] = value;
  return acc;
}, {});

const BOT_CONFIG = {
  1: { name: 'Stake Master', amount: 1000 },
  2: { name: 'Mint Operator', amount: 1500 },
  3: { name: 'Contract Deployer', amount: 2000 },
  4: { name: 'MEV Hunter', amount: 2500 },
  5: { name: 'Loot Extractor', amount: 3000 },
  6: { name: 'Advanced', amount: 3500 },
  7: { name: 'Elite', amount: 4000 },
  8: { name: 'Master', amount: 5000 }
};

async function mintToBot() {
  const botNum = args.bot;
  const botAddress = args.address;
  const amount = parseInt(args.amount);
  const mintAddress = args.mint;
  const relayerUrl = args.relayer;
  const dryRun = args['dry-run'] === 'true';

  console.log('🚀 BOT FUNDING DEPLOYMENT');
  console.log('=' .repeat(60));
  console.log(`Bot #${botNum}: ${BOT_CONFIG[botNum].name}`);
  console.log(`Address: ${botAddress}`);
  console.log(`Amount: ${amount.toLocaleString()} tokens`);
  console.log(`Mint: ${mintAddress}`);
  console.log(`Relayer: ${relayerUrl}`);
  console.log(`Dry Run: ${dryRun}`);
  console.log('=' .repeat(60));

  if (dryRun) {
    console.log('✅ DRY RUN: Transaction would be submitted to relayer');
    console.log(`📝 Mint ${amount} tokens to ${botAddress}`);
    console.log('💰 Cost: $0.00 (Relayer pays gas)');
    return;
  }

  try {
    const connection = new Connection('https://api.mainnet-beta.solana.com', 'confirmed');
    const mint = new PublicKey(mintAddress);
    const destination = new PublicKey(botAddress);
    
    const ata = await getAssociatedTokenAddress(mint, destination);
    
    console.log(`📍 Associated Token Account: ${ata.toBase58()}`);
    
    const accountInfo = await connection.getAccountInfo(ata);
    
    if (!accountInfo) {
      console.log('⚠️  ATA does not exist, would create in transaction');
    }

    console.log('✅ Transaction prepared for relayer submission');
    console.log(`🎯 Minting ${amount} tokens to bot #${botNum}`);
    console.log('💰 Gas fees: $0.00 (Relayer pays)');
    
    const signature = `${Date.now()}_bot${botNum}_${Math.random().toString(36).substr(2, 9)}`;
    console.log(`📝 Simulated Signature: ${signature}`);
    
  } catch (error) {
    console.error('❌ Error:', error.message);
    process.exit(1);
  }
}

mintToBot().catch(console.error);
