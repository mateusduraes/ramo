# Authentication

Ramo is method-agnostic. It passes through any of a known list of auth env vars (plus user-declared additions) to every container it starts. Host steps inherit the full host environment.

## Recognized env vars

The following host env vars are forwarded to every container automatically if set:

| Variable | Purpose |
|----------|---------|
| `ANTHROPIC_API_KEY` | Direct API key auth |
| `CLAUDE_CODE_OAUTH_TOKEN` | Claude subscription OAuth token |
| `ANTHROPIC_BASE_URL` | Custom API base URL (e.g. corporate proxy) |
| `AWS_REGION` | AWS Bedrock region |
| `AWS_PROFILE` | AWS profile name |
| `AWS_ACCESS_KEY_ID` | AWS credentials |
| `AWS_SECRET_ACCESS_KEY` | AWS credentials |
| `AWS_SESSION_TOKEN` | AWS session token |

## Claude Max subscription

If you pay for Claude Max (the subscription product), generate an OAuth token:

```
claude setup-token
```

This stores a token and prints `CLAUDE_CODE_OAUTH_TOKEN=...` for you to export. Add to your shell rc or a `.env` file you source before running ramo.

## API key

Export `ANTHROPIC_API_KEY=sk-...` before running ramo. No other config needed.

## Bedrock / Vertex

Set the AWS or GCP credential env vars for your method. The Claude CLI handles the rest; ramo just forwards the env.

## Adding more env vars

To pass additional env vars through to every container:

```go
orchestrator.Config{
    // ...
    Env: []string{"COMPANY_PROXY", "NPM_TOKEN"},
}
```

Any of these that are set in the host environment at run time will appear inside every container.

## Host steps

Host steps run directly on your machine with the worktree as cwd. They inherit the full host environment, including:

- `~/.ssh` (for `git push`)
- `~/.config/gh` (for `gh` CLI)
- `~/.gitconfig`
- `$PATH` (so you can call any locally installed tool)

That's what makes the auto-PR host step practical — the `gh`/`glab`/`git` binaries all work with your existing auth.

## Preflight

At the start of every run, ramo checks whether any of the recognized auth env vars are set. If none are, it emits a warning event (`preflight_warning`) but does **not** abort. The first `claude` invocation will surface the real error if auth is genuinely missing.
