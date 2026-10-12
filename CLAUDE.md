# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

**Note**: For comprehensive policies on session logging, file system handling, and ticket workflows, use the skill:
```
/cctx
```
The skill at `~/.claude/skills/cctx/skill.md` provides detailed policies that augment the architectural guidance below.

## Project Overview

Claude Context Manager (`cctx`) is a Go CLI tool for managing `claude.md` context files across multiple projects using symlinks. It provides centralized storage of context files with symlink-based access from projects.

## Development Commands

### Building

```bash
# Build binary (outputs to ./cctx)
make build

# Build from cli directory directly
cd cli && go build -o ../cctx .

# Install to ~/bin
make install

# Install to /usr/local/bin (requires sudo)
make install-global
```

After installation, clear shell cache:
```bash
hash -r    # bash/zsh
rehash     # zsh alternative
```

### Testing

```bash
# Run all tests
make test

# Run specific test suites
make test-ticket        # Ticket-related tests
make test-link          # Link/unlink tests
make test-global        # Global context tests
make test-integration   # Integration tests

# Run specific test by name
cd cli && go test -v ./cmd -run TestTicketCreate

# Run with coverage
make test-coverage      # Generates cli/coverage.html
```

### Code Quality

```bash
# Format code
make fmt

# Run go vet
make vet

# Run all checks (fmt + vet + test)
make check
```

## Architecture

### Core Concepts

**Symlink-Based Management**: Projects contain symlinks pointing to centralized context files stored in `~/.cctx/contexts/`. This keeps project directories clean while maintaining single source of truth for context files.

**Data Directory Structure**:
```
~/.cctx/
├── config.json              # Single source of truth, including archive records
├── contexts/
│   ├── project-name/        # Per-project contexts
│   │   └── claude.md
│   ├── _global/            # Shared global contexts
│   │   ├── script.md
│   │   └── python.md
│   ├── _tickets/           # Symlinks for legacy single-file tickets only
│   └── _archived/          # One new dir per archive run, never reused
│       └── 2026-08-12_TICKET-123/
└── templates/              # User template overrides

<project>/.cctx/             # Project workspace (git-ignored, never committed)
├── mission.md               # cctx init-specs
├── tech-stack.md
├── roadmap.md
└── TICKET-123/              # cctx ticket create
    ├── requirements.md
    ├── plan.md
    ├── validation.md
    └── changelog.md
```

### Ticket System

**Directory tickets** (created by `cctx ticket create`): the ticket is exactly its
workspace, `./.cctx/<ID>/` in the directory where it was created. Config records
the canonical path as `workspace_path`; `linked_projects` and
`primary_context_name` stay empty and no symlinks are created. Files outside the
workspace are never part of the ticket.

**Legacy tickets** (empty `workspace_path`): the older single `<ID>.md` in the
project plus symlinks under `_tickets/`. They can still be listed, shown,
linked, restored and deleted, but `cctx archive <ID>` and `ticket complete`
refuse them and print the command to archive their files by name.

### Archive Engine

Everything that moves files into `_archived/` goes through `archiveItems` in
`cli/cmd/archiveengine.go`: pre-flight refusals (git-tracked files, the data
dir, anything containing cwd, nested `.git`/`.cctx`, an active ticket's
workspace), copy to a `.staging-*` dir, SHA-256 verify, reserve a new
`<date>_<name>[-n]` dir, save config (record + caller's mutation), then delete
sources. A failed save removes the new archive, so sources are only deleted once
config says they're archived. Each run appends one `ArchiveRecord` (`kind`:
`ticket`, `path`, `glob`, `backfill`); only `ticket` records are owned by their
ticket and removed by `ticket delete`.

### Ticket File Recognition

