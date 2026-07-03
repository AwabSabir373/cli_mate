# Code Review & Verification Rules

## Review Process

1. Verify changes compile and pass `gofmt`, `go vet`, and `go test`
2. Check for proper error handling — all errors should be checked or explicitly ignored
3. Ensure no sensitive data (API keys, tokens) is logged or exposed
4. Verify workspace path security — all file operations must be constrained to the workspace root
5. Check for proper context propagation and cancellation

## Quality Standards

- Tests should cover interaction flows and regressions
- No global mutable state
- Clear, useful error messages
- Follow Go idioms and project conventions
