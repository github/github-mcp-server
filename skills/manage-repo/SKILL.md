---
name: manage-repo
description: Create repos, manage branches, and push file changes. Use when creating a new repository, making a branch, committing files via the API, forking a repo, or managing repository contents.
allowed-tools:
  - create_repository
  - fork_repository
  - create_branch
  - create_or_update_file
  - push_files
  - delete_file
  - get_file_contents
  - search_repositories
---

# Manage Repository

Create repos, branches, and manage file contents.

## Available Tools
- `create_repository` — create a new repo
- `fork_repository` — fork an existing repo
- `create_branch` — create a branch
- `create_or_update_file` — single file create/update with commit
- `push_files` — push multiple files in one commit
- `delete_file` — delete a file with commit
- `get_file_contents` — read files and directories
- `search_repositories` — find existing repos

## Tips
- Use `push_files` for multi-file changes — creates a single atomic commit.
- Use `create_or_update_file` only for single-file operations.
- Include README, LICENSE, and .gitignore when creating new repos.
- Fork for contributing to others' projects. Create new repos for new projects.
