#!/bin/bash

MORALIS_API_KEY="c4d1d108f46144f1955612d3ac03dcd5"
MORALIS_NODE_URL="https://site2.moralis-nodes.com/eth/c4d1d108f46144f1955612d3ac03dcd5"

echo "=== MORALIS API SETUP ==="
echo ""
echo "API Key: $MORALIS_API_KEY"
echo "Node URL: $MORALIS_NODE_URL"
echo ""

# Create .env.moralis
cat > .env.moralis << EOF
MORALIS_API_KEY=$MORALIS_API_KEY
MORALIS_NODE_URL=$MORALIS_NODE_URL
MORALIS_NETWORK=mainnet
EOF

echo "✓ Created .env.moralis"
echo ""

# Test connection
echo "Testing connection..."
RESPONSE=$(curl -s -X POST "$MORALIS_NODE_URL" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}')

echo "Response: $RESPONSE"
echo ""

if [ ! -z "$RESPONSE" ]; then
    echo "✓ Moralis API connected successfully"
else
    echo "✗ Connection failed"
fi

echo ""
echo "=== MORALIS CONFIGURATION ==="
echo "API Key: $MORALIS_API_KEY"
echo "Endpoint: $MORALIS_NODE_URL"
echo "Network: Ethereum Mainnet"
echo ""
echo "✓ Setup complete"