## Contributing

[fork]: https://github.com/github/github-mcp-server/fork
[pr]: https://github.com/github/github-mcp-server/compare
[style]: https://github.com/github/github-mcp-server/blob/main/.golangci.yml

Hi there! We're thrilled that you'd like to contribute to this project. Your help is essential for keeping it great.

Contributions to this project are [released](https://help.github.com/articles/github-terms-of-service/#6-contributions-under-repository-license) to the public under the [project's open source license](LICENSE).

Please note that this project is released with a [Contributor Code of Conduct](CODE_OF_CONDUCT.md). By participating in this project you agree to abide by its terms.

## What we're looking for

We can't guarantee that every tool, feature, or pull request will be approved or merged. Our focus is on supporting high-quality, high-impact capabilities that advance agentic workflows and deliver clear value to developers.

To increase the chances your request is accepted:
* Include real use cases or examples that demonstrate practical value
* Please create an issue outlining the scenario and potential impact, so we can triage it promptly and prioritize accordingly.
* If your request stalls, you can open a Discussion post and link to your issue or PR
* We actively revisit requests that gain strong community engagement (👍s, comments, or evidence of real-world use)

Thanks for contributing and for helping us build toolsets that are truly valuable!

## Prerequisites for running and testing code

These are one time installations required to be able to test your changes locally as part of the pull request (PR) submission process.

1. Install Go 1.26.8 or later [through download](https://go.dev/doc/install) | [through Homebrew](https://formulae.brew.sh/formula/go)
2. [Install golangci-lint v2.14.0](https://golangci-lint.run/welcome/install/#local-installation), or let `script/lint` install the repository-pinned version. The pinned version supports both Go 1.26 and Go 1.27.

### UI development

Use Node.js 26.x for the UI in `ui/`. This matches the Docker UI build stage
and `@types/node`; GitHub Actions reads the Node version from `ui/package.json`.
Node 26 is a supported Current release, with LTS scheduled for October 2026.

From `ui/`, run `npm ci`, `npm run typecheck`, and `npm run build`.
Type checking uses the native TypeScript 7 compiler through `tsc`; Vite handles
transpilation and bundling separately. No TypeScript compiler API integration,
typescript-eslint, or ts-node is required.

### MCP Apps UI

The `ui/` views use React 19 and Primer React 38. With Node.js 20.19+ or 22.12+, run `cd ui && npm ci && npm run typecheck && npm run build && npm audit` before `script/test`. The build writes self-contained HTML to `pkg/github/ui_dist/`, which the Go server embeds; these generated files are not committed.

Primer 38 no longer exports `Box` or accepts `sx`/styled-system props. Use semantic HTML, native `style` props for dynamic/layout styles, and CSS Modules for nested selectors. `AppProvider` loads Primer's primitive tokens and light/dark themes, so use CSS variables rather than JavaScript theme values. Custom element typings must augment `react/jsx-runtime` rather than the global `JSX` namespace.

For UI dependency upgrades, compare all four views (`get-me`, `issue-write`, `pr-write`, and `pr-edit`) in light/dark themes and at narrow widths in an MCP Apps host, including menus, Markdown editing/preview, and completed-result views. Include before/after screenshots in the pull request.

## Submitting a pull request

1. [Fork][fork] and clone the repository
2. Make sure the tests pass on your machine: `go test -v ./...`
3. Make sure linter passes on your machine: `golangci-lint run`
4. Create a new branch: `git checkout -b my-branch-name`
5. Add your changes and tests, and make sure the Action workflows still pass
    - Run linter: `script/lint`
    - Update snapshots and run tests: `UPDATE_TOOLSNAPS=true go test ./...`
    - Update readme documentation: `script/generate-docs`
    - If renaming a tool, add a deprecation alias (see [Tool Renaming Guide](docs/tool-renaming.md))
    - For toolset and icon configuration, see [Toolsets and Icons Guide](docs/toolsets-and-icons.md)
    - For typed tool registration and compatibility schemas, see [Typed Tool Schemas](docs/typed-tool-schemas.md)
6. Push to your fork and [submit a pull request][pr] targeting the `main` branch
7. Pat yourself on the back and wait for your pull request to be reviewed and merged.

Here are a few things you can do that will increase the likelihood of your pull request being accepted:

- Follow the [style guide][style].
- Write tests.
- Keep your change as focused as possible. If there are multiple changes you would like to make that are not dependent upon each other, consider submitting them as separate pull requests.
- Write a [good commit message](http://tbaggery.com/2008/04/19/a-note-about-git-commit-messages.html).

## Resources

- [How to Contribute to Open Source](https://opensource.guide/how-to-contribute/)
- [Using Pull Requests](https://help.github.com/articles/about-pull-requests/)
- [GitHub Help](https://help.github.com)
