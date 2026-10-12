# SDD Ticket Workspace & Unified Archive — Specification

## Capability Map

| Module id         | Responsibility | Depends on |
|-------------------|----------------|------------|
| archive-engine    | Safe copy → verify → delete of a file or directory tree into `~/.cctx/contexts/_archived/`, with refusal rules and an archive record in config | — |
| archive-verb      | The single `cctx archive` command: resolves ticket ID / path / glob, ticket mode for directory tickets, `--all-active`; `ticket complete` routed through it; old archive commands removed | archive-engine |
| archive-list      | `cctx archive --list [--all] [--limit N]` plus backfill of records for pre-existing archives | archive-engine |
| ticket-workspace  | `cctx ticket create` scaffolds `./.cctx/<ID>/`; restore, delete, show, link/unlink, verify, cleanup become directory-aware | archive-engine |
| init-specs        | `cctx init-specs` scaffolds the project constitution in `./.cctx/` | ticket-workspace (shared guards) |
| cctx-skill        | Update the out-of-repo Claude skill `~/.claude/skills/cctx/SKILL.md` to the new layout | ticket-workspace |

Build order: archive-engine → archive-verb → archive-list → ticket-workspace → init-specs → cctx-skill

## Objective

`cctx` is a Go CLI that manages per-ticket context files for Claude Code. Today a ticket
is one Markdown file (`CBP-123.md`) in the project directory. AI-assisted work has moved
to spec-driven development (SDD), where a skill — the user's own, or none — produces a
*directory* of documents (requirements, plan, validation, changelog…). These files must
never be committed to org repositories, and once the work is done they must be cleaned
up reliably. This change makes `cctx` primarily a **robust archive/cleanup tool for
directory-based SDD work**, keeps a quick-start scaffold for people without a skill, and
collapses today's three archive commands into one.

## Terminology

- **Data dir** — `~/.cctx/` (overridable by `--data-dir` / `CCTX_DATA_DIR`). Holds
  `config.json`, `contexts/_tickets/`, `contexts/_archived/`, `templates/`.
- **Project workspace** — a directory named `.cctx/` inside a project (created in the
  current working directory). Holds SDD files. Not the data dir.
- **Directory ticket** — a ticket created by this version; its files live in
  `<project>/.cctx/<ID>/`. Identified in config by a non-empty `workspace_path`.
- **Legacy ticket** — a ticket created before this change (single `<ID>.md`, optional
  `SESSIONS.md`, companions like `PLAN-<ID>.md`, symlinks in `_tickets/` and secondary
  projects). Identified by an empty `workspace_path`. **Ticket-mode archive and `ticket
  complete` do not support legacy tickets** (user decision); their files are archived
  by glob/path and the record dropped with `ticket delete`.
- **Directory ticket = its workspace only.** Files outside `.cctx/<ID>/` (e.g. a
  `PLAN-CBP-1.md` in the project root) are never part of a directory ticket; archive them
  by name or glob if needed (user decision: no "companion" handling).
- **Archive record** — an entry in the new `archives` list in `config.json` describing
  one directory under `contexts/_archived/`.
- **SDD (spec-driven development)** — writing requirements/plan/validation documents
  with an AI assistant *before* implementing, then implementing against them.
- **SDD skill** — a Claude Code skill (prompt package) that writes spec documents. `cctx`
  never invokes skills; it only creates and archives files.
- **Constitution** — the three project-wide SDD docs `mission.md`, `tech-stack.md`,
  `roadmap.md` (one set per project, shared by all tickets).
- **AskUserQuestion** — a Claude Code built-in tool that asks the user multiple-choice
  questions. **Atlassian MCP server** — a Claude Code integration that reads Jira issues.
  Both are referenced only in template *text* (instructions to Claude); `cctx` code never
  calls them.
- **Managed project** — an entry in `config.managed_projects` (`context_name` +
  `project_path`). `GetProjectContext()` resolves the current one from `-p`,
  `CCTX_PROJECT`, or cwd.
- **Primary project** — `ticket.primary_context_name`: the managed project holding a
  legacy ticket's concrete files.
- **Working tickets** — `config.current_working_tickets`: tickets the user is currently
  working on; `ticket create` warns about them; completing/archiving removes the entry.
- **`cfgMgr.GetRepoRoot()`** — returns the data dir (historical name). `archived_path`
  and `ArchiveRecord.ArchiveDir` are relative to it.
- **Ticket ID prefixes** `CBP-`, `BEE-`, `SECO-` are the user's Jira projects; nothing in
  `cctx` code depends on them except the out-of-repo skill (§7).

## Scope & Non-Goals

**In scope**
1. One archive engine with explicit safety rules, used by every command that moves files
   into `_archived/`.
2. `cctx archive <ticket-id|path|glob>` as the only archive verb; `--all-active`;
   `--list`.
3. Recording every archive in `config.json` so archives can be listed per repo.
4. `cctx ticket create` scaffolding `./.cctx/<ID>/` with four templated files.
5. `cctx init-specs` scaffolding `./.cctx/{mission,tech-stack,roadmap}.md`.
6. Directory-aware `ticket restore`, `delete`, `show`, `link`/`unlink`, `admin verify`,
   `admin cleanup`, `admin reset`, and `ticket complete`.
7. Removing `cctx ticket archive` and `cctx ticket archive-all` (no aliases).
8. Updating the `cctx` Claude skill file (outside this repo).

**Non-goals**
- `cctx` running, installing, or configuring skills.
- Configurable or named template *sets* (the existing single-file override
  `~/.cctx/templates/<name>.md` is the only customisation).
- Restoring directories archived by path or glob (no ticket record → no CLI restore).
- Migrating existing legacy tickets or archives to the directory layout.
- Ticket-mode archiving / `ticket complete` of legacy tickets (they get an error with a
  how-to). Legacy `restore`, `delete`, `link`, `unlink`, `show` keep today's behaviour.
- Ever adding files into an existing archive dir (user decision: every archive operation
  creates a new dir).
- Cross-project linking of directory tickets (no symlinks are created for them at all).
- Handling a `backlog/` directory (skills own it; it can be archived by path like any dir).
- Editing any `.gitignore` file.

## Current Situation

All paths relative to repo root.

- **Ticket creation** — `runTicketCreate` in `cli/cmd/ticket.go` (≈line 122). Ticket ID
  comes from the argument or the git branch (not main/master), with collision handling
  (link to existing / project-scoped `<project>-<ID>` / numeric suffix / cancel). It
  writes `<cwd>/<ID>.md` from the embedded `ticket` template
  (`cli/internal/templates/ticket.md`, `{{TICKET_ID}}` substituted, `# Ticket: <ID>`
  header), symlinks `~/.cctx/contexts/_tickets/<ID>/<ID>.md` → that file, appends the
  ticket to `config.Tickets.Active`, and adds it to `current_working_tickets`.
- **Templates** — `cli/internal/templates/embed.go` embeds `*.md` (flat, no subdirs).
  `GetTemplate(name, dataDir)` returns `~/.cctx/templates/<name>.md` if present, else the
  embedded file. `cctx admin global templates reset|sync` copies embedded → user dir.
