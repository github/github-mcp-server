#!/usr/bin/env node
const fs = require('fs');
const path = require('path');

const SENSITIVE_PATTERNS = [
  /[0-9a-fA-F]{64}/g, // Private keys
  /sk_[a-zA-Z0-9]{32,}/g, // Secret keys
  /api[_-]?key["\s:=]+[a-zA-Z0-9]{20,}/gi,
  /moralis[_-]?api[_-]?key/gi,
  /helius[_-]?api[_-]?key/gi,
  /quicknode/gi
];

function scanDirectory(dir, results = []) {
  const files = fs.readdirSync(dir);
  
  for (const file of files) {
    const filePath = path.join(dir, file);
    const stat = fs.statSync(filePath);
    
    if (stat.isDirectory() && !file.startsWith('.') && file !== 'node_modules') {
      scanDirectory(filePath, results);
    } else if (stat.isFile() && (file.endsWith('.js') || file.endsWith('.json') || file.endsWith('.md'))) {
      const content = fs.readFileSync(filePath, 'utf8');
      
      for (const pattern of SENSITIVE_PATTERNS) {
        const matches = content.match(pattern);
        if (matches) {
          results.push({ file: filePath, matches: matches.length, pattern: pattern.toString() });
        }
      }
    }
  }
  
  return results;
}

console.log('🔒 Security Scan - Checking for exposed secrets...\n');
const results = scanDirectory('/workspaces/github-mcp-server');

if (results.length > 0) {
  console.log('⚠️  Potential secrets found:');
  results.forEach(r => console.log(`   ${r.file}: ${r.matches} matches`));
  console.log('\n🔐 Review these files and move secrets to .env');
} else {
  console.log('✅ No exposed secrets detected');
}
