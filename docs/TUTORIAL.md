# Claude Context Manager - Tutorial

Complete guide to using `cctx` for managing context files across your projects.

## Quick Start

### 1. Initialize

```bash
cctx admin init
```

This creates `~/.cctx/` with the necessary structure.

### 2. Link Your First Project

```bash
cd /path/to/your/project
cctx admin link .
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

## Shell Completion Usage

Once installed, shell completion helps you discover commands, flags, and options.

### Complete Commands

```bash
cctx <TAB>
# Shows: cleanup  completion  global  home  init  link  list  ticket  unlink  verify
```

### Complete Subcommands

```bash
cctx ticket <TAB>
# Shows: archive-all  complete  create  delete  link  list  show

cctx admin global <TAB>
# Shows: create  disable  enable  link  list  unlink
```

### Complete Flags (type -- first)

```bash
cctx admin cleanup --<TAB>
# Shows: --data-dir  --dry-run  --force  --help  --project  --restore  --ticket  --verbose

cctx --<TAB>
# Shows global flags: --data-dir  --dry-run  --help  --project  --ticket  --verbose

cctx ticket create --<TAB>
# Shows: --help  --tags  --title
```

### Complete Short Flags (type - first)

```bash
cctx -<TAB>
# Shows: -d  -h  -p  -t  -v

cctx admin cleanup -<TAB>
# Shows: -f  -h  -r  (plus global: -d, -p, -t, -v)
```

### Partial Matching

```bash
cctx cle<TAB>
# Completes to: cctx admin cleanup

cctx admin cleanup --fo<TAB>
# Completes to: cctx admin cleanup --force

cctx ticket cr<TAB>
# Completes to: cctx ticket create
```

### Smart Context Completion

```bash
cctx admin unlink <TAB>
# Shows your managed project names

cctx -p <TAB>
# Shows your managed project names

cctx -t <TAB>
# Shows your active ticket IDs

cctx admin global link <TAB>
# Shows available global context names
```

**Tip:** Always type `--` before pressing TAB to see flag completions. Without the prefix, the shell shows file completions instead.

## Project Management

### Linking Projects

```bash
# Link current directory
cctx admin link .

# Link specific project with auto-generated name
cctx admin link /path/to/project

# Link with custom name
cctx admin link /path/to/project my-custom-name
```

### Listing Projects

```bash
# Simple list
cctx admin list

# Detailed information
cctx admin list --verbose
```

### Unlinking Projects

```bash
# Unlink and delete context file
cctx admin unlink project-name

# Unlink but keep the context file in ~/.cctx
cctx admin unlink project-name --keep-content
```

### Health Checks

```bash
# Check all symlinks
cctx admin verify

# Auto-repair broken symlinks
cctx admin verify --fix
```

### Cleanup

```bash
# Interactive cleanup (shows both delete and restore options)
cctx admin cleanup

# Preview without making changes
cctx admin cleanup --dry-run --verbose

# Delete orphaned items (skip confirmation)
cctx admin cleanup --force

# Restore orphaned items to config.json
cctx admin cleanup --restore
cctx admin cleanup --restore --force
```

## Ticket Workspaces

### Creating Tickets

```bash
# Create with explicit ticket ID
cctx ticket create JIRA-123 --title "Add user authentication" --tags "backend,auth"

# Auto-detect ticket ID from current git branch (recommended)
git checkout -b CBP-33146
cctx ticket create --title "Add feature"  # Uses "CBP-33146" as ticket ID

# If ticket exists, suffix is auto-appended: CBP-33146-1, CBP-33146-2, etc.
```

This automatically creates:
- `ticket.md` - Ticket context file
- `SESSIONS.md` - Session tracking file
- Symlinks in your project directory

### Linking Tickets to Projects

```bash
# Using flags
cctx -t JIRA-123 ticket link project1 project2
cctx -t JIRA-123 -p project1 ticket link

# Using environment variables (recommended for workflow)
export CCTX_TICKET=JIRA-123
export CCTX_PROJECT=project1

# Now commands are simpler
cctx ticket link                        # Links JIRA-123 to project1
cctx ticket list                        # List tickets for project1
cctx -p project2 ticket link           # Also link to project2
```

### Showing Ticket Details

```bash
# Auto-detect from current branch
cctx ticket show

# Or specify explicitly
cctx -t JIRA-123 ticket show
```

### Completing Tickets

```bash
# Auto-detect from branch and commit from current dir
cctx ticket complete

# Manual override
cctx ticket complete --commits "abc123,def456" --prs "42,43"
```

This automatically:
- Archives the ticket
- Removes symlinks from all projects
- Moves to `~/.cctx/contexts/_archived/`

### Bulk Operations

```bash
# Archive all active tickets (all projects)
cctx ticket archive-all

