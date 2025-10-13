# 🔐 Allowlist Workflow Guide

## Overview

GitHub Actions workflows now support dynamic allowlist management with write permissions.

---

## Features

### Permissions
```yaml
permissions:
  contents: write
  actions: write
  pull-requests: write
```

### Workflow Inputs

#### Bot Funding Deployment
- `bot_number`: Choice (1-8 or all)
- `dry_run`: Boolean (true/false)
- `allowlist_addresses`: String (comma-separated addresses)

#### Cross-Chain Deployment
- `chain`: Choice (solana/evm/both)
- `dry_run`: Boolean (true/false)
- `allowlist_addresses`: String (comma-separated addresses)
- `update_allowlist`: Boolean (enable allowlist update)

---

## Usage

### Add Addresses During Bot Deployment

```bash
gh workflow run bot-funding-deployment.yml \
  -f bot_number=all \
  -f dry_run=false \
  -f allowlist_addresses="addr1,addr2,addr3"
```

### Add Addresses During Cross-Chain Deployment

```bash
gh workflow run cross-chain-deploy.yml \
  -f chain=both \
  -f dry_run=false \
  -f update_allowlist=true \
  -f allowlist_addresses="0xABC...,SolanaAddr..."
```

### Manual Allowlist Update

```bash
# Using the utility script
node scripts/update-allowlist.js "addr1,addr2,addr3"

# Or with environment variable
NEW_ADDRESSES="addr1,addr2,addr3" node scripts/update-allowlist.js
```

---

## Moralis Wallet Query

Query EVM wallet token balances:

```bash
# Default wallet
go run scripts/moralis-wallet-query.go

# Custom wallet
WALLET_ADDRESS=0xYourAddress go run scripts/moralis-wallet-query.go

# Different chain
CHAIN=polygon WALLET_ADDRESS=0xYourAddress go run scripts/moralis-wallet-query.go

# Custom API key
MORALIS_API_KEY=your_key WALLET_ADDRESS=0xYourAddress go run scripts/moralis-wallet-query.go
```

### Supported Chains
- `eth` - Ethereum
- `polygon` - Polygon
- `bsc` - Binance Smart Chain
- `arbitrum` - Arbitrum
- `optimism` - Optimism

---

## Workflow Behavior

### Allowlist Update Job
1. Checks if `allowlist_addresses` input provided
2. Runs `update-allowlist.js` script
3. Adds new addresses to `VERCEL_DEPLOYMENT_ALLOWLIST.json`
4. Commits changes (if not dry-run)
5. Pushes to repository

### Auto-Commit
```bash
git config user.name "github-actions[bot]"
git config user.email "github-actions[bot]@users.noreply.github.com"
git add VERCEL_DEPLOYMENT_ALLOWLIST.json
git commit -m "🔐 Update allowlist: addr1,addr2,addr3"
git push
```

---

## Examples

### Example 1: Deploy Bots + Add Addresses
```bash
gh workflow run bot-funding-deployment.yml \
  -f bot_number=all \
  -f dry_run=false \
  -f allowlist_addresses="HKBJoeUWH6pUQuLd9CZWrJBzGSE9roEW4bshnxd9AHsR,NqGHDaaLWmND7uShuaZkVbGNQFy6pS96qHyfR3pGR2d"
```

### Example 2: Cross-Chain + Update Allowlist
```bash
gh workflow run cross-chain-deploy.yml \
  -f chain=both \
  -f dry_run=false \
  -f update_allowlist=true \
  -f allowlist_addresses="0x742d35Cc6634C0532925a3b8D4C9db96C4b4d8b6"
```

### Example 3: Dry Run Test
```bash
gh workflow run bot-funding-deployment.yml \
  -f bot_number=1 \
  -f dry_run=true \
  -f allowlist_addresses="TestAddress123"
```

---

## Script Details

### update-allowlist.js
```javascript
// Usage
node scripts/update-allowlist.js "addr1,addr2,addr3"

// Features
- Reads VERCEL_DEPLOYMENT_ALLOWLIST.json
- Adds new addresses to allowlist array
- Skips duplicates
- Writes updated JSON
- Reports changes
```

### moralis-wallet-query.go
```go
// Usage
go run scripts/moralis-wallet-query.go

// Features
- Queries Moralis API for wallet tokens
- Supports multiple chains
- Shows token balances
- Identifies spam tokens
- Verifies contracts
```

---

## File Structure

```
CryptonoutController/
├── .github/workflows/
│   ├── bot-funding-deployment.yml    # With allowlist support
│   └── cross-chain-deploy.yml        # With allowlist support
├── scripts/
│   ├── update-allowlist.js           # Allowlist utility
│   └── moralis-wallet-query.go       # Moralis query tool
└── VERCEL_DEPLOYMENT_ALLOWLIST.json  # Allowlist file
```

---

## Security Notes

- Workflow requires write permissions
- Auto-commits use github-actions bot
- Dry-run mode skips commits
- Duplicate addresses automatically skipped
- All changes tracked in git history

---

## Troubleshooting

### Permission Denied
Ensure repository has Actions write permissions enabled in Settings > Actions > General.

### Allowlist Not Updated
Check workflow logs for errors. Verify `VERCEL_DEPLOYMENT_ALLOWLIST.json` exists.

### Moralis API Error
Verify API key is valid. Check rate limits. Ensure wallet address format is correct.

---

**Status**: ✅ READY  
**Commit**: 6cb4c19  
**Features**: Allowlist write + Moralis integration
