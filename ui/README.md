# MCP Server UI

React-based UI apps for the GitHub MCP Server, built with [Vite](https://vitejs.dev/) and [Primer React](https://primer.style/react).

## Apps

- **get-me** - Displays current GitHub user profile card
- **issue-write** - Create/edit GitHub issues with rich markdown editor
- **pr-write** - Create pull requests with full form controls

## Development

```bash
# Install dependencies
npm install

# Build all apps
npm run build

# Type check
npm run typecheck
```

## Testing

The UI apps have E2E tests using [Playwright](https://playwright.dev/) with mocked MCP communication.

```bash
# Run all E2E tests
npm run test:e2e

# Run tests with UI mode (interactive)
npm run test:e2e:ui

# View test report
npm run test:e2e:report
```

### Test Structure

```
e2e/
├── fixtures/
│   ├── test.ts        # Extended test fixture with MCP mocking
│   └── mcp-mocks.ts   # Mock data for MCP tool responses
└── tests/
    ├── get-me.spec.ts
    ├── issue-write.spec.ts
    └── pr-write.spec.ts
```

### How Mocking Works

The tests mock the MCP ext-apps communication layer by intercepting `window.parent.postMessage` calls. This allows testing the UI components without a real MCP host:

1. Tests use `gotoApp()` fixture to navigate to an app with mocks injected
2. Mock responses are defined in `mcp-mocks.ts`
3. The fixture intercepts JSON-RPC messages and responds with appropriate mock data

## Building for Production

Apps are built as single-file HTML bundles that embed all CSS and JS:

```bash
npm run build
```

Output is placed in `dist/` directory with each app as a standalone HTML file.
