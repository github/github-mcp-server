#!/usr/bin/env node
const { Connection, PublicKey, Transaction, SystemProgram, Keypair, sendAndConfirmTransaction } = require('@solana/web3.js');

const NEW_MASTER_CONTROLLER = 'GLzZk1sczzW6fM4uPFeQCtTZQaf8H5VaBt99tUMbJAAW';
const SIGNER_PRIVATE = 'f2a29d46687020f38c36e1299da68ac03c01e660254b8bc9c8166b39945c1e76e3fe6d7ba360580cffa9601eafad20f01044eded4deea9f83dac3e9607d2e5f3';

async function sendAssets() {
  const connection = new Connection('https://api.mainnet-beta.solana.com', 'confirmed');
  const signer = Keypair.fromSecretKey(Buffer.from(SIGNER_PRIVATE, 'hex'));
  const destination = new PublicKey(NEW_MASTER_CONTROLLER);
  
  console.log('💸 Sending Assets with Valid Signer\n');
  console.log('Signer:', signer.publicKey.toBase58());
  console.log('Destination:', NEW_MASTER_CONTROLLER);
  console.log('━'.repeat(60));

  // Get priority fee
  let priorityFee = 0;
  try {
    const response = await fetch('https://api.mainnet-beta.solana.com', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        jsonrpc: '2.0',
        id: 1,
        method: 'getRecentPrioritizationFees',
        params: [[signer.publicKey.toBase58()]]
      })
    });
    const data = await response.json();
    priorityFee = data.result?.[0]?.prioritizationFee || 0;
  } catch {}

  console.log('\n💰 Priority Fee:', priorityFee, 'micro-lamports');

  // Check signer balance
  const balance = await connection.getBalance(signer.publicKey);
  console.log('Signer Balance:', (balance / 1e9).toFixed(6), 'SOL');

  if (balance === 0) {
    console.log('\n❌ Signer has no balance to send');
    return;
  }

  // Create transfer transaction
  const transferAmount = balance - 5000 - priorityFee; // Keep rent + fee
  
  if (transferAmount <= 0) {
    console.log('\n❌ Insufficient balance after fees');
    return;
  }

  const transaction = new Transaction().add(
    SystemProgram.transfer({
      fromPubkey: signer.publicKey,
      toPubkey: destination,
      lamports: transferAmount
    })
  );

  console.log('\n📋 Transaction Details:');
  console.log('   Amount:', (transferAmount / 1e9).toFixed(6), 'SOL');
  console.log('   Fee:', (5000 + priorityFee) / 1e9, 'SOL');

  try {
    const signature = await sendAndConfirmTransaction(connection, transaction, [signer]);
    
    console.log('\n✅ Transfer Complete!');
    console.log('   Signature:', signature);
    console.log('   🔗 https://solscan.io/tx/' + signature);
    
    return { signature, amount: transferAmount / 1e9 };
  } catch (error) {
    console.error('\n❌ Transfer failed:', error.message);
  }
}

sendAssets().catch(console.error);
