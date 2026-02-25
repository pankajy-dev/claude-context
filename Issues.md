# Issues

## ✅ Fixed (2026-02-25)

### 1. Ticket delete command should accept ticket ID as positional argument
**Status:** Fixed
**Description:** User tried `cctx ticket delete BEE-32496` but it required `-t BEE-32496` flag.
**Fix:** Modified command to accept optional ticket ID as positional argument: `cctx ticket delete [<ticket-id>]`
**Usage:**
- `cctx ticket delete TICKET-123` (positional arg)
- `cctx -t TICKET-123 ticket delete` (flag)
- `export CCTX_TICKET=TICKET-123 && cctx ticket delete` (env var)

### 2. SESSIONS.md symlinks not removed when deleting tickets
**Status:** Fixed
**Description:** When running `cctx ticket delete -t BEE-32496`, the SESSIONS.md symlink remained in project directories.
**Fix:** Enhanced delete command to remove both ticket.md AND SESSIONS.md symlinks from all managed projects.
**Code Changes:** cli/cmd/ticket.go:1386-1434

### 3. SESSIONS.md creation fails if file already exists
**Status:** Fixed
**Description:** If SESSIONS.md already exists as a file, creating a new ticket fails to create the symlink.
**Fix:** Modified to create unique filename with ticket ID suffix if SESSIONS.md already exists:
- First try: `SESSIONS.md`
- If exists: `SESSIONS-TICKET-123.md`
**Code Changes:** cli/cmd/ticket.go:267-287

### 4. Write git SHA to SESSIONS.md when ticket is completed
**Status:** Fixed
**Description:** Enhance SESSIONS.md to record git commits, branch, and PRs when marking ticket as completed.
**Fix:** Automatically appends completion entry to SESSIONS.md including:
- Completion timestamp
- Status (Completed)
- Git branch name
- Commit SHAs
- Pull request numbers
**Code Changes:** cli/cmd/ticket.go:982-1011
**Note:** Silently skips if data is not available (no errors).

---

## ❌ Closed - Won't Fix

### 5. Instead of symlinks, create actual file and once done then move this file to the place where symlinks are created
**Status:** Closed - Current behavior is correct
**Reason:** Current symlink model is the correct design. Symlinks provide:
- Single source of truth in `~/.cctx/contexts/_tickets/`
- Real-time sync across multiple project directories
- Clean separation between code (git-tracked) and context files (user-managed)
- Consistent behavior when ticket is linked to multiple projects