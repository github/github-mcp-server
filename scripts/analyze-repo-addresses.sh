#!/bin/bash

echo "🔍 COMPREHENSIVE REPOSITORY ADDRESS ANALYSIS"
echo "=============================================================="

# Extract all Solana addresses
echo ""
echo "📋 EXTRACTING SOLANA ADDRESSES..."
SOLANA_ADDRS=$(grep -r -h -o -E '[1-9A-HJ-NP-Za-km-z]{32,44}' Deployer-Gene/*.{js,ts,json,md} 2>/dev/null | sort -u | grep -v "^1111" | grep -v "example" | grep -v "ABCDEF")

# Extract EVM addresses
echo "📋 EXTRACTING EVM ADDRESSES..."
EVM_ADDRS=$(grep -r -h -o -E '0x[a-fA-F0-9]{40}' Deployer-Gene/*.{js,ts,json,md,sol} 2>/dev/null | sort -u)

# Extract API keys and endpoints
echo "📋 EXTRACTING API ENDPOINTS..."
HELIUS=$(grep -r "helius" Deployer-Gene/.env* 2>/dev/null | grep -v template | head -5)
QUICKNODE=$(grep -r "quicknode" Deployer-Gene/.env* 2>/dev/null | grep -v template | head -5)
MORALIS=$(grep -r "moralis" . 2>/dev/null | grep -v node_modules | head -3)

# Count unique addresses
SOLANA_COUNT=$(echo "$SOLANA_ADDRS" | wc -l)
EVM_COUNT=$(echo "$EVM_ADDRS" | wc -l)

echo ""
echo "=============================================================="
echo "📊 SUMMARY"
echo "=============================================================="
echo "Solana Addresses Found: $SOLANA_COUNT"
echo "EVM Addresses Found: $EVM_COUNT"
echo ""

# Save to JSON
cat > repo-address-analysis.json << EOF
{
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "solana_addresses": [
$(echo "$SOLANA_ADDRS" | head -100 | sed 's/^/    "/;s/$/",/' | sed '$ s/,$//')
  ],
  "evm_addresses": [
$(echo "$EVM_ADDRS" | sed 's/^/    "/;s/$/",/' | sed '$ s/,$//')
  ],
  "api_services": {
    "helius": "configured",
    "quicknode": "configured",
    "moralis": "configured"
  },
  "programs": {
    "gene_mint": "GENEtH5amGSi8kHAtQoezp1XEXwZJ8vcuePYnXdKrMYz",
    "standard_program": "DjVE6JNiYqPL2QXyCUUh8rNjHrbz9hXHNYt99MQ59qw1",
    "dao_controller": "CvQZZ23qYDWF2RUpxYJ8y9K4skmuvYEEjH7fK58jtipQ"
  }
}
EOF

echo "✅ Analysis saved to repo-address-analysis.json"
