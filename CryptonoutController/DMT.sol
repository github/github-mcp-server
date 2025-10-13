// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/// @title Future SKALE Token (Mint-with-Cap, Finalize-able, Pre-set Deployer, + 3 TraderGenes)
/// @notice Owner is set to the fixed DEPLOYER_ADDRESS, initial mint goes there.
///         Adds 3 expendable on-chain TraderGene actors which can execute transfers up to an allowance.
contract FutureSkaleTokenWithTraders is ERC20, Ownable {
    uint256 public immutable cap;           // maximum total supply (in smallest units)
    bool public mintingFinalized;           // once true, no more minting possible

    // <--- SET DEPLOYER ADDRESS HERE --->
    address public constant DEPLOYER_ADDRESS = 0xE38FB59ba3AEAbE2AD0f6FB7Fb84453F6d145D23;
    // ------------------------------------

    // TraderGene struct
    struct TraderGene {
        address addr;        // address of the trader actor (EOA or contract)
        uint256 allowance;   // allowance in smallest units they may spend from contract
        bool active;         // active flag (expendable)
    }

    TraderGene[3] public traders; // fixed array of 3 TraderGenes

    // Events
    event Mint(address indexed to, uint256 amount);
    event MintingFinalized();
    event FinalizedAndRenounced(address indexed previousOwner);

    event TraderSet(uint8 indexed index, address indexed traderAddr, uint256 allowance);
    event TraderRevoked(uint8 indexed index, address indexed previousAddr);
    event TraderExecuted(uint8 indexed index, address indexed traderAddr, address indexed to, uint256 amount);
    event TraderReclaimed(uint8 indexed index, address indexed reclaimedTo, uint256 amountReclaimed);

    /// @param name_ token name
    /// @param symbol_ token symbol
    /// @param initialMint amount minted immediately to DEPLOYER_ADDRESS (in whole tokens; decimals applied)
    /// @param cap_ maximum supply (in whole tokens; decimals applied)
    constructor(
        string memory name_,
        string memory symbol_,
        uint256 initialMint,
        uint256 cap_
    ) ERC20(name_, symbol_) {
        require(cap_ > 0, "cap must be > 0");
        uint256 decimalsFactor = 10 ** decimals();
        cap = cap_ * decimalsFactor;
        uint256 initialAmount = initialMint * decimalsFactor;
        require(initialAmount <= cap, "initialMint > cap");

        // Mint initial allocation directly to the fixed deployer address
        _mint(DEPLOYER_ADDRESS, initialAmount);
        emit Mint(DEPLOYER_ADDRESS, initialAmount);

        // Transfer ownership to the fixed deployer address so that only it can mint / finalize
        _transferOwnership(DEPLOYER_ADDRESS);

        // initialize traders as inactive
        for (uint8 i = 0; i < 3; i++) {
            traders[i] = TraderGene({ addr: address(0), allowance: 0, active: false });
        }
    }

    /* ---------------- Minting / Finalize (owner) ---------------- */

    /// @notice Owner (DEPLOYER_ADDRESS) mints `amount` tokens to `to`. Only allowed before finalizeMinting().
    /// @param to recipient address
    /// @param amount amount in token smallest units (wei)
    function mint(address to, uint256 amount) external onlyOwner {
        require(!mintingFinalized, "Minting finalized");
        require(totalSupply() + amount <= cap, "Cap exceeded");
        _mint(to, amount);
        emit Mint(to, amount);
    }

    /// @notice Permanently disable minting. Owner remains (unless they renounce).
    function finalizeMinting() public onlyOwner {
        require(!mintingFinalized, "Already finalized");
        mintingFinalized = true;
        emit MintingFinalized();
    }

    /// @notice Finalizes minting and renounces ownership in one transaction => permanently ownerless & no minting.
    /// @dev Must be called by DEPLOYER_ADDRESS (owner).
    function finalizeAndRenounce() external onlyOwner {
        finalizeMinting();
        address prevOwner = owner();
        renounceOwnership(); // from Ownable
        emit FinalizedAndRenounced(prevOwner);
    }

    /// @notice Helper to return cap in human-readable whole tokens (not smallest units)
    function capInWhole() external view returns (uint256) {
        return cap / (10 ** decimals());
    }

    /* ---------------- TraderGene Management (owner) ---------------- */

    /// @notice Set or replace a TraderGene slot (index 0..2). Sets address, allowance (smallest units), and activates it.
    /// @param index 0..2
    /// @param traderAddr address of trader actor (EOA or contract)
    /// @param allowance amount in smallest units the trader may spend from contract
    function setTrader(uint8 index, address traderAddr, uint256 allowance) external onlyOwner {
        require(index < 3, "index out of range");
        require(traderAddr != address(0), "invalid trader address");

        traders[index].addr = traderAddr;
        traders[index].allowance = allowance;
        traders[index].active = true;

        emit TraderSet(index, traderAddr, allowance);
    }

    /// @notice Revoke (disable) a trader at index. Does not automatically move funds.
    /// @param index 0..2
    function revokeTrader(uint8 index) external onlyOwner {
        require(index < 3, "index out of range");
        address prev = traders[index].addr;
        traders[index].active = false;
        emit TraderRevoked(index, prev);
    }

    /// @notice Reclaim up to the trader's allowance (transfer tokens from contract to `to`) and zero the allowance.
    /// @dev Useful to recover funds if the trader is expendable or being retired.
    /// @param index 0..2
    /// @param to recipient to receive reclaimed funds (e.g., DEPLOYER_ADDRESS)
    function reclaimUnusedAllowance(uint8 index, address to) external onlyOwner {
        require(index < 3, "index out of range");
        require(to != address(0), "invalid recipient");
        uint256 remaining = traders[index].allowance;
        if (remaining == 0) {
            emit TraderReclaimed(index, to, 0);
            return;
        }

        // Ensure contract has balance to cover reclaim (if not, reclaim as much as possible)
        uint256 contractBal = balanceOf(address(this));
        uint256 reclaimAmount = remaining <= contractBal ? remaining : contractBal;

        if (reclaimAmount > 0) {
            // transfer from contract to recipient
            _transfer(address(this), to, reclaimAmount);
        }

        // zero the trader allowance (we treat remaining allowance as reclaimed)
        traders[index].allowance = 0;
        traders[index].active = false;

        emit TraderReclaimed(index, to, reclaimAmount);
    }

    /* ---------------- TraderGene Execution (trader addresses) ---------------- */

    /// @notice Execute an outgoing transfer from the contract to a target, callable only by the TraderGene address and only up to its allowance.
    /// @param to recipient address
    /// @param amount amount in smallest units
    function traderExecuteTrade(address to, uint256 amount) external {
        // identify which trader (if any) is calling
        uint8 foundIndex = 255;
        for (uint8 i = 0; i < 3; i++) {
            if (traders[i].addr == msg.sender) {
                foundIndex = i;
                break;
            }
        }
        require(foundIndex < 3, "caller not a TraderGene");
        TraderGene storage tg = traders[foundIndex];
        require(tg.active, "trader not active");
        require(amount > 0, "amount zero");
        require(tg.allowance >= amount, "allowance exceeded");

        // ensure contract has the tokens to move
        uint256 contractBal = balanceOf(address(this));
        require(contractBal >= amount, "contract insufficient balance");

        // perform the transfer from contract to recipient
        _transfer(address(this), to, amount);

        // decrement allowance
        tg.allowance = tg.allowance - amount;

        emit TraderExecuted(foundIndex, msg.sender, to, amount);
    }

    /* ---------------- Utility / Safety notes ---------------- */

    /// @notice Owner helper to fund the contract address by transferring tokens from owner to contract.
    /// @dev Owner can also mint directly to the contract via `mint(address(this), amount)` before finalizing.
    /// This helper just moves tokens from owner to the contract when called after approval.
    function fundContractFromOwner(uint256 amount) external onlyOwner {
        require(amount > 0, "amount zero");
        // Owner must have approved this contract to move `amount` on its behalf
        _transfer(msg.sender, address(this), amount);
    }

    /// @notice View remaining allowance for a trader in whole tokens (human readable).
    function traderAllowanceInWhole(uint8 index) external view returns (uint256) {
        require(index < 3, "index out of range");
        return traders[index].allowance / (10 ** decimals());
    }

    /// @notice View trader info
    function getTrader(uint8 index) external view returns (address addr, uint256 allowance, bool active) {
        require(index < 3, "index out of range");
        TraderGene memory tg = traders[index];
        return (tg.addr, tg.allowance, tg.active);
    }
}
