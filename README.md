# Claude Context Manager (`cctx`)

**Keep ticket context, decisions, and working notes organized across Claude Code sessions and repositories.**

`cctx` is a command-line tool for ticket-driven development. It creates a context workspace for each ticket, links that context to the repositories where you work, and archives ticket-related files when the work is complete.

## AI-assisted development

This project was built with assistance from **Anthropic's Claude and Claude Code**. I used Claude to help with implementation, documentation, and development workflows, while guiding the design and reviewing the resulting changes. The project is also designed to improve how developers maintain context when working across Claude Code sessions.

## Why `cctx`?

When work spans multiple Claude Code sessions, requirements and decisions can get buried in chat history. `cctx` keeps them in files you can review and maintain.

- **Ticket context:** Keep requirements, decisions, and progress in a ticket-specific file.
- **Session logs:** Record findings and decisions across work sessions.
- **Companion files:** Keep plans and ticket-related notes together.
- **Multi-repository work:** Link the same ticket context to more than one repository.
- **Archiving:** Move completed ticket files out of the working tree without deleting them.
- **Claude Code skill:** Use a reusable skill to help load ticket context and follow repository-specific workflows.

`cctx` can be used with Jira, Linear, GitHub Issues, and other ticket-driven workflows. Ticket identifiers should follow the conventions used by your project.

## Quick start

### Requirements

- Go 1.21 or later to build from source
- Git
- Claude Code for the Claude Code integration and skill

### Install

```bash
git clone https://github.com/pankajy-dev/claude-context.git
cd claude-context
make install

# Add ~/bin to PATH if it is not already there.
export PATH="$HOME/bin:$PATH"

# Initialize cctx's data directory.
cctx admin init
```

If your shell caches command locations, refresh the cache:

```bash
hash -r   # bash and zsh
# or
rehash    # zsh
```

### Start working on a ticket

Create or switch to a branch that follows your project's naming convention, then create a ticket context:

```bash
cd ~/my-service
git checkout -b PROJ-123
cctx ticket create
```

This creates the ticket context and session log and links them to the project according to `cctx`'s configuration.

Then ask Claude Code to start work on the ticket. With the skill and any required integrations configured, Claude Code can use the context file to review requirements, record decisions, and maintain session notes.

### Finish and archive the ticket

```bash
cctx ticket complete
cctx ticket archive
```

Archiving moves the ticket context and matching companion files into the archive directory under `~/.cctx/contexts/_archived/`. Use a dry run to preview archive operations:

```bash
cctx ticket archive PROJ-123 --dry-run
```

## Common commands

| Command | Purpose |
|---|---|
| `cctx ticket create PROJ-123` | Create a ticket context |
| `cctx ticket link PROJ-123` | Link an existing ticket to another repository |
| `cctx ticket list` | List ticket workspaces |
| `cctx ticket show PROJ-123` | Show a ticket workspace |
| `cctx ticket complete` | Mark ticket work complete |
| `cctx ticket archive PROJ-123` | Archive ticket files |
| `cctx archive "*.md" --dry-run` | Preview archiving matching files |
| `cctx admin verify` | Check managed links |
| `cctx admin cleanup --dry-run` | Preview cleanup of orphaned data |

Ticket IDs can be supplied explicitly or resolved using the supported flags, environment variable, and current Git branch. See the tutorial for the full resolution order and command options.

## Claude Code integration

The `cctx` CLI manages ticket context; the Claude Code skill provides instructions for using that context during a work session. The skill is intended to be reusable across repositories and should follow each repository's own conventions rather than assume organization-specific ticket prefixes or policies.

See the [Claude Code skill](docs/SKILL.md) and [Claude Code integration guide](docs/claude-code-integration.md) for setup instructions. Jira issue retrieval requires the [Atlassian MCP integration](docs/atlassian_mcp_integration.md).

## Documentation

- [Tutorial](docs/TUTORIAL.md): Detailed workflows and command examples.
- [Claude Code skill](docs/SKILL.md): Session setup and ticket-context workflow.
- [Claude Code integration guide](docs/claude-code-integration.md): Configure Claude Code to use ticket files and session logs.
- [Atlassian MCP integration](docs/atlassian_mcp_integration.md): Set up Jira issue retrieval.
- [Architecture and development](CLAUDE.md): Design decisions, implementation guidance, and tests.

## Configuration and data

By default, `cctx` stores its configuration, contexts, templates, and archived tickets under `~/.cctx/`. Set `CCTX_DATA_DIR` to use a different location.

Templates are embedded in the binary. User overrides can be stored under `~/.cctx/templates/`.

See the [tutorial](docs/TUTORIAL.md) for configuration details, template customization, shell completion, project-context management, reset operations, and troubleshooting.

## Contributing

Contributions are welcome. Before making changes, read the development guide, run the relevant tests, and follow the conventions used by the repository.

```bash
make build
make test
make check
```

Report bugs or request features through [GitHub Issues](https://github.com/pankajy-dev/claude-context/issues).

## License

MIT
