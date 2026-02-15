# Miroir

Miroir is a WIP declarative git repo manager for synchronizing multiple remotes
(pull/push), executing concurrent commands in multiple repos (exec), and editing
repo metadata (visibility, description, etc.) with supported forges.

## Todo

- CLI flags
  - `-h/--help` missing shorthand (cmdliner limitation, needs argv rewriting)
  - `-v/--version` missing shorthand (cmdliner limitation, needs argv rewriting)

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
