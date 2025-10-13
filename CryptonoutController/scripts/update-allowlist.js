#!/usr/bin/env node

const fs = require('fs');
const path = require('path');

const ALLOWLIST_FILE = process.env.ALLOWLIST_FILE || 'VERCEL_DEPLOYMENT_ALLOWLIST.json';
const NEW_ADDRESSES = process.env.NEW_ADDRESSES || process.argv[2] || '';

function updateAllowlist() {
  console.log('🔐 ALLOWLIST UPDATE UTILITY');
  console.log('=' .repeat(60));

  if (!NEW_ADDRESSES) {
    console.log('❌ No addresses provided');
    console.log('Usage: node update-allowlist.js "addr1,addr2,addr3"');
    process.exit(1);
  }

  const addresses = NEW_ADDRESSES.split(',').map(a => a.trim()).filter(Boolean);
  console.log(`📝 Adding ${addresses.length} addresses`);

  if (!fs.existsSync(ALLOWLIST_FILE)) {
    console.log('❌ Allowlist file not found:', ALLOWLIST_FILE);
    process.exit(1);
  }

  const allowlist = JSON.parse(fs.readFileSync(ALLOWLIST_FILE, 'utf8'));
  const currentList = allowlist.allowlist || [];
  const currentCount = currentList.length;

  addresses.forEach(addr => {
    if (!currentList.includes(addr)) {
      currentList.push(addr);
      console.log(`  ✅ Added: ${addr}`);
    } else {
      console.log(`  ⏭️  Skipped (exists): ${addr}`);
    }
  });

  allowlist.allowlist = currentList;
  
  fs.writeFileSync(ALLOWLIST_FILE, JSON.stringify(allowlist, null, 2));
  
  console.log('=' .repeat(60));
  console.log(`✅ Allowlist updated: ${currentCount} → ${currentList.length}`);
  console.log(`📁 File: ${ALLOWLIST_FILE}`);
}

updateAllowlist();
