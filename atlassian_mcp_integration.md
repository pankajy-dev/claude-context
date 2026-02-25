# Atlassian MCP Server Integration

## References

- [Atlassian Remote MCP Server Guide](https://support.atlassian.com/atlassian-rovo-mcp-server/docs/getting-started-with-the-atlassian-remote-mcp-server/)
- [Claude MCP Servers](https://code.claude.com/docs/en/mcpClaude)

## Setup Steps

### 1. Add MCP Server

1. Run the command below in a terminal.
```bash
claude mcp add --transport http atlassian https://mcp.atlassian.com/v1/mcp
```
It will add an entry in `~/.claude.json`

```json
"mcpServers": {
  "atlassian": {
    "type": "http",
    "url": "https://mcp.atlassian.com/v1/mcp"
  }
}
```

2. Above will be added to the project's scope. To keep this for all the projects, move the block to the root.


### 2. Verify Installation

```bash
claude mcp list
```

Or inside Claude:
```
/mcp
```

### 3. Authenticate

- Above command will prompt for authentication
- Approve access when requested

### 4. Test Integration

Ask Claude to fetch a Jira ticket to verify working connection.
