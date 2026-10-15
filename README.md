# Claude Context Manager

**A spec-driven workspace per ticket for Claude Code, kept out of git and archived safely when you're done.**

## The Problem

You work a ticket in Claude Code with a spec-driven workflow: requirements, a plan,
validation criteria, a changelog — and often a skill that writes its own set of
files. None of it belongs in the org repo, and when the work is done it all needs
to go somewhere safe without anyone hand-picking files to delete.

## The Solution

- **One directory per ticket**: `.cctx/CBP-123/` in the project, with
  `requirements.md`, `plan.md`, `validation.md` and `changelog.md` to start from
- **Bring your own skill**: skip the templates and let your SDD skill write
  wherever it likes; `cctx` archives any directory
- **Never committed**: everything lives under `.cctx/`, one global gitignore line
- **Archived, not deleted**: one `cctx archive` verb copies, verifies, records and
  only then removes — and refuses anything git-tracked

## Quick Example

```bash
# Install and initialize
git clone https://github.com/yourusername/claude-context.git
cd claude-context && make install && cctx admin init
echo '.cctx/' >> ~/.gitignore_global

# Start work — ticket ID comes from the branch (feature/CBP-123 works too)
cd ~/my-service && git checkout -b CBP-123
cctx ticket create

# Creates .cctx/CBP-123/{requirements,plan,validation,changelog}.md
# Now tell Claude: "Let's work on the ticket"

# When you're done — marks it completed and archives the workspace
cctx ticket complete
```

## Common Workflows

### Tickets

```bash
cctx ticket create PROJ-123     # scaffold .cctx/PROJ-123/
cctx ticket list                # what's active
cctx ticket show PROJ-123       # details, including the workspace path
cctx ticket complete            # complete + archive (ID from the branch)
cctx ticket restore PROJ-123    # bring an archived workspace back into ./.cctx/
```

Ticket ID resolution is the same everywhere: an explicit argument wins, then
`--ticket/-t`, then `CCTX_TICKET`, then the current git branch (unless it's
`main`/`master`; for `feature/PROJ-123` the last segment is used).

### Project specs

```bash
cctx init-specs   # .cctx/mission.md, tech-stack.md, roadmap.md (never overwrites)
```

Each template opens with instructions for Claude to interview you (using the
AskUserQuestion tool) before writing anything.

### Archiving

`cctx archive` is the only archive command. Every run creates a new directory
under `~/.cctx/contexts/_archived/` and records it in `config.json`.

```bash
cctx archive PROJ-123                # a ticket's workspace (active → completed first)
cctx archive specs/2026-10-12-login  # any directory, e.g. one your skill created
cctx archive "v4-*.md" notes.md      # files by glob or name
cctx archive --all-active            # every active ticket (-p <project> to limit)
cctx archive --list                  # what was archived from this repo
cctx archive --list --all            # ... from everywhere
cctx archive specs/old --dry-run     # preview
```

Files are copied to a staging directory, verified (SHA-256), recorded in config,
and only then deleted. `cctx` refuses to archive anything git-tracked, anything
containing the current directory or a nested git repo, and anything inside the
cctx data directory. It works the same in directories that aren't git repos.

Tickets created before directory workspaces (a single `CBP-123.md` file) are not
archived by ticket ID; `cctx archive CBP-123` prints the exact command to archive
their files by name instead.

### Project Contexts (Legacy)

The original feature: a shared `claude.md` per project, plus global contexts
symlinked into many projects. Still supported, now under `cctx admin`.

```bash
# Add a project (creates claude.md)
cd my-project && cctx admin init

# View all managed projects
cctx admin list

# Remove project from management
cd my-project && cctx admin unlink

# Complete cleanup: remove all files, directories, and config entries
cd my-project && cctx admin reset project
```

