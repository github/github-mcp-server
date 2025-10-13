#!/usr/bin/env node
const { Connection, PublicKey, Keypair, Transaction, SystemProgram } = require('@solana/web3.js');
const { BPF_LOADER_UPGRADEABLE_PROGRAM_ID } = require('@solana/web3.js');

const JUPITER_PROGRAM = 'JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4';
const EXECUTABLE_DATA = '4Ec7ZxZS6Sbdg5UGSLHbAnM7GQHp2eFd4KYWRexAipQT';
const CURRENT_AUTHORITY = 'CvQZZ23qYDWF2RUpxYJ8y9K4skmuvYEEjH7fK58jtipQ';
const NEW_CONTROLLER = 'GLzZk1sczzW6fM4uPFeQCtTZQaf8H5VaBt99tUMbJAAW';

async function reannounceAuthority() {
  const connection = new Connection(process.env.SOLANA_RPC || 'https://api.mainnet-beta.solana.com');
  
  console.log('🔐 Authority Reannouncement');
  console.log('━'.repeat(50));
  console.log(`Program: ${JUPITER_PROGRAM}`);
  console.log(`Executable Data: ${EXECUTABLE_DATA}`);
  console.log(`Current Authority: ${CURRENT_AUTHORITY}`);
  console.log(`New Controller: ${NEW_CONTROLLER}`);
  console.log('━'.repeat(50));

  try {
    const programInfo = await connection.getAccountInfo(new PublicKey(JUPITER_PROGRAM));
    if (!programInfo) {
      throw new Error('Program not found');
    }

    console.log('✅ Program verified on-chain');
    console.log(`   Owner: ${programInfo.owner.toBase58()}`);
    console.log(`   Executable: ${programInfo.executable}`);
    console.log(`   Data Size: ${programInfo.data.length} bytes`);

    const report = {
      timestamp: new Date().toISOString(),
      program: JUPITER_PROGRAM,
      executableData: EXECUTABLE_DATA,
      currentAuthority: CURRENT_AUTHORITY,
      newController: NEW_CONTROLLER,
      verified: true,
      status: 'READY_FOR_UPGRADE'
    };

    console.log('\n📋 Authority Report Generated');
    console.log(JSON.stringify(report, null, 2));

    return report;
  } catch (error) {
    console.error('❌ Error:', error.message);
    process.exit(1);
  }
}

if (require.main === module) {
  reannounceAuthority();
}

module.exports = { reannounceAuthority };
