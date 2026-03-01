# Running Contract Upgrade Report

This report scans the repository for contract addresses and highlights the running contracts that should be kept allowlisted for upgrades to owned program contracts.

## Method
- `scripts/scan-contracts.js` walks the repo (excluding build/vendor caches) to find Solana base58 and EVM `0x` addresses, and records whether each is allowlisted.
- The scan output is stored in `contract_scan_results.json` with file-level occurrences and allowlist status.
- Allowlist sources: `VERCEL_DEPLOYMENT_ALLOWLIST.json` and `COMPREHENSIVE_ALLOWLIST_UPDATE.json`.

**Scan summary (current run):**
- Total addresses discovered: **98**
- Allowlisted: **44**
- Not allowlisted: **54**

## Upgrade-Critical Contracts (Allowlisted)
These addresses are the running contracts that must stay allowlisted for owned-program upgrades and operations:

### Solana Owned Programs
- Gene Mint: `GENEtH5amGSi8kHAtQoezp1XEXwZJ8vcuePYnXdKrMYz`
- Standard Program: `DjVE6JNiYqPL2QXyCUUh8rNjHrbz9hXHNYt99MQ59qw1`
- DAO Controller: `CvQZZ23qYDWF2RUpxYJ8y9K4skmuvYEEjH7fK58jtipQ`
- Primary Program: `jaJrDgf4U8DAZcUD3t5AwL7Cfe2QnkpXZXGegdUHc4ZE`

### Backfill / Ledger Anchors
- OMEGA Primary: `EoRJaGA4iVSQWDyv5Q3ThBXx1KGqYyos3gaXUFEiqUSN`
- OMEGA Alt: `2YTrK8f6NwwUg7Tu6sYcCmRKYWpU8yYRYHPz87LTdcgx`
- Earnings Vault: `F2EkpVd3pKLUi9u9BU794t3mWscJXzUAVw1WSjogTQuR`

### Core & DEX Programs
- Core: `TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA`, `TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb`, `ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL`, `metaqbxxUerdq28cj1RbAWkYQm3ybzjb6a8bt518x1s`
- DEX: `JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4`, `LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo`, `675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8`

### Token Mint
- USDC: `EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v`

### Bot & Treasury Surfaces
- Bot wallets: `HKBJoeUWH6pUQuLd9CZWrJBzGSE9roEW4bshnxd9AHsR`, `NqGHDaaLWmND7uShuaZkVbGNQFy6pS96qHyfR3pGR2d`, `DbhKvqweZECTyYQ7PRJoHmKt8f262fsBCGHxSaD5BPqA`, `7uSCVM1MJPKctrSRzuFN7qfVoJX78q6V5q5JuzRPaK41`, `3oFCkoneQShDsJMZYscXew4jGwgLjpxfykHuGo85QyLw`, `8duk9DzqBVXmqiyci9PpBsKuRCwg6ytzWywjQztM6VzS`, `96891wG6iLVEDibwjYv8xWFGFiEezFQkvdyTrM69ou24`, `2A8qGB3iZ21NxGjX4EjjWJKc9PFG1r7F4jkcR66dc4mb`
- Bot contracts: `EAy5Nfn6fhs4ixC4sMcKQYQaoedLokpWqbfDtWURCnk6`, `HUwjG8LFabw28vJsQNoLXjxuzgdLhjGQw1DHZggzt76`, `FZxmYkA6axyK3Njh3YNWXtybw9GgniVrXowS1pAAyrD1`, `5ynYfAM7KZZXwT4dd2cZQnYhFNy1LUysE8m7Lxzjzh2p`, `DHBDPUkLLYCRAiyrgFBgvWfevquFkLR1TjGXKD4M4JPD`
- Treasury & control: `zhBqbd9tSQFPevg4188JxcgpccCj3t1Jxb29zsBc2R4`, `FsQPFuje4WMdvbyoVef6MRMuzNZt9E8HM9YBN8T3Zbdq`, `5kDqr3kwfeLhz5rS9cb14Tj2ZZPSq7LddVsxYDV8DnUm`, `4gLAGDEHs6sJ6AMmLdAwCUx9NPmPLxoMCZ3yiKyAyQ1m`, `4eJZVbbsiLAG6EkWvgEYEWKEpdhJPFBYMeJ6DBX98w6a`, `EdFC98d1BBhJkeh7KDq26TwEGLeznhoyYsY6Y8LFY4y6`, `8cRrU1NzNpjL3k2BwjW3VixAcX6VFc29KHr4KZg8cs2Y`
- DAO signers: `mQBipzeneXqnAkWNL8raGvrj2c8dJv87LXs2Hn7BeXk`, `J1toHzrhyxaoFTUoxrceFMSqd1vTdZ1Wat3xQVa8E5Jt`

### EVM & Cross-Chain Contracts
- Primary multi-chain wallet: `0x742d35Cc6634C0532925a3b8D4C9db96C4b4d8b6`
- Stablecoin/interaction contracts: `0xdAC17F958D2ee523a2206206994597C13D831ec7`, `0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174`, `0x55d398326f99059fF775485246999027B3197955`, `0xA0b86a33E6441e6e80D0c4C34F0b1e4E6a7c4b8d`
- SKALE: OPT token `0xc6D31F2F6CcBcd101604a92C6c08e0aee2937B3a`, Deployer `0xE38FB59ba3AEAbE2AD0f6FB7Fb84453F6d145D23`

## Allowlist Alignment
- The master allowlist now mirrors the Vercel deployment allowlist, adding the three new bot wallets and SKALE deployer so all upgrade-critical contracts remain enabled.
- For addresses not yet allowlisted (54 discovered in the current scan), see `contract_scan_results.json` for file-level context to decide whether they require onboarding.
