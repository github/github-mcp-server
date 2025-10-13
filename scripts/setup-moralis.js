#!/usr/bin/env node

require('dotenv').config({ path: '.env.moralis' });
const https = require('https');

const MORALIS_API_KEY = process.env.MORALIS_API_KEY || 'c4d1d108f46144f1955612d3ac03dcd5';
const MORALIS_NODE_URL = process.env.MORALIS_NODE_URL || 'https://site2.moralis-nodes.com/eth/c4d1d108f46144f1955612d3ac03dcd5';

console.log('=== MORALIS API SETUP ===');
console.log('');
console.log('API Key:', MORALIS_API_KEY);
console.log('Node URL:', MORALIS_NODE_URL);
console.log('');

// Test connection
const url = new URL(MORALIS_NODE_URL);

const options = {
  hostname: url.hostname,
  path: url.pathname,
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
  }
};

const data = JSON.stringify({
  jsonrpc: '2.0',
  method: 'eth_blockNumber',
  params: [],
  id: 1
});

console.log('Testing connection...');

const req = https.request(options, (res) => {
  let body = '';
  
  res.on('data', (chunk) => {
    body += chunk;
  });
  
  res.on('end', () => {
    console.log('');
    console.log('Response:', body);
    console.log('');
    console.log('✓ Moralis API connected successfully');
  });
});

req.on('error', (error) => {
  console.error('');
  console.error('✗ Connection failed:', error.message);
});

req.write(data);
req.end();

module.exports = {
  MORALIS_API_KEY,
  MORALIS_NODE_URL
};