# Skip confirmation
cctx ticket archive-all --force

# Archive all tickets from specific project
cctx -p project1 ticket archive-all
export CCTX_PROJECT=project1 && cctx ticket archive-all
```

### Deleting Archived Tickets

```bash
# Permanently delete an archived ticket
cctx -t JIRA-123 ticket delete --force
```

## Global Contexts

### Creating Global Contexts

```bash
# Create a new global context
cctx admin global init python --description "Python coding guidelines"
cctx admin global init script --description "Shell scripting guidelines"
```

### Editing Global Contexts

```bash
# Edit the global context file
vim ~/.cctx/contexts/_global/python.md
```

### Enabling/Disabling Global Contexts

```bash
# Enable globally (all new projects get it)
cctx admin global enable python

# Disable globally
cctx admin global disable python
```

### Linking Global Contexts to Projects

```bash
# Link to specific projects only
cctx admin global link python my-project
cctx admin global link script my-project another-project

# Unlink from a project
cctx admin global unlink python my-project
```

### Listing Global Contexts

```bash
# Simple list
cctx admin global list

# Detailed information
cctx admin global list --verbose
```

## Working with Templates

### Listing Templates

```bash
# List all available templates
cctx admin global templates list

# Show template sources (user/embedded) and paths
cctx admin global templates list --verbose
```

### Viewing Template Content

```bash
# View templates
cctx admin global templates show default       # Default project template
cctx admin global templates show global        # Global context template
cctx admin global templates show ticket        # Ticket workspace template
cctx admin global templates show sessions      # Session tracking template
cctx admin global templates show script        # Script writing guidelines
```

### Customizing Templates

```bash
# Copy embedded template for customization
cp $(cctx admin home)/embedded-template.md ~/.cctx/templates/custom-name.md

# Or manually create in ~/.cctx/templates/
vim ~/.cctx/templates/my-custom.md
```

**Template Priority:**
1. User overrides: `~/.cctx/templates/` (if file exists)
2. Embedded defaults: Built into binary from `cli/internal/templates/`

## Navigating to Data Directory

### Using the home Command

```bash
# Print path to data directory
cctx admin home

# Navigate to data directory
cd $(cctx admin home)

# Show directory tree
cctx admin home --list

# Open in file manager
cctx admin home --open

# Open new shell in data directory
cctx admin home --shell

# Useful patterns
cd $(cctx admin home)/contexts/_tickets    # Navigate to tickets
ls $(cctx admin home)/contexts              # List all contexts
```

### Manual Navigation

```bash
# Navigate to data directory
cd $(cctx admin home)

# List contexts
ls $(cctx admin home)/contexts

# Navigate to specific ticket
cd $(cctx admin home)/contexts/_tickets/JIRA-123

# Edit context file
vim $(cctx admin home)/contexts/my-project/claude.md

# Remove archived ticket
rm -rf $(cctx admin home)/contexts/_archived/old-ticket
```

## Environment Variables and Flags

### Using Environment Variables

```bash
# Set ticket context
export CCTX_TICKET=JIRA-123

# Set project context
export CCTX_PROJECT=my-project

# Set custom data directory
export CCTX_DATA_DIR=/custom/path

# Now commands use these defaults
cctx ticket show                    # Uses JIRA-123
cctx ticket link                    # Links JIRA-123 to my-project
cctx admin list                           # Uses /custom/path
```

### Using Flags

```bash
# Override ticket
cctx -t JIRA-123 ticket show

# Override project
cctx -p my-project ticket list

# Override data directory
cctx --data-dir /custom/path list

# Combine multiple flags
cctx -t JIRA-123 -p my-project ticket link
```

**Priority:** Flags > Environment Variables > Auto-detection

## Session Tracking

Each ticket workspace automatically includes a `SESSIONS.md` file for tracking interactions with Claude Code.

### Workflow

1. Create a ticket: `cctx ticket create JIRA-123 --title "Add feature"`
2. `SESSIONS.md` is automatically created and symlinked
3. Document each session in `SESSIONS.md` as you work
4. Include: what was done, key decisions, issues resolved, files changed

### Example Session Entry

```markdown
## Session 2026-02-19: Added Cleanup Command

### Summary
Created cleanup command with delete and restore modes.

### What Was Done
- Implemented cctx admin cleanup with --restore flag
- Fixed shell completion ordering in zshrc
- Updated documentation

### Key Decisions
- **Decision**: Two-mode approach (delete vs restore)
  - **Rationale**: Gives users flexibility to recover accidentally removed items

