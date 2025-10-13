#!/usr/bin/env node
const { Connection, PublicKey } = require('@solana/web3.js');

const SIGNER = 'FsQPFuje4WMdvbyoVef6MRMuzNZt9E8HM9YBN8T3Zbdq';
const NEW_CONTROLLER = 'GLzZk1sczzW6fM4uPFeQCtTZQaf8H5VaBt99tUMbJAAW';

async function checkSignerReady() {
  const connection = new Connection('https://api.mainnet-beta.solana.com');
  
  console.log('🔍 Checking Signer Readiness\n');
  
  try {
    const signerPubkey = new PublicKey(SIGNER);
    const controllerPubkey = new PublicKey(NEW_CONTROLLER);
    
    const [signerInfo, controllerInfo] = await Promise.all([
      connection.getAccountInfo(signerPubkey),
      connection.getAccountInfo(controllerPubkey)
    ]);
    
    const signerBalance = signerInfo ? await connection.getBalance(signerPubkey) : 0;
    const controllerBalance = controllerInfo ? await connection.getBalance(controllerPubkey) : 0;
    
    console.log('✅ Signer Address:', SIGNER);
    console.log('   Balance:', (signerBalance / 1e9).toFixed(4), 'SOL');
    console.log('   Status:', signerBalance > 0 ? '✅ READY' : '❌ NEEDS FUNDING');
    
    console.log('\n✅ New Controller:', NEW_CONTROLLER);
    console.log('   Balance:', (controllerBalance / 1e9).toFixed(4), 'SOL');
    console.log('   Status:', controllerBalance >= 0 ? '✅ VALID' : '❌ INVALID');
    
    const ready = signerBalance > 0;
    console.log('\n' + '━'.repeat(50));
    console.log(ready ? '✅ SIGNER READY FOR TRANSACTION' : '❌ SIGNER NEEDS FUNDING');
    console.log('━'.repeat(50));
    
    return { signer: SIGNER, controller: NEW_CONTROLLER, ready, signerBalance, controllerBalance };
  } catch (error) {
    console.error('❌ Error:', error.message);
    process.exit(1);
  }
}

checkSignerReady();