- **File recognition** — `cli/cmd/ticketfiles.go`: `IsTicketFileName` (whole-word,
  case-insensitive, extension-agnostic), `FindTicketFiles(dir, id)` (ticket file first,
  then companions; **skips directories**; `SESSIONS.md` never matches since it lacks the
  ID), `FindExtraTicketFiles` (companions only; explicitly excludes `SESSIONS.md`). `ticketID + ".md"` (spacing varies) is hard-coded at ~30 other call sites.
- **Archiving today — three code paths:**
  1. `cctx ticket complete [<ID>]` (`runTicketComplete`, ≈line 1283): marks completed,
     records branch/commit, removes symlinks from linked projects, copies `<ID>.md`,
     `SESSIONS.md` and user-selected companions (interactive `promptExtraFiles`) to
     `contexts/_archived/<YYYY-MM-DD>_<ID>[-n]/`, deletes originals, moves the ticket to
     `Tickets.Archived` with `archived_path`, removes it from working tickets.
  2. `cctx ticket archive [<ID>]` (`runTicketArchive`, ≈line 1764): refuses active
     tickets; otherwise similar copy/delete using `FindTicketFiles`; if the ticket is
     already archived it moves leftover cwd files into the existing archive; if the ID is
     unknown but files exist in cwd it delegates to `runArchive`.
  3. `cctx archive <pattern...>` (`cli/cmd/archive.go`): glob-matches **files only** in
     cwd (bare ticket IDs fall back to `FindTicketFiles`), lists them, prompts
     `[Y/n/select]`, copies each to `resolveArchiveDir` (existing ticket archive if the
     name matches an archived ticket, else `_archived/<date>_<cwd-basename>/`), deletes
     the source. No config record is written.
  - `cctx ticket archive-all` (`runTicketArchiveAll`, ≈line 2519): marks **all active**
    tickets completed and archives them; optional `-p <project>` filter; `-f` skips
    confirmation.
  - All paths delete the source after `common.CopyFile` without verifying content.
- **Restore** — `runTicketRestore` (≈line 2014): by ID only; copies
  `FindTicketFiles(archive)` + `SESSIONS.md` into cwd, refuses to overwrite, recreates
  `_tickets/` symlinks, sets the ticket active in cwd's project.
- **Config** — `cli/internal/config/config.go`. `Ticket` has `archived_path` (relative to
  data dir, e.g. `contexts/_archived/2026-03-10_CBP-34426`), `linked_projects`,
  `primary_context_name`. There is no record of non-ticket archives and no field for a
  directory workspace.
- **Other ticket-aware commands** — `ticket delete`, `ticket show`, `ticket link/unlink`,
  `admin verify` (`cli/cmd/verify.go`, checks `_tickets/<ID>/<ID>.md` symlinks for active
  tickets), `admin cleanup` (`cli/cmd/cleanup.go`, scans `_tickets/` and project symlinks).
- **Shell completion** — `cli/cmd/completions.go` assigns `ValidArgsFunction` to
  `ticketArchiveCmd` (must be removed with the command).
- **Help/docs** — `cli/cmd/root.go` long help, `README.md`, `CLAUDE.md`, and the
  `ticket abandon` success hint all mention `cctx ticket archive`.
- **Git ignoring** — not done by the tool. The user's `~/.gitignore_global` contains
  `*[Cc][Bb][Pp]-[0-9]*.md` etc., which do not match directories.
