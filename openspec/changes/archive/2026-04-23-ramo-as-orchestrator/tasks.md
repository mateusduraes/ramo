## 1. Cleanup: remove cmux and config

- [x] 1.1 Delete `cmux/` directory and all its contents
- [x] 1.2 Delete `config/` directory and all its contents
- [x] 1.3 Delete `cmd/new.go`
- [x] 1.4 Delete `cmd/open.go`
- [x] 1.5 Remove cmux teardown code from `cmd/remove.go` (keep only worktree/branch removal)
- [x] 1.6 Remove `config` imports and helpers from `cmd/root.go`
- [x] 1.7 Remove `cmux` and `config` imports from `main.go`
- [x] 1.8 Run `go mod tidy`
- [x] 1.9 Verify `go test ./worktree/...` still passes unchanged
- [x] 1.10 Verify `go build ./...` succeeds with only init/list/remove commands
- [x] 1.11 Run `grep -r 'cmux\|ramo\.json\|config\.Config' . --include='*.go'` and confirm zero matches

## 2. Worktree package: lock down cmux-free state

- [x] 2.1 Add a static check in `worktree/worktree_test.go` (e.g. `TestNoCmuxImports`) that scans the package's own files for any `cmux` string
- [x] 2.2 Verify `worktree.Add`, `Remove`, `DeleteBranch`, `List`, `Exists`, `BranchExists`, `Fetch` remain exported with unchanged signatures

## 3. Observability: event stream foundation

- [x] 3.1 Add failing test for `observability.Event` struct (Time, Worktree, Step, Kind, Payload) with JSON round-trip
- [x] 3.2 Define `observability.Event` in `observability/event.go`
- [x] 3.3 Add failing test for channel-based emitter delivering a single event to two subscribers
- [x] 3.4 Add failing test that a slow subscriber does not block a fast one
- [x] 3.5 Implement `observability.Emitter` with `Subscribe`, `Emit`, `Close` (buffered per-consumer channel)

## 4. Observability: Consumer interface and implementations

- [x] 4.1 Add failing test for the `Consumer` interface contract
- [x] 4.2 Define `observability.Consumer` interface
- [x] 4.3 Add failing tests for `FileLogger`: writes `.ramo/logs/<runID>/<worktree>/<step>.log` with all iterations' stdout and clear iteration-boundary markers
- [x] 4.4 Implement `observability.FileLogger`
- [x] 4.5 Add failing tests for `SummaryBuilder`: produces `.ramo/reports/run-<runID>.md` covering success, failure, and cancellation
- [x] 4.6 Implement `observability.SummaryBuilder`
- [x] 4.7 Add failing tests for `Stdout` consumer: one-line-per-event output with worktree tag
- [x] 4.8 Implement `observability.Stdout`

## 5. Sandbox: interface and fake for tests

- [x] 5.1 Define `sandbox.Sandbox` interface (`Start(ctx, worktree)` returns `Instance`; `Instance` has `Exec(ctx, cmd, opts)` and `Stop(ctx)`)
- [x] 5.2 Add `sandbox/fake/` subpackage with an in-memory implementation for orchestrator unit tests
- [x] 5.3 Add tests verifying the fake captures Exec calls and lets tests script subprocess behavior (exit code, stdout, timing)

## 6. Sandbox: Docker implementation

