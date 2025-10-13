// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract InfinityEarningsMatrix {
    address public token;
    address public vault;
    address public reinvestPool;
    address public upgradeFund;
    address public bountyNova;
    address public orchestrator;

    constructor(
        address _token,
        address _vault,
        address _reinvestPool,
        address _upgradeFund,
        address _bountyNova,
        address _orchestrator
    ) {
        token = _token;
        vault = _vault;
        reinvestPool = _reinvestPool;
        upgradeFund = _upgradeFund;
        bountyNova = _bountyNova;
        orchestrator = _orchestrator;
    }

    receive() external payable {
        uint256 amount = msg.value;
        payable(reinvestPool).transfer((amount * 60) / 100);
        payable(upgradeFund).transfer((amount * 30) / 100);
        payable(bountyNova).transfer((amount * 10) / 100);
    }
}
