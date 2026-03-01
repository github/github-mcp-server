#!/usr/bin/env node

/**
 * Scans the repository for contract addresses (Solana base58 and EVM 0x)
 * and reports their locations plus allowlist coverage.
 *
 * Usage:
 *   node scripts/scan-contracts.js [startDir]
 *
 * Outputs:
 *   - Prints a summary to stdout
 *   - Writes a JSON report to contract_scan_results.json
 */

const fs = require("fs");
const path = require("path");

const ROOT = path.join(__dirname, "..");
const START_DIR = path.resolve(process.argv[2] || ROOT);
const MAX_FILE_BYTES = 2 * 1024 * 1024; // skip files larger than 2 MB
const EXCLUDED_DIRS = new Set([
  ".git",
  "node_modules",
  ".next",
  "dist",
  "build",
  "tmp",
  "vendor",
  ".venv",
  "venv",
]);

const SOLANA_REGEX = /\b[1-9A-HJ-NP-Za-km-z]{32,44}\b/g;
const EVM_REGEX = /\b0x[a-fA-F0-9]{40}\b/g;

const allowlistSources = [
  path.join(ROOT, "VERCEL_DEPLOYMENT_ALLOWLIST.json"),
  path.join(ROOT, "COMPREHENSIVE_ALLOWLIST_UPDATE.json"),
];

function loadAllowlist() {
  const addresses = new Set();
  for (const source of allowlistSources) {
    if (!fs.existsSync(source)) continue;
    const data = JSON.parse(fs.readFileSync(source, "utf8"));
    if (Array.isArray(data.allowlist)) {
      data.allowlist.forEach((addr) => addresses.add(addr));
    }
    if (Array.isArray(data.master_allowlist)) {
      data.master_allowlist.forEach((addr) => addresses.add(addr));
    }
  }
  return addresses;
}

function walk(dir, visitor) {
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  for (const entry of entries) {
    if (entry.isDirectory()) {
      if (EXCLUDED_DIRS.has(entry.name)) continue;
      walk(path.join(dir, entry.name), visitor);
    } else if (entry.isFile()) {
      visitor(path.join(dir, entry.name));
    }
  }
}

function scanFile(filePath, allowlist, results) {
  try {
    const stat = fs.statSync(filePath);
    if (stat.size > MAX_FILE_BYTES) return;
    const content = fs.readFileSync(filePath, "utf8");
    const matches = [
      ...new Set([
        ...(content.match(SOLANA_REGEX) || []),
        ...(content.match(EVM_REGEX) || []),
      ]),
    ];
    if (!matches.length) return;

    const relativePath = path.relative(ROOT, filePath);
    for (const address of matches) {
      const type = address.startsWith("0x") ? "evm" : "solana";
      if (!results[address]) {
        results[address] = { type, files: new Set() };
      }
      results[address].files.add(relativePath);
      results[address].allowlisted = allowlist.has(address);
    }
  } catch {
    /* skip unreadable files */
  }
}

function main() {
  const allowlist = loadAllowlist();
  const results = {};

  walk(START_DIR, (file) => scanFile(file, allowlist, results));

  const normalized = Object.entries(results).map(([address, data]) => ({
    address,
    type: data.type,
    allowlisted: Boolean(data.allowlisted),
    files: Array.from(data.files).sort(),
  }));

  normalized.sort((a, b) => a.address.localeCompare(b.address));

  const summary = {
    scanned_from: START_DIR,
    total_addresses: normalized.length,
    allowlisted: normalized.filter((r) => r.allowlisted).length,
    not_allowlisted: normalized.filter((r) => !r.allowlisted).length,
    addresses: normalized,
  };

  const outputPath = path.join(ROOT, "contract_scan_results.json");
  fs.writeFileSync(outputPath, JSON.stringify(summary, null, 2));

  console.log("Contract scan complete.");
  console.log(`  Total addresses: ${summary.total_addresses}`);
  console.log(`  Allowlisted:     ${summary.allowlisted}`);
  console.log(`  Not allowlisted: ${summary.not_allowlisted}`);
  console.log(`  Report written to ${outputPath}`);
}

main();
