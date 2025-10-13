const hre = require("hardhat");

async function main() {
  const [signer] = await hre.ethers.getSigners();

  const Relayer = await hre.ethers.getContractAt(
    "RelayerGene",
    "0xYourRelayerGeneAddress" // Already deployed sponsor relayer
  );

  // Example: deploy token
  const Token = await hre.artifacts.readArtifact("FutureSkaleTokenOwnerless");
  const tokenBytecode = Token.bytecode;
  const salt = hre.ethers.id("FST45");
  const tx = await Relayer.deploy(tokenBytecode, salt);
  const receipt = await tx.wait();
  console.log("🚀 Token deployed:", receipt.logs[0].address);
}

main().catch(console.error);
