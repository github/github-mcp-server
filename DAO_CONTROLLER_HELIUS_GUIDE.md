# DAO Controller + Helius Verification & Interaction Guide

This guide explains how to (1) verify all deployed programs and (2) interact with them through the DAO controller multisig using the Helius RPC. All secrets (API keys, keypairs) must be supplied locally via environment variables—no keys are stored in the repo.

## 1) Prerequisites
- Node.js 18+ and `curl`
- Helius RPC key exported as `HELIUS_API_KEY`
  ```bash
  export HELIUS_API_KEY="YOUR_HELIUS_KEY"
  HELIUS_RPC="https://mainnet.helius-rpc.com/?api-key=$HELIUS_API_KEY"
  ```
- DAO controller multisig signers (from `DAO_SIGNERS_REPORT.md`):
  - Controller: `CvQZZ23qYDWF2RUpxYJ8y9K4skmuvYEEjH7fK58jtipQ`
  - Signers: `mQBipzeneXqnAkWNL8raGvrj2c8dJv87LXs2Hn7BeXk`, `J1toHzrhyxaoFTUoxrceFMSqd1vTdZ1Wat3xQVa8E5Jt`
- Multisig account (from `scripts/verify-on-chain.js`): `7ZyDFzet6sKgZLN4D89JLfo7chu2n7nYdkFt5RCFk8Sf`

## 2) Program set to verify
- Owned programs: `GENEtH5amGSi8kHAtQoezp1XEXwZJ8vcuePYnXdKrMYz`, `DjVE6JNiYqPL2QXyCUUh8rNjHrbz9hXHNYt99MQ59qw1`, `CvQZZ23qYDWF2RUpxYJ8y9K4skmuvYEEjH7fK58jtipQ`, `jaJrDgf4U8DAZcUD3t5AwL7Cfe2QnkpXZXGegdUHc4ZE`
- Backfill anchors: `EoRJaGA4iVSQWDyv5Q3ThBXx1KGqYyos3gaXUFEiqUSN`, `2YTrK8f6NwwUg7Tu6sYcCmRKYWpU8yYRYHPz87LTdcgx`, `F2EkpVd3pKLUi9u9BU794t3mWscJXzUAVw1WSjogTQuR`
- Core/DEX helpers: `TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA`, `TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb`, `ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL`, `metaqbxxUerdq28cj1RbAWkYQm3ybzjb6a8bt518x1s`, `JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4`, `LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo`, `675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8`

## 3) Verify programs via Helius
1) **Account snapshot**
   ```bash
   curl -s "$HELIUS_RPC" \
     -H "Content-Type: application/json" \
     -d '{
       "jsonrpc":"2.0","id":"acct",
       "method":"getAccountInfo",
       "params":["CvQZZ23qYDWF2RUpxYJ8y9K4skmuvYEEjH7fK58jtipQ", {"encoding":"jsonParsed"}]
     }' | jq '.result.value'
   ```
   - Confirm `owner`, `lamports`, and `executable` for programs; for SPL helpers confirm data layouts.

2) **Recent activity with pagination**
   ```bash
   BEFORE_SIG="" # fill after first page if more history is needed
   curl -s "$HELIUS_RPC" \
     -H "Content-Type: application/json" \
     -d "{
       \"jsonrpc\":\"2.0\",\"id\":\"sigs\",
       \"method\":\"getSignaturesForAddress\",
       \"params\":[\"CvQZZ23qYDWF2RUpxYJ8y9K4skmuvYEEjH7fK58jtipQ\", {\"limit\":100,\"before\":\"$BEFORE_SIG\"}]
     }" | jq
   ```
   - Iterate `before` with the last signature to paginate.

