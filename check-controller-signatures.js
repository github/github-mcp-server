const API_KEY = '4fe39d22-5043-40d3-b2a1-dd8968ecf8a6';
const RPC_URL = `https://mainnet.helius-rpc.com/?api-key=${API_KEY}`;
const CONTROLLER = 'GLzZk1sczzW6fM4uPFeQCtTZQaf8H5VaBt99tUMbJAAW';

async function getSignatures() {
  const response = await fetch(RPC_URL, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      jsonrpc: '2.0',
      id: 1,
      method: 'getSignaturesForAddress',
      params: [CONTROLLER, { limit: 10 }]
    })
  });
  
  const data = await response.json();
  
  console.log('🔍 Controller Authority Signature Check\n');
  console.log('Address:', CONTROLLER);
  console.log('Result:', JSON.stringify(data.result, null, 2));
  
  if (data.result && data.result.length > 0) {
    console.log(`\n✅ Found ${data.result.length} signatures`);
    data.result.forEach((sig, i) => {
      console.log(`\n${i + 1}. ${sig.signature}`);
      console.log(`   Slot: ${sig.slot}`);
      console.log(`   Status: ${sig.err ? 'FAILED' : 'SUCCESS'}`);
      if (sig.blockTime) {
        console.log(`   Time: ${new Date(sig.blockTime * 1000).toISOString()}`);
      }
    });
  } else {
    console.log('\n⚠️  No signatures found - account does not exist');
  }
}

getSignatures();
