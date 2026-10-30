---
description: |
  Triages one pull request from a contributor without write access. The check
  job compares the pull request with the pull request template in code, and
  the agent labels and comments only when a check fails. pr-triage-dispatch.yml
  starts one run when a pull request is opened or marked ready.

on:
  # Runs from fork pull requests cannot authenticate the agent in a public
  # repository, so pr-triage-dispatch.yml starts this workflow instead.
  workflow_dispatch:
    inputs:
      pr_number:
        description: Pull request number
        required: true
        type: number

run-name: "PR Triage #${{ inputs.pr_number }}"

# Skip the agent when the check job found nothing to act on.
if: needs.check.outputs.result == 'invalid' || needs.check.outputs.result == 'missing_details'

concurrency:
  job-discriminator: ${{ inputs.pr_number }}

permissions:
  contents: read
  copilot-requests: write

jobs:
  # Reads the pull request in code, so its content never reaches the agent.
  check:
    runs-on: ubuntu-latest
    permissions:
      pull-requests: read
    outputs:
      result: ${{ steps.check.outputs.result }}
      failed_sections: ${{ steps.check.outputs.failed_sections }}
    steps:
      - name: Check the pull request against the template
        id: check
        uses: actions/github-script@3a2844b7e9c422d3c10d287c895573f7108da1b3 # v9.0.0
        with:
          script: |
            const prNumber = Number(context.payload.inputs.pr_number);
            const { data: pr } = await github.rest.pulls.get({ ...context.repo, pull_number: prNumber });
            if (pr.state !== 'open' || pr.draft) {
              core.setOutput('result', 'skip');
              return;
            }
            if (pr.changed_files === 0) {
              core.setOutput('result', 'invalid');
              return;
            }

            const sections = new Map();
            let current = null;
            let inFence = false;
            for (const line of (pr.body ?? '').replace(/<!--[\s\S]*?-->/g, '').split(/\r?\n/)) {
              if (/^\s*(```|~~~)/.test(line)) inFence = !inFence;
              const heading = !inFence && line.match(/^#{1,6}\s+(.*?)\s*$/);
              if (heading) {
                current = [];
                sections.set(heading[1].toLowerCase(), current);
              } else if (current) {
                current.push(line.trim());
              }
            }
            const section = (name) =>
              [...sections].find(([heading]) => heading.startsWith(name.toLowerCase()))?.[1] ?? null;
            const isBox = (line) => /^[-*]\s*\[[ xX]\]/.test(line);
            const checked = (lines, label = '') =>
              lines.some((line) => line.match(/^[-*]\s*\[[xX]\]\s*(.*)$/)?.[1].startsWith(label));
            const hasText = (lines) =>
              lines.some((line) => line !== '' && !/^[-*]$/.test(line) && !/^Fixes\s*#$/i.test(line) && !isBox(line));

            const failed = [];
            const requireText = (name) => {
              const lines = section(name);
              if (!lines || !hasText(lines)) failed.push(name);
            };
            const requireBox = (name) => {
              const lines = section(name);
              if (!lines || !checked(lines)) failed.push(name);
            };

            requireText('Summary');
            requireText('Why');
            requireText('What changed');
            requireBox('MCP impact');
            const impact = section('MCP impact') ?? [];
            if (checked(impact, 'Tool schema or behavior changed') || checked(impact, 'New tool added')) {
              requireText('Prompts tested');
            }
            requireBox('Security / limits');
            const renaming = section('Tool renaming') ?? [];
            const renames = checked(renaming, 'I am renaming tools');
            if (renames === checked(renaming, 'I am not renaming tools') ||
                (renames && !checked(renaming, 'I have added the new tool aliases'))) {
              failed.push('Tool renaming');
            }
            const lint = section('Lint & tests');
            if (!lint || !(checked(lint) || hasText(lint))) failed.push('Lint & tests');
            requireBox('Docs');

            core.setOutput('result', failed.length > 0 ? 'missing_details' : 'pass');
            core.setOutput('failed_sections', failed.join(', '));

tools:
  # The agent only acts on the check job's results, so it gets no shell and no
  # GitHub tools.
  bash: false
  cli-proxy: false
  github: false

max-ai-credits: 100

safe-outputs:
  # Previews outputs in the run summary without writing anything. Remove this
  # line once the previews look right.
  staged: true
  # Keep run diagnostics in the Actions logs instead of public tracking issues.
  report-failure-as-issue: false
  report-missing-tool:
    create-issue: false
  report-incomplete:
    create-issue: false
  noop:
    report-as-issue: false
  threat-detection:
    report-as-issue: false
    max-ai-credits: 100
  # Pinned to the dispatched pull request.
  add-labels:
    target: ${{ inputs.pr_number }}
    allowed: [waiting-for-reply, invalid]
    max: 1
    max-labels: 1
  add-comment:
    target: ${{ inputs.pr_number }}
    max: 1

timeout-minutes: 10
---

# Pull Request Triage

The check job compared pull request #${{ inputs.pr_number }} with the pull
request template. Act only on its results:

- Result: `${{ needs.check.outputs.result }}`
- Failed sections: ${{ needs.check.outputs.failed_sections }}

## Decide

1. If the result is `invalid`, add the `invalid` label and post the `INVALID`
   comment.
2. If the result is `missing_details`, add the `waiting-for-reply` label and post
   the `MISSING_DETAILS` comment.
3. Otherwise, call `noop` with the result. Don't add a label or post a comment.

## Comments

Post the comment exactly as written, replacing only the text in angle brackets.

`INVALID`:

```markdown
### AI Assessment: Invalid

This pull request doesn't change any files. Please push your changes, or close
it if you opened it by mistake.
```

`MISSING_DETAILS`:

```markdown
### AI Assessment: Missing Details

Thanks for the contribution. Please edit the description to fill in these
sections of the pull request template:

- <one line for each failed section, in the order listed above>
```
