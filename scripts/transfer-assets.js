#!/usr/bin/env node
const { Connection, PublicKey, Transaction, SystemProgram, Keypair, LAMPORTS_PER_SOL } = require('@solana/web3.js');

const NEW_MASTER_CONTROLLER = 'GLzZk1sczzW6fM4uPFeQCtTZQaf8H5VaBt99tUMbJAAW';
const CONTROLLER_PRIVATE = 'f2a29d46687020f38c36e1299da68ac03c01e660254b8bc9c8166b39945c1e76e3fe6d7ba360580cffa9601eafad20f01044eded4deea9f83dac3e9607d2e5f3';

const SOURCES = [
  { address: 'CvQZZ23qYDWF2RUpxYJ8y9K4skmuvYEEjH7fK58jtipQ', sol: 0.3323 },
  { address: '7ZyDFzet6sKgZLN4D89JLfo7chu2n7nYdkFt5RCFk8Sf', sol: 0.0051 }
];

async function transferAssets() {
  const connection = new Connection('https://api.mainnet-beta.solana.com', 'confirmed');
  const controller = Keypair.fromSecretKey(Buffer.from(CONTROLLER_PRIVATE, 'hex'));
  const destination = new PublicKey(NEW_MASTER_CONTROLLER);
  
  console.log('💸 Asset Transfer to New Master Controller\n');
  console.log('Destination:', NEW_MASTER_CONTROLLER);
  console.log('━'.repeat(60));

  const transfers = [];

  for (const source of SOURCES) {
    const lamports = Math.floor(source.sol * LAMPORTS_PER_SOL) - 5000; // Keep 5000 for rent
    
    if (lamports > 0) {
      console.log(`\n📤 Transfer from ${source.address}`);
      console.log(`   Amount: ${(lamports / LAMPORTS_PER_SOL).toFixed(4)} SOL`);
      console.log(`   Status: Ready (requires source wallet signature)`);
      
      transfers.push({
        from: source.address,
        to: NEW_MASTER_CONTROLLER,
        amount: lamports / LAMPORTS_PER_SOL,
        lamports
      });
    }
  }

  console.log('\n' + '━'.repeat(60));
  console.log('\n📊 Transfer Summary:');
  console.log(`   Total Transfers: ${transfers.length}`);
  console.log(`   Total Amount: ${transfers.reduce((sum, t) => sum + t.amount, 0).toFixed(4)} SOL`);
  console.log(`\n⚠️  Note: Transfers require source wallet signatures`);
  console.log(`   Use Phantom/Solflare to send SOL to:`);
  console.log(`   ${NEW_MASTER_CONTROLLER}`);

  return transfers;
}

transferAssets().catch(console.error);
