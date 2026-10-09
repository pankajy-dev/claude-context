---
name: cctx
description: Start work on a new ticket or a clearly defined task, load its context, and keep session notes when the repository already uses ticket-based context files.
triggers:
  - let's start working on a ticket
  - let's work on a ticket
  - start a new task
  - let's start working
---

# Work Session Setup

Use this skill when starting work on a ticket or a new, clearly defined task. Adapt to the repository's existing conventions. Do not assume that every project uses Jira, a particular ticket prefix, or the same branch naming scheme.

## 1. Identify the Work Item

1. Inspect the current directory and Git repository.
2. Check the current branch and working-tree status before making changes.
3. Identify the ticket or task ID from the user's request, current branch, or existing context files.
4. If a ticket ID is available, look for a matching context file in the repository root. Use the repository's existing naming convention when one is documented. A common Jira-style format is `PROJECT-123.md`, where `PROJECT` is the project's issue key and `123` is the issue number.
5. If the repository has no matching context file, do not create one automatically unless the user or project instructions ask for it.

## 2. Ticket Context Policy

When a matching ticket context file exists:

1. Read it before starting implementation. Treat it as the local source of truth for the ticket's requirements, decisions, progress, and known constraints.
2. If the ticket references an issue tracker such as Jira and the relevant integration is available, retrieve the issue details and update the context file with any missing or changed requirements before implementation.
3. Preserve useful existing notes. Do not replace documented decisions or silently discard context.
4. If the context file conflicts with the user's latest instructions or the issue tracker, identify the conflict and resolve it before making consequential changes.
5. Ask before checking out a new branch. Follow the repository's branch naming convention; if none is documented, use the ticket ID when available, for example `PROJECT-123`.
6. Do not assume that a ticket file or branch naming convention from one repository applies to another.

## 3. Session Logging Policy (Conditional)

Only maintain session logs if the repository already has a matching ticket context file in its root. If no matching file exists, do not create a session log automatically.

### Running totals

If the existing file contains a running totals section, update that section in place when recording a session. Track the fields that the file already supports, such as:

- Total cost, when cost information is available
- Total number of sessions
- Usage by model, when usage data is available

Do not invent usage or cost figures. If the file has no running totals section, do not add one unless the project's existing template or the user requests it.

### Session entries

Append one entry for each distinct work session or ticket task. Keep each entry to 30 lines or fewer. Include relevant items from this list:

- Date in ISO format (`YYYY-MM-DD`)
- Git branch
- Ticket ID, if applicable
- Summary of work and decisions
- Research findings
- Important trade-offs or unresolved questions
- Session cost, only when reliably available, using the existing file's format

Treat session entries as append-only. Do not rewrite or delete earlier entries to make a new entry fit.

### Logging rules

- Update an existing running totals section whenever a new session entry is written, if reliable data is available.
- Append new session entries rather than editing previous ones.
- Do not add duplicate cost summaries to individual entries if the existing template already records cost elsewhere.
- If cost or token usage is unavailable, omit it rather than estimating.
- Follow any additional project-specific logging requirements that are documented in the repository.

## 4. File System and Symlink Policy

Repositories may contain symbolic links.

When navigating or reading files:

1. Inspect symlinks when a path's destination is unclear.
2. Follow symlinks when needed to understand the project, while distinguishing the link path from the target path.
3. Use supported commands such as `ls -l` and `readlink` to inspect links. Use `find -L` only when following links is appropriate and safe.
4. Before writing through a symlink, verify the resolved target so you do not unintentionally modify a file outside the repository.
5. If a symlink is broken or points somewhere unexpected, report that fact before relying on it.

Do not assume every read or editing tool follows symlinks automatically. Check the behavior of the tool being used.

## 5. Repository and Change Safety

Before implementation:

1. Review the working-tree status and relevant recent changes.
2. Do not overwrite or discard the user's uncommitted work.
3. Read the project's contribution guide, development instructions, and test guidance when present.
4. Identify the smallest relevant set of files and tests.
5. Confirm before switching branches or performing other consequential repository operations when required by the user's instructions or environment.

During implementation:

- Keep changes focused on the requested work.
- Follow local naming, formatting, testing, and documentation conventions.
- Run relevant tests or checks when practical, and report what was and was not run.
- Do not claim tests passed unless they were actually executed successfully.

## 6. Open-Source Project Guidance

For open-source repositories, follow the project's public contribution process and documented conventions. For example, when contributing to Jenkins or a Jenkins plugin:

- Read the repository's `README`, contribution guide, and development documentation.
- Check the supported Java and Jenkins versions before changing compatibility-sensitive code.
- Follow the project's test, formatting, and plugin release conventions.
- Keep changes scoped to the issue and explain compatibility or migration implications when relevant.
- Do not assume these Jenkins-specific checks apply to unrelated repositories.

## 7. Completion Summary

At the end of the work session, provide a concise summary containing:

- What changed
- Important decisions or trade-offs
- Tests and checks run, including failures or omissions
- Relevant files changed
- Any remaining work or blockers

If the repository uses a ticket context file and session logging, update it according to the policy above.