`cli/cmd/ticketfiles.go` (`IsTicketFileName`, `FindTicketFiles`,
`FindExtraTicketFiles`) still decides which loose files belong to an ID — used by
glob archiving and the legacy how-to. `filterIDCollisions` in
`cli/cmd/archiveticket.go` drops files that belong to a longer ticket ID
(`CBP-7-x.md` is not `CBP-7`'s).

### Configuration System

**Single config.json**: All state lives in `~/.cctx/config.json`:
- Managed projects list
- Global contexts definitions
- Active and archived tickets
- Ticket linkages to projects
- Settings

**Key struct**: `config.Config` in `cli/internal/config/config.go`

**Manager pattern**: `config.Manager` provides:
- `Load()` - Read config
- `Save()` - Write config atomically
- Project/ticket CRUD operations

### Template System

**Embedded templates**: Default templates compiled into binary via `//go:embed` in `cli/internal/templates/embed.go`

**Template lookup priority**:
1. User override: `~/.cctx/templates/<name>.md` (if exists)
2. Embedded fallback: Built-in templates from source

**Template management commands**:
```bash
cctx admin global templates list              # List all templates
cctx admin global templates show <name>       # View template content
cctx admin global templates reset <name>      # Reset one to default
cctx admin global templates reset --all       # Reset all to defaults
cctx admin global templates sync              # Add missing templates (non-destructive)
```

**Development workflow**: After editing templates in `cli/internal/templates/*.md`:
```bash
make build                              # Rebuild binary
cctx admin global templates reset --all      # Update ~/.cctx/templates/
```

### Command Tree

The visible surface is deliberately small — ticket work is what this tool is
actually used for:

```
cctx ticket <verb>    Day-to-day work (create, complete, abandon, restore, list, show, ...)
cctx archive [...]    The only archive verb: ticket ID, path or glob; --all-active; --list
cctx init-specs       Scaffold mission/tech-stack/roadmap in ./.cctx/
cctx admin <verb>     Setup and maintenance: init, home, verify, cleanup, reset
                      Project contexts (legacy claude.md symlinks): sync, link, unlink, list, global
```

`admin` (`cli/cmd/admin.go`) exists to keep setup and the rarely-used
project-context feature out of the top-level listing. There are no
backwards-compatible aliases: `cctx verify` and `cctx ticket archive` are
unknown commands; the spellings are `cctx admin verify` and `cctx archive`.

### Command Implementation Pattern

Commands live in `cli/cmd/<command>.go`, follow this pattern:

```go
var cmdName = &cobra.Command{
    Use:   "command <args>",
    Short: "Brief description",
    Long:  `Detailed description with examples`,
    Args:  cobra.ExactArgs(1),
    RunE:  runCommandName,
}

func init() {
    // adminCmd for setup/maintenance/project-context commands,
    // ticketCmd for ticket verbs, rootCmd only for genuinely top-level ones
    adminCmd.AddCommand(cmdName)
    cmdName.Flags().StringVar(&flagVar, "flag", "", "Description")
}

func runCommandName(cmd *cobra.Command, args []string) error {
    dataDir := GetDataDirOrExit()

    if dryRun {
        dryRunMsg("Would do something")
        return nil
    }

    // Implementation
    successMsg("Operation complete")
    return nil
}
```

**Global flags** (available to all commands):
- `--data-dir, -d`: Override data directory
- `--dry-run`: Preview changes without executing
- `--verbose, -v`: Detailed output
- `--project, -p`: Specify project context
- `--ticket, -t`: Specify ticket ID

### Symlink Management

**Creation**: `common.CreateSymlink(target, linkPath)` in `cli/internal/common/utils.go`
- Handles relative/absolute paths
- Cross-platform compatible
- Error handling for existing files

**Health checking**: `cctx admin verify` command
- Detects broken symlinks
- Reports orphaned files
- Auto-repair with `--fix` flag

**Cleanup**: `cctx admin cleanup` command
- Finds orphaned symlinks (not in config)
- Finds stale context files (not referenced)
- Option to delete or restore to config
- Dry-run support

## Key Development Patterns

### Error Handling

Use descriptive errors with context:
```go
return fmt.Errorf("failed to create symlink: %w", err)
```

### Output Functions

Consistent user messaging via helpers in `cli/cmd/root.go`:
```go
successMsg("Operation complete")   // ✓ prefix
errorMsg("Something failed")       // ✗ prefix
warningMsg("Be careful")          // ⚠ prefix
infoMsg("Informational")          // ℹ prefix
dryRunMsg("Would do X")           // [DRY RUN] prefix
```

### Config Operations

Always use Manager pattern:
```go
cfgMgr := config.NewManager(dataDir)
cfg, err := cfgMgr.Load()
if err != nil {
    return err
}

// Modify cfg...

if err := cfgMgr.Save(cfg); err != nil {
    return err
}
```

### Dry Run Support

All mutating operations must respect `dryRun` flag:
```go
if dryRun {
    dryRunMsg("Would create file: " + path)
    return nil
}

// Actual operation
```

### Testing Patterns

Tests in `cli/cmd/*_test.go` use:
- Temp directories for isolation
- Table-driven tests for multiple scenarios
- Helper functions for common setup/teardown
- Integration tests marked with `Test*Integration` prefix

Example:
```go
func TestTicketCreate(t *testing.T) {
    tempDir := t.TempDir()

    tests := []struct {
        name    string
        ticketID string
        wantErr bool
    }{
        {"valid ticket", "TICKET-123", false},
        {"invalid format", "invalid", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

## Important File Locations

- **Entry point**: `cli/main.go`
- **Command definitions**: `cli/cmd/*.go`
- **Core config**: `cli/internal/config/config.go`
- **Templates**: `cli/internal/templates/*.md` (source), `~/.cctx/templates/` (user overrides)
- **Utilities**: `cli/internal/common/utils.go`
- **Build config**: `Makefile`

## Common Development Tasks

### Adding a New Command

1. Create `cli/cmd/newcommand.go`
2. Define cobra.Command struct
3. Implement `runNewCommand()` function
4. Register in `init()` under the right parent — `ticketCmd`, `adminCmd`, or
   `rootCmd` only if it belongs on the top-level listing
5. Add tests in `cli/cmd/newcommand_test.go`
6. Update help documentation

### Adding a New Template

1. Add `cli/internal/templates/newtemplate.md`
2. Template automatically embedded on build
3. Access via `templates.GetTemplate("newtemplate", dataDir)`
4. Test with `cctx admin global templates show newtemplate`

### Modifying Config Schema

1. Update structs in `cli/internal/config/config.go`
2. Handle migration in `config.Load()` if needed
3. Update all commands that read/write affected fields
4. Add tests for new schema
5. Consider backward compatibility

## Completion System

```bash
cctx admin completion bash|zsh|fish|powershell
```

Commands and flags are discovered automatically from the cobra definitions, so
no manual maintenance is needed. Dynamic values (ticket IDs, project names,
global context names) come from the `ValidArgsFunction` helpers in
`cli/cmd/completions.go`.

Cobra's default top-level `completion` command is disabled
(`rootCmd.CompletionOptions.DisableDefaultCmd`) and replaced by
`completionCmd` under `admin`. This does not affect the hidden `__complete`
command that powers dynamic completion — cobra registers that separately.

## Git Workflow

This repository uses conventional commits and tracks tickets via commit messages. When working on tickets:
- Create feature branch: `git checkout -b TICKET-123`
- Use ticket commands: `cctx ticket create TICKET-123`
- Link to projects: Auto-detects branch name or use `--ticket` flag
- Document in SESSIONS.md as work progresses