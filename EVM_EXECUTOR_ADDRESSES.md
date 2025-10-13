# EVM Program Executor Addresses

## 🔑 Primary EVM Deployer Address

### Solidity Contract Deployer
**Address:** `0xE38FB59ba3AEAbE2AD0f6FB7Fb84453F6d145D23`
- **Type:** Fixed Deployer Address (Hardcoded in DMT.sol)
- **Role:** Contract Owner & Initial Mint Recipient
- **Contract:** FutureSkaleTokenWithTraders
- **File:** `CryptonoutController/DMT.sol`

---

## 🌐 EVM Network Contracts

### SKALE Network
**Network:** honorable-steel-rasalhague
**RPC:** `https://mainnet.skalenodes.com/v1/honorable-steel-rasalhague`

#### Deployed Contracts
- **OPT Token:** `0xc6D31F2F6CcBcd101604a92C6c08e0aee2937B3a`
- **Fee Token:** `0xD2Aaa00700000000000000000000000000000000`
- **Relayer:** `https://relayer.skale.network`

---

## 📋 Contract Features (DMT.sol)

### FutureSkaleTokenWithTraders
- **Type:** ERC20 Token with Trading Capabilities
- **Owner:** `0xE38FB59ba3AEAbE2AD0f6FB7Fb84453F6d145D23`
- **Features:**
  - Mintable with cap
  - Finalizable minting
  - 3 TraderGene slots (expendable on-chain actors)
  - Allowance-based execution
  - Owner can reclaim unused allowances

### TraderGene Capabilities
Each of the 3 TraderGene slots can:
- Execute transfers up to their allowance
- Be activated/deactivated by owner
- Have allowances reclaimed by owner
- Execute trades via `traderExecuteTrade(address to, uint256 amount)`

---

## 🔧 EVM Backfill Deployment

### Supported Networks
The deployment script supports multiple EVM networks:
- Ethereum Mainnet
- Polygon
- BSC (Binance Smart Chain)
- Arbitrum
- Optimism
- Avalanche
- SKALE

### Deployment Configuration
- **Moralis API:** Configured for wallet queries
- **Vercel:** Automated deployment enabled
- **Allowlist:** Contract interactions tracked

---

## 🔗 Cross-Chain Integration

### Solana ↔ EVM Bridge
- **Solana Programs:** Connected to EVM contracts
- **Fee Token (Solana):** `EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v` (USDC)
- **Octane Endpoint:** `https://octane-devnet.breakroom.show`

---

## 📊 Deployment Scripts

### Available Scripts
1. **deploy-evm-backfill.js** - Deploy to multiple EVM networks
2. **deploy-relayer.js** - Deploy relayer infrastructure
3. **cross-chain-bridge.js** - Bridge between Solana and EVM

### Usage
```bash
# Deploy to EVM networks
node scripts/deploy-evm-backfill.js

# Deploy relayer
node scripts/deploy-relayer.js

# Cross-chain bridge
node scripts/cross-chain-bridge.js
```

---

## ⚠️ Security Notes

1. **Fixed Deployer Address** - Hardcoded in contract, cannot be changed
2. **Owner Controls** - Only deployer can:
   - Mint tokens (before finalization)
   - Set/revoke TraderGenes
   - Reclaim allowances
   - Finalize minting
3. **TraderGene Execution** - Limited to allowance amounts
4. **Finalization** - Once finalized, minting is permanently disabled

---

## 🔍 Verification Links

### SKALE Explorer
- **OPT Token:** https://honorable-steel-rasalhague.explorer.mainnet.skalenodes.com/address/0xc6D31F2F6CcBcd101604a92C6c08e0aee2937B3a
- **Deployer:** https://honorable-steel-rasalhague.explorer.mainnet.skalenodes.com/address/0xE38FB59ba3AEAbE2AD0f6FB7Fb84453F6d145D23

### Etherscan (if deployed on Ethereum)
- **Deployer:** https://etherscan.io/address/0xE38FB59ba3AEAbE2AD0f6FB7Fb84453F6d145D23

---

## 📝 Contract ABI Functions

### Owner Functions
- `mint(address to, uint256 amount)`
- `finalizeMinting()`
- `finalizeAndRenounce()`
- `setTrader(uint8 index, address traderAddr, uint256 allowance)`
- `revokeTrader(uint8 index)`
- `reclaimUnusedAllowance(uint8 index, address to)`
- `fundContractFromOwner(uint256 amount)`

### TraderGene Functions
- `traderExecuteTrade(address to, uint256 amount)`

### View Functions
- `capInWhole()` - Returns cap in whole tokens
- `traderAllowanceInWhole(uint8 index)` - Returns trader allowance
- `getTrader(uint8 index)` - Returns trader info

---

## 🎯 Summary

**Primary EVM Executor:** `0xE38FB59ba3AEAbE2AD0f6FB7Fb84453F6d145D23`
- Deploys and owns FutureSkaleTokenWithTraders contract
- Controls 3 TraderGene slots for automated trading
- Integrated with SKALE network for gas-free transactions
- Cross-chain bridge to Solana via Octane