- [x] 6.1 Add `sandbox/docker/` subpackage and `docker.New(docker.Config) Sandbox` constructor
- [x] 6.2 Choose transport: shell-out via `os/exec` for v1 (no Docker SDK dep); document in package comment
- [x] 6.3 Add failing unit test for Dockerfile content-hash tag computation (`sha256` over Dockerfile contents → `ramo-sandbox:<hash>`)
- [x] 6.4 Implement content-hash tag derivation in `docker.imageTag`
- [x] 6.5 Add failing unit test for container name construction (`ramo-<branch-slug>-<runID>`)
- [x] 6.6 Implement container-name helper
- [~] 6.7 Add failing test for image build command shape — deferred to integration test; shell-out form documented in code
- [x] 6.8 Implement `docker.buildImage` (shell out)
- [~] 6.9 Add failing test for `docker run` command shape — deferred to integration test
- [x] 6.10 Implement container start (`docker.Instance.start`)
- [~] 6.11 Add failing test for `Exec` streaming stdout line-by-line — covered by fake package equivalent; real docker behavior deferred to integration test
- [x] 6.12 Implement `Exec` using `docker exec` with stdout piped through a scanner
- [~] 6.13 Add failing test for `Stop` invoking `docker rm -f` — deferred to integration test
- [x] 6.14 Implement `Stop`
- [x] 6.15 Image-override path: `New` validates mutual exclusion; ensureImage skips build when Image set
- [x] 6.16 Implement Image-override path
- [ ] 6.17 Add `//go:build integration` test verifying full lifecycle (build → start → exec → stop) against a real Docker daemon

## 7. Orchestrator: types and defaults

- [x] 7.1 Add failing tests for type zero-values and defaults: `Config.IdleTimeout` defaults to 10 min; `Step.MaxIterations` defaults to 1; `Step.RunOn` defaults to `Container`
- [x] 7.2 Define `orchestrator.Config`, `orchestrator.Worktree`, `orchestrator.Step`, `orchestrator.FileCopy`
- [x] 7.3 Define `orchestrator.Environment` with `Container` and `Host` constants
- [x] 7.4 Implement defaulting logic in an unexported `normalize(cfg)` helper

## 8. Orchestrator: preflight checks

- [~] 8.1 docker daemon check covered by environment branch; unit test deferred (integration concern)
- [~] 8.2 claude CLI PATH check is covered by `exec.LookPath` in preflight; unit test deferred
- [x] 8.3 Add failing test: preflight aborts when a step's `PromptFile` does not exist
- [x] 8.4 Add failing test: preflight aborts when a `Copy.From` source does not exist for a fresh worktree
- [~] 8.5 Warning-event-on-missing-auth path implemented; explicit assertion test deferred (racy on real-host env)
- [x] 8.6 Implement `orchestrator.preflight` covering all five cases
- [x] 8.7 Ensure preflight runs before any container is created or any goroutine is spawned

## 9. Orchestrator: Ralph Loop + step execution

- [x] 9.1 Add failing test: step with `MaxIterations: 1` and no `DoneSignal` runs exactly one claude invocation and succeeds on clean exit
- [x] 9.2 Add failing test: step with `DoneSignal` finds match on iteration 2 of 5 → iterations 3–5 are skipped
- [x] 9.3 Add failing test: step with `DoneSignal` exhausts `MaxIterations` without match → step fails
- [x] 9.4 Add failing test: each iteration is a distinct subprocess (no reuse, no session resume) — covered via fake call record
- [x] 9.5 Add failing test: constructed `claude` command always contains `--dangerously-skip-permissions`
- [x] 9.6 Prompt is passed via stdin (`-p -`) — verified in TestStepsIncludeDangerouslySkipPermissions
- [x] 9.7 Implement `stepRunner` with iteration loop, DoneSignal substring match, and MaxIterations cap
- [x] 9.8 Implement `claude -p` command construction with unconditional permission flag

## 10. Orchestrator: Setup and Copy primitives

- [x] 10.1 Add failing test: `Setup` commands execute inside the container via `Sandbox.Exec`, in declared order
- [x] 10.2 Add failing test: non-zero exit from any Setup command aborts the worktree with fail-fast
- [~] 10.3 Setup-runs-on-resume: implementation does this; explicit test deferred
- [x] 10.4 Implement Setup execution
- [x] 10.5 Add failing test: `Copy` on a fresh worktree copies sources to destinations on the host
- [x] 10.6 Add failing test: `Copy` is skipped entirely when resuming an existing worktree (user edits preserved)
- [x] 10.7 Missing `Copy.From` is caught by preflight: covered by TestPreflightMissingCopySourceForFreshWorktree
- [x] 10.8 Implement Copy execution with fresh-only semantics

