# CLI Specification

### Requirement: `ramo init` scaffolds a new orchestrator project

The `ramo init` command SHALL write four files into the user's repository: `.ramo/Dockerfile` (starter container image), `ramo.go` (starter orchestrator script with example worktree and steps), `.ramo/open-pr.prompt.md` (host-step prompt for commit + PR creation), and `.ramo/.gitignore` (ignoring `logs/` and `reports/`). The command SHALL NOT overwrite any of these files if they already exist; it SHALL emit an error for each existing file and exit non-zero.

#### Scenario: Fresh init writes all four files
- **GIVEN** a repository without a `.ramo` directory
- **WHEN** `ramo init` is run at the repository root
- **THEN** `.ramo/Dockerfile`, `ramo.go`, `.ramo/open-pr.prompt.md`, and `.ramo/.gitignore` are created

#### Scenario: Init refuses to overwrite existing files
- **GIVEN** a repository with a pre-existing `.ramo/Dockerfile`
- **WHEN** `ramo init` is run
- **THEN** the command exits non-zero with an error message naming the existing file, and no files are modified or created

### Requirement: `ramo init --provider` controls the generated PR prompt

The `ramo init` command SHALL accept a `--provider` flag with values `github` (default) or `gitlab`. The provider value SHALL affect only the contents of the generated `.ramo/open-pr.prompt.md`: the `github` variant SHALL reference `gh pr create --draft`; the `gitlab` variant SHALL reference `glab mr create --draft`. No other file SHALL differ between providers. The provider value SHALL NOT be persisted anywhere beyond the generated prompt file.

#### Scenario: github provider uses gh in the prompt
- **WHEN** `ramo init --provider github` is run
- **THEN** the generated `.ramo/open-pr.prompt.md` contains a `gh pr create` invocation and does not contain `glab`

#### Scenario: gitlab provider uses glab in the prompt
- **WHEN** `ramo init --provider gitlab` is run
- **THEN** the generated `.ramo/open-pr.prompt.md` contains a `glab mr create` invocation and does not contain `gh pr create`

### Requirement: `ramo list` lists managed worktrees

The `ramo list` command SHALL print one line per ramo-managed worktree (those residing under the default worktrees directory), showing the branch name and the worktree path. It SHALL NOT output any cmux-related information.

#### Scenario: List prints existing worktrees
- **GIVEN** two ramo-managed worktrees `feat/x` and `feat/y`
- **WHEN** `ramo list` is run
- **THEN** stdout includes exactly two lines, one for each worktree, with branch name and path

#### Scenario: Empty list prints no worktrees
- **GIVEN** no ramo-managed worktrees on disk
- **WHEN** `ramo list` is run
- **THEN** stdout is empty (or contains only a header), and the command exits zero

### Requirement: `ramo remove <branch>` removes a worktree and its branch

The `ramo remove <branch>` command SHALL remove the worktree for the given branch via `git worktree remove --force`, then delete the branch via `git branch -D`. It SHALL NOT invoke any cmux commands.

#### Scenario: Remove deletes worktree and branch
- **GIVEN** a worktree `feat/x` and a local branch `feat/x`
- **WHEN** `ramo remove feat/x` is run
- **THEN** the worktree directory is gone and the branch no longer appears in `git branch`

### Requirement: Dropped commands and config

The `ramo` binary SHALL NOT expose the commands `ramo new` or `ramo open`. The binary SHALL NOT read or write `ramo.json` or any file under `config/`. Invoking `ramo new` or `ramo open` on the new binary SHALL produce an "unknown command" error.

#### Scenario: `ramo new` is unknown
- **WHEN** `ramo new feat/x` is run
- **THEN** the binary exits non-zero with an error indicating "unknown command" (or equivalent cobra output)

#### Scenario: No ramo.json is read or written
- **GIVEN** a `ramo.json` file present in the repository root
- **WHEN** any `ramo` subcommand is run
- **THEN** the command executes without reading, parsing, or referencing the `ramo.json` file
