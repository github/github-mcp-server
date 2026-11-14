---
name: label-scheme-management
description: "USE FOR: curating the label vocabulary a repository uses — adding ones it lacks, auditing which are actually applied, renaming and recolouring them, merging overlapping ones, and tidying away the unused. NOT FOR: applying an existing label to an issue, or deciding which issues matter."
---

# Curating what a repository's labels mean

Labels are a shared vocabulary, and every change to one changes the meaning of every
issue already carrying it. This is a repository-wide edit dressed up as a small one.

## Sequence

1. **See the whole set at once.** `list_label` returns every label ordered by issue
   count, most used first. That ordering is the audit: the tail is where the dead
   vocabulary lives.
2. **Find out what is actually in use.** For anything you are considering changing,
   `list_issues` filtered by that label, omitting `state` to include open and closed issues, shows what carries
   it. A label with no issues has no cost to retire; one with two hundred is load-bearing.
3. **Read the one you are about to change.** `get_label` returns its exact name, colour
   and description. Names are case-sensitive in ways that are easy to get wrong, and the
   description is often the only record of what the label was meant to distinguish.
4. **Make the change.** `label_write` with `method` set to `create`, `update` or
   `delete`. Renaming is `update` with `new_name`.
5. **Report what moved**, including how many issues were affected by each change.

## Decision points

- **Retire or leave?** A label unused for a long time is clutter; a label used once a
  year is a category someone will look for. Check the age of the issues carrying it
  before calling it dead.
- **Rename or create-and-migrate?** `update` with `new_name` carries every existing
  issue with it, keeping the history intact. Creating a second label and applying it by
  hand splits the set in two, and the older issues stop appearing in the filter people
  use. Rename unless the meaning is genuinely changing.
- **Merging two overlapping labels is not one call.** The issues on the losing label
  have to be moved before it is removed, and moving them is issue-by-issue work that
  belongs to whoever is triaging. Say that rather than removing the label and orphaning
  the classification.
- **Colour carries meaning.** Projects usually encode severity or kind in hue. Read the
  existing set before picking; a new colour that clashes with an established convention
  is worse than a dull one that fits.
- **Descriptions are worth writing.** A label whose meaning has to be inferred from its
  name gets applied inconsistently, and that is the actual cause of most label sprawl.

## Fields that matter

- `name` — required for every operation, and it is the identifier. For a rename it is
  the *current* name; `new_name` is the target.
- `color` — six hex characters with no leading marker. Required when creating.
- `description` — optional, and the difference between a scheme people follow and one
  they guess at.
- `method` — `create`, `update`, `delete`. There is no bulk form; each label is its own
  call and its own decision.

## Confirm before changing the scheme

- **Present the full intended set of changes before the first call** — every label, what
  happens to it, and how many issues it currently carries. These changes are visible to
  everyone using the repository immediately.
- **Removing a label removes it from every issue that had it.** The issues survive; the
  classification does not, and it cannot be restored by recreating the label. State the
  issue count and get explicit agreement naming that label.
- **Renaming is safer but not invisible** — saved filters, automation and templates that
  reference the old name stop matching. Say so when the label is one of the heavily used
  ones.
- **Never infer the set from a pattern.** "Clean up the unused labels" is an instruction
  to produce a list with counts and ask, not an instruction to remove anything.
- **Creating labels wholesale is also a scheme change.** Importing a standard set into a
  repository that already has its own convention is a bigger change than it looks;
  propose it as one.

## Empty and failure cases

- **No labels at all** — a repository with an empty set has usually had them removed
  deliberately, or is new. Ask which before proposing a scheme.
- **A label is already present when creating** — names collide case-insensitively even
  though they display with their original case. Read the existing one and decide whether
  to update it rather than retrying.
- **A rename is rejected because the target exists** — this is the merge case, not a
  rename. Two labels cannot be collapsed by renaming one onto the other; the issues have
  to be moved first.
- **The account cannot write** — label administration usually needs more access than
  commenting does. Report the intended changes as a list so someone else can apply them.
- **A label appears unused but the count is from open issues only** — omit `state`
  when checking before concluding. Closed issues carry labels too, and they are the
  history.

## Report coverage

Say what was checked and what was not, so an unchecked label is not read as a clean one:

| Label | Issues carrying it | Change | Result |
| --- | --- | --- | --- |
| bug | 214 | recoloured | ✅ applied |
| wontfix | 0 | proposed for removal | ⚠️ awaiting agreement |
| stale | not checked | none | ⚠️ usage not established |
