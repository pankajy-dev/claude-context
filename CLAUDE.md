# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

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
├── config.json              # Single source of truth for all configuration
├── contexts/
│   ├── project-name/        # Per-project contexts
│   │   └── claude.md
│   ├── _global/            # Shared global contexts
│   │   ├── script.md
│   │   └── python.md
│   ├── _tickets/           # Ticket workspaces
│   │   └── TICKET-123/
│   │       ├── ticket.md -> /path/to/project/TICKET-123.md
│   │       └── SESSIONS.md -> /path/to/project/SESSIONS.md
│   └── _archived/          # Archived tickets
└── templates/              # User template overrides
```

### V2 Architecture (Ticket System)

**Key principle**: Concrete files live in the project directory, data directory contains symlinks pointing to them.

When creating a ticket:
1. **Concrete files** created in project directory: `TICKET-123.md`, `SESSIONS.md`
2. **Symlinks created in data directory**: `~/.cctx/contexts/_tickets/TICKET-123/ticket.md` → project concrete file
3. **Primary context** tracked in config: First linked project owns the concrete files
4. **Other projects** get symlinks pointing to primary project's concrete files

This design ensures:
- Files are version-controlled with the project
- Easy discovery (concrete files in project, not hidden in ~/.cctx)
- Multiple projects can reference same ticket via symlinks
- Deleting from primary project removes concrete files; deleting from secondary only removes symlinks

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
cctx global templates list              # List all templates
cctx global templates show <name>       # View template content
cctx global templates reset <name>      # Reset one to default
cctx global templates reset --all       # Reset all to defaults
cctx global templates sync              # Add missing templates (non-destructive)
```

**Development workflow**: After editing templates in `cli/internal/templates/*.md`:
```bash
make build                              # Rebuild binary
cctx global templates reset --all      # Update ~/.cctx/templates/
```

### .clauderc Integration

**Purpose**: Auto-manage `.clauderc` files in projects to include all relevant context files.

**Manager**: `clauderc.Manager` in `cli/internal/clauderc/clauderc.go`

**Behavior**:
- Automatically adds/removes files from `.clauderc.additionalContext[]`
- Creates `.clauderc` if missing when linking first context
- Preserves existing entries not managed by cctx
- Atomic writes (temp file + rename)
- Case-sensitive file detection (`CLAUDE.md` vs `claude.md`)

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
    rootCmd.AddCommand(cmdName)
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

**Health checking**: `cctx verify` command
- Detects broken symlinks
- Reports orphaned files
- Auto-repair with `--fix` flag

**Cleanup**: `cctx cleanup` command
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
4. Register in `init()` function
5. Add tests in `cli/cmd/newcommand_test.go`
6. Update help documentation

### Adding a New Template

1. Add `cli/internal/templates/newtemplate.md`
2. Template automatically embedded on build
3. Access via `templates.GetTemplate("newtemplate", dataDir)`
4. Test with `cctx global templates show newtemplate`

### Modifying Config Schema

1. Update structs in `cli/internal/config/config.go`
2. Handle migration in `config.Load()` if needed
3. Update all commands that read/write affected fields
4. Add tests for new schema
5. Consider backward compatibility

## Completion System

Shell completion auto-generated by Cobra:
```bash
cctx completion bash|zsh|fish|powershell
```

No manual maintenance needed - commands/flags discovered automatically from cobra definitions.

## Git Workflow

This repository uses conventional commits and tracks tickets via commit messages. When working on tickets:
- Create feature branch: `git checkout -b TICKET-123`
- Use ticket commands: `cctx ticket create TICKET-123`
- Link to projects: Auto-detects branch name or use `--ticket` flag
- Document in SESSIONS.md as work progresses