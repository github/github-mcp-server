---
description: |
  Triages new and reopened issues: checks that the issue template was filled in,
  flags spam and invalid submissions, and posts one assessment comment. The
  check job reads the issue in code, and the agent only acts on its result.

on:
  issues:
    types: [opened, reopened]
  # Most issues here come from people without write access, which the default
  # roles (admin, maintainer, write) would skip.
  roles: all

# Skip the agent when the check job found nothing to act on.
if: needs.check.outputs.result != 'none'

permissions:
  contents: read
  copilot-requests: write

jobs:
  # Reads the issue in code, so its content never reaches the agent.
  check:
    runs-on: ubuntu-latest
    permissions: {}
    outputs:
      result: ${{ steps.check.outputs.result }}
    steps:
      - name: Check the issue against the issue templates
        id: check
        uses: actions/github-script@3a2844b7e9c422d3c10d287c895573f7108da1b3 # v9.0.0
        with:
          script: |
            const { title, body } = context.payload.issue;
            const text = (body ?? '').replace(/<!--[\s\S]*?-->/g, '');
            const norm = (s) => s.replace(/[‘’]/g, "'").replace(/[“”]/g, '"').replace(/\s+/g, ' ').trim().toLowerCase();

            // Sections of the templates in .github/ISSUE_TEMPLATE and their own text, which
            // doesn't count as content. Keep in sync with the templates.
            const templates = [
              {
                sections: {
                  "Describe the bug": ["A clear and concise description of what the bug is."],
                  "Affected version": ["Please run ` docker run -i --rm ghcr.io/github/github-mcp-server ./github-mcp-server --version` and paste the output below"],
                  "Steps to reproduce the behavior": ["1. Type this '...'", "2. View the output '....'", "3. See error"],
                  "Expected vs actual behavior": ["A clear and concise description of what you expected to happen and what actually happened."],
                  "Logs": ["Paste any available logs. Redact if needed."],
                },
                required: ['Describe the bug', 'Affected version', 'Steps to reproduce the behavior', 'Expected vs actual behavior'],
              },
              {
                sections: {
                  "Describe the feature or problem you’d like to solve": ["A clear and concise description of what the feature or problem is."],
                  "Proposed solution": ["How will it benefit GitHub MCP Server and its users?"],
                  "Example prompts or workflows (for tools/toolsets only)": ["If it's a new tool or improvement, share 3–5 example prompts or workflows it would enable. Just enough detail to show the value. Clear, valuable use cases are more likely to get approved."],
                  "Additional context": ["Add any other context like screenshots or mockups are helpful, if applicable."],
                },
                required: ['Describe the feature or problem you’d like to solve', 'Proposed solution'],
              },
            ];

            // Lines with fence lines removed, marking which are inside fenced code.
            const content = [];
            let fence = null;
            for (const line of text.split(/\r?\n/)) {
              const marker = line.match(/^ {0,3}(`{3,}|~{3,})(.*)$/);
              if (marker && !fence) {
                fence = marker[1];
                continue;
              }
              if (marker && marker[1][0] === fence?.[0] && marker[1].length >= fence.length && marker[2].trim() === '') {
                fence = null;
                continue;
              }
              if (line.trim() !== '') content.push({ line: line.trim(), code: fence !== null });
            }

            const sections = new Map([['', []]]);
            let section = sections.get('');
            for (const { line, code } of content) {
              const heading = !code && line.match(/^#{1,6}\s+(.*?)\s*$/);
              if (heading) {
                section = [];
                sections.set(norm(heading[1]), section);
              } else {
                section.push(line);
              }
            }

            // Fields of the Insiders feedback template, such as "Feature: ...". Each runs
            // until the next field.
            const fields = new Map();
            const preamble = [];
            let field = null;
            let duplicateField = false;
            for (const { line, code } of content) {
              const match = !code && line.replace(/^#+\s*/, '').match(/^(Version|Feature|Feedback):(.*)$/);
              if (match) {
                duplicateField ||= fields.has(match[1]);
                field = match[1];
                fields.set(field, [match[2].trim()]);
              } else if (field) {
                fields.get(field).push(line);
              } else {
                preamble.push(line);
              }
            }
            const fieldText = (name) => (fields.get(name) ?? []).filter(Boolean).join(' ');

            // Use the first template whose required headings are all present. A partly
            // used template counts as no template, so it's left alone.
            const template = templates.find((t) => t.required.every((heading) => sections.has(norm(heading))));
            const insiders = !template && title.startsWith('Insiders Feedback:') && fields.has('Feature') && fields.has('Feedback');
            if (insiders && duplicateField) {
              core.setOutput('result', 'none');
              return;
            }

            let parts;
            let written;
            if (template) {
              const own = (heading) => {
                const placeholders = Object.entries(template.sections)
                  .filter(([name]) => norm(name) === heading)
                  .flatMap(([, lines]) => lines.map(norm));
                return (sections.get(heading) ?? []).filter((line) => !placeholders.includes(norm(line)));
              };
              parts = template.required.map((heading) => own(norm(heading)).join(' '));
              written = [...sections.keys()].flatMap(own);
            } else if (insiders) {
              parts = [fieldText('Feature'), fieldText('Feedback')];
              written = [...preamble, ...parts, fieldText('Version') === 'Insiders' ? '' : fieldText('Version')];
            } else {
              written = content.map(({ line }) => line);
              parts = [written.join(' ')];
            }

            const urls = `${title}\n${text}`.match(/https?:\/\/\S+/gi) ?? [];
            const externalLink = urls.some((url) => {
              try {
                const host = new URL(url.replace(/[)\]}>.,;:!?'"*_]+$/, '')).hostname.toLowerCase();
                return host !== 'github.com' && host !== 'www.github.com';
              } catch {
                return false;
              }
            });
            const withoutLinks = (line) => line.replace(/\[[^\]]*\]\([^)]*\)/g, '').replace(/https?:\/\/\S+/gi, '');
            const empty = written.every((line) => withoutLinks(line).trim() === '');
            const missing = parts.some((part) => part === '');
            const titleWords = title.replace(/^Insiders Feedback:/, '').split(/\s+/).filter(Boolean).length;

            let result = 'none';
            if (empty && externalLink) result = 'spam';
            else if (empty && titleWords <= 2) result = 'invalid';
            else if (missing) result = 'missing_details';
            else if (template || insiders) result = 'ready_for_review';
            core.setOutput('result', result);

tools:
  # The agent only acts on the check job's result, so it gets no shell and no
  # GitHub tools.
  bash: false
  cli-proxy: false
  github: false

max-ai-credits: 100
max-daily-ai-credits: 1000

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
  add-labels:
    allowed: [waiting-for-reply, invalid, spam]
    max: 1
    max-labels: 1
  add-comment:
    max: 1

timeout-minutes: 10
source: githubnext/agentics/workflows/issue-triage.md@139fa09ef17cde4135e696ff8e7af09bc5ddfa6e
---

# Issue Triage

The check job compared issue #${{ github.event.issue.number }} with the issue
templates. Its result is `${{ needs.check.outputs.result }}`. Act only on this
result:

1. `spam`: add the `spam` label. Don't comment.
2. `invalid`: add the `invalid` label and post the `INVALID` comment.
3. `missing_details`: add the `waiting-for-reply` label and post the
   `MISSING_DETAILS` comment.
4. `ready_for_review`: post the `READY_FOR_REVIEW` comment. Don't add a label.
5. Anything else: call `noop`.

Post the comment word for word, and don't close the issue.

`READY_FOR_REVIEW`:

```markdown
### AI Assessment: Ready for Review

This issue has the details maintainers need.
```

`MISSING_DETAILS`:

```markdown
### AI Assessment: Missing Details

Thanks for opening this issue. Some details maintainers need are missing.
Please edit the issue to fill in each section of the issue template, or, if you
didn't use a template, to describe the problem or request.
```

`INVALID`:

```markdown
### AI Assessment: Invalid

This issue doesn't describe a problem or request yet. Please edit it to add
the details, or close it if you opened it by mistake.
```
