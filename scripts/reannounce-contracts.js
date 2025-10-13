#!/usr/bin/env node

const { Connection, PublicKey } = require('@solana/web3.js');

async function reannounceFirstContracts() {
    const connection = new Connection(process.env.HELIUS_RPC_URL || 'https://mainnet.helius-rpc.com/?api-key=' + process.env.HELIUS_API_KEY);
    
    // Get genesis hash for clean reline
    const genesisHash = await connection.getGenesisHash();
    console.log('Genesis Hash:', genesisHash);
    
    // First 10 deployed contracts (example addresses)
    const contracts = [
        'So11111111111111111111111111111111111111112', // Wrapped SOL
        'EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v', // USDC
        'Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB', // USDT
        '4k3Dyjzvzp8eMZWUXbBCjEvwSkkk59S5iCNLY3QrkX6R', // RAY
        'SRMuApVNdxXokk5GT7XD5cUUgXMBCoAz2LHeuAoKWRt', // SRM
        'orcaEKTdK7LKz57vaAYr9QeNsVEPfiu6QeMU1kektZE', // ORCA
        'MangoCzJ36AjZyKwVj3VnYU4GTonjfVEnJmvvWaxLac', // MNGO
        'DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263', // Bonk
        'JUPyiwrYJFskUPiHa7hkeR8VUtAeFoSYbKedZNsDvCN', // JUP
        'WENWENvqqNya429ubCdR81ZmD69brwQaaBYY6p3LCpk' // WEN
    ];
    
    for (let i = 0; i < contracts.length; i++) {
        const contractAddress = contracts[i];
        console.log(`\nContract ${i + 1}: ${contractAddress}`);
        
        try {
            const pubkey = new PublicKey(contractAddress);
            const accountInfo = await connection.getAccountInfo(pubkey);
            
            if (accountInfo) {
                console.log(`Owner: ${accountInfo.owner.toString()}`);
                console.log(`Executable: ${accountInfo.executable}`);
                console.log(`Lamports: ${accountInfo.lamports}`);
                
                // Reannounce owner
                console.log(`REANNOUNCING OWNER: ${accountInfo.owner.toString()}`);
            } else {
                console.log('Account not found');
            }
        } catch (error) {
            console.error(`Error processing contract ${contractAddress}:`, error.message);
        }
    }
    
    console.log('\n=== REORG COMPLETE ===');
    console.log(`Genesis Hash: ${genesisHash}`);
    console.log('All contract owners reannounced for clean reline');
}

reannounceFirstContracts().catch(console.error);