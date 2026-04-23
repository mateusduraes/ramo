# Implement tasks from tasks.md

You are running inside a Docker container. The bind-mounted worktree is at `/workspace` — that is your cwd. The file `tasks.md` in that worktree is your single source of truth for what to do and what has been done.

## Your job

1. Read `tasks.md`. Each task is a line that looks like `- [ ] <description>` (unchecked) or `- [x] <description>` (checked).
2. Pick the next **1 to 3** unchecked tasks that belong to the same area. Complete them fully: write the code, write or update tests, make sure the tests pass.
3. Mark completed tasks as `- [x]`. If you hit a task you cannot complete (missing dependency, blocked on external work, unclear scope), leave it unchecked and add an HTML comment on the next line:

   ```
   - [ ] Set up Stripe webhook handler
   <!-- blocked: waiting on Stripe secret from infra team -->
   ```

4. When you finish this iteration, verify `tasks.md` still parses (every task line is either `- [ ]` or `- [x]`) and that the project still builds / tests still pass locally.

## Done signal

When — and **only when** — `tasks.md` has zero remaining `- [ ]` items, emit exactly this line as the final thing you print:

```
<ramo>DONE</ramo>
```

If there are still unchecked tasks, do NOT emit the done signal. Just stop normally. Ramo will launch the next iteration, which will read `tasks.md` again and continue.

## Constraints

- Do not reorder or rewrite existing completed tasks.
- Do not delete tasks without a comment explaining why.
- Keep each commit's scope small and self-contained; prefer many commits over one giant one.
- If you introduce a new dependency, update the lockfile and mention it in the commit message.
