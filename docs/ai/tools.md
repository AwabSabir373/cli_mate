# Tool-Use Policy

## Available Tools

- **file_read**: Read a UTF-8 text file from the workspace
- **file_edit**: Edit a file by replacing an exact old string with a new string
- **file_write**: Create or overwrite a file inside the workspace
- **shell**: Run non-destructive shell commands (tests, builds, formatting)
- **glob**: Find files matching a glob pattern
- **grep**: Search file contents using regex
- **file_list**: List files and directories at a path
- **read_subtree**: Read a directory subtree with parsed function/variable names

## Rules

1. Always read before editing — understand context before making changes
2. Prefer file_edit over file_write to preserve existing content
3. After Go edits, run `gofmt` and relevant verification
4. Tool output is limited to 64KB to prevent token overflow
5. Shell commands have a configurable timeout (default 30s)
6. Dangerous commands (rm -rf, shutdown, etc.) are blocked
7. Use file_list instead of `ls`/`dir` for directory listing
8. Use read_subtree to understand package structure before diving into files
