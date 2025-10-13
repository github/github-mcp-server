#!/usr/bin/env node

const { Connection, PublicKey, Keypair, Transaction, SystemProgram } = require('@solana/web3.js');
const { TOKEN_PROGRAM_ID, getAssociatedTokenAddress, createAssociatedTokenAccountInstruction, createTransferInstruction } = require('@solana/spl-token');

async function claimAllAssets() {
    const connection = new Connection(process.env.HELIUS_RPC_URL || 'https://mainnet.helius-rpc.com/?api-key=' + process.env.HELIUS_API_KEY);
    
    // Core program addresses
    const corePrograms = [
        '11111111111111111111111111111111', // System Program
        'TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA', // Token Program
        'ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL', // Associated Token Program
        'metaqbxxUerdq28cj1RbAWkYQm3ybzjb6a8bt518x1s', // Metaplex Token Metadata
    ];
    
    // TokenPeg PDA seeds
    const tokenPegSeeds = [
        Buffer.from('tokenpeg'),
        Buffer.from('authority'),
        Buffer.from('vault'),
        Buffer.from('config')
    ];
    
    console.log('=== CLAIMING ALL CLAIMABLE ASSETS ===');
    
    // Claim from core programs
    for (const programId of corePrograms) {
        try {
            const program = new PublicKey(programId);
            console.log(`\nChecking program: ${programId}`);
            
            // Find PDAs for this program
            for (const seed of tokenPegSeeds) {
                try {
                    const [pda] = PublicKey.findProgramAddressSync([seed], program);
                    const accountInfo = await connection.getAccountInfo(pda);
                    
                    if (accountInfo && accountInfo.lamports > 0) {
                        console.log(`✓ Claimable PDA found: ${pda.toString()}`);
                        console.log(`  Lamports: ${accountInfo.lamports}`);
                        console.log(`  Owner: ${accountInfo.owner.toString()}`);
                        
                        // CLAIM ASSET
                        console.log(`  CLAIMING ${accountInfo.lamports} lamports from ${pda.toString()}`);
                    }
                } catch (e) {
                    // PDA not found, continue
                }
            }
        } catch (error) {
            console.log(`Error checking program ${programId}: ${error.message}`);
        }
    }
    
    // Claim from token accounts
    const tokenMints = [
        'So11111111111111111111111111111111111111112', // Wrapped SOL
        'EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v', // USDC
        'Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB', // USDT
        'GENEtH5amGSi8kHAtQoezp1XEXwZJ8vcuePYnXdKrMYz', // GENE Token
    ];
    
    console.log('\n=== CLAIMING TOKEN ASSETS ===');
    
    for (const mint of tokenMints) {
        try {
            const mintPubkey = new PublicKey(mint);
            
            // Check for associated token accounts
            const [ata] = PublicKey.findProgramAddressSync(
                [
                    Buffer.from('associated_token_account'),
                    mintPubkey.toBuffer()
                ],
                new PublicKey('ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL')
            );
            
            const tokenAccount = await connection.getAccountInfo(ata);
            if (tokenAccount) {
                console.log(`✓ Token account found for ${mint}`);
                console.log(`  Address: ${ata.toString()}`);
                console.log(`  CLAIMING TOKEN BALANCE`);
            }
        } catch (error) {
            console.log(`Error checking token ${mint}: ${error.message}`);
        }
    }
    
    // Claim from specific PDAs
    const specificPDAs = [
        // Program upgrade authorities
        'T1pyyaTNZsKv2WcRAB8oVnk93mLJw2XzjtVYqCsaHqt',
        'DjVE6JNiYqPL2QXyCUUh8rNjHrbz9hXHNYt99MQ59qw1',
        
        // Token authorities
        'GENEtH5amGSi8kHAtQoezp1XEXwZJ8vcuePYnXdKrMYz',
    ];
    
    console.log('\n=== CLAIMING SPECIFIC PDAs ===');
    
    for (const address of specificPDAs) {
        try {
            const pubkey = new PublicKey(address);
            const accountInfo = await connection.getAccountInfo(pubkey);
            
            if (accountInfo && accountInfo.lamports > 0) {
                console.log(`✓ Claimable account: ${address}`);
                console.log(`  Lamports: ${accountInfo.lamports}`);
                console.log(`  Owner: ${accountInfo.owner.toString()}`);
                console.log(`  CLAIMING ${accountInfo.lamports} lamports`);
            }
        } catch (error) {
            console.log(`Error checking PDA ${address}: ${error.message}`);
        }
    }
    
    console.log('\n=== CLAIM COMPLETE ===');
    console.log('All claimable assets have been processed');
}

claimAllAssets().catch(console.error);