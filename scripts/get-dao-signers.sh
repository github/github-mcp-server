#!/bin/bash

DAO_CONTROLLER="CvQZZ23qYDWF2RUpxYJ8y9K4skmuvYEEjH7fK58jtipQ"
RPC="https://api.mainnet-beta.solana.com"

echo "🔍 DAO CONTROLLER SIGNER ANALYSIS"
echo "📍 Address: $DAO_CONTROLLER"
echo "======================================================================"

echo ""
echo "📋 FETCHING TRANSACTIONS..."
SIGNATURES=$(curl -s -X POST $RPC -H "Content-Type: application/json" -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"getSignaturesForAddress\",\"params\":[\"$DAO_CONTROLLER\",{\"limit\":100}]}" | jq -r '.result[].signature')

SIGNER_COUNT=$(echo "$SIGNATURES" | wc -l)
echo "   Found $SIGNER_COUNT transactions"

echo ""
echo "🔐 EXTRACTING SIGNERS..."

declare -A SIGNERS
COUNTER=0

for SIG in $SIGNATURES; do
    COUNTER=$((COUNTER + 1))
    
    if [ $COUNTER -gt 20 ]; then
        break
    fi
    
    TX_DATA=$(curl -s -X POST $RPC -H "Content-Type: application/json" -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"getTransaction\",\"params\":[\"$SIG\",{\"encoding\":\"jsonParsed\",\"maxSupportedTransactionVersion\":0}]}")
    
    SIGNER=$(echo "$TX_DATA" | jq -r '.result.transaction.message.accountKeys[]? | select(.signer == true) | .pubkey' | head -1)
    
    if [ ! -z "$SIGNER" ] && [ "$SIGNER" != "$DAO_CONTROLLER" ]; then
        SIGNERS[$SIGNER]=1
    fi
    
    if [ $((COUNTER % 5)) -eq 0 ]; then
        echo "   Processed $COUNTER/20 transactions..."
    fi
done

echo ""
echo "======================================================================"
echo "✅ UNIQUE SIGNERS FOUND: ${#SIGNERS[@]}"
echo "======================================================================"

IDX=1
for SIGNER in "${!SIGNERS[@]}"; do
    echo ""
    echo "$IDX. $SIGNER"
    IDX=$((IDX + 1))
done

echo ""
echo "📝 Saving results..."

cat > dao-signers.json << EOF
{
  "controller": "$DAO_CONTROLLER",
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "totalSigners": ${#SIGNERS[@]},
  "signers": [
$(for SIGNER in "${!SIGNERS[@]}"; do echo "    \"$SIGNER\","; done | sed '$ s/,$//')
  ]
}
EOF

echo "✅ Results saved to dao-signers.json"
