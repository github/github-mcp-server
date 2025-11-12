const { Connection, PublicKey, Transaction, SystemProgram } = require('@solana/web3.js');
const bs58 = require('bs58').default || require('bs58');

const HELIUS_API_KEY = process.env.HELIUS_API_KEY;
const RPC_URL = HELIUS_API_KEY 
  ? `https://mainnet.helius-rpc.com/?api-key=${HELIUS_API_KEY}`
  : 'https://api.mainnet-beta.solana.com';

const NEW_MASTER_CONTROLLER = 'GLzZk1sczzW6fM4uPFeQCtTZQaf8H5VaBt99tUMbJAAW';
const BPFLOADER = 'BPFLoader2111111111111111111111111111111111';

async function buildTransaction() {
  const connection = new Connection(RPC_URL, 'confirmed');
  
  const newAccount = new PublicKey(NEW_MASTER_CONTROLLER);
  const feePayer = new PublicKey(BPFLOADER);
  
  console.log('\n🔨 Building Transaction for New Master Controller\n');
  console.log('Target Address:', NEW_MASTER_CONTROLLER);
  console.log('Fee Payer:', BPFLOADER);
  console.log('RPC:', RPC_URL.includes('helius') ? 'Helius' : 'Public', '\n');
  
  try {
    // Get recent blockhash
    const { blockhash, lastValidBlockHeight } = await connection.getLatestBlockhash();
    
    // Get rent exemption amount
    const rentExemption = await connection.getMinimumBalanceForRentExemption(0);
    
    // Create transaction
    const transaction = new Transaction({
      feePayer,
      recentBlockhash: blockhash,
    }).add(
      SystemProgram.createAccount({
        fromPubkey: feePayer,
        newAccountPubkey: newAccount,
        lamports: rentExemption,
        space: 0,
        programId: SystemProgram.programId,
      })
    );
    
    // Get transaction message for signature
    const message = transaction.compileMessage();
    const messageBytes = message.serialize();
    
    console.log('✅ Transaction Built Successfully\n');
    console.log('📋 Transaction Details:');
    console.log('- Blockhash:', blockhash);
    console.log('- Last Valid Block Height:', lastValidBlockHeight);
    console.log('- Rent Exemption:', rentExemption / 1e9, 'SOL');
    console.log('- Instructions:', transaction.instructions.length);
    console.log('\n🔐 Signature Requirements:');
    console.log('- Fee Payer (BPFLoader) must sign');
    console.log('- New Account must sign (if using keypair)');
    console.log('\n📦 Serialized Transaction:');
    console.log('- Message (base58):', bs58.encode(messageBytes));
    console.log('- Message (hex):', messageBytes.toString('hex'));
    
    // Get signatures for the address
    console.log('\n🔍 Fetching Recent Signatures for New Master Controller...');
    const signatures = await connection.getSignaturesForAddress(newAccount, { limit: 5 });
    
    if (signatures.length > 0) {
      console.log('\n✅ Found', signatures.length, 'signatures:');
      signatures.forEach((sig, i) => {
        console.log(`${i + 1}. ${sig.signature}`);
        console.log(`   Block Time: ${new Date(sig.blockTime * 1000).toISOString()}`);
        console.log(`   Status: ${sig.err ? 'ERROR' : 'SUCCESS'}`);
      });
    } else {
      console.log('\n⚠️  No signatures found - account does not exist yet');
    }
    
    return {
      transaction,
      blockhash,
      messageBase58: bs58.encode(messageBytes),
      messageHex: messageBytes.toString('hex'),
      rentExemption
    };
    
  } catch (error) {
    console.error('\n❌ Error:', error.message);
    if (error.message.includes('429')) {
      console.log('\n💡 Tip: Set HELIUS_API_KEY environment variable to avoid rate limits');
    }
    throw error;
  }
}

buildTransaction().catch(() => process.exit(1));
