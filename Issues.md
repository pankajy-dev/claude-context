1.  aws-codepipeline-invoke-actions git:(main) cctx ticket delete BEE-32496
  Error: unknown command "BEE-32496" for "cctx ticket delete"
  Usage:
  cctx ticket delete [flags]

Flags:
-f, --force   Skip confirmation prompt
-h, --help    help for delete

Global Flags:
-d, --data-dir string   Path to data directory (default: ~/.cctx, or CCTX_DATA_DIR env var)
--dry-run           Show what would be done without executing
-p, --project string    Project context name (default: CCTX_PROJECT env var or current directory)
-t, --ticket string     Ticket ID (default: CCTX_TICKET env var)
-v, --verbose           Show detailed information

➜  aws-codepipeline-invoke-actions git:(main) cctx ticket delete -t BEE-32496

⚠ About to permanently delete ticket: BEE-32496
ℹ Status: active
ℹ Linked projects: 0

⚠ This will remove:
⚠   - Ticket from config.json
⚠   - Ticket directory and all files
⚠   - Symlinks from all linked projects
⚠   - .clauderc entries in projects

✗ This action CANNOT be undone!

Are you sure you want to delete this ticket? [y/N]: y

✓ Deleted ticket directory
✓ Updated configuration

✓ Successfully deleted ticket: BEE-32496

2. cctx is not removing the symlink
   cctx ticket delete -t BEE-32496
I ran this cmd and symlink still exists in the project directory. I have to remove it manually. I am not sure if this is a bug or expected behavior. Please clarify.

3. if there is already an active file then cctx is not creating sessions.md. it should create new file with sequence1 or ticket id

4. Make an enhancement to cct SESSIONS.md to also wrote the git sha that are done one the branch when we mark the ticket as completed. no error if there data is not available.

5. Instead of symlinks, create actual file and once done then move this file to the place where symlinks are created