# Solana CLI Installation

## Issue
Network connection to release.solana.com failed in current environment.

## Alternative Installation Methods

### Method 1: Direct Download
```bash
# Download specific version
wget https://github.com/solana-labs/solana/releases/download/v1.18.0/solana-release-x86_64-unknown-linux-gnu.tar.bz2

# Extract
tar jxf solana-release-x86_64-unknown-linux-gnu.tar.bz2

# Add to PATH
export PATH=$PWD/solana-release/bin:$PATH

# Verify
solana --version
```

### Method 2: Using Package Manager (if available)
```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y solana

# Or via snap
sudo snap install solana --classic
```

### Method 3: Build from Source
```bash
git clone https://github.com/solana-labs/solana.git
cd solana
./scripts/cargo-install-all.sh .
export PATH=$PWD/bin:$PATH
```

### Method 4: Use in Different Environment
Run these commands on a machine with proper internet access:
```bash
sh -c "$(curl -sSfL https://release.solana.com/stable/install)"
export PATH="$HOME/.local/share/solana/install/active_release/bin:$PATH"
```

## After Installation

### Configure Solana
```bash
# Set network
solana config set --url https://api.mainnet-beta.solana.com

# Set keypair
solana config set --keypair ~/.config/solana/id.json

# Check balance
solana balance
```

### Run Upgrade Commands
```bash
# Gene Mint
solana program set-upgrade-authority GENEtH5amGSi8kHAtQoezp1XEXwZJ8vcuePYnXdKrMYz 4gLAGDEHs6sJ6AMmLdAwCUx9NPmPLxoMCZ3yiKyAyQ1m

# Standard Program
solana program set-upgrade-authority DjVE6JNiYqPL2QXyCUUh8rNjHrbz9hXHNYt99MQ59qw1 4gLAGDEHs6sJ6AMmLdAwCUx9NPmPLxoMCZ3yiKyAyQ1m

# DAO Master Controller
solana program set-upgrade-authority CvQZZ23qYDWF2RUpxYJ8y9K4skmuvYEEjH7fK58jtipQ 4gLAGDEHs6sJ6AMmLdAwCUx9NPmPLxoMCZ3yiKyAyQ1m
```

## Current Status
❌ Installation failed due to network connectivity
✓ Commands prepared and ready to execute
✓ Scripts created at `scripts/upgrade-program-authority.sh`

## Next Steps
1. Install Solana CLI using one of the alternative methods above
2. Configure with your keypair
3. Run `./scripts/upgrade-program-authority.sh`