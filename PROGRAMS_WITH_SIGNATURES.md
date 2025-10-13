# Programs with Valid Signatures & Priority Fees

## ✅ Summary
- **Total Programs Checked:** 11
- **Found:** 10
- **Not Found:** 1
- **Rate Limited:** Public RPC (429 errors)

---

## 🎯 Programs with Valid Signatures

### 1. GENEtH5amGSi8kHAtQoezp1XEXwZJ8vcuePYnXdKrMYz
- **Balance:** 6.452433 SOL
- **Owner:** TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA
- **Executable:** No
- **Signatures:** 5 recent
- **Recent Transactions:**
  - `5B15yUZk5cWDjwUx...` - Fee: 0.000020 SOL | Priority: 118,155 CU
  - `4ojFBvNr5mKxP7qP...` - Fee: 0.000005 SOL | Priority: 208,075 CU

### 2. DjVE6JNiYqPL2QXyCUUh8rNjHrbz9hXHNYt99MQ59qw1
- **Balance:** 2.161992 SOL
- **Owner:** BPFLoader2111111111111111111111111111111111
- **Executable:** Yes ✅
- **Signatures:** 5 recent
- **Type:** Deployed Program

### 3. CvQZZ23qYDWF2RUpxYJ8y9K4skmuvYEEjH7fK58jtipQ
- **Balance:** 0.332269 SOL
- **Owner:** System Program (11111111111111111111111111111111)
- **Executable:** No
- **Signatures:** 5 recent
- **Type:** Regular Account

### 4. JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4 (Jupiter)
- **Balance:** 2.729681 SOL
- **Owner:** BPFLoaderUpgradeab1e11111111111111111111111
- **Executable:** Yes ✅
- **Signatures:** 5 recent
- **Type:** Upgradeable Program

### 5. 4Ec7ZxZS6Sbdg5UGSLHbAnM7GQHp2eFd4KYWRexAipQT
- **Balance:** 20.131083 SOL 💰
- **Owner:** BPFLoaderUpgradeab1e11111111111111111111111
- **Executable:** No
- **Signatures:** 5 recent
- **Type:** Program Data Account

### 6. SMPLecH534NA9acpos4G6x7uf3LWbCAwZQE9e8ZekMu (Squads)
- **Balance:** 0.004443 SOL
- **Owner:** BPFLoaderUpgradeab1e11111111111111111111111
- **Executable:** Yes ✅
- **Signatures:** 5 recent
- **Recent Transactions:**
  - `2xBW4fvQTxPyqjxt...` - Fee: 0.000029 SOL | Priority: 39,738 CU

### 7. 7ZyDFzet6sKgZLN4D89JLfo7chu2n7nYdkFt5RCFk8Sf (Multisig)
- **Balance:** 0.005081 SOL
- **Owner:** SMPLecH534NA9acpos4G6x7uf3LWbCAwZQE9e8ZekMu
- **Executable:** No
- **Signatures:** 5 recent
- **Recent Transactions:**
  - `2U1a9LXX1bqzCYMk...` - Fee: 0.000069 SOL | Priority: 118,937 CU

### 8. TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA (SPL Token)
- **Balance:** 5.299606 SOL
- **Owner:** BPFLoader2111111111111111111111111111111111
- **Executable:** Yes ✅
- **Signatures:** 5 recent
- **Recent Transactions:**
  - `3AGxi9nxwituFHfr...` - Fee: 0.000006 SOL | Priority: 91,603 CU (ERROR)
  - `5ohuTMuimtVsfHsV...` - Fee: 0.000005 SOL | Priority: 81,749 CU

### 9. So11111111111111111111111111111111111111112 (Wrapped SOL)
- **Balance:** 1,171.596895 SOL 💰💰💰
- **Owner:** TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA
- **Executable:** No
- **Signatures:** 5 recent
- **Type:** Native SOL Mint

### 10. 11111111111111111111111111111111 (System Program)
- **Balance:** 0.000000 SOL
- **Owner:** NativeLoader1111111111111111111111111111111
- **Executable:** Yes ✅
- **Signatures:** 5 recent
- **Type:** Native System Program

---

## ❌ Not Found

### GLzZk1sczzW6fM4uPFeQCtTZQaf8H5VaBt99tUMbJAAW
- **Status:** Account does not exist on mainnet
- **Note:** This is the "New Master Controller" address from the multisig docs

---

## 📊 Priority Fee Analysis

### Average Priority Fees (Compute Units)
- **Highest:** 208,075 CU (GENEtH5amGSi8kHAtQoezp1XEXwZJ8vcuePYnXdKrMYz)
- **Lowest:** 39,738 CU (SMPLecH534NA9acpos4G6x7uf3LWbCAwZQE9e8ZekMu)
- **Average:** ~109,842 CU

### Transaction Fees
- **Range:** 0.000005 - 0.000069 SOL
- **Average:** ~0.000026 SOL

---

## 🔑 Key Findings

1. **10/11 programs exist and have valid signatures**
2. **5 executable programs found** (actual deployed programs)
3. **Wrapped SOL has highest balance:** 1,171.6 SOL
4. **Program Data account has 20.13 SOL** (likely for upgrades)
5. **Public RPC hit rate limits** - recommend using Helius API key
6. **New Master Controller address doesn't exist yet** - needs to be created

---

## 🔗 Verification Links

All addresses can be verified on:
- **Solscan:** https://solscan.io/account/[ADDRESS]
- **Solana Explorer:** https://explorer.solana.com/address/[ADDRESS]

---

## ⚠️ Notes

- Public RPC returned many 429 errors (rate limiting)
- Use `HELIUS_API_KEY` environment variable for better results
- Some transaction details couldn't be fetched due to rate limits
- All native Solana programs verified successfully
