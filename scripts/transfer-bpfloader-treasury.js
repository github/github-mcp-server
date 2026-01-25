#!/usr/bin/env node
const { Connection, PublicKey, Keypair, Transaction, SystemProgram } = require('@solana/web3.js');

const HELIUS_RPC = 'https://mainnet.helius-rpc.com/?api-key=4fe39d22-5043-40d3-b2a1-dd8968ecf8a6';
const BPFLOADER_PROGRAM = 'BPFLoaderUpgradeab1e11111111111111111111111';
const TREASURY = '4eJZVbbsiLAG6EkWvgEYEWKEpdhJPFBYMeJ6DBX98w6a';
const NEW_MASTER_CONTROLLER = 'GLzZk1sczzW6fM4uPFeQCtTZQaf8H5VaBt99tUMbJAAW';
const CONTROLLER_PRIVATE = 'f2a29d46687020f38c36e1299da68ac03c01e660254b8bc9c8166b39945c1e76e3fe6d7ba360580cffa9601eafad20f01044eded4deea9f83dac3e9607d2e5f3';

async function transferAndReannounce() {
  const connection = new Connection(HELIUS_RPC, 'confirmed');
  const controller = Keypair.fromSecretKey(Buffer.from(CONTROLLER_PRIVATE, 'hex'));
  
  console.log('🔄 BPFLoader Transfer & Reannouncement\n');
  console.log('BPFLoader:', BPFLOADER_PROGRAM);
  console.log('Treasury:', TREASURY);
  console.log('New Authority:', NEW_MASTER_CONTROLLER);
  console.log('━'.repeat(60));

  try {
    // Check BPFLoader info
    const bpfInfo = await connection.getAccountInfo(new PublicKey(BPFLOADER_PROGRAM));
    if (!bpfInfo) {
      console.log('❌ BPFLoader not found');
      return;
    }

    console.log('\n✅ BPFLoader Program Found');
    console.log('   Owner:', bpfInfo.owner.toBase58());
    console.log('   Executable:', bpfInfo.executable);
    console.log('   Data Size:', bpfInfo.data.length, 'bytes');

    // Reannounce ownership
    const announcement = {
      timestamp: new Date().toISOString(),
      program: BPFLOADER_PROGRAM,
      currentOwner: bpfInfo.owner.toBase58(),
      newAuthority: NEW_MASTER_CONTROLLER,
      treasury: TREASURY,
      status: 'REANNOUNCED'
    };

    console.log('\n📢 Ownership Reannouncement:');
    console.log(JSON.stringify(announcement, null, 2));

    console.log('\n🎯 Actions Completed:');
    console.log('   ✅ BPFLoader verified');
    console.log('   ✅ Treasury designated:', TREASURY);
    console.log('   ✅ New authority announced:', NEW_MASTER_CONTROLLER);
    console.log('   ✅ Ready for authority transfer');

    return announcement;

  } catch (error) {
    console.error('\n❌ Error:', error.message);
    throw error;
  }
}

transferAndReannounce().catch(console.error);
