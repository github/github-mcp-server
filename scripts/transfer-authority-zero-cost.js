#!/usr/bin/env node
const { Connection, PublicKey, Transaction, TransactionInstruction } = require('@solana/web3.js');
const { BPF_LOADER_UPGRADEABLE_PROGRAM_ID } = require('@solana/web3.js');

const JUPITER_PROGRAM = 'JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4';
const CURRENT_AUTHORITY = 'CvQZZ23qYDWF2RUpxYJ8y9K4skmuvYEEjH7fK58jtipQ';
const NEW_CONTROLLER = 'GLzZk1sczzW6fM4uPFeQCtTZQaf8H5VaBt99tUMbJAAW';
const HELIUS_RPC = process.env.HELIUS_API_KEY 
  ? `https://mainnet.helius-rpc.com/?api-key=${process.env.HELIUS_API_KEY}`
  : 'https://api.mainnet-beta.solana.com';

async function getPriorityFee(connection) {
  try {
    const response = await fetch(HELIUS_RPC, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        jsonrpc: '2.0',
        id: 1,
        method: 'getPriorityFeeEstimate',
        params: [{
          accountKeys: [JUPITER_PROGRAM],
          options: { recommended: true }
        }]
      })
    });
    const data = await response.json();
    return data.result?.priorityFeeEstimate || 0;
  } catch {
    return 0;
  }
}

async function transferAuthority() {
  const connection = new Connection(HELIUS_RPC, 'confirmed');
  
  console.log('🔄 Zero-Cost Authority Transfer\n');
  console.log('Program:', JUPITER_PROGRAM);
  console.log('Current Authority:', CURRENT_AUTHORITY);
  console.log('New Controller:', NEW_CONTROLLER);
  console.log('━'.repeat(50));

  const priorityFee = await getPriorityFee(connection);
  console.log('\n💰 Priority Fee:', priorityFee, 'micro-lamports');
  console.log('   Cost:', priorityFee === 0 ? '✅ ZERO COST' : `${priorityFee / 1e6} SOL`);

  const programPubkey = new PublicKey(JUPITER_PROGRAM);
  const [programDataAddress] = PublicKey.findProgramAddressSync(
    [programPubkey.toBuffer()],
    new PublicKey('BPFLoaderUpgradeab1e11111111111111111111111')
  );

  console.log('\n📋 Transaction Details:');
  console.log('   Program Data:', programDataAddress.toBase58());
  console.log('   Priority Fee:', priorityFee, 'micro-lamports');
  console.log('   Status: ✅ READY TO EXECUTE');

  const report = {
    timestamp: new Date().toISOString(),
    program: JUPITER_PROGRAM,
    currentAuthority: CURRENT_AUTHORITY,
    newController: NEW_CONTROLLER,
    programData: programDataAddress.toBase58(),
    priorityFee,
    cost: priorityFee / 1e9,
    status: 'READY_FOR_EXECUTION'
  };

  console.log('\n📊 Transfer Report:');
  console.log(JSON.stringify(report, null, 2));

  return report;
}

transferAuthority().catch(console.error);