- **Other files touching tickets** — `cli/cmd/unlink.go` (`admin unlink` primary-project
  migration, iterates `Tickets.Active` by `primary_context_name`), `cli/cmd/init.go`
  (≈line 342, iterates `Tickets.Active` to refresh symlinks), `cli/cmd/reset.go`
  (iterates tickets' `linked_projects`; wipes `_archived/`), `cli/cmd/list.go`.
- **Existing data** — 1 active and 26 archived legacy tickets in config, most with empty
  `linked_projects` and `primary_context_name`; 29 dirs plus some loose files under
  `contexts/_archived/`.

## Proposed Solution

### 1. Config schema (`cli/internal/config/config.go`)

Add to `Ticket`:

```go
WorkspacePath string `json:"workspace_path,omitempty"` // canon path of <project>/.cctx/<ID>; empty = legacy ticket
```

Add to `Config`:

```go
Archives []ArchiveRecord `json:"archives,omitempty"`

type ArchiveRecord struct {
    ArchiveDir string    `json:"archive_dir"`          // relative to data dir, e.g. contexts/_archived/2026-10-12_CBP-123
    SourcePath string    `json:"source_path"`          // canon absolute path archived from ("" = unknown)
    TicketID   string    `json:"ticket_id,omitempty"`  // ticket the archive belongs to, if any
    ArchivedAt time.Time `json:"archived_at"`
    FileCount  int       `json:"file_count"`           // regular files + symlinks under ArchiveDir
    Kind       string    `json:"kind"`                 // "ticket" (ticket mode), "path", "glob", or "backfill"
}
```

Both fields are optional, so old configs load unchanged. `WorkspacePath` is retained
after archiving (restore uses it to know a ticket is a directory ticket). Because every
archive operation creates a new dir (§2), each record describes exactly one dir. Records
are added by archiving and backfill, changed or removed by restore and delete (§5), and
cleared by `admin reset`. Only `Kind == "ticket"` records are *owned* by their ticket
(`TicketID` on other kinds is an informational guess).
Directory tickets always have **empty** `linked_projects` and `primary_context_name`.

### 2. archive-engine (new file `cli/cmd/archiveengine.go`)

**Path canonicalisation.** `canon(p)` = `filepath.Abs` then `filepath.EvalSymlinks`; for
a path that does not exist, resolve its deepest existing ancestor and re-append the rest.
Every path comparison in this spec compares `canon` of **both** sides on
path-component boundaries (on macOS `t.TempDir()`'s `/var/...` is `/private/var/...`).

**Ticket ID validation.** `validTicketID(id)`: matches `^[A-Za-z0-9][A-Za-z0-9._-]*$`,
does not contain `..`, and does not end in `.md` (which would be read as a path). Enforced where a new path is built from an ID: `ticket
create` (explicit, or branch-derived — a branch-derived ID uses the branch's last path
segment, `feature/CBP-1` → `CBP-1`, the same rule as `resolveTicketID` and the skill)
and `ticket restore`. Not enforced on lookup/delete, so an
existing odd ID can always be deleted.

**Entry point**

```go
// Src is a file, symlink or directory. A directory item copies the directory's
// contents under RelDest ("." = archive root) and deletes the directory itself afterwards.
type archiveItem struct{ Src, RelDest string }
type archiveOpts struct {
    Name        string               // archive dir base name before the date prefix, e.g. "CBP-1"
    Record      ArchiveRecord        // TicketID/SourcePath pre-filled by the caller
    OwnTicketID string               // ticket mode: exempts this ticket's own workspace from refusal 9
    Mutate      func(cfg *config.Config, archiveRel string) error // extra config change in the same Save, given the new archive dir (may be nil); error → rollback as for a Save failure
    Decorate    func(stagingDir string) error // writes into staging only, after verification (may be nil)
}
// RelDest rules: a directory item's contents go to "." (archive root) unless a caller
// says otherwise; a file item goes to its base name at the archive root.
func archiveItems(cfgMgr *config.Manager, items []archiveItem, opts archiveOpts) (archiveDir string, err error)
```

The engine **loads config itself** at step 5 (fresh `cfgMgr.Load()`), applies the record
plus `Mutate`, and saves; callers never hold a mutated in-memory config across engine
calls. Every archive-producing command goes through it.

**Archive dir**: `Name` must pass `validTicketID`'s character rules; if not (e.g. a
legacy ID containing `/`), the engine uses the cwd base name instead. Always **new** —
`contexts/_archived/<YYYY-MM-DD>_<Name>`, with `-1`,
`-2`, … appended until the name is free. The name is *reserved* with `os.Mkdir` (which
fails if it exists → try the next suffix). Nothing is ever written into an archive dir
that existed before this call.

**Pre-flight refusals** (all evaluated for every item before anything is written; any
failure aborts the whole operation with a message naming the offending path; `--force`
never bypasses them):

1. Source does not exist (Lstat) → refuse.
2. `canon(Src)` equals or is an ancestor of `/`, `$HOME`, the active data dir, the
   default `~/.cctx`, cwd, or the git top-level of cwd (if in a repo) → refuse. Consequence: `cctx archive CBP-1` from
   inside `.cctx/CBP-1/` is refused with "run this from outside <path>".
3. Source is inside the active data dir or the default `~/.cctx` → refuse.
4. Source is named `.cctx`, or a directory item contains a `.cctx` entry anywhere in its
   tree → refuse. Children of a workspace (`.cctx/CBP-1/`, `.cctx/mission.md`) are fine.
5. Path mode: the explicit argument is a symlink → refuse ("pass the real path"). Glob
   matches that are symlinks are **skipped with a warning**
   (behaviour change: today `matchArchiveFiles` returns them and `CopyFile` archives the
   target's content). Symlinks *inside* a directory item are copied as symlinks.
6. Git-tracked content. All `git` calls run as `git -c core.fsmonitor=false -C <dir> …`.
   If `git` is on PATH: `rev-parse --is-inside-work-tree` in the item's dir (its parent
   for files) decides; outside a work tree → skipped; inside →
   `git --literal-pathspecs … ls-files -z -- <Src>`; non-empty → refuse, listing up to 10
   tracked paths. One `rev-parse` + one `ls-files` per distinct parent dir. If `git` is
   not on PATH and an ancestor of Src has a `.git` entry → refuse ("cannot verify
   git-tracked files: git not found"); else skipped.
   **Non-git directories**: a source that is not inside any git work tree (no `.git`
   ancestor) skips this check entirely and is archived normally — in every mode,
   including ticket mode for a workspace created in a non-git directory.
7. A directory item containing a `.git` entry anywhere in its tree → refuse.
8. Two items' **fully expanded** destination paths (every file under a directory item,
   joined with its `RelDest`) collide, compared case-insensitively (APFS default) →
   refuse.
9a. *(added in implementation)* Any path component of `canon(Src)` equal to `.git`
   → refuse ("part of a git repository's internals"). Refusal 6 is fail-closed:
   if `rev-parse` *errors* (rather than answering "false") and Src has a `.git`
   ancestor, refuse. All git calls drop `GIT_DIR`, `GIT_WORK_TREE`,
   `GIT_INDEX_FILE` and `GIT_COMMON_DIR` from the environment. On macOS, path
   comparisons (`isWithin`) are case-insensitive, matching APFS.
9. Path and glob mode: `canon(Src)` equals, contains, **or is inside** the
   `WorkspacePath` of a ticket in `Tickets.Active` → refuse with "this is part of ticket
   <ID>'s workspace; run `cctx archive <ID>`" (so `.cctx/CBP-1/plan.md` or
   `".cctx/CBP-1/*.md"` can't half-archive it). Archived tickets' stale `WorkspacePath`
   values are ignored. Ticket mode is exempt for its own ticket.

**Execution** (after all refusals pass and the caller's confirmation):

1. Create staging dir `contexts/_archived/.staging-<random hex>` (dot-prefixed, so it
   can never clash with an archive name, since IDs start alphanumeric). A deferred
   cleanup removes it on any error.
2. Copy every item into staging with a new Lstat-based `copyEntry` (not
   `common.CopyFile`, which follows symlinks): regular files with mode preserved;
   symlinks recreated with the same target string, never followed; directories
   recreated; any other file type → abort. Record each source regular file's size and
   mtime when it is read.
3. Verify every copied regular file's size and SHA-256 against the source, and every
   symlink's target string. Mismatch → abort; **sources untouched**. Then run
   `opts.Decorate(staging)` if set (writes into staging only).
4. Reserve the final archive dir with `os.Mkdir` (see above), then `os.Rename` each
   top-level entry of staging into it and remove the now-empty staging dir. Any rename
   failure → `os.RemoveAll` the final dir and staging, sources untouched, return error.
5. Reload config, append the record (`ArchiveDir`, `ArchivedAt` = now, `FileCount` from
   the archive, `Kind` from the caller), apply `opts.Mutate`, `saveConfigFn`. If
   `Mutate` returns an error (e.g. the ticket is no longer in the reloaded config) or the
   save fails: `os.RemoveAll` the final
   archive dir, leave sources untouched, return the error — a complete no-op.
6. Delete sources: files and symlinks first, then now-empty dirs bottom-up, including a
   directory item itself (e.g. `.cctx/CBP-1/`; the parent `.cctx/` stays). Immediately
   before deleting each regular file, Lstat it again; if size or mtime differ from step
   2, **keep it** and warn "changed during archive; kept <path> (archive holds the
   earlier version)". (Known gap: an edit that keeps both size and mtime identical is not
   detected.) Deletion problems never roll back: they are listed as warnings
   ("left in place: …") and the call still succeeds.

**Test seams**: `var copyEntryFn = copyEntry` (inject failing/corrupting copy) and
`var beforeDeleteHook func()` (simulate a file edited between steps 3 and 6), and
`var saveConfigFn = func(m *config.Manager, c *config.Config) error { return m.Save(c) }`
(inject a `Save` failure — file permissions are not reliable for this).

**Dry run**: refusals are evaluated and reported; no prompts; nothing created, copied,
deleted or saved; each planned move (and in ticket mode "would mark <ID> completed") is
printed with `dryRunMsg`.

