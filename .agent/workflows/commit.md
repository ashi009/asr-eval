---
description: How to commit changes effectively with atomic commits
---

# Effective Git Commit Workflow

## Core Principle: Atomic Commits
Each commit should represent ONE logical change. Never combine unrelated changes in a single commit.

## Workflow Overview

1.  **Work & Commit Locally**: Make atomic changes and commit them locally.
2.  **Refine History**: Use `git rebase -i` to clean up, combine, or reorder commits before sharing.
3.  **Review**: Ask for review before pushing to remote.
4.  **Push**: Push only when the history is clean and approved.

## 1. Commit Style (Go Style)

Follow the [Go project commit message style](https://go.dev/wiki/CommitMessage).

### Consistency Check

Before committing, check recent commits for the same package to ensure consistent naming (e.g., `pkg/evalv2` vs `evalv2`).

```bash
git log --oneline -- <path/to/package>
```

### Description Format

> Notably, for the subject (the first line of description):

- the name of the package affected by the change goes before the colon
- the part after the colon uses the verb tense + phrase that completes the blank in, “this change modifies Go to **___**”
- the verb after the colon is lowercase
- there is no trailing period
- it should be kept as short as possible (many git viewing tools prefer under ~72 characters, though Go isn’t super strict about this).

For the body (the rest of the description):

- the text should be wrapped to ~72 characters (to appease git viewing tools, mainly), unless you really need longer lines (e.g. for ASCII art, tables, or long links).
- the Fixes line goes after the body with a blank newline separating the two. (It is acceptable but not required to use a trailing period, such as Fixes #12345.).
- there is no Markdown in the commit message.
- we do not use Signed-off-by lines. Don’t add them. Our Gerrit server & GitHub bots enforce CLA compliance instead.
- when referencing CLs, prefer saying “CL nnn” or using a go.dev/cl/nnn shortlink over a direct Gerrit URL or git hash, since that’s more future-proof.
- when moving code between repos, include the CL, repository name, and git hash that it was moved from/to, so it is easier to trace history/blame.

### Example
```text
ui: integrate react-grab to dev server
```

## 2. Managing Local Commits

Ideally, every distinct change (refactor, feature, fix) is a separate commit.

```bash
# Work on feature A
git add <files_for_A>
git commit -m "ui/featureA: add basic structure"

# Work on fix B (unrelated)
git add <files_for_B>
git commit -m "server: fix nil pointer exception"
```

## 3. Refining History (Rebase)

Before sharing your code or asking for a final review (and DEFINITELY before pushing), clean up your local history.

1.  **Start Interactive Rebase**:
    ```bash
    # Rebase the last N commits
    git rebase -i HEAD~N
    ```

2.  **Action Commands**:
    - `pick`: Keep the commit as is.
    - `reword`: Keep changes but edit the message.
    - `edit`: Stop to amend the commit contents.
    - `squash` (or `fixup`): Combine into the previous commit.
    - `drop`: Remove the commit entirely.

3.  **Resolve Conflicts**: If conflicts occur, resolve them, `git add`, then `git rebase --continue`.

## 4. Pushing

> [!WARNING]
> **Avoid Force Push**
> Main branches usually ban force pushes. Never force push unless you are strictly working on a private feature branch that no one else pulls from.

1.  **Check Status**: `git status`
2.  **Pull Updates**: `git pull --rebase origin master` (pull changes from others before pushing)
3.  **Push**: `git push origin master`

## Quick Reference

```bash
# Check staged changes
git diff --cached

# Stage specific files
git add <file>

# Stage parts of files
git add -p <file>

# Interactive rebase
git rebase -i HEAD~3

# Amend the *latest* commit (fix typo, add forgotten file)
git add <file>
git commit --amend
```
