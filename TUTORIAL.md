# Claude Context Manager - Tutorial

Complete guide to using `cctx` for managing context files across your projects.

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

cctx global <TAB>
# Shows: create  disable  enable  link  list  unlink
```

### Complete Flags (type -- first)

```bash
cctx cleanup --<TAB>
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

cctx cleanup -<TAB>
# Shows: -f  -h  -r  (plus global: -d, -p, -t, -v)
```

### Partial Matching

```bash
cctx cle<TAB>
# Completes to: cctx cleanup

cctx cleanup --fo<TAB>
# Completes to: cctx cleanup --force

cctx ticket cr<TAB>
# Completes to: cctx ticket create
```

### Smart Context Completion

```bash
cctx unlink <TAB>
# Shows your managed project names

cctx -p <TAB>
# Shows your managed project names

cctx -t <TAB>
# Shows your active ticket IDs

cctx global link <TAB>
# Shows available global context names
```

**Tip:** Always type `--` before pressing TAB to see flag completions. Without the prefix, the shell shows file completions instead.

## Project Management

### Linking Projects

```bash
# Link current directory
cctx link .

# Link specific project with auto-generated name
cctx link /path/to/project

# Link with custom name
cctx link /path/to/project my-custom-name
```

### Listing Projects

```bash
# Simple list
cctx list

# Detailed information
cctx list --verbose
```

### Unlinking Projects

```bash
# Unlink and delete context file
cctx unlink project-name

# Unlink but keep the context file in ~/.cctx
cctx unlink project-name --keep-content
```

### Health Checks

```bash
# Check all symlinks
cctx verify

# Auto-repair broken symlinks
cctx verify --fix
```

### Cleanup

```bash
# Interactive cleanup (shows both delete and restore options)
cctx cleanup

# Preview without making changes
cctx cleanup --dry-run --verbose

# Delete orphaned items (skip confirmation)
cctx cleanup --force

# Restore orphaned items to config.json
cctx cleanup --restore
cctx cleanup --restore --force
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
cctx global create python --description "Python coding guidelines"
cctx global create script --description "Shell scripting guidelines"
```

### Editing Global Contexts

```bash
# Edit the global context file
vim ~/.cctx/contexts/_global/python.md
```

### Enabling/Disabling Global Contexts

```bash
# Enable globally (all new projects get it)
cctx global enable python

# Disable globally
cctx global disable python
```

### Linking Global Contexts to Projects

```bash
# Link to specific projects only
cctx global link python my-project
cctx global link script my-project another-project

# Unlink from a project
cctx global unlink python my-project
```

### Listing Global Contexts

```bash
# Simple list
cctx global list

# Detailed information
cctx global list --verbose
```

## Working with Templates

### Listing Templates

```bash
# List all available templates
cctx global templates list

# Show template sources (user/embedded) and paths
cctx global templates list --verbose
```

### Viewing Template Content

```bash
# View templates
cctx global templates show default       # Default project template
cctx global templates show global        # Global context template
cctx global templates show ticket        # Ticket workspace template
cctx global templates show sessions      # Session tracking template
cctx global templates show script        # Script writing guidelines
```

### Customizing Templates

```bash
# Copy embedded template for customization
cp $(cctx home)/embedded-template.md ~/.cctx/templates/custom-name.md

# Or manually create in ~/.cctx/templates/
vim ~/.cctx/templates/my-custom.md
```

**Template Priority:**
1. User overrides: `~/.cctx/templates/` (if file exists)
2. Embedded defaults: Built into binary (repository source of truth)

## Navigating to Data Directory

### Using Shell Functions (Recommended)

First, source the shell functions:

```bash
# Add to ~/.bashrc or ~/.zshrc
source /path/to/claude-context/cctx-shell-functions.sh
```

Then use:

```bash
# Actually cd to data directory (not just print path)
cctx home

# Show directory tree
cctx home --list

# Open in file manager
cctx home --open

# Open new shell in data directory
cctx home --shell

# Convenience aliases
cdhome        # Alias to cd to data directory
cdtickets     # cd to tickets directory
cdcontexts    # cd to contexts directory
```

### Manual Navigation

```bash
# Navigate to data directory
cd $(cctx home)

# List contexts
ls $(cctx home)/contexts

# Navigate to specific ticket
cd $(cctx home)/contexts/_tickets/JIRA-123

# Edit context file
vim $(cctx home)/contexts/my-project/claude.md

# Remove archived ticket
rm -rf $(cctx home)/contexts/_archived/old-ticket
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
cctx list                           # Uses /custom/path
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
- Implemented cctx cleanup with --restore flag
- Fixed shell completion ordering in zshrc
- Updated documentation

### Key Decisions
- **Decision**: Two-mode approach (delete vs restore)
  - **Rationale**: Gives users flexibility to recover accidentally removed items

### Commands Used
```bash
cctx cleanup --dry-run --verbose
cctx cleanup --restore
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
├── .clauderc               # Auto-managed, includes all contexts
└── ... (your code)
```

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

### .clauderc Management

The `.clauderc` file is automatically managed by `cctx`:

```yaml
# Auto-generated by cctx - DO NOT EDIT MANUALLY
include:
  - claude.md
  - python.md
  - JIRA-123.md
  - SESSIONS.md
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

1. **Run verify regularly**: `cctx verify --fix` to keep symlinks healthy
2. **Clean up orphans**: `cctx cleanup --dry-run --verbose` to preview, then clean
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
cctx init

# All commands now use custom location
cctx link .
cctx list
```

### Migrating from Old Version

```bash
# Simply run init - auto-detects and migrates
cctx init

# Migration moves:
# - config.json → ~/.cctx/config.json
# - contexts/ → ~/.cctx/contexts/
# - templates/ → ~/.cctx/templates/
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
cctx cleanup --dry-run --verbose
cctx ticket archive-all --dry-run
```

### Use Restore Instead of Delete

If you accidentally removed entries from config.json:

```bash
cctx cleanup --restore --dry-run  # Preview
cctx cleanup --restore            # Restore
```