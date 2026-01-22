# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Claude Context Manager is a CLI tool for managing `claude.md` context files across multiple projects using symlinks. All runtime data (contexts, tickets, config) is stored in a centralized location (`~/.cctx` by default), while this repository contains only the CLI code.

**Core Philosophy:**
- Centralized data directory (`~/.cctx`) for all context files and configuration
- This repository contains ONLY code (no user data)
- Symlinks in projects point to files in the data directory
- No git tracking of user data (contexts belong to the user, not the tool)

## Architecture

### Data Directory Structure (`~/.cctx`)

```
~/.cctx/
├── config.json              # All configuration
├── contexts/                # All context files
│   ├── project-name/
│   │   └── claude.md       # Project-specific context
│   ├── _global/            # Global contexts (shared)
│   │   ├── script.md
│   │   └── python.md
│   ├── _tickets/           # Ticket workspaces
│   │   └── TICKET-123/
│   │       ├── ticket.md
│   │       └── metadata.json
│   └── _archived/          # Archived tickets
└── templates/              # User-customizable templates
    ├── default.md
    ├── global.md
    └── ticket.md
```

### Repository Structure (code only)

```
claude-context/
├── cli/                    # Go CLI source code
│   ├── cmd/               # Command implementations
│   ├── internal/          # Internal packages
│   └── main.go
├── templates/             # Source templates (copied during init)
├── Makefile              # Build and install
├── README.md
├── CLAUDE.md
└── .gitignore
```

### Data Directory Location

Users can specify the data directory location:
1. `--data-dir` flag: `cctx --data-dir /custom/path <command>`
2. `CCTX_DATA_DIR` environment variable: `export CCTX_DATA_DIR=/custom/path`
3. Default: `~/.cctx`

## Commands

All commands now operate on `~/.cctx` (or custom data directory) instead of the repository.

### Initial Setup

```bash
# Build and install
make install             # Install to ~/bin
make install-global      # Install to /usr/local/bin (requires sudo)

# Initialize data directory (one-time setup)
cctx init

# If migrating from old version, init will automatically detect and migrate
```

### Managing Projects

```bash
# Link a new project (creates context + symlinks)
cctx link /path/to/project [context-name]

# List all managed projects
cctx list
cctx list --verbose     # Show detailed information

# Verify health of all symlinks
cctx verify
cctx verify --fix       # Auto-repair broken symlinks

# Unlink a project
cctx unlink context-name
cctx unlink context-name --keep-content  # Keep context file
```

### Managing Tickets

```bash
# Create a new ticket workspace
cctx ticket create TICKET-123 --title "Add feature" --tags "backend,api"

# Link ticket to projects (using flags)
cctx -t TICKET-123 ticket link project1 project2
cctx -t TICKET-123 -p project1 ticket link

# Or set environment variables for easier workflow
export CCTX_TICKET=TICKET-123
export CCTX_PROJECT=project1

# Now commands are much simpler
cctx ticket link                        # Links TICKET-123 to project1
cctx ticket list                        # List tickets for project1
cctx -p project2 ticket link           # Also link to project2

# Show ticket details
cctx ticket show                        # Or: cctx -t TICKET-123 ticket show

# Complete ticket (auto-archives and removes from all projects)
cctx ticket complete                    # Auto-detects branch + commit from current dir
cctx ticket complete --commits "abc123,def456" --prs "42,43"  # Manual override

# Delete archived ticket permanently (if needed)
cctx ticket delete --force

# Bulk Operations
# Archive all active tickets at once (useful for sprint cleanup)
cctx ticket archive-all                 # Archives all active tickets (all projects)
cctx ticket archive-all --force         # Skip confirmation
cctx -p project1 ticket archive-all     # Archive all tickets from specific project
export CCTX_PROJECT=project1 && cctx ticket archive-all  # Using env var
```

**Ticket Flag Usage:**
- Use `--ticket` / `-t` flag or `CCTX_TICKET` env var to specify ticket
- Use `--project` / `-p` flag or `CCTX_PROJECT` env var to specify project
- Setting env vars once makes commands much more concise throughout the day

**Auto-Complete Behavior:**
- `ticket complete` automatically archives the ticket (removes symlinks from all projects)
- Auto-detects git branch and latest commit from current directory
- Manual `ticket archive` command is no longer needed

**Bulk Operations:**
- `ticket archive-all`: Archives all active tickets across all projects (useful for sprint cleanup)
- `ticket archive-all -p <project>`: Archives all active tickets from a specific project only

### Managing Global Contexts

```bash
# Create a global context
cctx global create script --description "Shell scripting guidelines"

# Enable/disable global contexts
cctx global enable script
cctx global disable script

# Link global context to a project
cctx global link script project-name

# List global contexts
cctx global list --verbose
```

## Development Patterns

### When Modifying Go CLI Code

**CRITICAL: Always recompile after making changes**

1. **Make your code changes** in `cli/` directory

2. **Build the binary:**
   ```bash
   make build
   # or
   cd cli && go build -o ../cctx ./main.go
   ```

3. **Test the changes:**
   ```bash
   ./cctx --version
   ./cctx list
   ./cctx verify
   ```

