# Miroir

Miroir is a WIP declarative git repo manager for synchronizing multiple remotes
(pull/push), executing concurrent commands in multiple repos (exec), and editing
repo metadata (visibility, description, etc.) with supported forges.

## Todo

- CLI flags
  - `-a/--all` is not bool flag and should be global
  - `-c/--config` should be global
  - `-f/--force` missing (force push only? or change how current pull works to
    fetch remotes only, and only replace user working copy when this flag is
    passed?)
  - `-h/--help` missing shorthand
  - `-n/--name` should be global
  - `-v/--version` missing shorthand

- Move to GraphQL and test
  - Codeberg: no support
  - GitHub (todo): https://docs.github.com/en/graphql
  - GitLab (todo): https://docs.gitlab.com/api/graphql/
  - SourceHut (test): https://man.sr.ht/graphql.md

- Add support for fork syncing (match upstream)
  - Codeberg have support but idc
  - GitHub:
    https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/working-with-forks/syncing-a-fork
  - GitLab have support but idc
  - SourceHut not supported
