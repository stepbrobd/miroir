# Miroir

Declarative git repo manager and code search server. Synchronize multiple
remotes, execute commands across repos, manage forge metadata from a single TOML
config, and serve full-text code search via
[zoekt](https://github.com/sourcegraph/zoekt).

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
origin = true                 # Exactly one platform must be origin
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

[index]
listen = ":6070"              # HTTP listen address for search API
database = "/data/miroir/idx" # Zoekt shard storage (default: $XDG_DATA_HOME/miroir/index)
interval = 300                # Seconds between fetch+index cycles
bare = true                   # Clone managed repos as bare (default: true)
include = [                   # Extra directories of repos to index (one level deep)
  "/var/lib/gitea/repositories/alice",
]
```

### General

| Field                | Default  | Description                                                       |
| -------------------- | -------- | ----------------------------------------------------------------- |
| `home`               | `~/`     | Base directory containing managed repos                           |
| `branch`             | `master` | Default branch for all repos                                      |
| `concurrency.repo`   | `1`      | Max concurrent repo operations must be at least `1`               |
| `concurrency.remote` | `0`      | Max concurrent remote ops per repo (0 = no limit)                 |
| `env`                |          | Extra environment variables added unless already set in the shell |

### Platform

| Field    | Default | Description                                                                |
| -------- | ------- | -------------------------------------------------------------------------- |
| `origin` | `false` | Treat as origin remote (exactly one platform must set this to `true`)      |
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

### Index

| Field      | Default                       | Description                                   |
| ---------- | ----------------------------- | --------------------------------------------- |
| `listen`   | `:6070`                       | HTTP listen address                           |
| `database` | `$XDG_DATA_HOME/miroir/index` | Directory for zoekt index shards              |
| `interval` | `300`                         | Seconds between fetch+index cycles            |
| `bare`     | `true`                        | Clone managed repos as bare repos             |
| `include`  | `[]`                          | Extra directories to discover repos (1 level) |

The `include` paths are scanned one level deep for both bare and non-bare git
repos. No git operations (fetch/pull/push) are run on included repos -- they are
only indexed. This is useful for indexing self-hosted Gitea or GitLab
repositories directly from their storage directories.

## Usage

```
miroir <command> [flags]
```

### Target Selection

By default, miroir targets the repo matching your current directory.

- `-n, --name <repo>` -- Target a specific repo by name
- `-a, --all` -- Target all non-archived repos
- `-f, --force` -- Force operation

### Commands

**init** -- Clone and set up repo(s) with all configured remotes

```sh
miroir init                   # Init repo for cwd
miroir init -a                # Init all repos
```

Creates the directory, initializes git, adds all named platform remotes plus
`origin`, fetches, resets to `origin/<branch>`, and initializes submodules.

**fetch** -- Fetch from all remotes (concurrent)

```sh
miroir fetch -a
```

The platform marked `origin = true` is operated through the literal `origin`
remote so shell prompt tooling sees up-to-date upstream state, while progress
output still shows the configured platform name.

**pull** -- Pull from origin

```sh
miroir pull                   # Fails if working tree is dirty
miroir pull -f                # Hard reset then pull
```

Also updates submodules recursively.

**push** -- Push to all remotes (concurrent)

```sh
miroir push -a
miroir push -f                # Force push
```

**exec** -- Run a command in repo(s)

```sh
miroir exec -a -- git status
miroir exec -n myrepo -- make build
```

Runs sequentially with direct stdout/stderr passthrough.

**sync** -- Synchronize repo metadata to all forges

```sh
miroir sync -a
```

Creates repos that don't exist, updates description/visibility on existing ones,
and archives repos marked `archived = true`. Each forge API call has a 30-second
timeout.

**sweep** -- Remove archived and untracked repos from workspace

```sh
miroir sweep                  # Dry run
miroir sweep -f               # Actually delete
```

`sweep` assumes every top-level directory under `general.home` is a managed repo
directory. It is intended for dedicated miroir workspaces, not mixed folders
such as a general `~/Workspace`.

`sweep` does not use `--name` or `--all` to narrow its scope. It always scans
the whole workspace root and removes directories for archived repos plus
directories not present in `[repo.*]`.

**index** -- Start the index daemon (server-side)

```sh
miroir index
miroir index -c /path/to/config.toml
```

Starts a long-running daemon that:

1. Clones/fetches managed repos (from `[repo.*]` config) on a timer
2. Discovers repos from `[index].include` paths (one level deep, no git ops)
3. Indexes each managed repo's configured branch using zoekt's trigram indexer
4. Serves the zoekt search API and web UI over HTTP

The searcher hot-reloads index shards -- no restart needed after re-indexing.
Graceful shutdown on SIGINT/SIGTERM.

Compatible with any zoekt frontend (e.g.
[neogrok](https://github.com/isker/neogrok)):

```sh
ZOEKT_URL=http://localhost:6070 neogrok
```

**completion** -- Generate shell completions

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

- `github.com`, `github.*` -- GitHub
- `gitlab.com`, `gitlab.*` -- GitLab
- `codeberg.org` -- Codeberg
- `*.sr.ht`, `sr.ht` -- SourceHut

Set `forge = "..."` explicitly to override.

## Concurrency

Miroir runs git operations concurrently at two levels:

- **Repo-level**: Controlled by `concurrency.repo` (default 1)
- **Remote-level**: Controlled by `concurrency.remote` (default 0, no limit)

Keep `concurrency.repo` low (2-4) as some forges rate-limit SSH connections.

```toml
[general.concurrency]
repo = 2
remote = 0
```

## Display

When stdout is a TTY, miroir uses a real-time TUI showing per-repo and
per-remote progress. When piped, it falls back to structured log output. The
`index` command always uses structured logging (no TTY mode). When a git command
produces no stdout/stderr for a remote, miroir renders `[no output]` to preserve
the output row ordering.
