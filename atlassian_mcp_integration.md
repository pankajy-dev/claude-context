# Atlassian MCP Server Integration

## References

- [MCP Server Documentation](https://code.claude.com/docs/en/mcp)
- [Atlassian Remote MCP Server Guide](https://support.atlassian.com/atlassian-rovo-mcp-server/docs/getting-started-with-the-atlassian-remote-mcp-server/)
- [Claude MCP Servers](https://code.claude.com/docs/en/mcpClaude)

## Setup Steps

### 1. Add MCP Server

**Manual Configuration** (global command doesn't work, add manually):

1. Open root settings file: `~/.claude/settings.json`
**Project-level** (alternative, adds to current project `.claude.json`):
```bash
claude mcp add --transport http atlassian https://mcp.atlassian.com/v1/mcp
```
2. Add the following block to the root level, generated using above cmd:

```json
"mcpServers": {
  "atlassian": {
    "type": "http",
    "url": "https://mcp.atlassian.com/v1/mcp"
  }
}
```
### 2. Verify Installation

```bash
claude mcp list
```

Or inside Claude:
```
/mcp
```

### 3. Authenticate

- Command will prompt for authentication
- Approve access when requested

### 4. Test Integration

Ask Claude to fetch a Jira ticket to verify working connection.
