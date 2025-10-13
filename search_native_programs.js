const https = require('https');

const HELIUS_API_KEY = process.env.HELIUS_API_KEY || 'your-api-key';

const nativePrograms = {
  'System Program': '11111111111111111111111111111111',
  'Token Program': 'TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA',
  'Associated Token': 'ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL',
  'Wrapped SOL': 'So11111111111111111111111111111111111111112',
  'Stake Program': 'Stake11111111111111111111111111111111111111',
  'Vote Program': 'Vote111111111111111111111111111111111111111',
  'Config Program': 'Config1111111111111111111111111111111111111',
  'BPF Loader': 'BPFLoaderUpgradeab1e11111111111111111111111'
};

const searchAddresses = [
  '61aq585V8cR2sZBeawJFt2NPqmN7zDi1sws4KLs5xHXV',
  '4p1FfVusdT83PxejTPLEz6ZQ4keN9LVEkKhzSt6PJ5zw',
  'K6U4dQ8jANMEqQQycXYiDcf3172NGefpQBzdDbavQbA'
];

function rpcCall(method, params) {
  return new Promise((resolve, reject) => {
    const data = JSON.stringify({ jsonrpc: '2.0', id: 1, method, params });
    const options = {
      hostname: 'mainnet.helius-rpc.com',
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

async function checkAddress(addr, name) {
  console.log(`\n${'='.repeat(80)}`);
  console.log(`${name}: ${addr}`);
  console.log('='.repeat(80));
  
  const info = await rpcCall('getAccountInfo', [addr, { encoding: 'jsonParsed' }]);
  
  if (info.result?.value) {
    const { lamports, owner, executable } = info.result.value;
    console.log(`✅ EXISTS`);
    console.log(`Balance: ${lamports / 1e9} SOL`);
    console.log(`Owner: ${owner}`);
    console.log(`Executable: ${executable}`);
  } else {
    console.log('❌ NOT FOUND');
  }
}

async function main() {
  console.log('\n🔍 NATIVE SOLANA PROGRAMS:');
  for (const [name, addr] of Object.entries(nativePrograms)) {
    await checkAddress(addr, name);
  }
  
  console.log('\n\n🔍 SEARCH ADDRESSES:');
  for (const addr of searchAddresses) {
    await checkAddress(addr, 'Custom Address');
  }
}

main().catch(console.error);
