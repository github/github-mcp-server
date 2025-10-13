const https = require('https');

const HELIUS_API_KEY = process.env.HELIUS_API_KEY || 'your-api-key';
const addresses = [
  '4p1FfVusdT83PxejTPLEz6ZQ4keN9LVEkKhzSt6PJ5zw',
  'K6U4dQ8jANMEqQQycXYiDcf3172NGefpQBzdDbavQbA'
];

function rpcCall(method, params) {
  return new Promise((resolve, reject) => {
    const data = JSON.stringify({ jsonrpc: '2.0', id: 1, method, params });
    const options = {
      hostname: `mainnet.helius-rpc.com`,
      path: `/?api-key=${HELIUS_API_KEY}`,
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Content-Length': data.length }
    };
    
    const req = https.request(options, res => {
      let body = '';
      res.on('data', chunk => body += chunk);
      res.on('end', () => resolve(JSON.parse(body)));
    });
    req.on('error', reject);
    req.write(data);
    req.end();
  });
}

async function lookupAddresses() {
  for (const addr of addresses) {
    console.log(`\n${'='.repeat(80)}`);
    console.log(`ADDRESS: ${addr}`);
    console.log('='.repeat(80));
    
    const accountInfo = await rpcCall('getAccountInfo', [addr, { encoding: 'jsonParsed' }]);
    
    if (accountInfo.result?.value) {
      const { lamports, owner, data, executable } = accountInfo.result.value;
      console.log(`Balance: ${lamports / 1e9} SOL`);
      console.log(`Owner: ${owner}`);
      console.log(`Executable: ${executable}`);
      console.log(`Data:`, JSON.stringify(data, null, 2));
    } else {
      console.log('❌ Account not found or empty');
    }
  }
}

lookupAddresses().catch(console.error);
