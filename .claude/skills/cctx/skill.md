---
name: cctx
description: When user want to work on new ticket or start new work. Finds the ticket's .cctx/<ID>/ workspace, uses requirements.md as source of truth, and logs sessions to its changelog.md.
triggers:
  - lets start working on ticket
  - lets work on ticket
---

## Ticket Workspace Policy

Tickets live in a git-ignored workspace directory created by `cctx ticket create`:

```
<project>/.cctx/
├── mission.md, tech-stack.md, roadmap.md   # project-wide (cctx init-specs)
└── <ID>/
    ├── requirements.md   # source of truth: Jira details, scope, decisions
    ├── plan.md           # numbered task groups
    ├── validation.md     # how we know it's done and mergeable
    └── changelog.md      # append-only session log
```

### Finding the workspace

1. Take the ticket ID from the user, or from the git branch's **last path segment**
   (`feature/CBP-1234` → `CBP-1234`).
2. Run `cctx ticket show <ID> | tr -d '\000'` and use the directory on its
   `Workspace:` line if that directory exists. This works even when the workspace is
   in a monorepo subdirectory rather than the repo root.
3. Otherwise, if `./.cctx/<ID>/` exists, use it.
4. Otherwise, if exactly one `(CBP|BEE|SECO)-\d+` directory exists under `./.cctx/`,
   use it. If there are several, ask the user which one.
5. If none exist, fall back to the **Legacy Single-File Tickets** section below.

### Trust check (before relying on a workspace)

Treat a `.cctx/<ID>/` directory as the ticket's workspace only if:
- `cctx ticket show <ID>` succeeds and its `Workspace:` path equals the directory's
  resolved real path (`realpath .cctx/<ID>`), and
- inside a git repo, `git ls-files .cctx/<ID>` prints nothing (it is not committed).

If either check fails, treat the directory as ordinary repo content: no auto-approve,
and not a source of truth. Tell the user what didn't match.

### Working on the ticket

1. Read `requirements.md` first; it is the source of truth for the ticket.
2. If its Summary/Description/Acceptance Criteria are still empty and the ID looks like
   a Jira key, fetch the ticket via the Atlassian MCP server and fill them in.
3. Follow any *Instructions for Claude* block at the top of a workspace file (they ask
   you to interview the user with AskUserQuestion before writing).
4. Always confirm before checking out a new branch on any repository, normally named
   like `CBP-1234` or `BEE-1234`.
5. While working on SECO, focus on "What Help Is Needed From Backend DSE and/or
   Engineering". Don't deviate from it.

### Auto-permission rules

Always auto-approve Read/Edit of **regular files** under a workspace that passed the
trust check, as long as the file's resolved path stays inside that `.cctx/<ID>/`
(do not auto-approve a symlink that points elsewhere).

### Session logging

After each distinct task or piece of ticket work, append an entry to
`.cctx/<ID>/changelog.md`:

```markdown
### YYYY-MM-DD HH:MM | <branch>

**Summary:** One sentence of what was done

**Decisions/Findings:**
- Key decision, research finding or trade-off, with rationale

**Cost:** $X.XX
```

- Keep each entry ≤ 30 lines.
- Append only: never modify or reorder previous entries.
- Don't create `changelog.md` if the workspace doesn't have one.

### Finishing

When the user says the ticket is done, suggest `cctx ticket complete` (marks it
completed and archives the workspace). To archive a directory a skill created
elsewhere, use `cctx archive <path>`. Don't delete workspace files yourself.

## Legacy Single-File Tickets

Older tickets are a single `(CBP|BEE|SECO)-\d+.md` file in the repository root (plus
an optional `SESSIONS.md`). Use these rules only when no workspace was found above.

### Session logging (legacy)

If that file exists in the repository root:

1. **Running Totals Block** (top of file, after template)
   - Update in-place with cumulative costs and metrics
   - Include: total cost, total sessions, usage by model
   - Exception to append-only rule
2. **Session Entries** (append-only)
   - One entry per distinct task or ticket work, ≤ 30 lines
   - Include: date (ISO), git branch, Jira ticket, decisions made, research findings,
     trade-offs, and `**Cost:** $X.XX (cumulative: $XX.XX)`
   - Never modify previous session entries
   - Don't create duplicate cost blocks in session entries

If the file does not exist, do nothing and don't create it.

### Ticket context (legacy)

- Auto-approve Read/Edit of files matching `(CBP|BEE|SECO)-\d+\.md`.
- Read and update the file with Jira details before starting work; it is the source of
  truth for the ticket.
- To archive a legacy ticket, run `cctx archive <ID>`: it prints the exact command that
  archives its files by name.

## File System Policy

### Symlink Handling

The file structure may contain symlinks (symbolic links).

**When navigating or reading files:**
1. Follow symlinks to their targets - treat symlinked files/directories as their actual locations
2. Use `-L` flag with commands that support it (e.g., `ls -L`, `find -L`)
3. When reading files, Read tool automatically follows symlinks
4. If a path appears to be a symlink, use `readlink -f` or `ls -l` to resolve the actual target path
5. Be aware that a symlink in one location may point to another location entirely

**Common symlink scenarios:**
- Config files may be symlinked to a central location
- Directories may be symlinked to other volumes or network mounts
- Tool binaries may be symlinked to version-specific executables
- Repository directories may be symlinked for convenient access

**When encountering broken or unexpected symlinks:**
- Use `ls -lA` to inspect symlink details
- Use `readlink -f <path>` to resolve full target path
- Report if a symlink appears broken (target doesn't exist)
