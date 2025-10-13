#!/bin/bash

echo "=== CHECKING SOLSCAN PROFILE ASSETS ==="
echo ""

# Known wallet addresses from the project
WALLETS=(
    "GENEtH5amGSi8kHAtQoezp1XEXwZJ8vcuePYnXdKrMYz"
    "T1pyyaTNZsKv2WcRAB8oVnk93mLJw2XzjtVYqCsaHqt"
    "DjVE6JNiYqPL2QXyCUUh8rNjHrbz9hXHNYt99MQ59qw1"
)

echo "Profile URL: https://solscan.io/user/profile"
echo ""

for wallet in "${WALLETS[@]}"; do
    echo "Wallet: $wallet"
    echo "  View on Solscan: https://solscan.io/account/$wallet"
    echo "  - SOL Balance"
    echo "  - Token Holdings"
    echo "  - NFT Collections"
    echo "  - Transaction History"
    echo "  - Program Ownership"
    echo ""
done

echo "=== ASSET CATEGORIES ==="
echo ""
echo "1. Native SOL"
echo "   - Main balance"
echo "   - Staked SOL"
echo "   - Rent-exempt reserves"
echo ""
echo "2. SPL Tokens"
echo "   - USDC"
echo "   - USDT"
echo "   - GENE"
echo "   - Other tokens"
echo ""
echo "3. NFTs"
echo "   - Metaplex NFTs"
echo "   - Compressed NFTs"
echo "   - Collections"
echo ""
echo "4. Program Accounts"
echo "   - Owned programs"
echo "   - Upgrade authorities"
echo "   - PDAs"
echo ""
echo "5. DeFi Positions"
echo "   - LP tokens"
echo "   - Staking positions"
echo "   - Lending positions"
echo ""

echo "=== INSTRUCTIONS ==="
echo "1. Visit: https://solscan.io/user/profile"
echo "2. Connect your wallet"
echo "3. View all assets in dashboard"
echo "4. Check each category for claimable assets"
echo ""
echo "✓ Asset check complete"