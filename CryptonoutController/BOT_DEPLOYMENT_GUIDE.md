# 🤖 Bot Army Automated Deployment Guide

## 🎯 Overview

Automated GitHub Actions workflow for sequential bot funding using relayer deployment logic. Each bot is funded one-by-one for clear deployment tracking and verification.

---

## 🚀 Quick Start

### 1. Trigger Deployment

**Via GitHub UI:**
1. Go to Actions tab
2. Select "Bot Army Funding Deployment"
3. Click "Run workflow"
4. Choose bot number (1-8 or "all")
5. Set dry_run (true/false)

**Via GitHub CLI:**
```bash
# Fund all bots
gh workflow run bot-funding-deployment.yml -f bot_number=all -f dry_run=false

# Fund specific bot
gh workflow run bot-funding-deployment.yml -f bot_number=1 -f dry_run=false

# Dry run test
gh workflow run bot-funding-deployment.yml -f bot_number=all -f dry_run=true
```

---

## 📋 Bot Configuration

| # | Role | Address | Amount |
|---|------|---------|--------|
| 1 | Stake Master | HKBJoeUWH6pUQuLd9CZWrJBzGSE9roEW4bshnxd9AHsR | 1,000 |
| 2 | Mint Operator | NqGHDaaLWmND7uShuaZkVbGNQFy6pS96qHyfR3pGR2d | 1,500 |
| 3 | Contract Deployer | DbhKvqweZECTyYQ7PRJoHmKt8f262fsBCGHxSaD5BPqA | 2,000 |
| 4 | MEV Hunter | 7uSCVM1MJPKctrSRzuFN7qfVoJX78q6V5q5JuzRPaK41 | 2,500 |
| 5 | Loot Extractor | 3oFCkoneQShDsJMZYscXew4jGwgLjpxfykHuGo85QyLw | 3,000 |
| 6 | Advanced | 8duk9DzqBVXmqiyci9PpBsKuRCwg6ytzWywjQztM6VzS | 3,500 |
| 7 | Elite | 96891wG6iLVEDibwjYv8xWFGFiEezFQkvdyTrM69ou24 | 4,000 |
| 8 | Master | 2A8qGB3iZ21NxGjX4EjjWJKc9PFG1r7F4jkcR66dc4mb | 5,000 |

**Total**: 22,500 tokens

---

## 🔄 Deployment Flow

```
Bot 1 → Bot 2 → Bot 3 → Bot 4 → Bot 5 → Bot 6 → Bot 7 → Bot 8 → Summary
```

Each bot deployment:
1. ✅ Checkout code
2. ✅ Setup Node.js
3. ✅ Execute mint script
4. ✅ Submit to relayer
5. ✅ Verify transaction
6. ✅ Continue to next bot

---

## 💰 Relayer Logic

### Zero-Cost Deployment
- User signs transaction (no gas cost)
- Relayer submits to network
- Relayer pays all fees
- Mainnet-beta only

### Relayer Configuration
```
URL: https://api.helius.xyz/v0/transactions/submit
Network: mainnet-beta
Cost: $0.00
```

---

## 📊 Monitoring

### GitHub Actions Dashboard
- Real-time deployment status
- Per-bot success/failure
- Transaction signatures
- Deployment summary

### Verification Commands
```bash
# Check bot balance
solana balance HKBJoeUWH6pUQuLd9CZWrJBzGSE9roEW4bshnxd9AHsR

# Check token account
spl-token accounts 3i62KXuWERyTZJ5HbE7HNbhvBAhEdMjMjLQk3m39PpN4

# View on explorer
https://explorer.solana.com/address/{BOT_ADDRESS}
```

---

## 🔧 Local Testing

```bash
# Install dependencies
cd Deployer-Gene
npm install @solana/web3.js @solana/spl-token

# Test single bot (dry run)
node scripts/mint-bot.js \
  --bot=1 \
  --address=HKBJoeUWH6pUQuLd9CZWrJBzGSE9roEW4bshnxd9AHsR \
  --amount=1000 \
  --mint=3i62KXuWERyTZJ5HbE7HNbhvBAhEdMjMjLQk3m39PpN4 \
  --relayer=https://api.helius.xyz/v0/transactions/submit \
  --dry-run=true
```

---

## 🎯 Deployment Scenarios

### Scenario 1: Fund All Bots
```bash
gh workflow run bot-funding-deployment.yml \
  -f bot_number=all \
  -f dry_run=false
```

### Scenario 2: Fund Single Bot
```bash
gh workflow run bot-funding-deployment.yml \
  -f bot_number=3 \
  -f dry_run=false
```

### Scenario 3: Dry Run Test
```bash
gh workflow run bot-funding-deployment.yml \
  -f bot_number=all \
  -f dry_run=true
```

---

## ✅ Success Criteria

- ✅ All 8 bots funded sequentially
- ✅ Each transaction confirmed on-chain
- ✅ Token balances verified
- ✅ Zero gas cost to user
- ✅ Deployment summary generated

---

## 🔒 Security

- Private keys stored in GitHub Secrets
- Relayer handles all transactions
- Mainnet-beta only (no devnet)
- Sequential deployment for safety
- Dry run mode for testing

---

## 📝 Files

- `.github/workflows/bot-funding-deployment.yml` - GitHub Actions workflow
- `Deployer-Gene/scripts/mint-bot.js` - Minting script
- `BOT_DEPLOYMENT_GUIDE.md` - This guide
- `BOT-FUNDING-COMPLETE.md` - Original funding plan

---

## 🎉 Post-Deployment

After successful deployment:
1. Verify all bot balances
2. Activate trading operations
3. Monitor treasury accumulation
4. Scale operations as needed

---

**Status**: ✅ READY FOR DEPLOYMENT  
**Cost**: $0.00 (Relayer Pays)  
**Network**: Mainnet-Beta Only  
**Method**: Sequential One-by-One
