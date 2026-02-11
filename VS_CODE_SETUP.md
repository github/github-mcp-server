# VS Code Setup Instructions for github-mcp-server

## Prerequisites
1. Install [Visual Studio Code](https://code.visualstudio.com/) if you haven't already.
2. Ensure you have Node.js installed; you can download it from [nodejs.org](https://nodejs.org/).

## Step-by-Step Setup

### 1. Clone the Repository
Open the terminal in VS Code (or any terminal) and run:
```bash
git clone https://github.com/scutuatua-crypto/github-mcp-server.git
```

### 2. Open the Project
After cloning the repository, open the project folder in VS Code:
- Click on `File` -> `Open Folder...` and select the `github-mcp-server` folder.

### 3. Install Dependencies
Navigate to the project directory and install the required dependencies using npm:
```bash
cd github-mcp-server
npm install
```

### 4. Configure Environment Variables
Create a `.env` file in the root of the project. You can base this on the `.env.example` file if it exists. Make sure to fill in the required information.

### 5. Open Integrated Terminal
To run the project, you can use the integrated terminal in VS Code:
- Go to `View` -> `Terminal` or use the shortcut `` Ctrl + ` ``.

### 6. Start the Server
In the terminal, run:
```bash
npm start
```
This command will start the development server.

### 7. Access Your Application
Open your web browser and go to `http://localhost:3000` (or the specified port in your application) to see your application running.

### 8. Debugging
- You can set breakpoints and debug your application using the built-in debugger of VS Code by clicking on the left sidebar and selecting the `Run and Debug` icon.

### 9. Additional Extensions
Consider installing the following VS Code extensions to enhance your development experience:
- ESLint: for linting your JavaScript code.
- Prettier: for code formatting.
- GitLens: for enhanced Git capabilities.

## Conclusion
You are now all set up to work on the github-mcp-server project using Visual Studio Code! Happy coding!