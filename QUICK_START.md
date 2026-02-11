# Quick Start Guide

This document provides automatic setup instructions for a development environment using VS Code and Remix IDE.

## Prerequisites
- Ensure you have [Node.js](https://nodejs.org/) installed.
- Install the latest version of [VS Code](https://code.visualstudio.com/).

## Setting Up VS Code
1. Open VS Code.
2. Install the necessary extensions:
   - [JavaScript/TypeScript Nightly](https://marketplace.visualstudio.com/items?itemName=ms-vscode.vscode-typescript-next)
   - [Solidity by Juan Blanco](https://marketplace.visualstudio.com/items?itemName=JuanBlanco.solidity)

3. Clone the repository:
   ```bash
   git clone https://github.com/scutuatua-crypto/github-mcp-server.git
   cd github-mcp-server
   ```

4. Install dependencies:
   ```bash
   npm install
   ```

5. Open the project folder in VS Code and start developing!

## Setting Up Remix IDE
1. Go to the [Remix IDE](https://remix.ethereum.org/).
2. Click on the `File` icon to create a new file and name it appropriately (e.g., `MyContract.sol`).
3. Copy and paste your Solidity code into the Remix editor.
4. Select the appropriate compiler version from the "Solidity Compiler" plugin.
5. Compile your contract and deploy it using the `Deploy & Run Transactions` plugin.

For more information, check the [Remix Documentation](https://remix-ide.readthedocs.io/en/latest/).