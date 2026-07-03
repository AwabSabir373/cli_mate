# AI Instruction Files

These markdown files are meant to be loaded into the model as stable context.

Use them to keep responses consistent, efficient, and safe:

- `system.md`: base behavior for the coding agent.
- `models.md`: provider and model guidance.
- `tools.md`: tool-use policy.
- `repository.md`: project-specific architecture notes.
- `review.md`: code review and verification rules.

Recommended load order:

1. `AGENTS.md`
2. `docs/ai/system.md`
3. `docs/ai/repository.md`
4. `docs/ai/tools.md`
5. `docs/ai/models.md`
6. `docs/ai/review.md`
