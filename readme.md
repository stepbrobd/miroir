# Miroir

[![atelier](https://github.com/stepbrobd/miroir/actions/workflows/ci.yaml/badge.svg)](https://github.com/stepbrobd/atelier)

Binary Cache:

- Cache: <https://cache.ysun.co>
- Key: `cache.ysun.co-1:WxPYwT5g3kt9XhUhHPpNLZKI9HIOsVVAuqSHpok8Qt4=`

Declarative git repo manager and code search server. Synchronize multiple
remotes, execute commands across repos, manage forge metadata from a single TOML
config, and serve full-text code search via
[zoekt](https://github.com/sourcegraph/zoekt).

Miroir can double as a forge-to-forge migration tool, say, move everything off
GitHub (or any other forge):

1. Enumerate source repos via `gh repo list` (or each forge's API, or by hand)
   and write a `[repo.*]` entry per repo into your config
2. Add an SSH key and generate an API token for the source and each destination
   forge, then add them as `[platform.*]` entries. Mark the migration source as
   `origin = true`
3. `miroir init -a` clones from the current origin, `miroir sync -a` creates the
   destination repos with the configured description/visibility, and
   `miroir push -a` populates them across every platform remote
4. Mark the new forge `origin = true` and set migration source `origin = false`

<!-- deno-fmt-ignore -->
> [!Caution]
> Miroir `sync` command is reconciliation, not append. It treats your config as
> the source of truth for every configured forge. It will set repos `private`
> when the config says so (on GitHub this wipes stars, forks, and watchers,
> because they are public-graph artifacts attached to the public listing), flip
> `archived`, and overwrite descriptions. A typo or a misaimed config against
> the wrong account can do real damage. Review your TOML carefully, try a
> single repo end-to-end before `-a`, and keep the source forge intact until
> you have verified the destination.

## Config

Miroir looks for config in this order:

1. `--config` / `-c` flag
2. `MIROIR_CONFIG` environment variable
3. `$XDG_CONFIG_HOME/miroir/config.toml` (typically
   `~/.config/miroir/config.toml`)

Unknown or misspelled keys are rejected at load time, so a typo cannot silently
fall back to a default that `sync` would then reconcile onto your forges.

### Example

See my account's repo configuration
[here](https://github.com/stepbrobd/inc/blob/master/repos/config.toml) or some
screen recordings [here](https://github.com/stepbrobd/inc/issues/112) or a
simplified version below:

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
archived = true               # Excluded from git ops and archived on supporting forges via sync

[index]
listen = ":6070"              # HTTP listen address for search API
database = "/data/miroir/idx" # Zoekt shard storage (default: $XDG_DATA_HOME/miroir/index)
interval = 300                # Seconds between fetch+index cycles
bare = true                   # true = daemon bare repo synced from origin and indexed at HEAD
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

Managed repos are a flat set of direct children under `general.home`. Nested
repo names such as `group/repo` are not supported.

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
`MIROIR_GITHUB_TOKEN`). Platform names are uppercased and non-alphanumeric
characters are replaced with `_`, so `gitlab-main` maps to
`MIROIR_GITLAB_MAIN_TOKEN`. Platform names must normalize uniquely.

### Repo

| Field         | Default   | Description                                            |
| ------------- | --------- | ------------------------------------------------------ |
| `description` |           | Repo description synced to forges                      |
| `visibility`  | `private` | `public` or `private`                                  |
| `archived`    | `false`   | Skip in git ops; archive on supporting forges via sync |
| `branch`      |           | Per-repo branch override                               |

### Index

| Field      | Default                       | Description                                                                                                                                    |
| ---------- | ----------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| `listen`   | `:6070`                       | HTTP listen address                                                                                                                            |
| `database` | `$XDG_DATA_HOME/miroir/index` | Directory for zoekt index shards                                                                                                               |
| `interval` | `300`                         | Seconds between fetch+index cycles, must be at least `1`                                                                                       |
| `bare`     | `true`                        | `true` keeps a daemon-owned bare repo mirrored from origin and indexes `HEAD`, `false` keeps a normal clone and indexes its checked out `HEAD` |
| `include`  | `[]`                          | Extra directories to discover repos (1 level)                                                                                                  |

The `include` paths are scanned one level deep for both bare and non-bare git
repos. Included repos are only indexed, miroir never fetches, pulls, or pushes
them. This is useful for indexing self-hosted Gitea or GitLab repositories
directly from their storage directories.

## Usage

```
miroir <command> [flags]
```

### Target Selection

By default, miroir targets the repo matching your current directory.

- `-n, --name <repo>` targets a specific repo by name
- `-a, --all` targets all non-archived repos
- `-f, --force` forces the operation

Flags are scoped per command: `-n`/`-a` apply to `init`, `fetch`, `pull`,
`push`, `exec`, and `sync`. `-f` applies to `init`, `fetch`, `pull`, `push`, and
`sweep`. `-t` applies to `push`. `--tty`/`--no-tty` apply to the commands that
render progress output (`init`, `fetch`, `pull`, `push`, `exec`, `sync`). A flag
a command does not take is rejected with an error rather than silently ignored.

### Commands

**init** clones and sets up repo(s) with all configured remotes

```sh
miroir init                   # Init repo for cwd
miroir init -a                # Init all repos
```

Creates the directory, initializes git, adds all named platform remotes plus
`origin`, fetches, resets to `origin/<branch>`, and initializes submodules. When
the repo already exists, `init` refuses to overwrite a dirty working tree unless
you pass `-f`.

**fetch** fetches from all remotes concurrently

```sh
miroir fetch -a
```

The platform marked `origin = true` is operated through the literal `origin`
remote so shell prompt tooling sees up-to-date upstream state, while progress
output still shows the configured platform name.

**pull** pulls from origin

```sh
miroir pull                   # Fails if working tree is dirty
miroir pull -f                # Hard reset then pull
```

Also updates submodules recursively.

**push** pushes to all remotes concurrently

```sh
miroir push -a
miroir push -f                # Force push
miroir push -t                # Push every local tag as well
```

**exec** runs a command in repo(s)

```sh
miroir exec -a -- git status
miroir exec -n myrepo -- make build
```

Runs sequentially with direct stdout/stderr passthrough.

**sync** synchronizes repo metadata to all forges

```sh
miroir sync -a
```

Creates repos that don't exist, updates description/visibility on existing ones,
and archives repos marked `archived = true` on forges that support archiving.
Each forge API call has a 30-second timeout.

**sweep** removes archived and untracked repos from the workspace

```sh
miroir sweep                  # Dry run
miroir sweep -f               # Actually delete
```

`sweep` assumes every top-level directory under `general.home` is a managed repo
directory. It is intended for dedicated miroir workspaces, not mixed folders
such as a general `~/Workspace`.

`sweep` rejects `--name` and `--all`. It always scans the whole workspace root
and removes directories for archived repos plus directories not present in
`[repo.*]`.

**index** starts the index daemon on the server

```sh
miroir index
miroir index -c /path/to/config.toml
```

Starts a long-running daemon that:

1. Synchronizes managed repos (from `[repo.*]` config) on a timer
2. Discovers repos from `[index].include` paths (one level deep, no git ops)
3. Indexes each managed repo using zoekt's trigram indexer
4. Removes daemon-managed repo directories and shards for repos removed from
   config or marked archived, a shard counts as daemon-managed when its source
   sits under `general.home` or its name starts with the origin's
   `<domain>/<user>/`
5. Removes stale shards for disappeared `index.include` repos
6. Serves the zoekt search API and web UI over HTTP

With `index.bare = true`, miroir keeps a daemon-owned bare repo at
`general.home/<repo>.git`. Each cycle fetches origin heads straight into
`refs/heads/*` with prune and without tags, so the local branch set mirrors
origin, then points `HEAD` at the configured branch and indexes `HEAD`. Auto gc
runs in the foreground of the fetch, so no detached repack can remove a pack
while the indexer reads it.

With `index.bare = false`, miroir keeps a normal clone at `general.home/<repo>`.
The first clone uses the configured branch, later cycles only run
`git fetch --prune origin`, and indexing always follows the repo's current
checked out `HEAD`.

Included repos from `index.include` are never fetched or deleted by miroir. Only
their shards are removed if the source repo disappears from discovery.

The searcher hot-reloads index shards, so re-indexing needs no restart. On
SIGINT/SIGTERM, miroir stops serving immediately, cancels the current cycle,
waits for any in-flight fetch or index step to finish, and exits 0.

Compatible with any zoekt frontend (e.g.
[neogrok](https://github.com/isker/neogrok)):

```sh
ZOEKT_URL=http://localhost:6070 neogrok
```

**completion** generates shell completions

```sh
miroir completion bash >> ~/.bashrc
miroir completion zsh > ~/.zfunc/_miroir
miroir completion fish > ~/.config/fish/completions/miroir.fish
```

## Supported Forges

| Forge     | Sync | Archive via sync |
| --------- | ---- | ---------------- |
| GitHub    | Yes  | Yes              |
| GitLab    | Yes  | Yes              |
| Codeberg  | Yes  | Yes              |
| SourceHut | Yes  | No               |

`sync` creates missing repos and updates description, visibility, and archived
state on existing ones.

Forge type is auto-detected from the platform domain:

- `github.com` and `github.*` map to GitHub
- `gitlab.com` and `gitlab.*` map to GitLab
- `codeberg.org` maps to Codeberg
- `*.sr.ht` and `sr.ht` map to SourceHut

Set `forge = "..."` explicitly to override. Forge API calls target the
configured domain, so GitHub Enterprise, self-hosted GitLab, and Gitea or
Forgejo instances (via `forge = "codeberg"`) work with an explicit setting.

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

## License

The contents inside this repository, excluding all submodules, are licensed
under the [MIT License](license.txt). Third-party file(s) and/or code(s) are
subject to their original term(s) and/or license(s).
