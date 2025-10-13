#!/usr/bin/env node

const fetch = require('node-fetch');

const DAO_CONTROLLER = 'CvQZZ23qYDWF2RUpxYJ8y9K4skmuvYEEjH7fK58jtipQ';

async function getAccountInfo() {
  const response = await fetch('https://api.mainnet-beta.solana.com', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      jsonrpc: '2.0',
      id: 1,
      method: 'getAccountInfo',
      params: [DAO_CONTROLLER, { encoding: 'jsonParsed' }]
    })
  });
  return response.json();
}

async function getSignatures() {
  const response = await fetch('https://api.mainnet-beta.solana.com', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      jsonrpc: '2.0',
      id: 1,
      method: 'getSignaturesForAddress',
      params: [DAO_CONTROLLER, { limit: 1000 }]
    })
  });
  return response.json();
}

async function getTransaction(signature) {
  const response = await fetch('https://api.mainnet-beta.solana.com', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      jsonrpc: '2.0',
      id: 1,
      method: 'getTransaction',
      params: [signature, { encoding: 'jsonParsed', maxSupportedTransactionVersion: 0 }]
    })
  });
  return response.json();
}

async function main() {
  console.log('🔍 DAO CONTROLLER SIGNER ANALYSIS');
  console.log('📍 Address:', DAO_CONTROLLER);
  console.log('=' .repeat(70));

  const accountInfo = await getAccountInfo();
  if (accountInfo.result) {
    console.log('\n📋 ACCOUNT INFO:');
    console.log('   Owner:', accountInfo.result.value?.owner || 'N/A');
    console.log('   Lamports:', accountInfo.result.value?.lamports || 0);
    console.log('   Executable:', accountInfo.result.value?.executable || false);
  }

  console.log('\n🔐 FETCHING SIGNERS...');
  const sigData = await getSignatures();
  const signatures = sigData.result || [];
  
  console.log(`   Found ${signatures.length} transactions`);

  const signers = new Set();
  const signerDetails = [];

  for (let i = 0; i < Math.min(signatures.length, 50); i++) {
    const sig = signatures[i];
    const tx = await getTransaction(sig.signature);
    
    if (tx.result?.transaction) {
      const accountKeys = tx.result.transaction.message?.accountKeys || [];
      
      accountKeys.forEach((key, idx) => {
        const addr = typeof key === 'string' ? key : key.pubkey;
        const isSigner = typeof key === 'object' ? key.signer : (idx === 0);
        
        if (isSigner && addr !== DAO_CONTROLLER) {
          signers.add(addr);
          signerDetails.push({
            address: addr,
            signature: sig.signature,
            slot: sig.slot,
            blockTime: sig.blockTime
          });
        }
      });
    }
    
    if ((i + 1) % 10 === 0) {
      console.log(`   Processed ${i + 1}/${Math.min(signatures.length, 50)} transactions...`);
    }
  }

  console.log('\n' + '=' .repeat(70));
  console.log('✅ SIGNERS FOUND:', signers.size);
  console.log('=' .repeat(70));

  const uniqueSigners = Array.from(signers);
  uniqueSigners.forEach((signer, idx) => {
    console.log(`\n${idx + 1}. ${signer}`);
    const details = signerDetails.filter(d => d.address === signer);
    console.log(`   Transactions: ${details.length}`);
    if (details[0]) {
      const date = details[0].blockTime ? new Date(details[0].blockTime * 1000).toISOString() : 'N/A';
      console.log(`   Last seen: ${date}`);
    }
  });

  const fs = require('fs');
  fs.writeFileSync('dao-signers.json', JSON.stringify({
    controller: DAO_CONTROLLER,
    timestamp: new Date().toISOString(),
    totalSigners: signers.size,
    signers: uniqueSigners,
    details: signerDetails
  }, null, 2));

  console.log('\n✅ Results saved to dao-signers.json');
}

main().catch(console.error);
