# What each destructive call actually destroys

Ordered by how much is lost and how little can be recovered. The confirmation each one
needs is proportional to that, and none of them is satisfied by an implied instruction.

## Removing a repository

`delete_repository` removes the repository and everything attached to it: issues, pull
requests, releases, discussions, wikis, packages, Actions history, and every branch. A
fork does not preserve any of it. Recovery is possible only within a short window and
only through a support request by the owner, and it does not cover everything.

**Required before the call:**

- The owner and repository name written out by the person asking, in full. Not
  confirmed back from something you proposed — supplied by them.
- Explicit acknowledgement that issues and pull requests go with it. This is the part
  people do not expect.
- Confirmation that no fork is a substitute. If the intent is "I want it out of my
  way", archiving or transferring is what was meant, and neither is available here —
  say so rather than removing it.

**Never:**

- Remove a repository as one step of a larger task.
- Remove one because it looked unused, stale or empty.
- Batch it with anything else, or offer it as an option alongside reversible actions.

If any part of the confirmation is missing or approximate, stop and ask again. There is
no version of this that is worth being fast about.

## Removing a file

`delete_file` commits a removal to a branch. The content survives in the history, so it
is recoverable by someone who knows where to look — but every reference to it breaks
immediately, and on a default branch that is visible to everyone at once.

**Required before the call:**

- The exact paths, listed, each one named by the person asking or explicitly agreed
  from a list you presented.
- The branch, stated. A removal on a working branch and a removal on the default branch
  are different acts.
- A check that nothing obvious still points at it. One code search for the file's name
  costs one call and catches the common case.

**Never:** infer the set from a pattern. "Remove the old ones", "clear out the unused
files" and "get rid of the duplicates" are all instructions to produce a list and ask,
not instructions to remove.

## Replacing a file

`create_or_update_file` and `push_files` overwrite the file at that path with exactly
what you send. The previous content stays in the history; the working state does not.

The specific way this goes wrong is not force — it is truncation. A file is read in
part, or summarised, and then written back as though the summary were the whole thing.
Everything not in `content` is gone from that branch.

**Required before the call:**

- The complete intended contents, derived from a full read of the current file rather
  than from a partial one.
- For an existing file, a `sha` from that same read. A stale one means the file moved
  under you, and the failure is the safety mechanism working.

## Creating a repository

`create_repository` is not destructive, but it is not quietly reversible either: the
name is taken and the owner sees it. Visibility defaults to private; `private: false`
makes it public from the moment it exists. Removing it means the section above.

**Required before the call:** the name, the owner — personal account or the
`organization` — and the visibility, all three stated rather than defaulted.

## Forking

`fork_repository` is the safest write available here and the right answer when the
account cannot write to the original. Two things still deserve a word first: a fork is
public if the original is, and a second fork of something already forked leads to
changes landing on a copy nobody is watching. Check whether one already exists before
making another.

## The rule underneath all of this

The confirmation is not a formality to be got through, and a person saying "yes, go
ahead" to a summary they have not read is not agreement. Name the exact thing, name
what is lost, and let them say it back. If they will not, that is an answer.
