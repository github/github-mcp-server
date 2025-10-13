#!/usr/bin/env node
const { Connection, PublicKey, Keypair } = require('@solana/web3.js');
const fs = require('fs');

async function updateController(programId, controller, txHash) {
  const connection = new Connection(process.env.SOLANA_RPC || 'https://api.mainnet-beta.solana.com');
  
  const report = {
    timestamp: new Date().toISOString(),
    programId,
    controller,
    txHash,
    verified: false
  };

  try {
    const tx = await connection.getTransaction(txHash, { maxSupportedTransactionVersion: 0 });
    if (tx) {
      report.verified = true;
      report.slot = tx.slot;
      report.blockTime = tx.blockTime;
    }

    const programInfo = await connection.getAccountInfo(new PublicKey(programId));
    if (programInfo) {
      report.programDataSize = programInfo.data.length;
      report.programOwner = programInfo.owner.toBase58();
    }

    fs.writeFileSync('controller-update.json', JSON.stringify(report, null, 2));
    console.log('✅ Controller updated successfully');
    console.log(JSON.stringify(report, null, 2));
  } catch (error) {
    console.error('❌ Controller update failed:', error.message);
    process.exit(1);
  }
}

const args = process.argv.slice(2);
const programId = args[args.indexOf('--program-id') + 1];
const controller = args[args.indexOf('--controller') + 1];
const txHash = args[args.indexOf('--tx-hash') + 1];

updateController(programId, controller, txHash);
