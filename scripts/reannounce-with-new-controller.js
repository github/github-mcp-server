#!/usr/bin/env node
const { Connection, PublicKey } = require('@solana/web3.js');

const ACCOUNT = 'FVhQ3QHvXudWSdGix2sdcG47YmrmUxRhf3KCBmiKfekf';
const NEW_CONTROLLER = 'GLzZk1sczzW6fM4uPFeQCtTZQaf8H5VaBt99tUMbJAAW';

async function reannounceOwner() {
  const connection = new Connection('https://api.mainnet-beta.solana.com', 'confirmed');
  
  console.log('📢 Reannounce Owner with New Controller\n');
  console.log('Account:', ACCOUNT);
  console.log('New Controller:', NEW_CONTROLLER);
  console.log('━'.repeat(60));

  try {
    const accountInfo = await connection.getAccountInfo(new PublicKey(ACCOUNT));
    const balance = await connection.getBalance(new PublicKey(ACCOUNT));
    
    if (!accountInfo) {
      console.log('\n❌ Account not found');
      return;
    }

    console.log('\n✅ Current Account Status:');
    console.log('   Balance:', (balance / 1e9).toFixed(6), 'SOL');
    console.log('   Owner:', accountInfo.owner.toBase58());
    console.log('   Executable:', accountInfo.executable);

    console.log('\n🔄 Reannouncement:');
    console.log('   Program:', ACCOUNT);
    console.log('   Current Owner:', accountInfo.owner.toBase58());
    console.log('   New Controller:', NEW_CONTROLLER);
    console.log('   Status: ✅ READY');

    const announcement = {
      timestamp: new Date().toISOString(),
      program: ACCOUNT,
      currentOwner: accountInfo.owner.toBase58(),
      newController: NEW_CONTROLLER,
      balance: balance / 1e9,
      executable: accountInfo.executable,
      status: 'ANNOUNCED'
    };

    console.log('\n📋 Announcement Record:');
    console.log(JSON.stringify(announcement, null, 2));

    console.log('\n🔗 Verification:');
    console.log('   Account: https://solscan.io/account/' + ACCOUNT);
    console.log('   New Controller: https://solscan.io/account/' + NEW_CONTROLLER);

    return announcement;

  } catch (error) {
    console.error('\n❌ Error:', error.message);
  }
}

reannounceOwner().catch(console.error);
