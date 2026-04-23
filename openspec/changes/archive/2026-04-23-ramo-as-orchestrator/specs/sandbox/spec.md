## ADDED Requirements

### Requirement: Sandbox interface

The `sandbox` package SHALL define a `Sandbox` interface with methods to (a) start an isolated execution environment for a given worktree, (b) execute commands within that environment, and (c) tear it down. The orchestrator SHALL depend only on this interface, not on any concrete implementation.

#### Scenario: Orchestrator uses Sandbox without referencing docker package
- **WHEN** the orchestrator executes a container step
- **THEN** it invokes methods on a `Sandbox` instance without importing `sandbox/docker` directly (the docker instance is passed in via `Config.Sandbox`)

### Requirement: Docker implementation — `docker.New(...)`

The `sandbox/docker` package SHALL provide a `New(cfg Config) Sandbox` constructor returning a Docker-backed `Sandbox`. `Config` SHALL accept (a) a `Dockerfile` path and (b) an `Image` override path; exactly one of the two MUST be provided.

#### Scenario: docker.New with Dockerfile returns a working Sandbox
- **GIVEN** a valid Dockerfile path
- **WHEN** `docker.New(docker.Config{Dockerfile: path})` is called
- **THEN** the returned Sandbox value can be passed to `orchestrator.Run` and used to start at least one container

#### Scenario: docker.New with Image override skips local build
- **GIVEN** `docker.Config{Image: "myorg/sandbox:v1"}` and no Dockerfile configured
- **WHEN** the orchestrator starts a sandbox for a worktree
- **THEN** the Docker client pulls or uses the named image directly, without building any Dockerfile

### Requirement: Content-hash image tagging for Dockerfile builds

When configured with a `Dockerfile`, the sandbox SHALL tag the built image as `ramo-sandbox:<sha256-of-dockerfile-contents>` so that edits to the Dockerfile produce a fresh tag. Docker's layer cache SHALL be used for incremental rebuilds.

#### Scenario: Editing the Dockerfile changes the tag
- **GIVEN** a Dockerfile that has been built once and tagged `ramo-sandbox:<hash1>`
- **WHEN** the user adds a `RUN apt-get install -y chromium` line and re-runs
- **THEN** the rebuild produces a new tag `ramo-sandbox:<hash2>` where hash2 ≠ hash1

### Requirement: Worktree bind-mount at `/workspace`

The sandbox SHALL mount the host-side worktree directory into the container at `/workspace` as a bind mount. The container's working directory SHALL default to `/workspace`. Files written by the agent inside the container SHALL be immediately visible on the host, and vice versa.

#### Scenario: Agent-written file appears on host instantly
- **GIVEN** a running container with the worktree bind-mounted
- **WHEN** the agent executes `echo hello > /workspace/hello.txt` inside the container
- **THEN** the file `hello.txt` is visible at the host-side worktree path within seconds

### Requirement: Host UID mapping into the container

The sandbox SHALL run the container's processes with the host user's UID and GID (`--user $(id -u):$(id -g)`) so that files written by the agent inside the container are owned by the host user and editable without sudo.

#### Scenario: Agent-written file is owned by host user
- **GIVEN** a worktree started with the default configuration
- **WHEN** the agent creates a file at `/workspace/new.txt` inside the container
- **THEN** `stat new.txt` on the host shows the host user as the file owner

### Requirement: Long-lived container per worktree

The sandbox SHALL start one container per worktree, keep it alive for the duration of the worktree's pipeline (using `CMD ["sleep", "infinity"]` or equivalent), and execute each Setup command and each agent step via `docker exec` against that container. The container SHALL NOT be restarted between steps.

#### Scenario: Container persists across steps
- **GIVEN** a worktree with Steps `[implement, verify]` and a Setup that runs `pnpm install`
- **WHEN** the pipeline executes
- **THEN** the same container ID executes the Setup, the `implement` step, and the `verify` step

### Requirement: Container name includes worktree branch and run ID

Each container SHALL be named with a prefix identifying it as a ramo container, the worktree branch, and the run ID, e.g. `ramo-feat-signup-<runID>`. Container names SHALL NOT collide across concurrent runs.

#### Scenario: Two concurrent runs do not collide
- **GIVEN** two ramo runs starting simultaneously with overlapping worktree branches
- **WHEN** both runs attempt to create containers
- **THEN** each container has a unique name incorporating a distinct run ID, and both runs start successfully

### Requirement: Auth env passthrough on `docker run`

When starting a container, the sandbox SHALL pass through the host values of these env vars (when set): `ANTHROPIC_API_KEY`, `CLAUDE_CODE_OAUTH_TOKEN`, `ANTHROPIC_BASE_URL`, `AWS_REGION`, `AWS_PROFILE`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, plus any additional names declared in `Config.Env`. These env vars SHALL NOT be baked into the image at build time.

#### Scenario: Env vars appear inside container but not in image
- **GIVEN** `ANTHROPIC_API_KEY=sk-abc` in the host env
- **WHEN** a container is started and `docker inspect` is run on the image tag
- **THEN** `env` inside the container shows `ANTHROPIC_API_KEY=sk-abc`, but `docker inspect <image-tag>` does not show the key in the image layers

### Requirement: Teardown on run end

The sandbox SHALL provide a teardown mechanism that removes all containers it started. The orchestrator SHALL invoke teardown for every worktree's sandbox at the end of a run, regardless of success, failure, or cancellation.

#### Scenario: Successful run tears down containers
- **GIVEN** a run with three worktrees that all complete successfully
- **WHEN** `Run` returns
- **THEN** `docker ps` shows no containers whose name starts with `ramo-` for the completed run's run ID

#### Scenario: Ctrl-C tears down containers
- **GIVEN** a run with containers actively executing
- **WHEN** the user sends SIGINT
- **THEN** all containers created by the run are removed before `Run` returns
