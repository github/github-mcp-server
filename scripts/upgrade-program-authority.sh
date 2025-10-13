#!/bin/bash

NEW_AUTHORITY="4gLAGDEHs6sJ6AMmLdAwCUx9NPmPLxoMCZ3yiKyAyQ1m"

echo "=== UPGRADE PROGRAM AUTHORITY ==="
echo ""
echo "New Authority: $NEW_AUTHORITY"
echo ""

# Program IDs from deploy-ready-programs.js
PROGRAMS=(
    "GENEtH5amGSi8kHAtQoezp1XEXwZJ8vcuePYnXdKrMYz"  # Gene Mint
    "DjVE6JNiYqPL2QXyCUUh8rNjHrbz9hXHNYt99MQ59qw1"  # Standard Program
    "CvQZZ23qYDWF2RUpxYJ8y9K4skmuvYEEjH7fK58jtipQ"  # DAO Master Controller
)

echo "Programs to upgrade:"
for program in "${PROGRAMS[@]}"; do
    echo "  - $program"
done
echo ""

if ! command -v solana &> /dev/null; then
    echo "❌ Solana CLI not installed"
    echo ""
    echo "Commands to run (after installing Solana CLI):"
    echo ""
    for program in "${PROGRAMS[@]}"; do
        echo "solana program set-upgrade-authority $program $NEW_AUTHORITY"
    done
    exit 1
fi

echo "Upgrading authorities..."
echo ""

for program in "${PROGRAMS[@]}"; do
    echo "Setting authority for: $program"
    solana program set-upgrade-authority "$program" "$NEW_AUTHORITY"
    
    if [ $? -eq 0 ]; then
        echo "✓ Success"
    else
        echo "✗ Failed"
    fi
    echo ""
done

echo "=== UPGRADE COMPLETE ==="
echo ""
echo "Verify with:"
for program in "${PROGRAMS[@]}"; do
    echo "solana program show $program"
done