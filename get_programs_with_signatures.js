const { Connection, PublicKey, clusterApiUrl } = require('@solana/web3.js');

const programs = [
  'GENEtH5amGSi8kHAtQoezp1XEXwZJ8vcuePYnXdKrMYz',
  'DjVE6JNiYqPL2QXyCUUh8rNjHrbz9hXHNYt99MQ59qw1',
  'CvQZZ23qYDWF2RUpxYJ8y9K4skmuvYEEjH7fK58jtipQ',
  'JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4',
  '4Ec7ZxZS6Sbdg5UGSLHbAnM7GQHp2eFd4KYWRexAipQT',
  'GLzZk1sczzW6fM4uPFeQCtTZQaf8H5VaBt99tUMbJAAW',
  'SMPLecH534NA9acpos4G6x7uf3LWbCAwZQE9e8ZekMu',
  '7ZyDFzet6sKgZLN4D89JLfo7chu2n7nYdkFt5RCFk8Sf',
  'TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA',
  'So11111111111111111111111111111111111111112',
  '11111111111111111111111111111111'
];

async function main() {
  const rpc = process.env.HELIUS_API_KEY 
    ? `https://mainnet.helius-rpc.com/?api-key=${process.env.HELIUS_API_KEY}`
    : 'https://api.mainnet-beta.solana.com';
  
  const connection = new Connection(rpc, 'confirmed');
  
  console.log('🔍 Program Analysis with Signatures & Priority Fees\n');
  console.log('RPC:', rpc.includes('helius') ? 'Helius' : 'Public');
  console.log('='.repeat(80));

  const results = [];

  for (const addr of programs) {
    console.log(`\n📋 ${addr}`);
    
    try {
      const pubkey = new PublicKey(addr);
      const [accountInfo, signatures] = await Promise.all([
        connection.getAccountInfo(pubkey),
        connection.getSignaturesForAddress(pubkey, { limit: 5 })
      ]);

      if (!accountInfo) {
        console.log('❌ Not found');
        results.push({ address: addr, exists: false });
        continue;
      }

      const balance = await connection.getBalance(pubkey);
      
      console.log(`✅ Balance: ${(balance / 1e9).toFixed(6)} SOL`);
      console.log(`   Owner: ${accountInfo.owner.toBase58()}`);
      console.log(`   Executable: ${accountInfo.executable}`);
      console.log(`   Recent Signatures: ${signatures.length}`);

      const txDetails = [];
      for (const sig of signatures.slice(0, 3)) {
        try {
          const tx = await connection.getTransaction(sig.signature, {
            maxSupportedTransactionVersion: 0
          });
          
          if (tx?.meta) {
            const fee = tx.meta.fee / 1e9;
            const priorityFee = tx.meta.computeUnitsConsumed || 0;
            
            console.log(`   📝 ${sig.signature.slice(0, 16)}...`);
            console.log(`      Fee: ${fee.toFixed(6)} SOL | Priority: ${priorityFee} CU`);
            
            txDetails.push({
              signature: sig.signature,
              fee,
              priorityFee,
              slot: sig.slot,
              err: sig.err
            });
          }
        } catch {}
      }

      results.push({
        address: addr,
        balance: balance / 1e9,
        owner: accountInfo.owner.toBase58(),
        executable: accountInfo.executable,
        signatureCount: signatures.length,
        recentTransactions: txDetails,
        exists: true
      });

    } catch (e) {
      console.log(`❌ Error: ${e.message}`);
      results.push({ address: addr, exists: false, error: e.message });
    }
  }

  console.log('\n' + '='.repeat(80));
  console.log('\n📊 Summary:');
  console.log(`   Total Programs: ${programs.length}`);
  console.log(`   Found: ${results.filter(r => r.exists).length}`);
  console.log(`   Not Found: ${results.filter(r => !r.exists).length}`);

  require('fs').writeFileSync('program_signatures.json', JSON.stringify(results, null, 2));
  console.log('\n✅ Saved to program_signatures.json');
}

main().catch(console.error);
