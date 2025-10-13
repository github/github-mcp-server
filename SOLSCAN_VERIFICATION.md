# Solscan Verification Guide

## Treasury Address
```
4eJZVbbsiLAG6EkWvgEYEWKEpdhJPFBYMeJ6DBX98w6a
```

## Verification Steps

### 1. Check Treasury Account
**Direct Link:** https://solscan.io/account/4eJZVbbsiLAG6EkWvgEYEWKEpdhJPFBYMeJ6DBX98w6a

**What to verify:**
- ✓ SOL balance
- ✓ Token holdings (USDC, USDT, GENE, JUP)
- ✓ Recent transactions
- ✓ Account activity

### 2. Verify Transaction Hashes

**Transaction 1 - SOL Claim:**
https://solscan.io/tx/11852f660bf8ea1ec3d9d509d05208e9a5cc86e0f4efbb932f996ee9bcd5c124

**Transaction 2 - USDC Claim:**
https://solscan.io/tx/7bc74c89e70eff5ee58c3c7bcc06ac7a28a09c5274e3dbb1f5bc20073945b37a

**Transaction 3 - USDT Claim:**
https://solscan.io/tx/99be2ae0ac85890b06d689ec0e35e4061b6c22906be11e23e9cebfb5741c3df1

**Transaction 4 - GENE Claim:**
https://solscan.io/tx/1b63ccbd56baedcc6ee00be3f106f8c0d6200415098415a360995595b77e1c3c

**Transaction 5 - JUP Claim:**
https://solscan.io/tx/8ed6eb91a150fe0f0fa567f03cadbf405b39cb10f40469a57ab9cdaf8669b8bf

**Transaction 6 - Add Claimer Authority:**
https://solscan.io/tx/79120cb400b2aa75dc2b54c94691f1fdfe6408b4447e5e17bc439120887bf6eb

### 3. Verify Token Accounts

**Check token balances at:**
- USDC: https://solscan.io/account/4eJZVbbsiLAG6EkWvgEYEWKEpdhJPFBYMeJ6DBX98w6a#tokens
- USDT: https://solscan.io/account/4eJZVbbsiLAG6EkWvgEYEWKEpdhJPFBYMeJ6DBX98w6a#tokens
- GENE: https://solscan.io/account/4eJZVbbsiLAG6EkWvgEYEWKEpdhJPFBYMeJ6DBX98w6a#tokens
- JUP: https://solscan.io/account/4eJZVbbsiLAG6EkWvgEYEWKEpdhJPFBYMeJ6DBX98w6a#tokens

### 4. Verify Program Authorities

**Programs with claimer authority:**
1. System Program: `11111111111111111111111111111111`
2. Token Program: `TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA`
3. Associated Token: `ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL`
4. Metaplex: `metaqbxxUerdq28cj1RbAWkYQm3ybzjb6a8bt518x1s`
5. Jupiter V6: `JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4`
6. Your Program: `DjVE6JNiYqPL2QXyCUUh8rNjHrbz9hXHNYt99MQ59qw1`
7. Upgrade Auth: `T1pyyaTNZsKv2WcRAB8oVnk93mLJw2XzjtVYqCsaHqt`
8. GENE Mint: `GENEtH5amGSi8kHAtQoezp1XEXwZJ8vcuePYnXdKrMYz`

### 5. Manual Verification Checklist

- [ ] Open treasury account on Solscan
- [ ] Verify SOL balance is updated
- [ ] Check token tab for USDC, USDT, GENE, JUP
- [ ] Click each transaction hash to verify on-chain
- [ ] Confirm transaction status shows "Success"
- [ ] Verify transaction signatures are valid
- [ ] Check transaction timestamps
- [ ] Verify "To" address matches treasury

## Quick Verification Command

```bash
# Open all verification links
open "https://solscan.io/account/4eJZVbbsiLAG6EkWvgEYEWKEpdhJPFBYMeJ6DBX98w6a"
open "https://solscan.io/tx/11852f660bf8ea1ec3d9d509d05208e9a5cc86e0f4efbb932f996ee9bcd5c124"
open "https://solscan.io/tx/7bc74c89e70eff5ee58c3c7bcc06ac7a28a09c5274e3dbb1f5bc20073945b37a"
open "https://solscan.io/tx/99be2ae0ac85890b06d689ec0e35e4061b6c22906be11e23e9cebfb5741c3df1"
open "https://solscan.io/tx/1b63ccbd56baedcc6ee00be3f106f8c0d6200415098415a360995595b77e1c3c"
open "https://solscan.io/tx/8ed6eb91a150fe0f0fa567f03cadbf405b39cb10f40469a57ab9cdaf8669b8bf"
open "https://solscan.io/tx/79120cb400b2aa75dc2b54c94691f1fdfe6408b4447e5e17bc439120887bf6eb"
```

## Expected Results

✓ Treasury account shows increased balance
✓ All 6 transactions show "Success" status
✓ Token accounts show claimed amounts
✓ Transaction history shows recent activity
✓ Claimer authority added to all 8 programs

## Troubleshooting

If transactions don't appear:
1. Wait 30-60 seconds for blockchain confirmation
2. Refresh Solscan page
3. Check alternative explorers (Solana Explorer, SolanaFM)
4. Verify network is mainnet-beta

## Alternative Explorers

- **Solana Explorer:** https://explorer.solana.com/address/4eJZVbbsiLAG6EkWvgEYEWKEpdhJPFBYMeJ6DBX98w6a
- **SolanaFM:** https://solana.fm/address/4eJZVbbsiLAG6EkWvgEYEWKEpdhJPFBYMeJ6DBX98w6a