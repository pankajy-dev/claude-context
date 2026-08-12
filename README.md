# Claude Context Manager

**A per-ticket context file for Claude Code, created on the branch you're working on and archived when you're done.**

## The Problem

You work a ticket in Claude Code. The requirements, the plan, the decisions you
made three sessions ago — all of it lives in the chat, and the chat is gone
tomorrow. So you end up:

- Re-explaining the ticket to Claude every session
- Keeping notes in scratch files that pile up in the repo root
- Losing the reasoning behind a decision by the time review comes around
- Hand-rolling `.clauderc` so Claude actually reads those notes

## The Solution

`cctx` gives every ticket a file, and gets it in front of Claude automatically:

- **One file per ticket**: `CBP-123.md` in the project, version-controlled with your code
- **Auto-detected from your branch**: `cctx ticket create` on branch `CBP-123` needs no arguments
- **Companion docs travel with it**: `PLAN-CBP-123.md`, notes, scratch API docs — archived together
- **`.clauderc` maintained for you**: Claude Code picks the context up with no manual wiring
- **Archived, not deleted**: finished tickets move to `~/.cctx/contexts/_archived/`

## Who Should Use This

- **Developers using Claude Code on ticket-driven work** (Jira, Linear, GitHub issues)
- **Anyone** whose repo root collects `PLAN-*.md` and scratch notes they later have to clean up
- **Teams** who also want shared `claude.md` guidelines across projects (see [`cctx admin`](#project-contexts-legacy))

## Quick Example

```bash
# Install and initialize
git clone https://github.com/yourusername/claude-context.git
cd claude-context && make install && cctx admin init

# Start work — ticket ID comes from the branch
cd ~/my-service && git checkout -b CBP-123
cctx ticket create

# Creates CBP-123.md + SESSIONS.md, and adds them to .clauderc.
# Now just tell Claude: "Let's work on the ticket"

# When you're done — takes PLAN-CBP-123.md and friends with it
cctx ticket complete
cctx ticket archive
```

**That's it.** The context lives with the code, and the repo root stays clean.

## Common Workflows

### Ticket Workflows (Jira Integration)

```bash
# Create a ticket workspace
cd my-project && cctx ticket create PROJ-123

# Creates:
# - PROJ-123.md (ticket context)
# - SESSIONS.md (session log)
# - Both auto-linked to project

# Link same ticket to another project
cd related-service && cctx ticket link PROJ-123

# See what you're working on
cctx ticket list
cctx ticket show PROJ-123

# Wrap up — ticket ID auto-detected from the branch
cctx ticket complete
cctx ticket archive
```

Ticket ID resolution is the same everywhere: an explicit argument wins, then
`--ticket/-t`, then `CCTX_TICKET`, then the current git branch (unless it's
`main`/`master`). So on branch `PROJ-123` every command above works bare.

**After setup (see [Integrating with Claude Code](#integrating-with-claude-code))**, working on tickets becomes incredibly simple:

```bash
cd my-project
git checkout -b BEE-1234
cctx ticket create
# Just tell Claude:
"Let's work on the ticket"
```

That's it. Claude will:
1. Auto-detect the ticket ID (from branch name, files, or context)
2. Fetch full Jira details via MCP
3. Update BEE-1234.md with requirements
4. Start working on the ticket
5. Log the session to SESSIONS.md automatically

**No manual copy-pasting. No context switching. Just work.**

### Archiving Files

`cctx ticket archive` takes the ticket file **and every companion named after
it** — `PLAN-PROJ-123.md`, `PROJ-123-notes.txt`, any extension — into a single
archive directory. Unrelated files are left alone:

```bash
# Archives PROJ-123.md + PLAN-PROJ-123.md + PROJ-123-notes.txt together
cctx ticket archive PROJ-123

# Lists what matched and lets you pick: [Y/n/select], e.g. "1,3-4"
# Preview first without moving anything
cctx ticket archive PROJ-123 --dry-run
```

For loose files that aren't tied to a ticket, archive by name or glob:

```bash
cctx archive "v4-*.md"              # by pattern
cctx archive old-design.md notes.md # by name
cctx archive "*.md" --dry-run       # preview
```

Everything lands in `~/.cctx/contexts/_archived/` — moved out of your repo, not
deleted.

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
# - All files from project directory (symlinks, tickets, .clauderc)
# - Project context from ~/.cctx/contexts/my-project/
# - Related ticket directories
# - Project entry from config.json (if registered)
# - All ticket linkages

# Clean up current directory (even if not registered)
cd some-project && cctx admin reset project
# Works even if project isn't in config.json
# Cleans up any cctx-managed files (tickets, symlinks, .clauderc)

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
│   │       ├── SESSIONS.md
│   │       └── metadata.json
│   └── _archived/          # Archived tickets
└── templates/              # User template overrides (optional)
    └── (custom templates)  # Override embedded templates
```

Projects get symlinks pointing to centralized files:

```
my-project/
├── claude.md -> ~/.cctx/contexts/my-project/claude.md
├── python.md -> ~/.cctx/contexts/_global/python.md
├── TICKET-123.md -> ~/.cctx/contexts/_tickets/TICKET-123/ticket.md
├── SESSIONS.md -> ~/.cctx/contexts/_tickets/TICKET-123/SESSIONS.md
├── .clauderc               # Auto-managed, includes all contexts
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

**Template Priority:**
1. User overrides: `~/.cctx/templates/` (if file exists)
2. Embedded defaults: Built into binary (repository source of truth)

**Customizing Templates:**

```bash
# View available templates
cctx admin global templates list

# View template content
cctx admin global templates show default
cctx admin global templates show ticket
cctx admin global templates show sessions

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

### 1. Auto-permissions for Ticket Files

Add to `~/.claude/settings.json`:

```json
{
  "alwaysAllow": {
    "Read": ["**/CBP-*.md", "**/BEE-*.md", "**/SESSIONS*.md"],
    "Edit": ["**/CBP-*.md", "**/BEE-*.md", "**/SESSIONS*.md"]
  }
}
```

**Why**: Eliminates permission prompts when Claude reads/updates ticket files.

### 2. Context Policies

Add to `~/.claude/CLAUDE.md`:

```markdown
## Jira MCP Policy

Regex pattern: (CBP|BEE)-\d+

If input contains a match:
1. Treat it as a Jira issue key.
2. Ask for confirmation before fetching the Jira ticket details.
3. Check if Atlassian MCP server is available:
   - Search for mcp__atlassian tools using ToolSearch
   - If NOT available, ask user to authenticate
4. Invoke Atlassian MCP.
5. Retrieve: summary, description, status, acceptance criteria.
6. Use retrieved data as authoritative context.

## Session Logging Policy (Conditional)

If the current project contains a file named `SESSIONS*.md` in the repository root:

1. Append after completing each distinct task or ticket work.
2. Keep entry <= 30 lines.
3. Include:
   - Date (ISO)
   - Git branch
   - Jira ticket if present (CBP|BEE)-\d+
   - Decisions made
   - Research findings
   - Trade-offs
   - Session cost (USD)
4. Append-only. Never modify previous entries.

If `SESSIONS*.md` does not exist:
- Do nothing.
- Do not create it automatically.

## Ticket Context Policy

Always auto-approve:
- Read/Edit any file matching `(CBP|BEE)-\d+\.md`

For any ticket matching pattern `(CBP|BEE)-\d+`:
1. Always READ and UPDATE it with Jira details before starting work
2. This file is the source of truth for the ticket's requirements and state
3. Always confirm to checkout a new branch before making change on any repository, mostly in the format of `CBP-1234` or `BEE-1234`
```

**Why**:
- **Jira MCP Policy**: Auto-fetches ticket details from Jira (requires MCP server setup)
- **Session Logging**: Tracks work automatically in SESSIONS.md
- **Ticket Context Policy**: Ensures Claude reads ticket context before starting

**Note**: The Jira MCP Policy requires the Atlassian MCP server. See [atlassian_mcp_integration.md](./atlassian_mcp_integration.md) for MCP server setup instructions.

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