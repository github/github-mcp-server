const https = require('https');
const fs = require('fs');

const HELIUS_API_KEY = process.env.HELIUS_API_KEY || 'your-api-key';

const programs = [
  '4eJZVbbsiLAG6EkWvgEYEWKEpdhJPFBYMeJ6DBX98w6a',
  'jaJrDgf4U8DAZcUD3t5AwL7Cfe2QnkpXZXGegdUHc4ZE',
  '11111111111111111111111111111111',
  'TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA',
  'So11111111111111111111111111111111111111112',
  'ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL',
  'Stake11111111111111111111111111111111111111',
  'Vote111111111111111111111111111111111111111',
  'BPFLoaderUpgradeab1e11111111111111111111111',
  'JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4',
  '4Ec7ZxZS6Sbdg5UGSLHbAnM7GQHp2eFd4KYWRexAipQT'
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

async function main() {
  const results = [];
  
  for (const addr of programs) {
    console.log(`Checking: ${addr}`);
    const info = await rpcCall('getAccountInfo', [addr, { encoding: 'jsonParsed' }]);
    
    if (info.result?.value) {
      const { lamports, owner, executable } = info.result.value;
      results.push({
        address: addr,
        balance: lamports / 1e9,
        owner,
        executable,
        exists: true
      });
      console.log(`✅ ${lamports / 1e9} SOL`);
    } else {
      results.push({ address: addr, exists: false });
      console.log('❌ NOT FOUND');
    }
  }
  
  fs.writeFileSync('program_results.json', JSON.stringify(results, null, 2));
  console.log('\n✅ Results saved to program_results.json');
}

main().catch(console.error);
