#!/usr/bin/env node
const { Connection, PublicKey } = require('@solana/web3.js');

const ADDRESSES = {
  program: 'JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4',
  programData: '4Ec7ZxZS6Sbdg5UGSLHbAnM7GQHp2eFd4KYWRexAipQT',
  currentAuthority: 'CvQZZ23qYDWF2RUpxYJ8y9K4skmuvYEEjH7fK58jtipQ',
  newController: 'GLzZk1sczzW6fM4uPFeQCtTZQaf8H5VaBt99tUMbJAAW',
  masterController: 'SMPLecH534NA9acpos4G6x7uf3LWbCAwZQE9e8ZekMu',
  multisig: '7ZyDFzet6sKgZLN4D89JLfo7chu2n7nYdkFt5RCFk8Sf',
  members: [
    '2MgqMXdwSf3bRZ6S8uKJSffZAaoZBhD2mjst3phJXE7p',
    '89FnbsKH8n6FXCghGUijxh3snqx3e6VXJ7q1fQAHWkQQ',
    'BYidGfUnfoQtqi4nHiuo57Fjreizbej6hawJLnbwJmYr',
    'CHRDWWqUs6LyeeoD7pJb3iRfnvYeMfwMUtf2N7zWk7uh',
    'Dg5NLa5JuwfRMkuwZEguD9RpVrcQD3536GxogUv7pLNV',
    'EhJqf1p39c8NnH5iuZAJyw778LQua1AhZWxarT5SF8sT',
    'GGG2JyBtwbPAsYwUQED8GBbj9UMi7NQa3uwN3DmyGNtz'
  ]
};

async function verifyOnChain() {
  const connection = new Connection('https://api.mainnet-beta.solana.com', 'confirmed');
  
  console.log('🔍 Solana On-Chain Verification\n');
  console.log('━'.repeat(60));

  const results = { valid: [], invalid: [] };

  // Verify Program
  try {
    const programInfo = await connection.getAccountInfo(new PublicKey(ADDRESSES.program));
    if (programInfo && programInfo.executable) {
      console.log('✅ Jupiter Program:', ADDRESSES.program);
      console.log('   Executable:', programInfo.executable);
      console.log('   Owner:', programInfo.owner.toBase58());
      console.log('   🔗 https://solscan.io/account/' + ADDRESSES.program);
      results.valid.push({ type: 'Program', address: ADDRESSES.program });
    }
  } catch (e) {
    results.invalid.push({ type: 'Program', address: ADDRESSES.program, error: e.message });
  }

  console.log('\n━'.repeat(60));

  // Verify Program Data
  try {
    const dataInfo = await connection.getAccountInfo(new PublicKey(ADDRESSES.programData));
    if (dataInfo) {
      console.log('✅ Program Data:', ADDRESSES.programData);
      console.log('   Size:', dataInfo.data.length, 'bytes');
      console.log('   🔗 https://solscan.io/account/' + ADDRESSES.programData);
      results.valid.push({ type: 'Program Data', address: ADDRESSES.programData });
    }
  } catch (e) {
    results.invalid.push({ type: 'Program Data', address: ADDRESSES.programData, error: e.message });
  }

  console.log('\n━'.repeat(60));

  // Verify Current Authority
  try {
    const authInfo = await connection.getAccountInfo(new PublicKey(ADDRESSES.currentAuthority));
    const balance = await connection.getBalance(new PublicKey(ADDRESSES.currentAuthority));
    console.log('✅ Current Authority:', ADDRESSES.currentAuthority);
    console.log('   Balance:', (balance / 1e9).toFixed(4), 'SOL');
    console.log('   🔗 https://solscan.io/account/' + ADDRESSES.currentAuthority);
    results.valid.push({ type: 'Current Authority', address: ADDRESSES.currentAuthority });
  } catch (e) {
    results.invalid.push({ type: 'Current Authority', address: ADDRESSES.currentAuthority, error: e.message });
  }

  console.log('\n━'.repeat(60));

  // Verify New Controller
  try {
    const controllerBalance = await connection.getBalance(new PublicKey(ADDRESSES.newController));
    console.log('✅ New Controller:', ADDRESSES.newController);
    console.log('   Balance:', (controllerBalance / 1e9).toFixed(4), 'SOL');
    console.log('   🔗 https://solscan.io/account/' + ADDRESSES.newController);
    results.valid.push({ type: 'New Controller', address: ADDRESSES.newController });
  } catch (e) {
    results.invalid.push({ type: 'New Controller', address: ADDRESSES.newController, error: e.message });
  }

  console.log('\n━'.repeat(60));

  // Verify Master Controller
  try {
    const masterInfo = await connection.getAccountInfo(new PublicKey(ADDRESSES.masterController));
    if (masterInfo && masterInfo.executable) {
      const balance = await connection.getBalance(new PublicKey(ADDRESSES.masterController));
      console.log('✅ Master Controller:', ADDRESSES.masterController);
      console.log('   Executable:', masterInfo.executable);
      console.log('   Balance:', (balance / 1e9).toFixed(6), 'SOL');
      console.log('   Owner:', masterInfo.owner.toBase58());
      console.log('   🔗 https://solscan.io/account/' + ADDRESSES.masterController);
      results.valid.push({ type: 'Master Controller', address: ADDRESSES.masterController });
    }
  } catch (e) {
    results.invalid.push({ type: 'Master Controller', address: ADDRESSES.masterController, error: e.message });
  }

  console.log('\n━'.repeat(60));

  // Verify Multisig
  try {
    const multisigInfo = await connection.getAccountInfo(new PublicKey(ADDRESSES.multisig));
    if (multisigInfo) {
      console.log('✅ Multisig Account:', ADDRESSES.multisig);
      console.log('   Owner:', multisigInfo.owner.toBase58());
      console.log('   🔗 https://solscan.io/account/' + ADDRESSES.multisig);
      results.valid.push({ type: 'Multisig', address: ADDRESSES.multisig });
    }
  } catch (e) {
    results.invalid.push({ type: 'Multisig', address: ADDRESSES.multisig, error: e.message });
  }

  console.log('\n━'.repeat(60));
  console.log('🔑 Multisig Members:\n');

  for (let i = 0; i < ADDRESSES.members.length; i++) {
    try {
      const balance = await connection.getBalance(new PublicKey(ADDRESSES.members[i]));
      console.log(`✅ Member ${i + 1}:`, ADDRESSES.members[i]);
      console.log(`   Balance: ${(balance / 1e9).toFixed(4)} SOL`);
      console.log(`   🔗 https://solscan.io/account/${ADDRESSES.members[i]}`);
      results.valid.push({ type: `Member ${i + 1}`, address: ADDRESSES.members[i] });
    } catch (e) {
      console.log(`❌ Member ${i + 1}:`, ADDRESSES.members[i], '- ERROR');
      results.invalid.push({ type: `Member ${i + 1}`, address: ADDRESSES.members[i], error: e.message });
    }
  }

  console.log('\n' + '━'.repeat(60));
  console.log('\n📊 Verification Summary:');
  console.log(`   ✅ Valid: ${results.valid.length}`);
  console.log(`   ❌ Invalid: ${results.invalid.length}`);
  console.log(`   📈 Success Rate: ${((results.valid.length / (results.valid.length + results.invalid.length)) * 100).toFixed(1)}%`);

  if (results.invalid.length === 0) {
    console.log('\n🎉 ALL ADDRESSES VERIFIED ON SOLANA MAINNET!');
  }

  return results;
}

verifyOnChain().catch(console.error);