**Dates**: `YYYY-MM-DD` values use local time (as today's `time.Now().Format("2006-01-02")`).

**Concurrency**: no locking (single-user tool); two simultaneous runs can still
overwrite each other's config, as today. The `os.Mkdir` step prevents two runs sharing
an archive dir.

### 3. archive-verb — `cctx archive` (`cli/cmd/archive.go`)

```
cctx archive [<ticket-id> | <path|glob>...] [--all-active] [--list [--all] [--limit N]]
             [--commits ...] [--prs ...] [--force|-f] [--dry-run]
```

**Argument resolution**

- No positional args and no mode flag → ID from the shared resolver
  `resolveTicketID(cfg)` (new, in `root.go`): `GetTicketID()` (`-t`, `CCTX_TICKET`, or
  non-main branch); if that ID is not in config and came from a branch containing `/`,
  retry with the last path segment (`feature/CBP-1` → `CBP-1`, matching the skill, §7).
  Used by every ticket verb that auto-detects its ID (`archive`, `ticket complete`,
  `ticket show`, `ticket restore`). No ticket found → error "no ticket ID detected; pass
  a ticket ID, path or glob".
- Exactly one arg equal to a `TicketID` in `Tickets.Active` or `Tickets.Archived`
  (exact, case-sensitive, no suffix stripping) → **ticket mode**. So `cctx archive
  CBP-1.md` is path mode on that file, and a file or directory in cwd named exactly like
  a ticket ID is archived by path with `./CBP-1`.
- Otherwise each arg independently: an existing path (file or dir, absolute or relative)
  → **path mode**; anything else → **glob mode** (today's `matchArchiveFiles`, files
  only, including its bare-ticket-ID fallback via `FindTicketFiles`).
- Errors before anything runs: a ticket ID mixed with other args ("archive one ticket at
  a time"); `--list` with positional args, `--all-active`, `--commits` or `--prs`;
  `--all`/`--limit` without `--list`; negative `--limit`; `--all-active` with positional
  args; `--commits`/`--prs` outside ticket mode or with `--all-active`; `-t` together
  with positional args, `--list` or `--all-active`.

**ID-collision filter** (used by glob mode's bare-ticket-ID fallback, the glob grouping
below, and the legacy how-to file list):
`IsTicketFileName` treats `-` as a word boundary, so ID `CBP-7` also matches
`CBP-7-x.md`, which belongs to ticket `CBP-7-x` if that ID exists. Drop any matched file
that also matches a *different, longer* ticket ID in config (active or archived)
containing the requested ID, warning "skipped <file>: belongs to <other ID>".

**Path & glob mode** (any mix): all resolved items are shown in one numbered list
(directories with a trailing `/` and file count), one prompt `Archive all?
[Y/n/select]` (today's `parseSelection` syntax: comma lists and ranges like `1,3-5`;
`--force` = all). Grouping into engine calls, each producing one new archive dir:

| Selected items | Engine `Name` | Record |
|----------------|---------------|--------|
| each directory item, separately (`RelDest` = `.`: contents at the archive root) | the directory's base name | `Kind` = `path`; `SourcePath` = the dir; `TicketID` empty |
| file items whose name matches a ticket ID in config (longest matching ID wins), grouped per ID (`RelDest` = base name) | that ticket ID | `Kind` = `glob` (or `path` for an explicit file arg); `SourcePath` = parent dir of the first file; `TicketID` = that ID (informational) |
| all other file items, grouped per parent dir (`RelDest` = base name) | base name of cwd (as today) | `Kind` as above; `SourcePath` = that parent dir |

"Name matches a ticket ID" = `IsTicketFileName(name, id)` after the ID-collision filter.
Special case: a `SESSIONS.md` given in the same invocation as files that all belong to
exactly one ticket group joins that group (so the legacy how-to command yields one
archive dir). `resolveArchiveDir` (routing into existing dirs) is deleted.

**Ticket mode** — `archiveTicket(cfgMgr, id, opts)`, shared by `cctx archive <ID>`,
`ticket complete` and `--all-active`:

1. **Legacy ticket** (`WorkspacePath == ""`) → error, nothing changes. If it is
   already archived (`archived_path` set), the message is only "<id> is already archived
   at <path>; archive any leftover files by name with `cctx archive <file>...`" — it
   never suggests `ticket delete` (which would remove that archive). Otherwise the message
   lists the ticket's files found in cwd — ID-collision-filtered `FindTicketFiles(cwd, id)`
   plus `SESSIONS.md` if it is a regular file — and prints a ready-to-run command naming
   them explicitly, e.g. "<id> is a legacy single-file ticket. Archive its files with:
   `cctx archive CBP-1.md PLAN-CBP-1.md SESSIONS.md` then drop the record with `cctx
   ticket delete CBP-1`". If none are found in cwd it says to run it from the directory
   holding them. (Explicit names avoid a glob like `CBP-1*` sweeping in `CBP-10.md`.)
   Order: step 1, then the *already archived* branch of step 3 (which needs no files and
   may exit 0), then step 2's item collection and its "no files found" error, then the
   remaining status rules.
2. **Items**: exactly one directory item `{WorkspacePath, "."}`. Nothing outside the
   workspace is collected. If `WorkspacePath` does not exist → error "no files found for
   <id> (expected <WorkspacePath>). If
   the project moved, archive the workspace by path from its new location; otherwise
   drop the record with `cctx ticket delete <id>`". (Refusal 9 does not block that path
   archive because the stale `WorkspacePath` no longer matches.)
3. **Status** (a ticket in `Tickets.Active` is `active` or `abandoned` — abandon does not
   archive; completed tickets are normally already archived). The item list is always
   printed first. Then:
   - `active` → prompt `<id> is active — mark completed and archive? [y/N]` (`--force`
     and `ticket complete` skip it). Completion = status `completed`, `completed_at` =
     now, commits = `--commits` if given else `git -C <source dir> rev-parse --short
     HEAD`, `Branch: <branch>` from `git -C <source dir>` appended to `notes`, PRs from
     `--prs`; *source dir* = the parent of `.cctx/`; not a git repo → left empty.
     Extracted from the top of `runTicketComplete` into a helper.
   - `abandoned`, or `completed` but still in `Active` (e.g. from an interrupted older
     version) → prompt `Proceed with archive? [y/N]` (`--force` skips); no completion
     is (re)applied, and passing `--commits`/`--prs` errors ("only valid when
     completing an active ticket").
   - Already archived (`archived_path` set) → if the workspace still exists
     (*leftovers*, e.g. files kept by step 6 of the engine), list them, prompt `Proceed?
     [Y/n]` (`--force` skips) and archive them into a new dir via the engine with
     `Name` = id, `TicketID` = id, `Kind` = `ticket`, no status change and `archived_path` unchanged; with
     no leftovers print "already archived at <path>" and exit 0.
4. **Engine call** (`Name` = id, `TicketID` = id, `Kind` = `ticket`, `SourcePath` =
   `WorkspacePath`):
   - `Decorate` (only if completion was applied): append to the staged `changelog.md`
     if it exists: `### YYYY-MM-DD HH:MM | <branch>` (`-` when no branch) / `**Summary:** Ticket completed and
     archived` / commits and PRs as bullets.
   - `Mutate`: find the ticket in the freshly loaded config by ID; apply completion if
     approved; `archived_path` = the actual archive dir created; move `Active` →
     `Archived`; remove from `current_working_tickets`.
   - If the engine fails, nothing is saved; the ticket stays exactly as it was.

**`ticket complete` (`runTicketComplete`)** keeps its CLI (`--commits`, `--prs`) and
its errors: unknown or archived ID → "ticket not found"; already completed → "ticket is
already completed"; as today both first drop a stale `current_working_tickets` entry. For
a directory ticket it calls `archiveTicket` with completion pre-approved (an `abandoned`
ticket is completed and archived, as today). For a legacy ticket it returns the step-1
error. Its old copy/delete code and `promptExtraFiles` (if then unused) are deleted.

**`--all-active`**: all tickets in `Tickets.Active` — `active` ones are completed and
archived; `abandoned` (and completed-but-active) ones are archived without completion
(today's `archive-all` also took every ticket in `Active`). Optional project filter only when
`-p` or `CCTX_PROJECT` is set (as in today's `runTicketArchiveAll`): the name is resolved
with `GetProjectContext()` and must exist in `managed_projects`; a directory ticket
matches if `WorkspacePath` is under that project's `project_path`. Legacy tickets that
are in the selection (with `-p`: those whose `linked_projects` contain the project) are
listed as "skipped (legacy)". List, one confirmation (`--force` skips), then
`archiveTicket` each with prompts suppressed and completion approved for `active` ones.
Each call reloads config (engine step 5), so one failure never leaks into the next. A
failure is reported and the rest continue; exit non-zero if any failed; summary line
`Archived X of Y tickets (Z legacy skipped)`.

**Flags on `archiveCmd`**: `--force`/`-f` (own variable `archiveForce`), `--all-active`,
`--list`, `--all`, `--limit`, `--commits`, `--prs` (comma-separated strings, parsed with
`splitAndTrim` exactly as on `ticket complete`). `--force` skips confirmations only.

**Removed**: `ticketArchiveCmd`, `runTicketArchive`, `ticketArchiveAllCmd`,
`runTicketArchiveAll`, `resolveArchiveDir`, and their registration and help text; `cctx
ticket archive …` becomes cobra's unknown-command error. Wiring: `archiveCmd.Args` →
`cobra.ArbitraryArgs`; remove the `ticketArchiveCmd.ValidArgsFunction` line in
`completions.go`; give `archiveCmd` a `ValidArgsFunction` offering active+archived
ticket IDs with `cobra.ShellCompDirectiveDefault` (file completion fallback).

### 4. archive-list — `cctx archive --list`

- **Scope**: default = records whose `SourcePath` equals or is under the *scope root*
  (git top-level of cwd if in a repo, else cwd; both `canon`). `--all` = every record,
  including `SourcePath == ""`.
- **Order & limit**: newest `ArchivedAt` first; `--limit N` (default 20, `0` = all).
- **Columns**: date, archive dir base name, ticket ID or `-`, file count, source path or
  `unknown`; `(missing)` if the archive dir no longer exists.
- **Backfill** (before listing; skipped in dry-run; idempotent; only dirs without a
  record are walked): for every directory under `contexts/_archived/` without a record (ignoring non-directories and names starting
  with `.`), add one: `TicketID` = the ticket whose `archived_path` points at it; else,
  with `R` = the name minus its `YYYY-MM-DD_` prefix, a ticket whose ID equals `R`
  exactly; else, only if `R` ends in `-<digits>`, a ticket whose ID equals `R` with that
  suffix stripped (legacy `archived_path` values omit `-n`, so siblings match by name;
  exact matches win so ID `CBP-123-2` is never mistaken for `CBP-123`); else empty.
  `SourcePath` = that ticket's primary project path (`primary_context_name` →
  `managed_projects[].project_path`), else its first `linked_projects[].project_path`;
  if still empty, strip the date prefix and any `-<n>` suffix and, if the remainder
  equals a managed project's `context_name` or the base name of its `project_path`, use
  that path; else `""`. `ArchivedAt` = date from the prefix (else dir mtime);
  `FileCount` = walk count; `Kind` = `backfill`. Save if anything was added.
- **Known limitation**: most existing ticket archives have no recorded project, so they
  get `SourcePath = ""` and appear only under `--all`.
- Empty result → `infoMsg("No archives for <scope root>")` plus a hint to use `--all`.

### 5. ticket-workspace

**Shared guards** (`ensureWorkspaceDir(cwd)`, used by `ticket create`, `init-specs`, and
directory-ticket `restore`; all run before anything is created):

1. `canon(<cwd>/.cctx)` equals or is inside the active data dir **or** the default
   `~/.cctx` (they differ when `--data-dir`/`CCTX_DATA_DIR` is set) → error.
1b. `<cwd>/.cctx` exists and is a symlink or not a directory → error "`.cctx` must be a
   real directory"
   (keeps `WorkspacePath`'s parent the project dir for `-p`, `--list`, delete).
2. `<cwd>/.git` does not exist (file or dir) → `warningMsg("Current directory is not a
   git repository root; .cctx/ will be created in <cwd>")`, continue. (User decision:
   this fires in monorepo subdirectories too, deliberately.)
3. If cwd is inside a git work tree: `git check-ignore -q .cctx/` (works before the dir
   exists): exit 1 → `warningMsg(".cctx/ is not git-ignored. Add it globally: echo
   '.cctx/' >> ~/.gitignore_global")`; exit 0 → nothing; other exit or `git` missing →
   skip silently. Never modify any gitignore.

**`cctx ticket create [<ID>]`**

- ID detection and collision handling as today, **minus** option 1 "Link to existing
  ticket". Whatever kind the existing ticket is, the remaining options are offered:
  project-scoped `<project>-<ID>` (only when a managed project is detected and the ID is
  free, as today), numeric suffix, cancel. If the existing ticket is archived, an extra
  info line suggests `cctx ticket restore <ID>`. The final ID must pass `validTicketID`.
- `<cwd>/.cctx/<ID>/` already exists → error "workspace already exists: <path>".
- Writes `<cwd>/.cctx/<ID>/requirements.md`, `plan.md`, `validation.md`,
  `changelog.md` from embedded templates `ticket-requirements`, `ticket-plan`,
  `ticket-validation`, `ticket-changelog` via `GetTemplate` (so
  `~/.cctx/templates/ticket-plan.md` etc. override), substituting `{{TICKET_ID}}`.
- Creates no `<ID>.md`, no `SESSIONS.md`, no `_tickets/<ID>/`, no symlinks.
- If writing any file or saving config fails, the just-created `.cctx/<ID>/` is removed
  (only if this run created it) so a retry is not blocked.
- Config: new active ticket, `WorkspacePath` = `canon(<cwd>/.cctx/<ID>)`, empty
  `linked_projects` and `primary_context_name`; added to working tickets (project name =
  detected managed project or base name of cwd); `--title`, `--tags`, `--notes`
  unchanged.
- "Next steps" output lists the workspace path and the four files.

**Templates** (new files in `cli/internal/templates/`; thin, instruction-first — each is
a `# <Title>: {{TICKET_ID}}` heading, an *Instructions for Claude* block, then empty
section headings):

- `ticket-requirements.md` — if `{{TICKET_ID}}` looks like a Jira key
  (`[A-Z][A-Z0-9]+-\d+`), fetch it from Jira via the Atlassian MCP server and fill
  *Summary*, *Description*, *Acceptance Criteria*; otherwise ask the user for them; then
  interview the user with the AskUserQuestion tool before writing *Scope*, *Decisions*,
  *Context*. Sections: Summary, Description, Acceptance Criteria, Scope, Decisions,
  Context.
- `ticket-plan.md` — read `requirements.md`; interview with AskUserQuestion before
  writing; numbered task groups. Sections: Task Groups.
- `ticket-validation.md` — read `requirements.md` and `plan.md`; interview with
  AskUserQuestion before writing; how to know it succeeded and can be merged. Sections:
  Checks, Merge Criteria.
- `ticket-changelog.md` — append-only; one entry per session or change, newest last,
  ≤30 lines, never edit earlier entries; entry format `### YYYY-MM-DD HH:MM | <branch>`
  with *Summary* and *Decisions/Findings* bullets.

The legacy `ticket.md` template stays (used only by legacy flows).

**`cctx ticket restore [<ID>]`**

- Directory ticket: the ticket must be in `Tickets.Archived`, else today's error
  "ticket <ID> is not archived (status: …)". `validTicketID`, shared guards, target =
  `<cwd>/.cctx/<ID>/`. Only `archived_path` is restored; if other `Kind == "ticket"`
  records exist for this ID (leftover archives, which may hold *newer* versions of files
  kept during an earlier archive), print a warning listing them so the user can copy
  them manually (known limitation). The
  archive dir (`archived_path`) must exist, else error "archived directory not found:
  <path>" (as today), and `canon` of it must be directly under `contexts/_archived/` of
  the data dir (same check as delete), else error "refusing suspicious archived_path"
  before anything is copied or removed. Creates `<cwd>/.cctx/` if missing. Refuse if the
  target exists. Steps:
  1. Copy the whole archive into staging `<cwd>/.cctx/.staging-<hex>/` with `copyEntry`;
     verify (engine step 3). Failure → remove staging, change nothing. (A crash leaves a
     git-ignored `.cctx/.staging-*` dir; `ticket create`/`restore` ignore such names.)
  2. Rename staging → `<cwd>/.cctx/<ID>/` (re-check with Lstat just before; if the target
     appeared meanwhile, remove staging and abort).
  3. Reload config and update the ticket: status `active`; clear `archived_path`,
     `completed_at`, `abandoned_at`; `WorkspacePath` = `canon(target)`; move `Archived`
     → `Active`; add to working tickets; remove the record whose `ArchiveDir` equals
     `archived_path`. `Save` fails →
     remove the restored target, return the error.
  4. `os.RemoveAll` the archive dir. If that fails, warn and re-save the record with an
     updated `FileCount` and `TicketID` cleared.
- Legacy ticket: today's behaviour unchanged (files into cwd, symlinks recreated);
  afterwards the record whose `ArchiveDir` equals `archived_path` (if any) is removed when the archive dir is gone, else its
  `FileCount` is updated and `TicketID` cleared.

**Other commands**

- `ticket link` / `unlink` on a directory ticket → error "directory tickets live in
  <WorkspacePath> and cannot be linked to other projects". Legacy unchanged.
- `ticket delete <ID>`, directory ticket: list everything that will be removed, confirm
  (`--force` skips); remove the workspace dir, the `archived_path` dir, every dir of a
  record with this `TicketID` **and** `Kind == "ticket"`, those records, the config entry,
  and the working-ticket entry. Records of other kinds that mention the ID are kept with
  `TicketID` cleared. Nothing outside the workspace is touched: today's legacy sweep (`<ID>.md` / `SESSIONS.md` in cwd
  and managed projects, `_tickets/<ID>/`) is skipped for directory tickets. Safety (never
  bypassed): the workspace is removed only if
  `canon(WorkspacePath)` has base name == ID, parent base name == `.cctx`, and passes
  engine refusals 2, 3, 4, 6 and 7;
  an archive dir only if `canon` resolves directly under `contexts/_archived/` of the
  data dir. If the workspace is blocked by refusal 2 (cwd inside it) or by "git not
  found" (refusal 6), the whole delete aborts with that message (the user can fix it);
  any other failing path is left alone with a warning and the config entry is still
  removed. Legacy ticket: today's behaviour, with two changes: (a) it removes
  `archived_path` (as today, after the same containment check) and that dir's record
  whatever its `Kind`, but **never** removes dirs of other `glob`/`path` records that
  carry the ID (e.g. created by the legacy how-to) — those are kept with `TicketID`
  cleared; (b) it removes `SESSIONS.md` in cwd/managed projects only if it is a symlink
  into `_tickets/<ID>/` — a regular `SESSIONS.md` is left in place with a warning
  (today it is deleted regardless of which ticket it belongs to).
- `ticket show <ID>`: for directory tickets print `Workspace: <canon WorkspacePath>` on
  its own line (the skill parses it, §7) and its files, or `(missing)`.
- `ticket abandon`: hint becomes `cctx archive <ID>`.
- `admin verify`: skip `_tickets/` symlink checks for directory tickets; report an active
  directory ticket whose workspace is missing; `--fix` only suggests `cctx ticket
  delete`.
- `admin cleanup`: must not flag directory tickets as orphaned; reports
  `contexts/_archived/.staging-*` dirs as orphans, removed by its existing
  `deleteOrphanedItems` after its confirmation; `--restore` mode ignores them.
- `admin reset`: clears `archives` whenever it wipes `_archived/`, and lists the
  `WorkspacePath` of every directory ticket it drops ("left on disk: …"); it never
  deletes project workspaces.
- Any remaining code that builds `<ID>.md` paths from a ticket or relies on
  `primary_context_name`/`linked_projects` — `ticket.go`, `verify.go`, `unlink.go`,
  `init.go` (≈line 342), `reset.go`, `list.go`; find them with
  `grep -nE 'ID *\+ *"\.md"|PrimaryContextName|LinkedProjects' cli/cmd/*.go` — must either
  skip directory tickets (`WorkspacePath != ""`) or be shown to do nothing for them
  because both fields are empty.
- `root.go` long help, `README.md` and `CLAUDE.md` (Command Tree, V2 Architecture, Data
  Directory Structure) updated to the new commands and layout.

### 6. init-specs — `cctx init-specs`

Top-level command (`rootCmd`) — agreed with the user as day-to-day work, an explicit
exception to the "small top-level listing" rule in `CLAUDE.md`. Runs the shared guards,
creates `./.cctx/` if needed, writes `mission.md`, `tech-stack.md`, `roadmap.md` from
embedded templates `specs-mission`, `specs-tech-stack`, `specs-roadmap`. Existing files
are skipped (`infoMsg("exists, skipped: <path>")`), never overwritten. No config change.
Dry-run supported. Templates are thin: an *Instructions for Claude* block telling Claude
to read the project's README/TODO for stakeholder input and interview the user with the
AskUserQuestion tool (mission & target audience / tech-stack gaps / phase ordering
respectively) before writing; `roadmap.md` asks for very small, ordered phases.

### 7. cctx-skill (outside repo)

Edit `~/.claude/skills/cctx/SKILL.md`. Today it has three policies: *Session Logging*
(if a `(CBP|BEE|SECO)-\d+` file exists in the repo root, append ≤30-line session entries
plus a running-totals block to it), *File System* (follow symlinks), and *Ticket Context*
(auto-approve edits to `(CBP|BEE|SECO)-\d+\.md`; read/update it with Jira details before
work; confirm a branch before changes). Changes:

- **Which workspace**: take the branch's last path segment (`feature/CBP-1` → `CBP-1`)
  and run `cctx ticket show <that>`; if it prints a `Workspace:` line whose dir exists,
  use it (works for workspaces in monorepo subdirectories). Else if `./.cctx/<that>/`
  exists use it; else if exactly one `(CBP|BEE|SECO)-\d+` dir exists under `./.cctx/` use
  it; several → ask the user; none → the legacy root-file rule.
- **Trust check** before using a workspace: `cctx ticket show <ID>` must succeed and its
  `Workspace:` line must equal the dir's resolved real path (`realpath .cctx/<ID>`); if
  in a git repo, `git ls-files .cctx/<ID>` must be empty. Otherwise treat it as ordinary
  repo content (no auto-approve, not a source of truth).
- Log sessions to `./.cctx/<ID>/changelog.md` in its entry format.
- Auto-approve read/edit of regular files under a trusted `.cctx/<ID>/` whose resolved
  path stays inside it.
- Read `./.cctx/<ID>/requirements.md` as the ticket's source of truth.

## Acceptance Criteria

All Go behaviour is verified by tests in `cli/cmd/*_test.go` using `t.TempDir()` for the
data dir and project dirs; `make check` passes.

**archive-engine**
1. Archiving a directory with nested files and an internal symlink produces a new archive
   dir with the identical relative tree, byte-identical files, the symlink recreated with
   the same target, and the source removed.
2. A failing or corrupting `copyEntryFn` leaves every source intact and no archive or
   `.staging-*` dir behind.
3. Each refusal 1–9 has a test: the command errors; filesystem and config are unchanged.
4. A directory with one git-tracked file (temp repo with a commit) is refused, naming the
   file; with all files untracked it is archived. A file named `a[1].md` is checked
   literally. A directory containing a nested `.git` is refused.
5. Two items whose expanded destinations differ only by case are refused.
6. `--dry-run` changes nothing on disk or in config and prints each planned move and any
   refusal.
7. Each successful archive creates exactly one new dir and one record with correct
   `ArchiveDir`, canon `SourcePath`, `TicketID`, `FileCount`; an archive dir that already
   exists for the same date/name is never written to (the new one gets `-1`).
8. A source file modified via `beforeDeleteHook` is kept with a warning; the archive holds
   the earlier version.
9. With `saveConfigFn` failing at engine step 5, the new archive dir is removed and
   every source still exists unchanged.

**archive-verb**
10. `cctx archive CBP-1` on an active directory ticket, answering `y`, ends with: ticket
    in `Archived`, `status=completed`, `archived_path` = the actual dir (with `-n` if
    applicable), `workspace_path` retained; `.cctx/CBP-1/` gone; the archive holds its
    files plus a completion entry at the end of `changelog.md`; a `PLAN-CBP-1.md` in the
    project root is untouched; not in working tickets. Answering `n` changes nothing.
11. With only `CBP-8-x` in config, `cctx archive CBP-8` (not in config → glob fallback)
    takes `CBP-8.md` but not `CBP-8-x.md`, and warns.
12. In a plain directory that is not a git repo (no `.git` anywhere above it), `ticket
    create`, `cctx archive <ID>`, `cctx archive <dir>` and `cctx archive "<glob>"` all
    succeed with no git-related refusal (the not-repo-root warning still shows on
    create).
13. `cctx archive <legacy-ID>` and `cctx ticket complete <legacy-ID>` error with the
    how-to message (listing `CBP-1.md`, `PLAN-CBP-1.md`, `SESSIONS.md` but not
    `CBP-10.md`) and change nothing. Running the printed command then `ticket delete`
    leaves all three files in one new archive dir, still listed, and no `SESSIONS.md`
    deleted unarchived. `cctx archive <archived-legacy-ID>` prints "already archived at …"
    and never suggests `ticket delete`.
13a. A directory ticket with `status=completed` still in `Active` is archived by
    `cctx archive <ID>` without re-applying completion.
14. `cctx archive specs/2026-10-12-login` (untracked dir) archives into
    `_archived/<today>_2026-10-12-login/` with `SourcePath` = its canon path and `Kind` =
    `path`; `cctx archive .cctx/CBP-1`, `.cctx/CBP-1/plan.md` and `".cctx/CBP-1/*.md"` for
    an active ticket are all refused (refusal 9).
15. `cctx archive "v4-*.md"` keeps today's selection prompt; running it twice on the same
    day from the same dir produces `<date>_<cwd>` and `<date>_<cwd>-1`; a matched
    symlink is skipped with a warning.
16. `cctx ticket complete CBP-1` on a directory ticket gives the end state of criterion 10
    with branch/commit taken from the project dir even when run from another repo; "not
    found" / "already completed" still drop a stale working entry.
17. `cctx archive --all-active` archives every active directory ticket after one
    confirmation, reports legacy ones as skipped, honours `-p`; one failing ticket does
    not stop the others or leak its state into config; exit code non-zero on failure.
18. `cctx ticket archive` and `cctx ticket archive-all` give cobra's unknown-command
    error; every invalid flag combination listed in §3 errors before doing anything.
19. With no args on branch `CBP-1` or `feature/CBP-1` (ticket `CBP-1` in config),
    `cctx archive`, `ticket complete`, `ticket show` and `ticket restore` resolve `CBP-1`;
    `ticket create` with no args on `feature/CBP-2` creates `.cctx/CBP-2/`.

**archive-list**
20. With records from two repos, `--list` inside repo A (including from a subdirectory)
    shows only A's; `--all` shows both plus `unknown`; newest first; `--limit 1` shows one
    row; `--limit -1` errors.
21. Backfill adds records for existing `_archived/` dirs exactly once, links
    `<date>_<ID>` and `<date>_<ID>-<n>` dirs to `<ID>`, ignores `.staging-*` and loose
    files.

**ticket-workspace**
22. `cctx ticket create CBP-2` creates exactly
    `.cctx/CBP-2/{requirements,plan,validation,changelog}.md` with `{{TICKET_ID}}`
    replaced; no `CBP-2.md`, no `_tickets/CBP-2/`; config has a canon `workspace_path`
    and empty `linked_projects`/`primary_context_name`.
23. A user override `~/.cctx/templates/ticket-plan.md` is used instead of the embedded one.
24. Create errors and writes nothing when `.cctx/CBP-2/` exists, when `<cwd>/.cctx`
    resolves to the active or default data dir, when `<cwd>/.cctx` is a symlink, or when
    the ID is `../x` or `CBP-1.md`. `ticket restore` with `archived_path` edited to `""`
    or `../..` errors before copying or removing anything.
25. Warnings appear (creation still succeeds) when cwd has no `.git`, and when cwd is a git
    repo where `.cctx/` is not ignored; no warning when it is ignored.
26. `cctx ticket restore CBP-2` after archiving recreates `.cctx/CBP-2/` in the *current*
    directory with identical contents, sets the ticket active with an updated
    `workspace_path`, and removes the archive dir and record; it refuses when the target
    already exists; a failed copy or
    failed `Save` leaves the archive and config as they were and no target behind.
27. `ticket link`/`unlink` on a directory ticket error. `ticket delete` removes the
    workspace, all of the ticket's archive dirs, records and the config entry; with
    `workspace_path` hand-edited to the project root it deletes nothing on disk, warns,
    and still removes the entry; a `glob`/`backfill` record that merely carries the ID is
    not deleted (its `TicketID` is cleared). `admin verify` reports a missing workspace and no
    `_tickets/` symlink issues for directory tickets. `admin reset` clears `archives`.

**init-specs**
28. `cctx init-specs` creates `.cctx/{mission,tech-stack,roadmap}.md`; a second run
    skips all three and keeps edited contents; same guards as criteria 24–25.

**cctx-skill**
29. `~/.claude/skills/cctx/SKILL.md` contains the workspace-selection rule, trust check,
    changelog target and scoped auto-approve rule (manual check; not covered by
    `make test`).

## Alternatives Considered

- **Per-ticket constitution** — rejected: mission/tech-stack/roadmap are project-wide.
- **Workspace at git top-level** — rejected: user wants cwd (per-service constitutions in
  monorepos); a warning covers the not-repo-root case.
- **Ticket dirs in the project root with new global-ignore patterns** — rejected: needs a
  pattern per prefix; branch-derived IDs wouldn't match. One `.cctx/` rule is simpler.
- **CLI writes only an empty dir or a pointer** — rejected: user wants a usable scaffold
  for people without a skill.
- **Configurable/named template sets, a `sdd_skill` setting** — rejected as unnecessary.
- **Keeping `<ID>.md` as an index file** — rejected; `changelog.md` replaces the session
  log and Jira details go in `requirements.md`.
- **Data-dir symlink to the workspace** — rejected; the path in config is enough.
- **Separate archive verbs / a separate listing verb** — rejected as confusing.
- **`ticket complete` as status-only** — rejected; it keeps archiving, via the engine.
- **`archive --completed`** — rejected: completed tickets are already archived;
  `--all-active` keeps today's `archive-all` behaviour.
- **Restore to the original path / metadata files in path archives** — rejected; restore
  is by ID into cwd only.
- **Allowing archive of git-tracked files** — rejected; it would delete committed work.
- **Merging files into existing archive dirs** (today's glob routing and leftover moves)
  — rejected after review: overwrite risk (case-insensitive APFS, rename races) for
  little benefit; every operation now creates a new dir and records carry the ticket ID.
- **Companion files** (archiving `<ID>`-named files from the project root along with a
  directory ticket, restoring them to the root) — rejected by the user: a directory
  ticket is exactly its workspace; loose files are archived by name or glob.
- **Porting legacy tickets into ticket mode** — rejected after review: source discovery,
  symlink cleanup, `SESSIONS.md` handling and ID-collision rules generated most edge
  cases, for one remaining active legacy ticket. Legacy files are archived by glob.

## Constraints & Risks

- **Data loss is the primary risk.** Mitigations: pre-flight refusals; staging +
  SHA-256 verification; config saved *before* any source is deleted (Save failure is a
  no-op); re-check of each file before deleting it; never writing into existing archive
  dirs; tests for every refusal and failure point.
- **Git dependency**: tracked-file and ignore checks shell out to `git` (with
  `core.fsmonitor=false`). Inside a repo without `git` on PATH, archiving is refused;
  outside any repo the checks are skipped.
- **Large directories**: SHA-256 doubles read I/O; fine for doc-sized SDD dirs; no size
  limit.
- **Config growth**: one record per archive dir; same-day repeat glob archives add dirs
  (`-1`, `-2`, …) — negligible at tens per year.
- **Backwards compatibility**: old configs load (new fields optional). Legacy tickets keep
  restore/delete/link/show; ticket-mode archive and `complete` reject them with a how-to.
  The user's 1 active legacy ticket must be archived by glob. Removing `ticket
  archive`/`archive-all` breaks muscle memory and scripts — accepted (no aliases).
- **Refactor risk**: `ticket.go` is ~2900 lines with duplicated archive logic; three
  copies become one engine. Engine and parity tests are written before the old code is
  removed.
- **Out-of-repo deliverable**: the skill edit (§7) is not checked by CI.
- **Platform**: macOS/Linux only (existing code already uses POSIX symlinks).

## Reconciliation (after implementation, 2026-10-12)

Behaviour added or tightened during implementation and code review, in addition
to the sections above:

- **§3 path & glob mode**: every argument is `filepath.Clean`ed before the
  symlink check (a trailing `/` would make `Lstat` follow a symlink); items
  nested inside another selected directory are dropped; refusals for **all**
  groups are evaluated before the first archive is created, so a refused item
  never leaves a half-done run.
- **§3 ticket mode**: the leftovers branch applies only to tickets in
  `Tickets.Archived` — a ticket still in `Active` with a stale `archived_path`
  is archived normally. `--commits`/`--prs` also error for already-archived
  tickets. The completion note is appended only if the staged `changelog.md` is
  a regular file (never through a symlink). The workspace must have the shape
  `.cctx/<ID>` before ticket mode archives it. The printed legacy how-to
  shell-quotes every name (and the ID) that isn't `[A-Za-z0-9._-]+`.
- **§3 removed commands**: `ticketCmd` is runnable with `cobra.NoArgs`, so
  `cctx ticket archive` / `archive-all` produce cobra's unknown-command error
  instead of printing help.
- **§5 create**: the collision prompt is numbered 1 project-scoped ID, 2 numeric
  suffix (shown as `<ID>-1`), 3 cancel.
- **§5 delete (directory ticket)**: config is saved before the workspace and
  archives are removed; a failed save deletes nothing. The cwd-inside and
  git-not-found refusals abort the delete (sentinel errors, not string matching).
- **§5 delete (legacy)**: a ticket ID that isn't in config can never resolve
  outside `contexts/_tickets/` (e.g. `../_archived` is refused); the
  `archived_path` record is dropped when that dir was removed or no longer
  exists; a broken `SESSIONS.md` symlink is detected with `Lstat`.
- **§5 restore (legacy)**: `validTicketID`, the `safeArchiveDir` containment
  check, and "must be in `Tickets.Archived`" now apply to legacy restores too.
  Directory restores chmod the restored workspace back to the archived dir's mode.
- **§5 show**: takes an optional ID and falls back to `resolveTicketID`.
- `promptExtraFiles` was deleted; `FindExtraTicketFiles` is kept (still tested,
  small).

### Known gaps

- **§7 cctx-skill** was not implemented (out of this repo; do separately).
- **Removing legacy ticket symlink logic** (`ticket link/unlink`, `_tickets/`
  symlinks, legacy verify/cleanup symlink checks) was raised by the user after
  the spec and explicitly deferred.
- `ticket show` prints a separator line of NUL bytes (pre-existing
  `string(make([]byte, 60))`), which makes `grep` treat its output as binary;
  relevant to §7, which parses its `Workspace:` line. Not fixed.
- Not done (low risk): caching `git ls-files` per directory, and skipping
  in-progress `.staging-*` dirs in `admin cleanup` (deleting one only makes that
  archive abort with sources untouched).

