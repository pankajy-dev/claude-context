# Global Context for All Projects

This file contains preferences and guidelines that apply to all your projects.

## Coding Style Preferences

### Script Writing

[Define how you want scripts to be written across all projects]

**Example:**
- Use bash for shell scripts
- Include error handling with `set -e`
- Add descriptive comments for complex logic
- Use meaningful variable names
- Include usage examples in script headers

### Code Formatting

[Your preferred code formatting standards]

**Example:**
- Indentation: 4 spaces (or 2 spaces for certain languages)
- Line length: 80-100 characters
- Naming conventions: snake_case for functions, UPPER_CASE for constants

## Architecture Patterns

[Patterns you prefer across projects]

**Example:**
- Prefer composition over inheritance
- Use dependency injection
- Follow SOLID principles
- Keep functions small and focused

## Testing Standards

[Your testing preferences]

**Example:**
- Write unit tests for all new features
- Aim for >80% code coverage
- Use descriptive test names
- Follow AAA pattern (Arrange, Act, Assert)

## Documentation Standards

[How you want code documented]

**Example:**
- README.md for every project with quick start
- Inline comments for complex logic only
- API documentation for public interfaces
- Keep docs up-to-date with code changes

## Git Workflow

[Your preferred git practices]

**Example:**
- Write clear, concise commit messages
- Use conventional commits format
- Create feature branches for new work
- Squash commits before merging

## Security Best Practices

[Security guidelines for all projects]

**Example:**
- Never commit secrets or credentials
- Use environment variables for config
- Validate all user inputs
- Keep dependencies updated

## Error Handling

[How to handle errors consistently]

**Example:**
- Always handle potential errors
- Log errors with context
- Provide user-friendly error messages
- Use try-catch blocks appropriately

## Performance Considerations

[Performance guidelines]

**Example:**
- Profile before optimizing
- Cache expensive operations
- Avoid premature optimization
- Use appropriate data structures

## Dependencies Management

[How to manage dependencies]

**Example:**
- Pin dependency versions
- Regularly update and audit dependencies
- Prefer well-maintained libraries
- Document why each dependency is needed

## Notes

[Any other global preferences or guidelines]

---

**Note:** These are global preferences. Project-specific details should go in the project's `claude.md` file.
