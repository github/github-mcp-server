#!/bin/bash

CONTROLLER="4gLAGDEHs6sJ6AMmLdAwCUx9NPmPLxoMCZ3yiKyAyQ1m"

echo "=== SET UPGRADE AUTHORITY ==="
echo ""

# Check if Solana CLI is installed
if ! command -v solana &> /dev/null; then
    echo "❌ Solana CLI not installed"
    echo ""
    echo "Install with:"
    echo "sh -c \"\$(curl -sSfL https://release.solana.com/stable/install)\""
    exit 1
fi

echo "✓ Solana CLI installed"
echo ""

# Check if PROGRAM_ID is provided
if [ -z "$1" ]; then
    echo "Usage: $0 <PROGRAM_ID>"
    echo ""
    echo "Example:"
    echo "$0 Pent6171a77867d749b7ff764e2aab2ba12cba4813f13f08"
    echo ""
    echo "Controller: $CONTROLLER"
    exit 1
fi

PROGRAM_ID="$1"

echo "Program ID: $PROGRAM_ID"
echo "New Authority: $CONTROLLER"
echo ""

# Execute the command
echo "Executing:"
echo "solana program set-upgrade-authority $PROGRAM_ID $CONTROLLER"
echo ""

solana program set-upgrade-authority "$PROGRAM_ID" "$CONTROLLER"

if [ $? -eq 0 ]; then
    echo ""
    echo "✓ Upgrade authority set successfully"
    echo ""
    echo "Verify:"
    echo "solana program show $PROGRAM_ID"
else
    echo ""
    echo "❌ Failed to set upgrade authority"
    echo ""
    echo "Common issues:"
    echo "- Program ID doesn't exist"
    echo "- Current authority doesn't match signer"
    echo "- Insufficient SOL for transaction"
fi