```bash
# Create global contexts for common scenarios
cctx admin global init script    # Shell script guidelines
cctx admin global init python    # Python conventions
cctx admin global init security  # Security review checklist

# Link to projects that need them
cd my-app && cctx admin global link python security

# Update once, applies everywhere
vim ~/.cctx/contexts/_global/python.md
```

### Health & Maintenance

```bash
# Check for broken symlinks
cctx admin verify
cctx admin verify --fix  # Auto-repair

# Clean up orphaned data
cctx admin cleanup --dry-run
cctx admin cleanup  # Delete orphans
```

### Reset & Complete Cleanup

```bash
# Reset a single project (removes everything)
cctx admin reset project my-project
# Removes:
# - All files from project directory (symlinks, tickets)
# - Project context from ~/.cctx/contexts/my-project/
# - Related ticket directories
# - Project entry from config.json (if registered)
# - All ticket linkages

# Clean up current directory (even if not registered)
cd some-project && cctx admin reset project
# Works even if project isn't in config.json
# Cleans up any cctx-managed files (tickets, symlinks)

# Reset all projects (nuclear option)
cctx admin reset all
# Removes all files from all projects + entire ~/.cctx directory

# Keep config but remove all contexts
cctx admin reset all --keep-config

# Preview changes first (always recommended)
cctx admin reset project --dry-run -v
```

## Documentation

- **[TUTORIAL.md](TUTORIAL.md)**: Detailed usage guide with examples
- **[CLAUDE.md](CLAUDE.md)**: Architecture and development guide
- **[Atlassian MCP Integration](atlassian_mcp_integration.md)**: Jira ticket integration setup

## Installation

**Prerequisites:** Go 1.21+ (for building from source)

```bash
# Clone and install
git clone https://github.com/yourusername/claude-context.git
cd claude-context
make install

# Clear shell cache (important!)
hash -r  # or: rehash

# Add ~/bin to PATH (add to ~/.zshrc or ~/.bashrc)
export PATH="$HOME/bin:$PATH"

# Initialize data directory
cctx admin init
```

**That's it.** You're ready to go.

### Alternative: System-wide Install

```bash
sudo make install-global  # Installs to /usr/local/bin
```

### Other Commands

```bash
make build       # Build binary only
make test        # Run tests
make check       # Run all checks (fmt + vet + test)
make clean       # Clean build artifacts
make uninstall   # Remove from ~/bin
```

## Shell Completion (Optional but Recommended)

Enable tab completion for `cctx` commands:

**Zsh (macOS):**
```bash
cctx admin completion zsh > $(brew --prefix)/share/zsh/site-functions/_cctx
exec zsh
```

**Zsh (Linux):**
```bash
cctx admin completion zsh > "${fpath[1]}/_cctx"
source ~/.zshrc
```

**Bash:**
```bash
cctx admin completion bash > ~/.bash_completion.d/cctx
source ~/.bashrc
```

**Fish:**
```bash
cctx admin completion fish > ~/.config/fish/completions/cctx.fish
```