4. **Run all checks:**
   ```bash
   make check   # Runs fmt, vet, and tests
   ```

### Key Files

- **`cli/internal/config/config.go`**: Core data structures (Config, Project, Ticket, GlobalContext)
- **`cli/cmd/*.go`**: Command implementations
- **`cli/main.go`**: Entry point
- **`cli/cmd/root.go`**: Root command and GetDataDir() function

### Schema Consistency

- Go struct tags must match `config.json` field names exactly
- Example: `json:"created_at"` in Go must match `"created_at"` in JSON
- Schema mismatches cause silent deserialization failures (zero values)

### Adding New Commands

1. Create new file in `cli/cmd/` (e.g., `newcmd.go`)
2. Define cobra command with Use, Short, Long, RunE
3. Add command to parent in `init()` function
4. Use `GetDataDirOrExit()` to get data directory
5. Use `config.NewManager(dataDir)` to access config
6. **Rebuild:** `make build`
7. Test thoroughly

### Path Handling

- Always use `GetDataDir()` or `GetDataDirOrExit()` to get the data directory
- Never hardcode `~/.cctx` in commands
- All config and context paths are relative to the data directory
- Use `common.NormalizePath()` for user-provided paths (expands ~, makes absolute)

### No Git Tracking

- User data (contexts, config) is NOT git-tracked
- Users manage their own backups of `~/.cctx`
- This repository tracks only code
- All git-related code has been removed from commands

## Configuration

### config.json Structure

Located at `~/.cctx/config.json`:

```json
{
  "managed_projects": [
    {
      "context_name": "my-project",
      "project_path": "/path/to/project",
      "context_path": "contexts/my-project/claude.md",
      "created_at": "2026-01-21T10:00:00Z",
      "last_modified": "2026-01-21T10:00:00Z",
      "symlink_status": "active",
      "linked_globals": ["script", "python"]
    }
  ],
  "global_contexts": [
    {
      "name": "script",
      "description": "Shell scripting guidelines",
      "path": "contexts/_global/script.md",
      "enabled": true
    }
  ],
  "tickets": {
    "active": [],
    "archived": [],
    "settings": {
      "auto_archive": false
    }
  },
  "settings": {
    "auto_commit": false,
    "backup_on_unlink": true
  }
}
```

### Settings

- **auto_commit**: Always false (no git tracking)
- **backup_on_unlink**: Create .bak files before deletion

## Build and Installation

### Building

```bash
# Build binary
make build

# Build and install to ~/bin
make install

# Build and install to /usr/local/bin (requires sudo)
make install-global
```

### Testing

```bash
# Run tests
make test

# Format code
make fmt

# Run go vet
make vet

# Run all checks (fmt + vet + test)
make check
```

### Cleaning

```bash
# Remove build artifacts
make clean

# Uninstall from ~/bin
make uninstall

# Uninstall from /usr/local/bin
make uninstall-global
```

## Migration from Old Version

If you have an existing installation with data in the repository:

1. Run `cctx init` - it will automatically detect and migrate
2. Migration moves:
   - `config.json` → `~/.cctx/config.json`
   - `contexts/` → `~/.cctx/contexts/`
   - `templates/` → `~/.cctx/templates/`
3. All symlinks are automatically updated to point to `~/.cctx`
4. Old repository files can be safely removed after migration

## Troubleshooting

### "Data directory not initialized" error

**Cause:** `~/.cctx` doesn't exist or has no `config.json`
**Solution:** Run `cctx init`

### Compilation errors after code changes

- Ensure you've rebuilt: `make build`
- Check for struct field name mismatches in JSON tags
- Run `make check` to catch issues

### Broken symlinks after moving ~/.cctx

- Run `cctx verify --fix` to recreate symlinks
- Or specify new location: `export CCTX_DATA_DIR=/new/path`

### Custom data directory

```bash
# Set environment variable (recommended)
export CCTX_DATA_DIR=/custom/path
cctx init

# Or use flag every time
cctx --data-dir /custom/path init
cctx --data-dir /custom/path list
```

## Important Design Decisions

### Why ~/.cctx instead of in-repo?

- **Separation of concerns:** Code (git-tracked) vs data (user-owned)
- **Clean repository:** No user data in the code repository
- **User control:** Users manage their own backups of `~/.cctx`
- **Tool updates:** Can update the CLI without affecting user data

### Why no git tracking of user data?

- Context files contain project-specific information that users may not want to share
- Users can choose to git-track their `~/.cctx` separately if desired
- Simplifies the tool - no automatic commits, no git dependency
- Faster operations (no git overhead)

### Why symlinks?

- **Single source of truth:** Edit once in `~/.cctx`, affects all references
- **Disk efficiency:** No file duplication
- **Consistent updates:** Changes propagate immediately
- **Claude Code integration:** Works seamlessly with `.clauderc` includes

### Why Go instead of bash scripts?

- **Cross-platform:** Works on macOS, Linux, Windows
- **Better error handling:** Typed language with proper error management
- **Single binary:** No dependencies, easy to install
- **Better testing:** Unit tests, integration tests
- **Performance:** Faster than bash scripts
