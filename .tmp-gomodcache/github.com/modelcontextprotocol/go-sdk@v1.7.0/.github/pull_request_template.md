### PR Guideline

Typically, PRs should consist of a single commit, and so should generally follow
the [rules for Go commit messages](https://go.dev/wiki/CommitMessage).

You **must** follow the form:

```
net/http: handle foo when bar

[longer description here in the body]

Fixes #12345
```
Notably, for the subject (the first line of description):

- the name of the package affected by the change goes before the colon
- the part after the colon uses the verb tense + phrase that completes the blank in, “this change modifies this package to ___________”
- the verb after the colon is lowercase
- there is no trailing period
- it should be kept as short as possible

Additionally:

- Markdown is allowed.
- For a pervasive change, use "all" in the title instead of a package name.
- The PR description should provide context (why this change?) and describe the changes
  at a high level. Changes that are obvious from the diffs don't need to be mentioned.
