const hre = require("hardhat");

async function main() {
  const [deployer] = await hre.ethers.getSigners();
  console.log("Deploying contracts with the account:", deployer.address);
  console.log("Account balance:", (await deployer.getBalance()).toString());

  // Replace 'CryptonoutController' with your contract's name if different
  const Contract = await hre.ethers.getContractFactory("CryptonoutController");
  const contract = await Contract.deploy();

  await contract.deployed();

  console.log("Contract deployed to:", contract.address);

  // Optionally write address to a file
  require("fs").writeFileSync("deployment-info.txt", `Contract Address: ${contract.address}\nTxHash: ${contract.deployTransaction.hash}\n`);
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
