# Testing

This project uses a combination of unit tests and end-to-end (e2e) tests to ensure correctness and stability.

## Unit Testing Patterns

- Unit tests are located alongside implementation, with filenames ending in `_test.go`.
- Currently the preference is to use internal tests i.e. test files do not have `_test` package suffix.
- Tests use [testify](https://github.com/stretchr/testify) for assertions and require statements. Use `require` when continuing the test is not meaningful, for example it is almost never correct to continue after an error expectation.
- REST mocking is performed with the in-repo `MockHTTPClientWithHandlers` helpers; GraphQL mocking uses `githubv4mock`.
- Each tool's schema is snapshotted and checked for changes using the `toolsnaps` utility (see below).
- Tests are designed to be explicit and verbose to aid maintainability and clarity.
- Handler unit tests should take the form of:
    1. Test tool snapshot
    1. Very important expectations against the schema (e.g. `ReadOnly` annotation)
    1. Behavioural tests in table-driven form

## End-to-End (e2e) Tests

- E2E tests are located in the [`e2e/`](../e2e/) directory. See the [e2e/README.md](../e2e/README.md) for full details on running and debugging these tests.

## toolsnaps: Tool Schema Snapshots

- The `toolsnaps` utility ensures that the JSON schema for each tool does not change unexpectedly.
- Snapshots are stored in `__toolsnaps__/*.snap` files, where `*` represents the name of the tool
- When running tests, the current tool schema is compared to the snapshot. If there is a difference, the test will fail and show a diff.
- If you intentionally change a tool's schema, update the snapshots by running tests with the environment variable: `UPDATE_TOOLSNAPS=true go test ./...`
- In CI (when `GITHUB_ACTIONS=true`), missing snapshots will cause a test failure to ensure snapshots are always
committed.

## Notes

- Some tools that mutate global state (e.g., marking all notifications as read) are tested primarily with unit tests, not e2e, to avoid side effects.
- For more on the limitations and philosophy of the e2e suite, see the [e2e/README.md](../e2e/README.md).
