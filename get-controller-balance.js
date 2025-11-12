const API_KEY = '4fe39d22-5043-40d3-b2a1-dd8968ecf8a6';
const RPC_URL = `https://mainnet.helius-rpc.com/?api-key=${API_KEY}`;
const CONTROLLER = 'GLzZk1sczzW6fM4uPFeQCtTZQaf8H5VaBt99tUMbJAAW';

async function getBalance() {
  const response = await fetch(RPC_URL, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      jsonrpc: '2.0',
      id: 1,
      method: 'getBalance',
      params: [CONTROLLER]
    })
  });
  
  const data = await response.json();
  const lamports = data.result?.value || 0;
  const sol = lamports / 1e9;
  
  console.log('💰 Controller Authority Balance\n');
  console.log('Address:', CONTROLLER);
  console.log('Lamports:', lamports);
  console.log('SOL:', sol);
  console.log('Status:', lamports === 0 ? '❌ Account does not exist' : '✅ Account exists');
}

getBalance();
