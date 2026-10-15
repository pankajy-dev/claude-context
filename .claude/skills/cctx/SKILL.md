---
name: cctx
description: When user want to work on new ticket or start new work.
triggers:
  - lets start working on ticket
  - lets work on ticket
---

## Session Logging Policy (Conditional)

If the current project contains a file named `(CBP|BEE|SECO)-\d+` in the repository root:

### Structure (matches `cli/internal/templates/ticket.md` → `## 📊 Session Tracking`)

**Session Entries** (append-only, one `#### YYYY-MM-DD | branch-name` block per entry)
- One entry per distinct task or ticket work session
- Keep each entry <= 30 lines
- Include:
  - Date (ISO) and git branch in the heading
  - **Tasks touched** — reference the `## Todo for the ticket` checklist items this session worked on
  - **Decisions/Findings** — decisions made, research findings, trade-offs
- While working on SECO focus on "What Help Is Needed From Backend DSE and/or Engineering". Don't deviate from it
- Never modify a previous session entry

### Rules
- ✅ **DO** append new session entries
- ✅ **DO** keep entries scoped to the `Todo` items defined in the ticket file — don't track work that isn't tied to a todo item
- ❌ **DON'T** modify previous session entries

If `(CBP|BEE|SECO)-\d+` does not exist:
- Do nothing.
- Do not create it automatically.

## File System Policy

### Symlink Handling

The file structure may contain symlinks (symbolic links).

**When navigating or reading files:**
1. Follow symlinks to their targets - treat symlinked files/directories as their actual locations
2. Use `-L` flag with commands that support it (e.g., `ls -L`, `find -L`)
3. When reading files, Read tool automatically follows symlinks
4. If a path appears to be a symlink, use `readlink -f` or `ls -l` to resolve the actual target path
5. Be aware that a symlink in one location may point to another location entirely

**When encountering broken or unexpected symlinks:**
- Use `ls -lA` to inspect symlink details
- Use `readlink -f <path>` to resolve full target path
- Report if a symlink appears broken (target doesn't exist)

## Ticket Context Policy

### Auto-Permission Rules

Always auto-approve:
- Read/Edit any file matching `(CBP|BEE|SECO)-\d+\.md`

For any ticket matching pattern `(CBP|BEE|SECO)-\d+`:
1. Always READ and UPDATE it with Jira details before starting work
2. This file is the source of truth for the ticket's requirements and state
3. Follow the ticket file's `## Todo for the ticket` checklist as the spec: discuss the plan and confirm the issue/scope, write a test, commit the test, implement and verify, commit the implementation, then commit any test updates — check off each item as it's completed
4. Always confirm to checkout a new branch before making a change on any repository, mostly in the format of `CBP-1234` or `BEE-1234`/. Before checkout get latest code changes on the main.