3) **Transaction detail & authority checks**
   ```bash
   SIG="<signature-from-list>"
   curl -s "$HELIUS_RPC" \
     -H "Content-Type: application/json" \
     -d "{
       \"jsonrpc\":\"2.0\",\"id\":\"tx\",
       \"method\":\"getParsedTransaction\",
       \"params\":[\"$SIG\", {\"maxSupportedTransactionVersion\":0}]
     }" | jq '.result.transaction.message.accountKeys'
   ```
   - Confirm DAO controller or multisig accounts sign expected upgrades/interactions.

4) **Multisig state validation**
   ```bash
   curl -s "$HELIUS_RPC" \
     -H "Content-Type: application/json" \
     -d '{
       "jsonrpc":"2.0","id":"msig",
       "method":"getAccountInfo",
       "params":["7ZyDFzet6sKgZLN4D89JLfo7chu2n7nYdkFt5RCFk8Sf", {"encoding":"base64"}]
     }' | jq '.result.value'
   ```
   - Decode the returned data (client-side) to confirm threshold and member set match `DAO_SIGNERS_REPORT.md`.

## 4) Interact with programs via DAO controller multisig (Helius RPC)
**Goal:** construct a transaction, have signers approve it, then submit through Helius.

1) **Build the instruction locally (example using @solana/web3.js)**
   ```js
   // Pseudocode: replace PROGRAM_ID/IX_DATA/ACCOUNTS as needed
   const {Connection, PublicKey, TransactionInstruction, VersionedTransaction, TransactionMessage} = require('@solana/web3.js');
   const connection = new Connection(process.env.HELIUS_RPC, 'confirmed');

   const ix = new TransactionInstruction({
     programId: new PublicKey(process.env.TARGET_PROGRAM_ID),
     keys: [/* target accounts & signers (DAO controller as authority) */],
     data: Buffer.from(process.env.IX_DATA_HEX, 'hex'),
   });

   const recent = await connection.getLatestBlockhash();
   const messageV0 = new TransactionMessage({
     payerKey: new PublicKey(process.env.DAO_CONTROLLER),
     recentBlockhash: recent.blockhash,
     instructions: [ix],
   }).compileToV0Message();

   const tx = new VersionedTransaction(messageV0);
   const serialized = Buffer.from(tx.serialize({requireAllSignatures:false})).toString('base64');
   console.log(serialized);
   ```
   - `DAO_CONTROLLER` should be the multisig PDA/authority address, not an individual signer.

2) **Simulate before collecting signatures**
   ```bash
   BASE64_TX="<from step 1>"
   curl -s "$HELIUS_RPC" \
     -H "Content-Type: application/json" \
     -d "{
       \"jsonrpc\":\"2.0\",\"id\":\"sim\",
       \"method\":\"simulateTransaction\",
       \"params\":[\"$BASE64_TX\", {\"sigVerify\":false, \"commitment\":\"processed\"}]
     }" | jq '.result'
   ```

3) **Collect multisig approvals**
   - Route the base64 transaction through the multisig flow (e.g., Squads/Anchor-compatible interface). Each signer (`mQBipz...`, `J1toHz...`) adds their partial signature.
   - After threshold is met, export the fully-signed base64 transaction blob.

4) **Send via Helius**
   ```bash
   SIGNED_TX="<fully-signed-base64>"
   curl -s "$HELIUS_RPC" \
     -H "Content-Type: application/json" \
     -d "{
       \"jsonrpc\":\"2.0\",\"id\":\"send\",
       \"method\":\"sendTransaction\",
       \"params\":[\"$SIGNED_TX\", {\"skipPreflight\":false}]
     }" | jq
   ```
   - Record the returned signature and verify with `getParsedTransaction` (step 3) for final confirmation.

## 5) Tips for ongoing monitoring
- Run `scripts/scan-contracts.js` to refresh the address inventory and ensure new contracts are allowlisted.
- Track authority changes by diffing multisig state (step 3.4) before/after proposals.
- Keep the Helius pagination cursor (`before`) for each program to resume history checks without re-fetching recent slots.
