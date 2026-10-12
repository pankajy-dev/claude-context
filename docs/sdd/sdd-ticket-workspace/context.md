# Context: SDD ticket workspace for cctx

## Ticket landscape

- Idea ticket: none
- Discovery ticket: none — idea originates with the engineer
- Epic: none
- Jira project: n/a (Phases 5–6 to be decided; may stay repo-only)
- External references: three screenshots of an SDD workflow (AgentClinic demo), summarised below

## The idea (as stated)

Today `cctx ticket create CBP-123` writes a single `CBP-123.md` from a fixed template.
All work for the ticket happens in that one file; at the end, files matching the
ticket ID are archived.

AI-assisted development has moved toward spec-driven development (SDD). The goal is
to reshape `cctx` around that:

- A ticket becomes a **directory** `CBP-123/` instead of a single file.
- Inside it, SDD documents:
  - constitution-style: `mission.md`, `tech-stack.md`, `roadmap.md`
  - feature-work: `plan.md`, `requirements.md`, `validation.md`
- Developers may already have their own SDD skill installed. If so, use it — but the
  parent directory must still be `CBP-123/` so `cctx` can archive the whole directory.
- Otherwise `cctx` ships its own templates / skill for the repeated steps.
- A research **backlog** written as dated files (`YYYY-MM-DD-{description}.md`).

## Reference workflow (from screenshots)

1. **Constitution** — "Look in README.md for stakeholder input. Make a constitution in a
   `specs/` directory: `mission.md`, `tech-stack.md`, `roadmap.md` (from TODO.md, in very
   small phases). Interview me about mission, audience, tech-stack gaps. Use
   AskUserQuestion, grouped on these 3, before writing to disk."
2. **Feature spec skill** — "Find the next phase in `specs/roadmap.md`, make a branch, ask
   me about the feature spec. Create `specs/YYYY-MM-DD-feature-name/` containing
   `plan.md` (numbered task groups), `requirements.md` (scope, decisions, context),
   `validation.md` (how to know it succeeded and can merge). Refer to mission.md and
   tech-stack.md." — packaged as a local skill to stop repeating the prompt.
3. **Research backlog** — "Write this research to a backlog document in `backlog/`,
   file name like `YYYY-MM-DD-{description}`."

Note: in the reference, mission/tech-stack/roadmap are **project-level** (one per repo)
and feature dirs are **many per project**.

## Current implementation (relevant facts)

- `cctx ticket create` (`cli/cmd/ticket.go:122`): writes `<cwd>/<ID>.md` from the
  `ticket` template (`{{TICKET_ID}}` substitution, `# Ticket: <ID>` header), symlinks
  `~/.cctx/contexts/_tickets/<ID>/<ID>.md` → concrete file, records ticket in
  `config.json`, adds to `current_working_tickets`.
- Ticket ID comes from arg or git branch; collisions offer link / project-scoped / suffix.
- Templates: embedded `cli/internal/templates/*.md` (flat, `//go:embed *.md`), user
  overrides in `~/.cctx/templates/<name>.md`. `GetTemplate(name, dataDir)`. No notion of
  multi-file template sets or subdirectories.
- Current `ticket.md` template: Jira info (filled via Atlassian MCP), notes, append-only
  session tracking section.
- File recognition (`cli/cmd/ticketfiles.go`): `IsTicketFileName`, `FindTicketFiles`,
  `FindExtraTicketFiles`. **`ticketFileNames` skips directories** (`e.IsDir()` → skip).
- `ticketID+".md"` is hard-coded at ~30 sites across create, link, unlink, complete,
  archive, restore, delete, archive-all, verify.
- Archive (`ticket.go:1764`): copies `FindTicketFiles` + `SESSIONS.md` from the primary
  project into `~/.cctx/contexts/_archived/<date>_<ID>/`, removes originals and symlinks.
  Only `active` → refused; must be completed/abandoned first.
- Restore copies files back from `_archived/` and re-creates the data-dir symlink.
- Link to another project = symlink `<project>/<ID>.md` → primary project's file.
- Top-level `cctx archive <glob>` archives loose files; routes ticket-named files into
  that ticket's archive dir.
- Config `Ticket` struct has no field describing layout/kind of workspace.
- The `cctx` Claude skill (`~/.claude/skills/cctx/skill.md`) keys its session-logging
  policy off a `(CBP|BEE|SECO)-\d+` **file** in the repo root.
- Existing data: ~33 archived tickets, 2 active (`CBP-32494`, `CBP-62891`) — all flat-file.

## Early flaws / open questions to resolve in the interview

1. **Scope mismatch**: mission/tech-stack/roadmap are project-wide in the reference
   workflow, but the idea places them under each ticket dir. Per-ticket or per-project?
2. **CLI vs skill boundary**: a Go CLI cannot invoke a Claude skill. What exactly does
   `cctx` do (scaffold dirs/files, write a pointer to a skill) vs what the skill does?
3. **Recognition rewrite**: directory-based tickets break `FindTicketFiles` (skips dirs)
   and every `ID+".md"` site — link, verify, delete, restore all need a directory model.
4. **Cross-project linking**: symlink the whole directory or individual files?
5. **Session tracking**: where does the append-only session log live now?
6. **Backlog**: per ticket, per project, or global?
7. **Migration / coexistence** with existing flat-file tickets and archives.
8. **Custom skill contract**: how does a user's own skill know to write into `CBP-123/`?
9. **Git**: is `CBP-123/` committed, or ignored like today's file is in practice?

