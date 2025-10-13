#!/usr/bin/env node
const { Connection, PublicKey } = require('@solana/web3.js');

const ACCOUNT = 'FVhQ3QHvXudWSdGix2sdcG47YmrmUxRhf3KCBmiKfekf';
const HELIUS_RPC = process.env.HELIUS_API_KEY 
  ? `https://mainnet.helius-rpc.com/?api-key=${process.env.HELIUS_API_KEY}`
  : 'https://api.mainnet-beta.solana.com';

async function getAccountInfo() {
  const connection = new Connection(HELIUS_RPC, 'confirmed');
  
  console.log('🔍 Helius Account Query\n');
  console.log('Account:', ACCOUNT);
  console.log('RPC:', HELIUS_RPC.includes('helius') ? 'Helius' : 'Fallback');
  console.log('━'.repeat(60));

  try {
    const pubkey = new PublicKey(ACCOUNT);
    const [accountInfo, balance] = await Promise.all([
      connection.getAccountInfo(pubkey),
      connection.getBalance(pubkey)
    ]);

    if (!accountInfo) {
      console.log('\n❌ Account not found or has no data');
      return;
    }

    console.log('\n✅ Account Info:');
    console.log('   Balance:', (balance / 1e9).toFixed(6), 'SOL');
    console.log('   Owner:', accountInfo.owner.toBase58());
    console.log('   Executable:', accountInfo.executable);
    console.log('   Rent Epoch:', accountInfo.rentEpoch);
    console.log('   Data Size:', accountInfo.data.length, 'bytes');

    // Check if token account
    if (accountInfo.data.length === 165) {
      console.log('\n💰 Token Account Detected');
      try {
        const tokenInfo = await connection.getParsedAccountInfo(pubkey);
        if (tokenInfo.value?.data?.parsed) {
          const parsed = tokenInfo.value.data.parsed.info;
          console.log('   Mint:', parsed.mint);
          console.log('   Owner:', parsed.owner);
          console.log('   Amount:', parsed.tokenAmount?.uiAmountString || '0');
        }
      } catch {}
    }

    console.log('\n🔗 Links:');
    console.log('   Solscan: https://solscan.io/account/' + ACCOUNT);
    console.log('   Solana Explorer: https://explorer.solana.com/address/' + ACCOUNT);

    return {
      address: ACCOUNT,
      balance: balance / 1e9,
      owner: accountInfo.owner.toBase58(),
      executable: accountInfo.executable,
      dataSize: accountInfo.data.length
    };

  } catch (error) {
    console.error('\n❌ Error:', error.message);
  }
}

getAccountInfo().catch(console.error);