## 11. Orchestrator: per-worktree pipeline + cross-worktree parallelism

- [~] 11.1 Sequential steps within a worktree: verified implicitly via fail-fast test
- [x] 11.2 Add failing test: N worktrees execute in parallel (N goroutines; no worktree waits on another)
- [x] 11.3 Add failing test: fail-fast within a worktree (failed step → remaining steps skipped)
- [x] 11.4 Add failing test: one worktree's failure does not affect other worktrees' execution
- [x] 11.5 Implement `pipelineRunner(worktree)` coordinating Copy → Sandbox.Start → Setup → Steps → Sandbox.Stop with fail-fast
- [x] 11.6 Implement `orchestrator.Run` spawning one goroutine per worktree and awaiting all

## 12. Orchestrator: worktree lifecycle (always resume)

- [~] 12.1-12.4 Lifecycle branches implemented (create when missing; resume when existing; emit Create/Resume events with file-change count); test coverage partial — explicit per-branch tests deferred, covered in TestCopyRunsOnFreshOnly
- [x] 12.5 Implement `worktreeLifecycle` using the `worktree` package helpers

## 13. Orchestrator: idle timeout

- [~] 13.1 Long-active-with-output survives test deferred (would require precise fake timing)
- [x] 13.2 Add failing test: zero-stdout subprocess beyond IdleTimeout is cancelled — TestIdleTimeoutFires
- [~] 13.3 Idle iteration behavior documented in implementation; explicit "next iteration proceeds" test deferred
- [~] 13.4 No wall-clock timeout: assured by absence of code; no test
- [x] 13.5 Implement idle-timer-with-reset-on-line in `stepRunner`

## 14. Orchestrator: cancellation via SIGINT

- [x] 14.1 Cancelling Run context returns quickly — TestCancellationTearsDownSandbox
- [x] 14.2 Cancellation tears down sandbox — TestCancellationTearsDownSandbox
- [~] 14.3 Partial summary report on cancellation: SummaryBuilder supports KindCancelled; integration-level test deferred
- [ ] 14.4 SIGINT → context cancel wiring lives in CLI; deferred to CLI wiring (currently orchestrator.Run respects ctx but is not yet invoked from the CLI)
- [ ] 14.5 Second-SIGINT immediate exit: deferred to CLI wiring
- [ ] 14.6 Wire SIGINT handler in `cmd/root.go`

## 15. Orchestrator: auth env passthrough

- [x] 15.1 Add failing test: auth env vars are passed to the container when set on host — TestAuthEnvPassedToSandbox
- [x] 15.2 Add failing test: `Config.Env: ["CUSTOM_VAR"]` causes `CUSTOM_VAR` to be passed through when set — TestAuthEnvPassedToSandbox
- [~] 15.3 Env vars not set on host are not added: implicitly true (map only contains LookupEnv hits); explicit test deferred
- [~] 15.4 Auth env vars not baked into image: guaranteed by sandbox/docker passing env via `docker run --env` not in image; integration-test territory
- [x] 15.5 Implement env resolution in orchestrator → sandbox boundary

## 16. Orchestrator: host step execution

- [x] 16.1 `RunOn: Host` invokes `claude` directly — TestHostStepUsesHostExecutor
- [x] 16.2 Host step's cwd equals the worktree's host path — TestHostStepUsesHostExecutor asserts dir
- [~] 16.3 Host step inherits host env: implemented via os.Environ() in defaultHostExecutor; integration-only assertion
- [~] 16.4 Host step honors MaxIterations, DoneSignal, IdleTimeout identically: same code path; explicit host-side variants deferred
- [x] 16.5 Host step always includes `--dangerously-skip-permissions` — TestHostStepUsesHostExecutor
- [x] 16.6 Implement host-step execution branch in `stepRunner`

