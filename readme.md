# Miroir

Declarative git repo manager. Synchronize multiple remotes, execute commands
across repos, and manage forge metadata from a single TOML config.

Note that the repo from the beginning up until
`cabbdc468d421abd25f9869ef36967039903c38f` were in OCaml and since then I asked
LLMs to rewrite everything in Go as I found `cmdliner`, `eio` and how console
outputs are managed to be a bit too combersome to iterate effectively (most
likely my own skill issue).

## Config

Miroir looks for config in this order:

1. `--config` / `-c` flag
2. `MIROIR_CONFIG` environment variable
3. `$XDG_CONFIG_HOME/miroir/config.toml` (typically
   `~/.config/miroir/config.toml`)

### Example

```toml
[general]
home = "~/Workspace"
branch = "master"

[general.concurrency]
repo = 2
remote = 0                    # 0 = no limit

[general.env]
GIT_SSH_COMMAND = "ssh -o StrictHostKeyChecking=no"

[platform.github]
origin = true                 # At most one platform can be origin per repo
domain = "github.com"
user = "alice"
access = "ssh"                # "ssh" (default) or "https"

[platform.gitlab]
domain = "gitlab.com"
user = "alice"
access = "https"
forge = "gitlab"              # Auto-detected from domain if omitted
token = "glpat-xxxxx"         # Or set MIROIR_GITLAB_TOKEN env

[platform.codeberg]
domain = "codeberg.org"
user = "alice"

[repo.dotfiles]
description = "my dotfiles"
visibility = "public"

[repo.notes]
visibility = "private"
branch = "main"               # Per-repo branch override

[repo.old-project]
visibility = "private"
archived = true               # Excluded from git ops, archived on forges via sync
```

### General

| Field                | Default  | Description                                       |
| -------------------- | -------- | ------------------------------------------------- |
| `home`               | `~/`     | Base directory containing repos                   |
| `branch`             | `master` | Default branch for all repos                      |
| `concurrency.repo`   | `1`      | Max concurrent repo operations                    |
| `concurrency.remote` | `0`      | Max concurrent remote ops per repo (0 = no limit) |
| `env`                |          | Extra environment variables for git commands      |

### Platform

| Field    | Default | Description                                                                |
| -------- | ------- | -------------------------------------------------------------------------- |
| `origin` | `false` | Treat as origin remote (at most one per repo)                              |
| `domain` |         | Forge domain                                                               |
| `user`   |         | Username on forge                                                          |
| `access` | `ssh`   | `ssh` or `https`                                                           |
| `forge`  |         | `github`, `gitlab`, `codeberg`, or `sourcehut` (auto-detected from domain) |
| `token`  |         | API token for forge operations                                             |

Tokens can also be set via environment: `MIROIR_<PLATFORM_NAME>_TOKEN` (e.g.
`MIROIR_GITHUB_TOKEN`).

### Repo

| Field         | Default   | Description                                 |
| ------------- | --------- | ------------------------------------------- |
| `description` |           | Repo description synced to forges           |
| `visibility`  | `private` | `public` or `private`                       |
| `archived`    | `false`   | Skip in git ops; archive on forges via sync |
| `branch`      |           | Per-repo branch override                    |

## Usage

```
miroir <command> [flags]
```

### Target Selection

By default, miroir targets the repo matching your current directory.

- `-n, --name <repo>` — Target a specific repo by name
- `-a, --all` — Target all non-archived repos
- `-f, --force` — Force operation

### Commands

**init** — Clone and set up repo(s) with all configured remotes

```sh
miroir init                   # Init repo for cwd
miroir init -a                # Init all repos
```

Creates the directory, initializes git, adds all remotes, fetches, resets to
`origin/<branch>`, and initializes submodules.

**fetch** — Fetch from all remotes (concurrent)

```sh
miroir fetch -a
```

**pull** — Pull from origin

```sh
miroir pull                   # Fails if working tree is dirty
miroir pull -f                # Hard reset then pull
```

Also updates submodules recursively.

**push** — Push to all remotes (concurrent)

```sh
miroir push -a
miroir push -f                # Force push
```

**exec** — Run a command in repo(s)

```sh
miroir exec -a -- git status
miroir exec -n myrepo -- make build
```

Runs sequentially with direct stdout/stderr passthrough.

**sync** — Synchronize repo metadata to all forges

```sh
miroir sync -a
```

Creates repos that don't exist, updates description/visibility on existing ones,
and archives repos marked `archived = true`. Each forge API call has a 30-second
timeout.

**completion** — Generate shell completions

```sh
miroir completion bash >> ~/.bashrc
miroir completion zsh > ~/.zfunc/_miroir
miroir completion fish > ~/.config/fish/completions/miroir.fish
```

## Supported Forges

| Forge     | Create | Update | Archive | Delete | List | Sync |
| --------- | ------ | ------ | ------- | ------ | ---- | ---- |
| GitHub    | Yes    | Yes    | Yes     | Yes    | Yes  | Yes  |
| GitLab    | Yes    | Yes    | Yes     | Yes    | Yes  | Yes  |
| Codeberg  | Yes    | Yes    | Yes     | Yes    | Yes  | Yes  |
| SourceHut | Yes    | Yes    | No      | Yes    | Yes  | Yes  |

Forge type is auto-detected from the platform domain:

- `github.com`, `github.*` → GitHub
- `gitlab.com`, `gitlab.*` → GitLab
- `codeberg.org` → Codeberg
- `*.sr.ht`, `sr.ht` → SourceHut

Set `forge = "..."` explicitly to override.

## Concurrency

Miroir runs git operations concurrently at two levels:

- **Repo-level**: Controlled by `concurrency.repo` (default 1)
- **Remote-level**: Controlled by `concurrency.remote` (default 0, no limit)

Keep `concurrency.repo` low (2–4) as some forges rate-limit SSH connections.

```toml
[general.concurrency]
repo = 2
remote = 0
```

## Display

When stdout is a TTY, miroir uses a real-time TUI (bubbletea) showing per-repo
and per-remote progress. When piped, it falls back to structured log output.
