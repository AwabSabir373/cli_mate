# Prompt Guidance

## Effective Prompting

When using cli_mate, prompts are most effective when they:

1. **Specify the file** — mention the file path in the prompt or use @mentions
2. **Describe the desired change** — be specific about what to modify and how
3. **Include context** — mention related functions, types, or patterns

## Examples

```
@main.go explain this function

Add error handling to the Load function in config.go

Refactor the provider client to use a shared HTTP transport

Find all usages of the Tool interface and list them
```
