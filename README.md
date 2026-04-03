# Ramo

A Git worktree manager CLI with [cmux](https://github.com/manaflow-ai/cmux) workspace integration.

Ramo automates the creation and management of Git worktrees — isolated working copies of your repository. It pairs with cmux to automatically set up terminal workspaces with configured panes and commands for each worktree.

## Prerequisites

- [Go](https://go.dev/) 1.26+
- [Git](https://git-scm.com/)
- [cmux](https://github.com/manaflow-ai/cmux)

## Installation

```
go install github.com/mateusduraes/ramo@latest
```

## Quick Start

Initialize ramo in your repository:

```
ramo init
```

This creates a `ramo.json` config file. Edit it to configure your setup:

```json
{
  "worktreesDir": ".worktrees",
  "setup": ["npm install"],
  "copy": [".env.local"],
  "workspace": {
    "panes": [
      { "command": "vim" },
      { "command": "npm run dev" }
    ]
  }
}
```

Create a new worktree and workspace:

```
ramo new my-feature
```

Open an existing worktree:

```
ramo open my-feature
```

List worktrees:

```
ramo list
```

Remove a worktree:

```
ramo remove my-feature
```

## Configuration

Ramo is configured via a `ramo.json` file in your repository root.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `worktreesDir` | string | `.worktrees` | Directory where worktrees are created |
| `setup` | string[] | `[]` | Commands to run in a new worktree after creation |
| `copy` | string[] | `[]` | Files to copy from the main repo into the worktree |
| `workspace` | object | — | cmux workspace configuration |
| `workspace.panes` | object[] | — | List of panes to create, each with an optional `command` |

## Commands

| Command | Description |
|---------|-------------|
| `ramo init` | Create a default `ramo.json` in the current directory |
| `ramo new <branch>` | Create a worktree, run setup, and open a cmux workspace |
| `ramo open [branch]` | Open an existing worktree in a cmux workspace |
| `ramo list` | List all managed worktrees |
| `ramo remove <branch>` | Remove a worktree, its branch, and close the workspace |

## License

MIT
