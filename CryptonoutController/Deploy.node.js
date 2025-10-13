name: Node.js Master Deploy

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  build-and-deploy:
    runs-on: ubuntu-latest

    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'

      - name: Install dependencies
        run: npm install

      - name: Run tests
        run: npm test

      - name: Build app
        run: npm run build

      # Example: Deploy step (customize for your environment)
      # - name: Deploy to Server
      #   env:
      #     DEPLOY_KEY: ${{ secrets.DEPLOY_KEY }}
      #   run: |
      #     scp -i $DEPLOY_KEY -r ./dist user@yourserver:/path/to/deploy

      # Add more deployment options as needed
