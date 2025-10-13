# ⚡ Quick Deploy - Bot Army Funding

## 🚀 One-Command Deployment

```bash
gh workflow run bot-funding-deployment.yml -f bot_number=all -f dry_run=false
```

---

## 📋 Quick Reference

| Command | Action |
|---------|--------|
| `bot_number=all` | Deploy all 8 bots |
| `bot_number=1` | Deploy bot 1 only |
| `dry_run=true` | Test without execution |
| `dry_run=false` | Execute real deployment |

---

## 🤖 Bot Quick List

```
1. HKBJoeUWH6pUQuLd9CZWrJBzGSE9roEW4bshnxd9AHsR → 1,000 tokens
2. NqGHDaaLWmND7uShuaZkVbGNQFy6pS96qHyfR3pGR2d → 1,500 tokens
3. DbhKvqweZECTyYQ7PRJoHmKt8f262fsBCGHxSaD5BPqA → 2,000 tokens
4. 7uSCVM1MJPKctrSRzuFN7qfVoJX78q6V5q5JuzRPaK41 → 2,500 tokens
5. 3oFCkoneQShDsJMZYscXew4jGwgLjpxfykHuGo85QyLw → 3,000 tokens
6. 8duk9DzqBVXmqiyci9PpBsKuRCwg6ytzWywjQztM6VzS → 3,500 tokens
7. 96891wG6iLVEDibwjYv8xWFGFiEezFQkvdyTrM69ou24 → 4,000 tokens
8. 2A8qGB3iZ21NxGjX4EjjWJKc9PFG1r7F4jkcR66dc4mb → 5,000 tokens
```

**Total**: 22,500 tokens | **Cost**: $0.00

---

## ✅ Status Check

```bash
# View workflow runs
gh run list --workflow=bot-funding-deployment.yml

# Watch live deployment
gh run watch

# Check bot balance
solana balance {BOT_ADDRESS}
```

---

## 🔗 Links

- **Workflow**: `.github/workflows/bot-funding-deployment.yml`
- **Script**: `Deployer-Gene/scripts/mint-bot.js`
- **Guide**: `BOT_DEPLOYMENT_GUIDE.md`
- **Status**: `DEPLOYMENT_STATUS.md`
