# Todo

- Separate per-repo/per-remote concurrency control (now everything is wrapped
  under one variable)

- Change output format, `\r` flush each output line instead of printing in new
  lines
  - Delimit each repo by its name
  - In a new line after repo name, flush op updates

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
