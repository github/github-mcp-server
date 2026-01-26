<!-- Short PR template for toolsnaps / docs changes -->

Title: pkg/github: <short description of change>

What
- One-sentence summary of the change.

Why
- Short rationale and intended effect on MCP tools or docs.

Dev steps performed
- `script/lint` ✅
- `script/test` ✅
- `UPDATE_TOOLSNAPS=true go test ./...` (if applicable) ✅
- `script/generate-docs` (if applicable) ✅

Files to review
- `pkg/github/<file.go>`
- `pkg/github/__toolsnaps__/*.snap` (if changed)
- README.md / docs/ changes (if changed)

Checklist (required for toolsnaps/docs changes)
- [ ] I ran `script/lint` and fixed formatting/lint issues
- [ ] I ran `script/test` and all tests pass
- [ ] I updated tool snapshots and committed `.snap` files (when schema changed)
- [ ] I ran `script/generate-docs` and included README diffs (if applicable)
- [ ] CI passes: docs-check, lint, license-check
 - [ ] CI passes: docs-check, lint, license-check, link-check

Notes for reviewers
- Brief notes on anything reviewers should watch for (e.g., schema changes, backward-compat concerns).