**Tip:** Type `cctx <TAB>` to see available commands. See [TUTORIAL.md](TUTORIAL.md#shell-completion-usage) for details.

## Configuration

### Directory Structure

All data is stored in `~/.cctx/` (or custom location via `CCTX_DATA_DIR`):

```
~/.cctx/
├── config.json              # All configuration, including archive records
├── contexts/
│   ├── project-name/
│   │   └── claude.md       # Project-specific context (legacy feature)
│   ├── _global/            # Global contexts (shared)
│   ├── _tickets/           # Symlinks for legacy single-file tickets
│   └── _archived/          # One directory per archive run
│       └── 2026-10-12_CBP-123/
└── templates/              # User template overrides (optional)
```

Ticket workspaces live in the project, next to your code but git-ignored:

```
my-project/
├── .cctx/
│   ├── mission.md          # from cctx init-specs
│   ├── tech-stack.md
│   ├── roadmap.md
│   └── CBP-123/            # from cctx ticket create
│       ├── requirements.md
│       ├── plan.md
│       ├── validation.md
│       └── changelog.md
└── ... (your code)
```

### Settings

Edit `~/.cctx/config.json`:

```json
{
  "settings": {
    "backup_on_unlink": true  // Create .bak files before deletion
  }
}
```

### Templates

Templates are embedded in the binary and update when you update the tool.

**Source in repo:** `cli/internal/templates/*.md`, embedded via `//go:embed` in `cli/internal/templates/embed.go`.

**Template Priority:**
1. User overrides: `~/.cctx/templates/` (if file exists)
2. Embedded defaults: Built into binary (repository source of truth)

**Customizing Templates:**

```bash
# View available templates
cctx admin global templates list

# View template content
cctx admin global templates show default
cctx admin global templates show ticket-requirements
cctx admin global templates show specs-roadmap

# Create custom override
vim ~/.cctx/templates/default.md
```

User templates override embedded ones with the same name.

### Custom Data Directory

Use a custom location for data storage:

```bash
# Using environment variable (recommended)
export CCTX_DATA_DIR=/path/to/your/data
cctx admin init

# Or using flag
cctx --data-dir /path/to/data init
```

## Integrating with Claude Code
To get full value from cctx, configure Claude Code to automatically work with ticket files and session logs.

### 1. Auto-permissions for Workspace Files

Add to `~/.claude/settings.json`:

```json
{
  "alwaysAllow": {
    "Read": ["**/.cctx/**"],
    "Edit": ["**/.cctx/**"]
  }
}
```

**Why**: Eliminates permission prompts when Claude reads/updates the spec files.

### 2. Context Policies

Add to `~/.claude/CLAUDE.md`:

```markdown
## Ticket Workspace Policy

On a branch whose last path segment is a ticket ID (e.g. `feature/CBP-1234`):
1. Run `cctx ticket show <ID>`; use the directory on its `Workspace:` line.
2. Treat `requirements.md` there as the source of truth; fill it from Jira via the
   Atlassian MCP server if it is still empty.
3. After each distinct piece of work, append an entry (≤30 lines) to `changelog.md`.
   Append-only — never modify previous entries.
```

**Note**: Fetching Jira details requires the Atlassian MCP server. See
[atlassian_mcp_integration.md](./atlassian_mcp_integration.md) for setup.

## Troubleshooting

### "Data directory not initialized"

```bash
cctx admin init
```

### Old Binary Cached After Update

After `make install`, your shell may cache the old binary location. Clear the cache:

```bash
hash -r         # For bash/zsh
rehash          # For zsh (alternative)
```

**Symptoms**: Commands show old behavior even after successful install.

### Broken Symlinks

```bash
# Check health
cctx admin verify

# Auto-repair
cctx admin verify --fix
```

### Orphaned Symlinks or Stale Data

```bash
# Preview what would be cleaned
cctx admin cleanup --dry-run --verbose

# Option 1: Delete orphaned items
cctx admin cleanup

# Option 2: Restore orphaned items to config.json
cctx admin cleanup --restore
```

### Command Not Found

Ensure `~/bin` is in your PATH:

```bash
export PATH="$HOME/bin:$PATH"
```

Add to `~/.zshrc` or `~/.bashrc` to make permanent.

## For Developers

Want to contribute or modify cctx? See [CLAUDE.md](CLAUDE.md) for:
- Architecture and design decisions
- Development patterns
- Command implementation guide
- Testing approach

**Key commands:**
```bash
make build      # Build binary
make test       # Run tests
make check      # Format, vet, test
make install    # Install to ~/bin
```

## Support & Resources

- **Tutorial**: [TUTORIAL.md](TUTORIAL.md) - Complete usage guide
- **Architecture**: [CLAUDE.md](CLAUDE.md) - Developer documentation
- **Jira Integration**: [atlassian_mcp_integration.md](./atlassian_mcp_integration.md)
- **Issues**: [GitHub Issues](https://github.com/yourusername/claude-context/issues)

## License

MIT