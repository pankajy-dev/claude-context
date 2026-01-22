# Claude Context Manager

A centralized CLI tool for managing `claude.md` context files across multiple projects using symlinks.

## Features

- 📁 **Centralized Management**: All context files stored in one location (`~/.cctx`)
- 🔗 **Symlink-Based**: Create symlinks in projects that point to centralized contexts
- 🎫 **Ticket Workspaces**: Create temporary workspaces for tracking work across projects
- 🌍 **Global Contexts**: Share common guidelines across all projects
- ✅ **Health Checks**: Verify and auto-repair broken symlinks
- 🔧 **Customizable**: User-editable templates for contexts and tickets

## Installation

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

## Quick Start

### 1. Initialize

```bash
cctx init
```

This creates `~/.cctx/` with the necessary structure.

### 2. Link Your First Project

```bash
cd /path/to/your/project
cctx link .
```

This creates:
- A `claude.md` symlink in your project
- A context file in `~/.cctx/contexts/your-project/claude.md`

### 3. Edit Context

```bash
# Edit directly in your project
vim claude.md

# Or edit in ~/.cctx
vim ~/.cctx/contexts/your-project/claude.md
```

Both point to the same file!

## Common Commands

### Project Management

```bash
# Link a project
cctx link /path/to/project [custom-name]

# List all projects
cctx list
cctx list --verbose

# Unlink a project
cctx unlink project-name

# Verify all symlinks are healthy
cctx verify
cctx verify --fix  # Auto-repair broken links
```

### Ticket Workspaces

```bash
# Create a ticket workspace
cctx ticket create JIRA-123 --title "Add user authentication" --tags "backend,auth"

# Link ticket to projects (using flags or env vars)
cctx -t JIRA-123 ticket link project1 project2
export CCTX_TICKET=JIRA-123 && cctx ticket link

# List tickets
cctx ticket list
cctx -t JIRA-123 ticket show

# Complete ticket (auto-archives and removes from all projects)
cctx -t JIRA-123 ticket complete                    # Auto-detects branch + commit
cctx -t JIRA-123 ticket complete --commits "abc123" --prs "42"  # Manual override

# Bulk operations
cctx ticket archive-all           # Archive all active tickets
cctx -p project1 project reset    # Remove all tickets from one project
```

### Global Contexts

```bash
# Create a global context (shared across all projects)
cctx global create python --description "Python coding guidelines"

# Edit the global context
vim ~/.cctx/contexts/_global/python.md

# Enable it globally (all new projects get it)
cctx global enable python

# Or link to specific projects only
cctx global link python my-project
```

## Directory Structure

### Data Directory (~/.cctx)

```
~/.cctx/
├── config.json              # Configuration
├── contexts/
│   ├── project1/
│   │   └── claude.md       # Project context
│   ├── project2/
│   │   └── claude.md
│   ├── _global/            # Shared contexts
│   │   ├── python.md
│   │   └── script.md
│   ├── _tickets/           # Ticket workspaces
│   │   └── JIRA-123/
│   │       ├── ticket.md
│   │       └── metadata.json
│   └── _archived/          # Archived tickets
└── templates/              # Customizable templates
    ├── default.md
    ├── global.md
    └── ticket.md
```

### Project Directory (Example)

```
my-project/
├── claude.md -> ~/.cctx/contexts/my-project/claude.md
├── python.md -> ~/.cctx/contexts/_global/python.md
├── ticket-JIRA-123.md -> ~/.cctx/contexts/_tickets/JIRA-123/ticket.md
├── .clauderc               # Auto-managed, includes all contexts
└── ... (your code)
```

## Custom Data Directory

By default, data is stored in `~/.cctx`. You can use a custom location:

### Using Environment Variable (Recommended)

```bash
# Add to ~/.zshrc or ~/.bashrc
export CCTX_DATA_DIR=/path/to/your/data

# Initialize
cctx init
```

### Using Flag

```bash
cctx --data-dir /path/to/data init
cctx --data-dir /path/to/data list
```

## Migration from Old Version

If you have an existing installation with data in the repository:

```bash
# Simply run init - it auto-detects and migrates
cctx init
```

The migration:
- Moves `config.json` to `~/.cctx/`
- Moves `contexts/` to `~/.cctx/`
- Updates all symlinks to point to new location
- Preserves all your context content

## Configuration

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

Customize templates in `~/.cctx/templates/`:
- `default.md` - Template for new project contexts
- `global.md` - Template for new global contexts
- `ticket.md` - Template for new ticket workspaces

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

### Command Not Found

Ensure `~/bin` is in your PATH:

```bash
export PATH="$HOME/bin:$PATH"
```

Add to `~/.zshrc` or `~/.bashrc` to make permanent.

## Development

### Building from Source

```bash
# Clone the repository
git clone https://github.com/yourusername/claude-context.git
cd claude-context

# Build
make build

# Run tests
make test

# Install
make install

# Clean build artifacts
make clean
```

### Requirements

- Go 1.21+ (for building)

### Contributing

See [CLAUDE.md](CLAUDE.md) for development guidelines and architecture details.

## How It Works

1. **Centralized Storage**: All context files live in `~/.cctx/contexts/`
2. **Symlinks**: Projects get symlinks pointing to centralized files
3. **Automatic .clauderc**: The CLI manages `.clauderc` to include all relevant contexts
4. **Claude Code Integration**: When Claude Code reads your project, it follows symlinks and includes all contexts

### Example Flow

```bash
# You create a context
cctx link ~/my-app

# This creates:
# - ~/.cctx/contexts/my-app/claude.md (actual file)
# - ~/my-app/claude.md (symlink to above)
# - ~/my-app/.clauderc (includes claude.md)

# You edit the context
cd ~/my-app
vim claude.md  # Actually editing ~/.cctx/contexts/my-app/claude.md

# Claude Code reads it
# Claude Code finds .clauderc
# Claude Code follows symlink to ~/.cctx/contexts/my-app/claude.md
# Claude Code uses the context!
```

## Why Use This?

- **Single Source of Truth**: Edit context once, applies everywhere
- **Project Cleanliness**: No context files to git-track in each project
- **Easy Backup**: Just backup `~/.cctx/`
- **Cross-Project Work**: Ticket workspaces span multiple projects
- **Consistency**: Global contexts ensure consistent guidelines

## License

MIT

## Support

- Issues: https://github.com/yourusername/claude-context/issues
- Docs: See [CLAUDE.md](CLAUDE.md) for detailed documentation