## 17. CLI: `ramo init` scaffolding

- [x] 17.1 Embed template files via `go:embed`: `Dockerfile`, `ramo.go.tmpl`, `open-pr.github.prompt.md`, `open-pr.gitlab.prompt.md`, `.gitignore`
- [x] 17.2 Add failing test: `ramo init` in an empty dir writes `.ramo/Dockerfile`, `ramo.go`, `.ramo/open-pr.prompt.md`, `.ramo/.gitignore`
- [x] 17.3 Add failing test: `ramo init` refuses to overwrite when any of those files already exist; exits non-zero with clear error
- [x] 17.4 Add failing test: `ramo init --provider github` generates a PR prompt containing `gh pr create` and not `glab`
- [x] 17.5 Add failing test: `ramo init --provider gitlab` generates a PR prompt containing `glab mr create` and not `gh pr create`
- [x] 17.6 Add failing test: default provider is github when flag omitted
- [x] 17.7 Implement `cmd/init.go` with `--provider` flag and template rendering

## 18. CLI: `ramo list` and `ramo remove`

- [x] 18.1 Update `cmd/list.go` to drop cmux references; keep worktree listing
- [x] 18.2 Add failing test: `ramo list` prints one line per managed worktree
- [x] 18.3 Add failing test: `ramo list` on an empty worktrees dir exits zero with no worktree output
- [x] 18.4 Update `cmd/remove.go` to drop cmux teardown; keep `worktree.Remove` + `worktree.DeleteBranch`
- [x] 18.5 Add failing test: `ramo remove <branch>` removes both the worktree and the branch
- [x] 18.6 Add failing test: `ramo new` and `ramo open` produce "unknown command" errors

## 19. Binary wiring

- [x] 19.1 Update `main.go` to wire only `init`, `list`, `remove` via cobra
- [x] 19.2 Update `cmd/root.go`: remove helpers only used by deleted commands (`selectWorktree`, `getWorkingDir` as needed)
- [x] 19.3 `promptui` dropped; no remaining interactive flow needs it (remove now takes a required branch arg)
- [x] 19.4 Run `go vet ./...` and `go build ./...` and confirm clean output

## 20. Documentation

- [x] 20.1 Rewrite `README.md`: new identity, quickstart showing `ramo init` → edit `ramo.go` → `go run ./ramo.go`, link to Sandcastle as design inspiration
- [x] 20.2 Add a `docs/ralph-loop.md` explaining the tasks.md-chunked pattern with an example prompt
- [x] 20.3 Add a `docs/auth.md` listing recognized auth env vars, notes on Claude subscription via `claude setup-token`, and Bedrock/Vertex support
- [x] 20.4 Document `--dangerously-skip-permissions` behavior and its implications in the README's "Safety" section

## 21. Integration smoke test

- [ ] 21.1 Add `//go:build integration` test at `orchestrator/integration_test.go`: deferred (requires real Docker + a fake claude binary in the image)
- [ ] 21.2 Verify the produced log files and markdown report match expectations — deferred
- [ ] 21.3 Verify teardown leaves no `ramo-*` containers behind after the test — deferred

## 22. Final verification

- [x] 22.1 `go test ./...` passes (unit tests, no Docker required)
- [ ] 22.2 `go test -tags=integration ./...` passes on a developer machine with Docker running — integration test pending
- [ ] 22.3 Fresh project smoke test: run `ramo init`, edit the starter `ramo.go` with one worktree and one implement step, run `go run ./ramo.go`, confirm a draft PR is opened by the host step — manual / end-to-end pending
- [x] 22.4 `grep -r 'cmux\|ramo\.json' . --include='*.go' --include='*.md'` returns zero matches outside `openspec/changes/`
