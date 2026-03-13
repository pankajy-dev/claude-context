# Claude Context Manager

**Stop copying the same instructions into every project. Manage your Claude context files in one place.**

## The Problem

You work with Claude Code across multiple projects. Each project needs context files (`claude.md`, custom guidelines, ticket templates). You end up:

- Copying the same instructions into every project
- Maintaining duplicates when you update your workflow
- Losing track of which projects have which context
- Manually managing ticket workspaces and session logs

**There has to be a better way.**

## The Solution

`cctx` gives you centralized context management:

- **Write once, use everywhere**: Create global contexts that work across all projects
- **Project-specific contexts**: Manage per-project `claude.md` files from one place
- **Ticket workflows**: Create temporary workspaces for Jira tickets with automatic session tracking
- **Zero maintenance**: Symlinks keep projects in sync automatically

## Who Should Use This

- **Individual developers** using Claude Code across multiple projects
- **Teams** wanting consistent Claude guidelines across their codebase
- **Anyone** tired of copying the same instructions between projects

## Quick Example

```bash
# Install and initialize
git clone https://github.com/yourusername/claude-context.git
cd claude-context && make install && cctx init

# Create a global "Python best practices" context
cctx global create python
# Edit ~/.cctx/contexts/_global/python.md with your guidelines

# Link it to your projects
cd ~/my-python-api && cctx global link python
cd ~/my-data-pipeline && cctx global link python

# Now both projects automatically include your Python guidelines
# Update once in ~/.cctx, both projects stay in sync
```

**That's it.** No more copying. No more drift.

## Common Workflows

### Managing Projects

```bash
# Add a project (creates claude.md)
cd my-project && cctx init

# View all managed projects
cctx project list

# Remove project from management
cd my-project && cctx unlink

# Complete cleanup: remove all files, directories, and config entries
cd my-project && cctx reset project
```

### Global Contexts (Share Across Projects)

```bash
# Create global contexts for common scenarios
cctx global create script    # Shell script guidelines
cctx global create python    # Python conventions
cctx global create security  # Security review checklist

# Link to projects that need them
cd my-app && cctx global link python security

# Update once, applies everywhere
vim ~/.cctx/contexts/_global/python.md
```

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

# Archive when done
cctx ticket archive PROJ-123
```

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
3. Update BEe-1234.md with requirements
4. Start working on the ticket
5. Log the session to SESSIONS.md automatically

**No manual copy-pasting. No context switching. Just work.**

### Health & Maintenance

```bash
# Check for broken symlinks
cctx verify
cctx verify --fix  # Auto-repair

# Clean up orphaned data
cctx cleanup --dry-run
cctx cleanup  # Delete orphans
```

### Reset & Complete Cleanup

```bash
# Reset a single project (removes everything)
cctx reset project my-project
# Removes:
# - All files from project directory (symlinks, tickets, .clauderc)
# - Project context from ~/.cctx/contexts/my-project/
# - Related ticket directories
# - Project entry from config.json (if registered)
# - All ticket linkages

# Clean up current directory (even if not registered)
cd some-project && cctx reset project
# Works even if project isn't in config.json
# Cleans up any cctx-managed files (tickets, symlinks, .clauderc)

# Reset all projects (nuclear option)
cctx reset all
# Removes all files from all projects + entire ~/.cctx directory

# Keep config but remove all contexts
cctx reset all --keep-config

# Preview changes first (always recommended)
cctx reset project --dry-run -v
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
cctx init
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
cctx completion zsh > $(brew --prefix)/share/zsh/site-functions/_cctx
exec zsh
```

**Zsh (Linux):**
```bash
cctx completion zsh > "${fpath[1]}/_cctx"
source ~/.zshrc
```

**Bash:**
```bash
cctx completion bash > ~/.bash_completion.d/cctx
source ~/.bashrc
```

**Fish:**
```bash
cctx completion fish > ~/.config/fish/completions/cctx.fish
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
cctx global templates list

# View template content
cctx global templates show default
cctx global templates show ticket
cctx global templates show sessions

# Create custom override
vim ~/.cctx/templates/default.md
```

User templates override embedded ones with the same name.

### Custom Data Directory

Use a custom location for data storage:

```bash
# Using environment variable (recommended)
export CCTX_DATA_DIR=/path/to/your/data
cctx init

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
```

**Why**:
- **Jira MCP Policy**: Auto-fetches ticket details from Jira (requires MCP server setup)
- **Session Logging**: Tracks work automatically in SESSIONS.md
- **Ticket Context Policy**: Ensures Claude reads ticket context before starting

**Note**: The Jira MCP Policy requires the Atlassian MCP server. See [atlassian_mcp_integration.md](./atlassian_mcp_integration.md) for MCP server setup instructions.

## Troubleshooting

### "Data directory not initialized"

```bash
cctx init
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
cctx verify

# Auto-repair
cctx verify --fix
```

### Orphaned Symlinks or Stale Data

```bash
# Preview what would be cleaned
cctx cleanup --dry-run --verbose

# Option 1: Delete orphaned items
cctx cleanup

# Option 2: Restore orphaned items to config.json
cctx cleanup --restore
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