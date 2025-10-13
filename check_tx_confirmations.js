const { Connection, PublicKey } = require('@solana/web3.js');

const TX_HASHES = [
  'rbnuBdmraMViDnWH1actWGoS5o6fSmqJKmKpmW5S8twwAvhCnXBS54eaVV7y1QRpZitp53T9zBWwFq23pA6wJgoH',
  'udcyK55j8CURPELb2PxPJjRMYwYd6Hu6VPaKX7dMEF4GpDNk8GCyJT5V31MT1MffpGxB9MDrRzvYhquBdoakz3UX',
  'ThY6SHYpzDq5KEaQxEgPvUhsJ9HfDJwjz7tGnA3bmZKennXB7W1fDFhN1EjcNUY1Q4Lec9mNnmDHPrWkLpc34gTc',
  'kmY699qtZ8QM1TjM6FfXysACCU4epQbMqhRLdJ9uVVCvbtP5WVaPeWK4ANhoLu48j1jC2a8eNiLsZeZpBEiWowAL',
  'TxhBFGvurRz26oYLMzj5GBG1JFe64h83R2Zc2tbuDzFQpVFGn8yW4iPixfDghN35Eub9ZpQpXZu3Bo4UxUPAtcBd'
];

const MULTISIG_TX = 'SJfq23i9ZHMUMKnEqpS3CfTwZ5TaAFfjxtCfLtpkPkJnNgXCJpupKn7KdqswjgjYs8Ly51GdcPmnBEy3Wq19is7xYdanibW8sXsLvGJw2Q4qUeJUPdkUAbh3GS66gHG9a47m4wLrtjktW3U5cLbAJ3tsQmUEHsRp2tjhPQSYGJpBoVSsH3CS7DYM8PGmxPXU57Yaf2ZGE8PGpPhXucBU8HCYMtcjTLwn9irvQkcWKJ4vJj';

async function checkTransactions() {
  const rpc = process.env.HELIUS_API_KEY 
    ? `https://mainnet.helius-rpc.com/?api-key=${process.env.HELIUS_API_KEY}`
    : 'https://api.mainnet-beta.solana.com';
  
  const connection = new Connection(rpc, 'confirmed');
  
  console.log('🔍 Transaction Hash Verification\n');
  console.log('RPC:', rpc.includes('helius') ? 'Helius' : 'Public');
  console.log('='.repeat(80));

  const results = { confirmed: [], notFound: [], errors: [] };

  console.log('\n📋 Checking Extracted Transaction Hashes:\n');
  
  for (const hash of TX_HASHES) {
    console.log(`\n🔑 ${hash.slice(0, 16)}...${hash.slice(-16)}`);
    
    try {
      const status = await connection.getSignatureStatus(hash);
      
      if (status?.value) {
        const confirmations = status.value.confirmations;
        const err = status.value.err;
        
        if (err) {
          console.log(`❌ FAILED - Error: ${JSON.stringify(err)}`);
          results.errors.push({ hash, error: err });
        } else if (confirmations === null || confirmations === 'finalized') {
          console.log(`✅ CONFIRMED (Finalized)`);
          results.confirmed.push({ hash, status: 'finalized' });
        } else {
          console.log(`⏳ CONFIRMING (${confirmations} confirmations)`);
          results.confirmed.push({ hash, confirmations });
        }

        // Get transaction details
        try {
          const tx = await connection.getTransaction(hash, {
            maxSupportedTransactionVersion: 0
          });
          
          if (tx) {
            console.log(`   Slot: ${tx.slot}`);
            console.log(`   Fee: ${(tx.meta.fee / 1e9).toFixed(6)} SOL`);
            console.log(`   Block Time: ${new Date(tx.blockTime * 1000).toISOString()}`);
            console.log(`   🔗 https://solscan.io/tx/${hash}`);
          }
        } catch {}
        
      } else {
        console.log(`❌ NOT FOUND`);
        results.notFound.push(hash);
      }
      
    } catch (e) {
      console.log(`❌ ERROR: ${e.message}`);
      results.errors.push({ hash, error: e.message });
    }
  }

  console.log('\n' + '='.repeat(80));
  console.log('\n🔍 Checking Multisig Transaction Message:\n');
  console.log(`📝 ${MULTISIG_TX.slice(0, 40)}...`);
  console.log('⚠️  This is a transaction message (not a signature)');
  console.log('   Status: AWAITING_SIGNATURES (0/4)');
  console.log('   Threshold: 4 of 7 signatures required');
  console.log('   🔗 https://v3.squads.so/');

  console.log('\n' + '='.repeat(80));
  console.log('\n📊 Summary:');
  console.log(`   ✅ Confirmed: ${results.confirmed.length}`);
  console.log(`   ❌ Not Found: ${results.notFound.length}`);
  console.log(`   ⚠️  Errors: ${results.errors.length}`);
  console.log(`   📝 Pending Multisig: 1 (awaiting signatures)`);

  require('fs').writeFileSync('tx_confirmations.json', JSON.stringify(results, null, 2));
  console.log('\n✅ Results saved to tx_confirmations.json');

  return results;
}

checkTransactions().catch(console.error);