## Resolved intent (Phase 2 interview, confirmed 2026-10-12)

Open questions 1–9 above are all resolved below; where an early answer was later
reversed, only the final decision is recorded.

- **Outcome:** cctx becomes primarily a robust archive/cleanup tool for directory-based
  SDD work, plus a quick-start scaffold for people without a skill.
- **User:** the engineer and fellow CBP developers who use AI skills for spec-driven work
  but must keep those files out of org repos.
- **Why now:** the single-`CBP-123.md` workflow is dated; SDD skills produce multi-file
  directories that today's archive cannot handle.
- **Success:** any SDD directory — cctx-made or skill-made — can be archived, listed and
  (tickets) restored with one verb, with no data loss and no git-tracked file touched;
  legacy flat tickets keep working.
- **Constraint:** nothing cctx creates may show up in git. Everything lives under
  `./.cctx/`, ignored via `~/.gitignore_global`; cctx warns but never edits any gitignore.
- **Out of scope:** cctx running or installing skills; configurable template sets;
  restore of path-archived dirs; migration of existing flat tickets; directory-level
  cross-project linking; backlog handling (`backlog/` belongs to skills).

### Decisions

1. **Project workspace** = `./.cctx/` in the **current directory** (not git top-level).
   Warn (don't fail) if cwd has no `.git` entry. Refuse if the target resolves to the
   data dir (`~/.cctx`). Terminology: "project workspace" (`<repo>/.cctx/`) vs
   "data dir" (`~/.cctx/`).
2. **Gitignore check:** on `ticket create` and `init-specs`, run `git check-ignore -q .cctx`;
   if not ignored, warn with the fix `echo '.cctx/' >> ~/.gitignore_global`. Skip outside
   a git repo. Never modify any gitignore.
3. **`cctx ticket create <ID>`** scaffolds `./.cctx/<ID>/` with `requirements.md`,
   `plan.md`, `validation.md`, `changelog.md` from embedded templates
   (`{{TICKET_ID}}` substituted; existing `~/.cctx/templates/<name>.md` override applies).
   No `<ID>.md` file, no symlinks. Workspace path recorded in config.
   Templates are thin instructions to Claude:
   - `requirements.md`: fetch `{{TICKET_ID}}` from Jira via Atlassian MCP, fill summary /
     description / acceptance criteria, then interview the user with AskUserQuestion
     before writing scope, decisions, context.
   - `plan.md` (numbered task groups) and `validation.md` (how we know it's done and
     mergeable): refer to `requirements.md`, interview via AskUserQuestion before writing.
   - `changelog.md`: append-only log of changes/sessions (replaces session tracking).
4. **`cctx init-specs`** (top-level) writes `./.cctx/mission.md`, `tech-stack.md`,
   `roadmap.md` from thin AskUserQuestion-interview templates; skips existing files;
   same cwd/gitignore/data-dir guards.
5. **`cctx archive <ticket-id|path|glob>`** — the single archive verb. Resolution order:
   ticket ID present in config → existing path (file or dir, absolute or relative) → glob.
   - Ticket mode: archives `./.cctx/<ID>/` (via recorded workspace path) only.
     *Revised during spec review:* no companion files, and legacy tickets are not
     handled by ticket mode (error with a how-to); every archive creates a new dir;
     archiving works in non-git directories (git checks only apply inside a work tree).
     An active ticket prompts "mark completed and archive? [y/N]" and does both.
   - Safety rails (all modes): copy → verify → delete source (never delete on partial
     copy); preserve directory structure; refuse if any file under the path is
     git-tracked; refuse `$HOME`, data dir, repo root, and `.cctx/` itself; always list
     files and confirm; `--dry-run` honoured.
6. **`cctx archive --list [--all] [--limit N]`** — newest first, default limit 20.
   Default scope: archives whose source path is under cwd (or git top-level if present);
   `--all` = global. Every archive (ticket and path) is recorded in config with archive
   dir, absolute source path, ticket ID (optional), timestamp, file count. Existing
   archives backfilled from ticket project paths where possible; others shown under
   `--all` as `source: unknown`.
7. **`cctx archive --completed`** archives all completed/abandoned tickets (list +
   confirm). Replaces `ticket archive-all`.
8. **`cctx ticket restore <ID>`** — ID-based only; restores directory archives into
   `./.cctx/<ID>/` of the cwd; refuses to overwrite; sets ticket active. Legacy flat
   archives restore as today (files into cwd).
9. **Removed:** `cctx ticket archive`, `cctx ticket archive-all` (no aliases, per repo
   convention). `ticket link`/`unlink` not supported for directory tickets (error);
   legacy behaviour unchanged.
10. **Other ticket commands** (`delete`, `complete`, `verify`, `cleanup`, `show`) must
    become directory-aware; no migration of the 2 active / ~33 archived flat tickets.
11. **cctx Claude skill** (`~/.claude/skills/cctx/skill.md`, outside repo — separate
    deliverable, not covered by `make test`): trigger on `./.cctx/<ID>/` (ID from branch),
    log sessions to `changelog.md`, auto-approve edits under `.cctx/**`.
