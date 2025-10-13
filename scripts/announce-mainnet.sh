#!/bin/bash

echo "🚀 SOLANA MAINNET ANNOUNCEMENT"
echo "======================================================================"

# Create GitHub Release
echo ""
echo "📢 Creating GitHub Release v2.0.0..."

gh release create v2.0.0 \
  --title "v2.0.0 - Cross-Chain Integration (Solana + SKALE)" \
  --notes-file SOLANA_MAINNET_ANNOUNCEMENT.md \
  --latest

# Create Discussion
echo ""
echo "💬 Creating GitHub Discussion..."

gh api \
  --method POST \
  -H "Accept: application/vnd.github+json" \
  /repos/github/github-mcp-server/discussions \
  -f title="🚀 Solana Mainnet Deployment - 11 Bots Live!" \
  -f body="$(cat SOLANA_MAINNET_ANNOUNCEMENT.md)" \
  -f category_id="announcements"

# Update README
echo ""
echo "📝 Updating README with deployment status..."

cat >> README.md << 'EOF'

---

## 🚀 Latest: Solana Mainnet Deployment

**v2.0.0 Released** - October 13, 2025

We've successfully deployed our cross-chain bot army on Solana Mainnet-Beta and SKALE Mainnet!

### Highlights
- ✅ **11 Automated Agents** (8 Solana + 3 EVM)
- ✅ **Zero-Cost Deployment** (Relayer-based)
- ✅ **44 Allowlisted Addresses**
- ✅ **Cross-Chain Bridge** (Unified treasury)

[Read Full Announcement](SOLANA_MAINNET_ANNOUNCEMENT.md) | [View Changelog](CHANGELOG_V2.0.0.md)

EOF

# Commit and push
echo ""
echo "💾 Committing changes..."

git add .
git commit -m "🚀 Release v2.0.0: Solana Mainnet + Cross-Chain Integration

- Deploy 11 automated agents (8 Solana + 3 EVM)
- Zero-cost deployment via relayers
- 44 allowlisted addresses
- Cross-chain bridge integration
- GitHub Actions workflows
- Complete documentation

Networks: Solana Mainnet-Beta + SKALE Mainnet
Cost: \$0.00
Status: LIVE"

git push origin main

echo ""
echo "======================================================================"
echo "✅ ANNOUNCEMENT COMPLETE"
echo ""
echo "📍 Release: https://github.com/github/github-mcp-server/releases/tag/v2.0.0"
echo "💬 Discussion: Check GitHub Discussions"
echo "📝 Changelog: CHANGELOG_V2.0.0.md"
echo ""
echo "🎉 Solana Mainnet deployment announced successfully!"