### Commands Used
```bash
cctx admin cleanup --dry-run --verbose
cctx admin cleanup --restore
```
```

## How It Works

### Centralized Storage

All context files live in `~/.cctx/contexts/`:

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
│   │       ├── SESSIONS.md
│   │       └── metadata.json
│   └── _archived/          # Archived tickets
└── templates/              # Customizable templates
```

### Project Structure

Projects get symlinks pointing to centralized files:

```
my-project/
├── claude.md -> ~/.cctx/contexts/my-project/claude.md
├── python.md -> ~/.cctx/contexts/_global/python.md
├── JIRA-123.md -> ~/.cctx/contexts/_tickets/JIRA-123/ticket.md
├── SESSIONS.md -> ~/.cctx/contexts/_tickets/JIRA-123/SESSIONS.md
└── ... (your code)
```

### Example Flow

```bash
# You create a context
cctx admin link ~/my-app

# This creates:
# - ~/.cctx/contexts/my-app/claude.md (actual file)
# - ~/my-app/claude.md (symlink to above)

# You edit the context
cd ~/my-app
vim claude.md  # Actually editing ~/.cctx/contexts/my-app/claude.md

# Claude Code reads it
# Claude Code follows symlink to ~/.cctx/contexts/my-app/claude.md
# Claude Code uses the context!
```

## Best Practices

### For Projects

1. **Link once, use everywhere**: Link your project, then just work in it
2. **Use global contexts**: Share common guidelines across projects
3. **Keep contexts updated**: Edit context files as your project evolves

### For Tickets

1. **Use environment variables**: Set `CCTX_TICKET` and `CCTX_PROJECT` for easier workflow
2. **Auto-detect from branch**: Create feature branches with ticket IDs for automatic detection
3. **Track sessions**: Document your work in `SESSIONS.md` for future reference
4. **Complete tickets**: Use `cctx ticket complete` to auto-archive and clean up

### For Maintenance

1. **Run verify regularly**: `cctx admin verify --fix` to keep symlinks healthy
2. **Clean up orphans**: `cctx admin cleanup --dry-run --verbose` to preview, then clean
3. **Backup your data**: Regularly backup `~/.cctx/` directory

## Why Use This?

- **Single Source of Truth**: Edit context once, applies everywhere
- **Project Cleanliness**: No context files to git-track in each project
- **Easy Backup**: Just backup `~/.cctx/`
- **Cross-Project Work**: Ticket workspaces span multiple projects
- **Consistency**: Global contexts ensure consistent guidelines
- **Seamless Integration**: Works transparently with Claude Code

## Common Workflows

### Starting a New Feature

```bash
# Create feature branch (ticket ID in name)
git checkout -b JIRA-123-add-auth

# Create ticket workspace (auto-detects JIRA-123)
cctx ticket create --title "Add authentication" --tags "backend,security"

# Link to current project (auto-detects from current directory)
cctx ticket link

# Start working - Claude Code now has ticket context
```

### Working on Multiple Projects

```bash
# Set ticket once
export CCTX_TICKET=JIRA-123

# Link to multiple projects
cctx -p frontend ticket link
cctx -p backend ticket link
cctx -p api ticket link

# Work in any project - ticket context is available everywhere
cd ~/projects/frontend
# JIRA-123.md symlink is present

cd ~/projects/backend
# JIRA-123.md symlink is also present here
```

### Sprint Cleanup

```bash
# Archive all completed tickets from current project
export CCTX_PROJECT=my-project
cctx ticket archive-all

# Or all tickets from all projects
cctx ticket archive-all --force
```

## Advanced Usage

### Custom Data Directory

```bash
# Set custom location
export CCTX_DATA_DIR=/mnt/shared/contexts

# Initialize
cctx admin init

# All commands now use custom location
cctx admin link .
cctx admin list
```

### Migrating from Old Version

```bash
# Simply run init - auto-detects and migrates
cctx admin init

# Migration moves:
# - config.json → ~/.cctx/config.json
# - contexts/ → ~/.cctx/contexts/
# - Updates all symlinks automatically
```

## Tips and Tricks

### Use Shell Completion

Install shell completion for a much better experience:
- Type `--` before TAB for flags
- Partial matching works (e.g., `cle<TAB>` → `cleanup`)
- Context-aware completion for project names and ticket IDs

### Use Environment Variables

Set common variables in your shell config:

```bash
# In ~/.zshrc or ~/.bashrc
export CCTX_PROJECT=my-main-project
export CCTX_DATA_DIR=/custom/path
```

### Use Dry Run

Always preview before destructive operations:

```bash
cctx admin cleanup --dry-run --verbose
cctx ticket archive-all --dry-run
```

### Use Restore Instead of Delete

If you accidentally removed entries from config.json:

```bash
cctx admin cleanup --restore --dry-run  # Preview
cctx admin cleanup --restore            # Restore
```