#!/usr/bin/env node
const { Connection, PublicKey, Keypair, Transaction, TransactionInstruction, SystemProgram } = require('@solana/web3.js');
const bs58 = require('bs58');

const JUPITER_PROGRAM = 'JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4';
const PROGRAM_DATA = '4Ec7ZxZS6Sbdg5UGSLHbAnM7GQHp2eFd4KYWRexAipQT';
const CURRENT_AUTHORITY = 'CvQZZ23qYDWF2RUpxYJ8y9K4skmuvYEEjH7fK58jtipQ';
const NEW_CONTROLLER = 'GLzZk1sczzW6fM4uPFeQCtTZQaf8H5VaBt99tUMbJAAW';
const NEW_CONTROLLER_PRIVATE = 'f2a29d46687020f38c36e1299da68ac03c01e660254b8bc9c8166b39945c1e76e3fe6d7ba360580cffa9601eafad20f01044eded4deea9f83dac3e9607d2e5f3';

const MULTISIG_MEMBERS = [
  '2MgqMXdwSf3bRZ6S8uKJSffZAaoZBhD2mjst3phJXE7p',
  '89FnbsKH8n6FXCghGUijxh3snqx3e6VXJ7q1fQAHWkQQ',
  'BYidGfUnfoQtqi4nHiuo57Fjreizbej6hawJLnbwJmYr',
  'CHRDWWqUs6LyeeoD7pJb3iRfnvYeMfwMUtf2N7zWk7uh',
  'Dg5NLa5JuwfRMkuwZEguD9RpVrcQD3536GxogUv7pLNV',
  'EhJqf1p39c8NnH5iuZAJyw778LQua1AhZWxarT5SF8sT',
  'GGG2JyBtwbPAsYwUQED8GBbj9UMi7NQa3uwN3DmyGNtz'
];

async function executeTransfer() {
  const connection = new Connection('https://api.mainnet-beta.solana.com', 'confirmed');
  const signer = Keypair.fromSecretKey(Buffer.from(NEW_CONTROLLER_PRIVATE, 'hex'));
  
  console.log('🔐 Authority Transfer Execution\n');
  console.log('Signer:', signer.publicKey.toBase58());
  console.log('Program:', JUPITER_PROGRAM);
  console.log('New Authority:', NEW_CONTROLLER);
  console.log('━'.repeat(50));

  const BPF_LOADER = new PublicKey('BPFLoaderUpgradeab1e11111111111111111111111');
  
  const setAuthorityIx = new TransactionInstruction({
    programId: BPF_LOADER,
    keys: [
      { pubkey: new PublicKey(PROGRAM_DATA), isSigner: false, isWritable: true },
      { pubkey: new PublicKey(CURRENT_AUTHORITY), isSigner: true, isWritable: false },
      { pubkey: new PublicKey(NEW_CONTROLLER), isSigner: false, isWritable: false }
    ],
    data: Buffer.from([4, 0, 0, 0])
  });

  const tx = new Transaction().add(setAuthorityIx);
  tx.feePayer = signer.publicKey;
  tx.recentBlockhash = (await connection.getLatestBlockhash()).blockhash;

  const message = tx.serializeMessage();
  const messageBase58 = bs58.default.encode(message);

  console.log('\n📋 Transaction Created');
  console.log('Message (Base58):', messageBase58.slice(0, 32) + '...');
  console.log('Full Message:', messageBase58);
  console.log('\n🔑 Multisig Members (4 of 7 required):');
  MULTISIG_MEMBERS.forEach((m, i) => console.log(`   ${i + 1}. ${m}`));
  
  console.log('\n✅ Transaction ready for multisig approval');
  console.log('Multisig Account: 7ZyDFzet6sKgZLN4D89JLfo7chu2n7nYdkFt5RCFk8Sf');
  console.log('Threshold: 4 of 7 signatures required');

  return {
    message: messageBase58,
    signer: signer.publicKey.toBase58(),
    multisigMembers: MULTISIG_MEMBERS,
    threshold: '4 of 7',
    status: 'AWAITING_SIGNATURES'
  };
}

executeTransfer().then(console.log).catch(console.error);
