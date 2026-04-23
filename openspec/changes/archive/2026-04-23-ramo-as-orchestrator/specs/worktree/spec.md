## ADDED Requirements

### Requirement: Add a worktree for a given branch

The `worktree` package SHALL provide an `Add(repoDir, worktreesDir, branch)` function that creates a git worktree for the given branch under `worktreesDir`. If the branch does not exist locally or remotely, the function SHALL create a new branch with that name. If the branch exists, the function SHALL check it out into the new worktree.

#### Scenario: Adding a worktree for a new branch
- **GIVEN** a git repository with no existing branch named `feat/x`
- **WHEN** `worktree.Add(repoDir, worktreesDir, "feat/x")` is called
- **THEN** a new branch `feat/x` is created and a worktree for it is added under `worktreesDir/feat/x`

#### Scenario: Adding a worktree for an existing branch
- **GIVEN** a git repository with an existing branch `feat/y`
- **WHEN** `worktree.Add(repoDir, worktreesDir, "feat/y")` is called
- **THEN** a worktree for the existing branch is added under `worktreesDir/feat/y` without creating a new branch

### Requirement: Remove a worktree

The package SHALL provide a `Remove(repoDir, worktreesDir, branch)` function that runs `git worktree remove --force` for the specified worktree, deleting it from disk.

#### Scenario: Removing an existing worktree
- **GIVEN** an existing worktree at `worktreesDir/feat/x`
- **WHEN** `worktree.Remove(repoDir, worktreesDir, "feat/x")` is called
- **THEN** the worktree directory is removed and `git worktree list` no longer lists it

### Requirement: Delete a branch

The package SHALL provide a `DeleteBranch(repoDir, branch)` function that runs `git branch -D <branch>` against `repoDir`.

#### Scenario: Deleting an existing branch
- **GIVEN** a branch `feat/x` in the repository
- **WHEN** `worktree.DeleteBranch(repoDir, "feat/x")` is called
- **THEN** the branch no longer appears in `git branch`

### Requirement: List managed worktrees

The package SHALL provide a `List(repoDir, worktreesDir)` function that returns the set of worktrees under `worktreesDir`, filtered to those managed by ramo (i.e. whose path is within `worktreesDir`).

#### Scenario: List returns only worktrees under worktreesDir
- **GIVEN** a repository with a git worktree outside `worktreesDir` and two worktrees under `worktreesDir`
- **WHEN** `worktree.List(repoDir, worktreesDir)` is called
- **THEN** the returned list contains exactly the two worktrees under `worktreesDir`

### Requirement: Check worktree existence

The package SHALL provide an `Exists(worktreesDir, branch)` function that returns true if the worktree directory for the given branch exists on disk under `worktreesDir`, and false otherwise.

#### Scenario: Exists returns true for an existing worktree
- **GIVEN** a worktree at `worktreesDir/feat/x`
- **WHEN** `worktree.Exists(worktreesDir, "feat/x")` is called
- **THEN** it returns true

### Requirement: Check branch existence

The package SHALL provide a `BranchExists(repoDir, branch)` function that returns true if the branch exists either locally or on a remote tracked by the repository, and false otherwise.

#### Scenario: BranchExists returns true for a local branch
- **GIVEN** a local branch `feat/x` in the repository
- **WHEN** `worktree.BranchExists(repoDir, "feat/x")` is called
- **THEN** it returns true

### Requirement: Fetch remote refs

The package SHALL provide a `Fetch(repoDir)` function that runs `git fetch` against `repoDir` to update remote-tracking branches before worktree operations that may need knowledge of remote branches.

#### Scenario: Fetch updates remote-tracking branches
- **GIVEN** a remote with a new branch that the local repo has not fetched
- **WHEN** `worktree.Fetch(repoDir)` is called
- **THEN** the new remote-tracking branch is visible in `git branch -r`

### Requirement: No cmux coupling

The `worktree` package SHALL NOT import any cmux-related package, SHALL NOT reference terminal-multiplexer concepts, and SHALL NOT contain cmux-specific fields or functions. Its sole concern is git worktree CRUD.

#### Scenario: Package has no cmux imports
- **WHEN** grepping the final `worktree/` package for imports
- **THEN** no import path contains `cmux`

### Requirement: Preserved test coverage

The existing `worktree_test.go` suite SHALL continue to pass after the cmux removal, covering `Add`, `AddDuplicateBranch`, `Remove`, `List`, and `DeleteBranch`.

#### Scenario: Existing tests pass post-migration
- **WHEN** `go test ./worktree/...` is run after the cmux removal changes
- **THEN** all existing tests in `worktree_test.go` pass with no modifications
