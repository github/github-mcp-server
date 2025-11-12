const HELIUS_API_KEY = process.env.HELIUS_API_KEY || 'your-api-key';
const BASE_URL = `https://api-mainnet.helius-rpc.com/v0`;

// Controller Authority Address (New Master Controller)
const CONTROLLER_ADDRESS = 'GLzZk1sczzW6fM4uPFeQCtTZQaf8H5VaBt99tUMbJAAW';

const ADDRESSES = [CONTROLLER_ADDRESS];

async function getTransactionHistory(address) {
  const url = `${BASE_URL}/addresses/${address}/transactions?api-key=${HELIUS_API_KEY}&limit=5`;
  const response = await fetch(url);
  if (!response.ok) throw new Error(`${response.status}: ${await response.text()}`);
  return response.json();
}

async function parseTransactions(signatures) {
  const url = `${BASE_URL}/transactions/?api-key=${HELIUS_API_KEY}`;
  const response = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ transactions: signatures })
  });
  if (!response.ok) throw new Error(`${response.status}: ${await response.text()}`);
  return response.json();
}

async function main() {
  console.log('🔍 Fetching Enhanced Transaction Data\n');
  
  for (const address of ADDRESSES) {
    console.log(`\n📍 ${address}`);
    try {
      const txs = await getTransactionHistory(address);
      console.log(`   Found: ${txs.length} transactions`);
      
      if (txs.length > 0) {
        const sigs = txs.slice(0, 2).map(t => t.signature);
        const parsed = await parseTransactions(sigs);
        
        parsed.forEach((tx, i) => {
          console.log(`   ${i + 1}. ${tx.signature}`);
          console.log(`      Type: ${tx.type}`);
          console.log(`      Fee: ${tx.fee / 1e9} SOL`);
          console.log(`      Description: ${tx.description}`);
        });
      }
    } catch (error) {
      console.log(`   ❌ Error: ${error.message}`);
    }
  }
}

main();
