const API_KEY = '4fe39d22-5043-40d3-b2a1-dd8968ecf8a6';
const RPC_URL = `https://mainnet.helius-rpc.com/?api-key=${API_KEY}`;
const CONTROLLER = 'GLzZk1sczzW6fM4uPFeQCtTZQaf8H5VaBt99tUMbJAAW';

async function getTransactions() {
  const response = await fetch(RPC_URL, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      jsonrpc: '2.0',
      id: 1,
      method: 'getTransactionsForAddress',
      params: [
        CONTROLLER,
        {
          transactionDetails: 'full',
          sortOrder: 'asc',
          limit: 10
        }
      ]
    })
  });
  
  const data = await response.json();
  console.log(JSON.stringify(data, null, 2));
}

getTransactions();
