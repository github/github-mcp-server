#!/bin/bash

echo "🚀 PUSH TO CRYPTONOUTCONTROLLER"
echo "======================================================================"

TARGET_REPO="/workspaces/github-mcp-server/CryptonoutController"

if [ ! -d "$TARGET_REPO" ]; then
  echo "❌ CryptonoutController directory not found"
  exit 1
fi

cd "$TARGET_REPO"

echo ""
echo "📁 Creating directory structure..."
mkdir -p .github/workflows
mkdir -p scripts
mkdir -p Deployer-Gene/scripts

echo ""
echo "📋 Copying GitHub Actions workflows..."
cp ../.github/workflows/bot-funding-deployment.yml .github/workflows/
cp ../.github/workflows/cross-chain-deploy.yml .github/workflows/

echo ""
echo "📜 Copying scripts..."
cp ../scripts/cross-chain-bridge.js scripts/
cp ../scripts/deploy-evm-backfill.js scripts/
cp ../scripts/announce-mainnet.sh scripts/
cp ../Deployer-Gene/scripts/mint-bot.js Deployer-Gene/scripts/

echo ""
echo "📚 Copying documentation..."
cp ../CHANGELOG_V2.0.0.md .
cp ../SOLANA_MAINNET_ANNOUNCEMENT.md .
cp ../CROSS_CHAIN_INTEGRATION.md .
cp ../BOT_DEPLOYMENT_GUIDE.md .
cp ../INTEGRATION_COMPLETE.md .
cp ../VERCEL_DEPLOYMENT_ALLOWLIST.json .
cp ../.env.moralis .

echo ""
echo "✅ Files copied successfully"
echo ""
echo "📊 Summary:"
ls -lh .github/workflows/*.yml
ls -lh scripts/*.js scripts/*.sh
ls -lh *.md

echo ""
echo "======================================================================"
echo "🎯 Ready to commit and push"
echo ""
echo "Run these commands:"
echo ""
echo "  cd $TARGET_REPO"
echo "  git add ."
echo "  git commit -m '🚀 Add cross-chain deployment automation'"
echo "  git push origin main"
echo ""
echo "======================================================================"
