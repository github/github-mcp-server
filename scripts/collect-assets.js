#!/usr/bin/env node
const { Connection, PublicKey, Transaction, SystemProgram, LAMPORTS_PER_SOL } = require('@solana/web3.js');
const { TOKEN_PROGRAM_ID, getAssociatedTokenAddress } = require('@solana/spl-token');

const NEW_MASTER_CONTROLLER = 'GLzZk1sczzW6fM4uPFeQCtTZQaf8H5VaBt99tUMbJAAW';
const STABLECOINS = {
  USDC: 'EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v',
  USDT: 'Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB'
};

const SOURCE_ADDRESSES = [
  'FsQPFuje4WMdvbyoVef6MRMuzNZt9E8HM9YBN8T3Zbdq',
  'CvQZZ23qYDWF2RUpxYJ8y9K4skmuvYEEjH7fK58jtipQ',
  '7ZyDFzet6sKgZLN4D89JLfo7chu2n7nYdkFt5RCFk8Sf'
];

async function collectAssets() {
  const connection = new Connection('https://api.mainnet-beta.solana.com', 'confirmed');
  const destination = new PublicKey(NEW_MASTER_CONTROLLER);
  
  console.log('💰 Asset Collection Report\n');
  console.log('Destination:', NEW_MASTER_CONTROLLER);
  console.log('━'.repeat(60));

  const assets = { sol: [], tokens: [] };
  let totalSOL = 0;

  for (const addr of SOURCE_ADDRESSES) {
    try {
      const pubkey = new PublicKey(addr);
      const balance = await connection.getBalance(pubkey);
      const solAmount = balance / LAMPORTS_PER_SOL;
      
      if (balance > 0) {
        console.log(`\n✅ ${addr}`);
        console.log(`   SOL: ${solAmount.toFixed(4)}`);
        assets.sol.push({ address: addr, balance: solAmount, lamports: balance });
        totalSOL += solAmount;
      }

      // Check USDC
      try {
        const usdcMint = new PublicKey(STABLECOINS.USDC);
        const usdcAta = await getAssociatedTokenAddress(usdcMint, pubkey);
        const usdcAccount = await connection.getTokenAccountBalance(usdcAta);
        if (usdcAccount.value.uiAmount > 0) {
          console.log(`   USDC: ${usdcAccount.value.uiAmount}`);
          assets.tokens.push({ address: addr, token: 'USDC', amount: usdcAccount.value.uiAmount, ata: usdcAta.toBase58() });
        }
      } catch {}

      // Check USDT
      try {
        const usdtMint = new PublicKey(STABLECOINS.USDT);
        const usdtAta = await getAssociatedTokenAddress(usdtMint, pubkey);
        const usdtAccount = await connection.getTokenAccountBalance(usdtAta);
        if (usdtAccount.value.uiAmount > 0) {
          console.log(`   USDT: ${usdtAccount.value.uiAmount}`);
          assets.tokens.push({ address: addr, token: 'USDT', amount: usdtAccount.value.uiAmount, ata: usdtAta.toBase58() });
        }
      } catch {}

    } catch (e) {
      console.log(`❌ ${addr} - Error: ${e.message}`);
    }
  }

  console.log('\n' + '━'.repeat(60));
  console.log('\n📊 Summary:');
  console.log(`   Total SOL: ${totalSOL.toFixed(4)}`);
  console.log(`   Token Accounts: ${assets.tokens.length}`);
  console.log(`\n🎯 Destination: ${NEW_MASTER_CONTROLLER}`);
  console.log(`   🔗 https://solscan.io/account/${NEW_MASTER_CONTROLLER}`);

  return assets;
}

collectAssets().catch(console.error);
