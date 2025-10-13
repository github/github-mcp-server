const { Connection } = require('@solana/web3.js');

const VALID_SIGNATURES = [
  '2635rwgwvPekyQzkDPiPh1PH3WkqYydaBKY42rfUFnzqNtdWaApexipp32XGzcTHiGNdifSXEfuNWRMQovLHdoSd',
  '2A5EXWVmemFrMFktCD4vmyKTBrhoqeQuDSRdvB2rnkJ7pnCxzixpt2BNDQyPJpVDHQ6Xf8wmMyvRCXTXPShdijKc',
  '2AHAs1gdHSGn2REJbARigh5CLoRuR9gdNTMTKu5UJBVVovXUxhPYeLFYTVgov7gyes4QkwLhgw89PAsGZbUjK2Yv',
  '2FVSeJQX3Sd8tVUSFGN7fY1fW5cKcnph9XzSYruvf88C4geMimDQNqkKadWXqioGwpwbvCsGQ9cjbKXUvNuZR1ca',
  '2GtCKQ5AY1NMupCqDBrA58YZ48BaouMxoFRWiQLET2vn973BhuFUqeDXKzBiubvLqb5kzQ5huAxoCN5z2CE5ZeUU'
];

async function main() {
  const connection = new Connection('https://api.mainnet-beta.solana.com', 'confirmed');
  
  console.log('🔍 Valid Transaction Signature Check\n');
  console.log('='.repeat(80));

  const results = { confirmed: [], notFound: [] };

  for (const sig of VALID_SIGNATURES) {
    console.log(`\n📝 ${sig.slice(0, 20)}...${sig.slice(-20)}`);
    
    try {
      const status = await connection.getSignatureStatus(sig);
      
      if (status?.value) {
        console.log(`✅ FOUND`);
        console.log(`   Confirmations: ${status.value.confirmations || 'Finalized'}`);
        console.log(`   Error: ${status.value.err || 'None'}`);
        
        const tx = await connection.getTransaction(sig, { maxSupportedTransactionVersion: 0 });
        if (tx) {
          console.log(`   Slot: ${tx.slot}`);
          console.log(`   Fee: ${(tx.meta.fee / 1e9).toFixed(6)} SOL`);
          console.log(`   Time: ${new Date(tx.blockTime * 1000).toISOString()}`);
          console.log(`   🔗 https://solscan.io/tx/${sig}`);
          results.confirmed.push({ signature: sig, slot: tx.slot, fee: tx.meta.fee });
        }
      } else {
        console.log(`❌ NOT FOUND`);
        results.notFound.push(sig);
      }
    } catch (e) {
      console.log(`❌ ERROR: ${e.message}`);
      results.notFound.push(sig);
    }
  }

  console.log('\n' + '='.repeat(80));
  console.log(`\n📊 Summary: ${results.confirmed.length} confirmed, ${results.notFound.length} not found`);
  
  require('fs').writeFileSync('valid_tx_check.json', JSON.stringify(results, null, 2));
  console.log('✅ Saved to valid_tx_check.json');
}

main().catch(console.error);
