require("@nomiclabs/hardhat-ethers");

module.exports = {
  solidity: "0.8.20",
  networks: {
    skale: {
      url: process.env.SKALE_RPC_URL,
      accounts: [process.env.PRIVATE_KEY],
    },
  },
};
