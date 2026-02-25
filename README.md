# Claude Context Manager

A centralized CLI tool for managing `claude.md` context files across multiple projects using symlinks.

## Features

- 📁 **Centralized Management**: All context files stored in one location (`~/.cctx`)
- 🔗 **Symlink-Based**: Create symlinks in projects that point to centralized contexts
- 🎫 **Ticket Workspaces**: Create temporary workspaces for tracking work across projects
- 🌍 **Global Contexts**: Share common guidelines across all projects
- ✅ **Health Checks**: Verify and auto-repair broken symlinks
- 🧹 **Smart Cleanup**: Delete or restore orphaned symlinks and stale data
- 🔧 **Customizable**: User-editable templates for contexts and tickets

**Quick Links:**
- **[Tutorial](TUTORIAL.md)**: Complete usage guide with examples and workflows
- **[CLAUDE.md](CLAUDE.md)**: Development guidelines and architecture details

## Installation

### Prerequisites

- Go 1.21+ (for building from source)

### Quick Install

```bash
# Clone the repository
git clone https://github.com/yourusername/claude-context.git
cd claude-context

# Build and install to ~/bin
make install

# Add ~/bin to PATH if not already (add to ~/.zshrc or ~/.bashrc)
export PATH="$HOME/bin:$PATH"

# Initialize the data directory
cctx init
```

### Global Install (Optional)

```bash
# Install to /usr/local/bin (requires sudo)
make install-global
```

### Build Commands

```bash
# Build binary only
make build

# Run tests
make test

# Run all checks (fmt + vet + test)
make check

# Clean build artifacts
make clean

# Uninstall
make uninstall          # Remove from ~/bin
make uninstall-global   # Remove from /usr/local/bin
```

## Shell Completion Setup

Enable tab completion for `cctx` commands (highly recommended):

### Zsh (macOS with Homebrew)

```bash
# Install completion script
cctx completion zsh > $(brew --prefix)/share/zsh/site-functions/_cctx

# Add Homebrew completions to fpath (add to ~/.zshrc if not already present)
echo 'if type brew &>/dev/null; then' >> ~/.zshrc
echo '  FPATH="$(brew --prefix)/share/zsh/site-functions:${FPATH}"' >> ~/.zshrc
echo 'fi' >> ~/.zshrc

# Restart your shell
exec zsh

# Test: cctx <TAB> should show all commands
```

### Zsh (Linux)

```bash
cctx completion zsh > "${fpath[1]}/_cctx"
source ~/.zshrc
```

### Bash

```bash
# System-wide
cctx completion bash > /usr/local/etc/bash_completion.d/cctx

# Or user-level
mkdir -p ~/.bash_completion.d
cctx completion bash > ~/.bash_completion.d/cctx
echo 'source ~/.bash_completion.d/cctx' >> ~/.bashrc
source ~/.bashrc
```

### Fish

```bash
cctx completion fish > ~/.config/fish/completions/cctx.fish
# Completions are loaded automatically
```

**Tip:** Type `--` before pressing TAB to see flag completions. See [TUTORIAL.md](TUTORIAL.md#shell-completion-usage) for detailed usage.

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

## Recommended Claude Code Settings

For optimal integration with Claude Code, configure auto-permissions and policies.

### 1. Update `~/.claude/settings.json`

Add auto-permissions for ticket files:

```json
{
  "alwaysAllow": {
    "Read": [
      "**/CBP-*.md",
      "**/BEE-*.md",
      "**/SESSIONS*.md"
    ],
    "Edit": [
      "**/CBP-*.md",
      "**/BEE-*.md",
      "**/SESSIONS*.md"
    ],
    "Bash": [
      {
        "command": "ls -la CBP-*.md"
      },
      {
        "command": "ls -la BEE-*.md"
      }
    ]
  }
}
```

**Benefits:**
- Auto-approves Read/Edit operations on ticket files
- Auto-approves `ls -la` commands for symlink detection (Glob misses symlinks)
- Eliminates permission prompts for routine operations

### 2. Update `~/.claude/CLAUDE.md`

Add these policies to configure Claude's behavior with cctx-generated files:

```markdown
## Jira MCP Policy

Regex pattern:
(CBP|BEE)-\d+

If input contains a match:
1. Treat it as a Jira issue key.
2. Ask for the confirmation before fetching the Jira ticket details.
3. Invoke Atlassian MCP.
4. Retrieve: summary, description, status, acceptance criteria.
5. Use retrieved data as authoritative context.
6. Do not infer ticket details without MCP retrieval.

## Session Logging Policy (Conditional)

If the current project contains a file named `sessions*.md` in the repository root:

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

If `sessions*.md` does not exist:
- Do nothing.
- Do not create it automatically.

## Ticket Context Policy

### Auto-Permission Rules

Always auto-approve:
- Read/Edit any file matching `(CBP|BEE)-\d+\.md`
- Bash `ls -la` for ticket context checks

For any ticket matching pattern `(CBP|BEE)-\d+`:
1. Use Bash `ls -la {TICKET_KEY}.md` to check if symlink exists (Glob misses symlinks)
2. Always READ and UPDATE it with Jira details before starting work
3. This file is the source of truth for the ticket's requirements and state
```

**What These Policies Do:**
- **Jira MCP Policy**: Automatically detects Jira ticket IDs and fetches details via Atlassian MCP
- **Session Logging**: Tracks work sessions in SESSIONS.md files for documentation
- **Ticket Context Policy**: Ensures Claude always reads/updates ticket context files before starting work

**Jira MCP Server Integration**: See [Atlassian MCP Server Documentation](https://github.com/anthropics/anthropic-tools/tree/main/mcp/atlassian) for setting up Jira integration with Claude Code.

## Migration from Old Version

If you have an existing installation with data in the repository:

```bash
# Simply run init - it auto-detects and migrates
cctx init
```

Migration moves:
- `config.json` → `~/.cctx/config.json`
- `contexts/` → `~/.cctx/contexts/`
- `templates/` → `~/.cctx/templates/`
- Updates all symlinks automatically

## Troubleshooting

### "Data directory not initialized"

```bash
cctx init
```

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

### Compilation Errors

After code changes:

```bash
# Rebuild
make build

# Run all checks
make check
```

## Development

### Requirements

- Go 1.21+

### Building from Source

```bash
# Clone repository
git clone https://github.com/yourusername/claude-context.git
cd claude-context

# Build
make build

# Run tests
make test

# Format code
make fmt

# Run go vet
make vet

# Run all checks (fmt + vet + test)
make check

# Install
make install

# Clean build artifacts
make clean
```

### Key Files

- **`cli/internal/config/config.go`**: Core data structures
- **`cli/cmd/*.go`**: Command implementations
- **`cli/main.go`**: Entry point
- **`cli/internal/templates/`**: Embedded templates

### Contributing

See [CLAUDE.md](CLAUDE.md) for:
- Architecture details
- Development patterns
- Design decisions
- Command implementation guide

## Documentation

- **[TUTORIAL.md](TUTORIAL.md)**: Complete usage guide with examples and workflows
- **[CLAUDE.md](CLAUDE.md)**: Development guidelines and architecture details

## License

MIT

## Support

- **Issues**: https://github.com/yourusername/claude-context/issues
- **Tutorial**: See [TUTORIAL.md](TUTORIAL.md)
- **Development**: See [CLAUDE.md](CLAUDE.md)