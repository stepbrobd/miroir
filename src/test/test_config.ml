open Alcotest
open Miroir

let from filename =
  let path = Filename.concat "fixture" filename in
  In_channel.with_open_text path In_channel.input_all
;;

let test_config_default () =
  let toml = "[general]" in
  let cfg = Config.config_of_string toml in
  check string "default home" "~/" cfg.general.home;
  check string "default branch" "master" cfg.general.branch;
  check int "default concurrency.repo" 1 cfg.general.concurrency.repo;
  check int "default concurrency.remote" 1 cfg.general.concurrency.remote;
  check int "platform count" 0 (List.length cfg.platform);
  check int "repo count" 0 (List.length cfg.repo)
;;

let test_config_simple () =
  let cfg = Config.config_of_string (from "simple.toml") in
  check string "home" "~/Workspace" cfg.general.home;
  check string "branch" "master" cfg.general.branch;
  check int "concurrency.repo" 1 cfg.general.concurrency.repo;
  check int "concurrency.remote" 1 cfg.general.concurrency.remote;
  check int "env length" 0 (List.length cfg.general.env);
  check int "platform count" 1 (List.length cfg.platform);
  let github = List.assoc "github" cfg.platform in
  check bool "github origin" true github.origin;
  check string "github domain" "github.com" github.domain;
  check string "github user" "stepbrobd" github.user;
  check int "repo count" 1 (List.length cfg.repo);
  let miroir = List.assoc "miroir" cfg.repo in
  check
    (option string)
    "miroir description"
    (Some "repo manager wannabe?")
    miroir.description;
  check bool "miroir archived" false miroir.archived
;;

let test_config_complex () =
  let cfg = Config.config_of_string (from "complex.toml") in
  check string "home" "~/Workspace" cfg.general.home;
  check string "branch" "master" cfg.general.branch;
  check int "concurrency.repo" 2 cfg.general.concurrency.repo;
  check int "concurrency.remote" 3 cfg.general.concurrency.remote;
  check int "env length" 1 (List.length cfg.general.env);
  let git_ssh = List.assoc "GIT_SSH_COMMAND" cfg.general.env in
  check
    string
    "git ssh command"
    "ssh -o TcpKeepAlive=no -o ServerAliveInterval=10"
    git_ssh;
  check int "platform count" 4 (List.length cfg.platform);
  let github = List.assoc "github" cfg.platform in
  let gitlab = List.assoc "gitlab" cfg.platform in
  let codeberg = List.assoc "codeberg" cfg.platform in
  let srht = List.assoc "sourcehut" cfg.platform in
  check bool "github origin" true github.origin;
  check bool "gitlab origin" false gitlab.origin;
  check bool "codeberg origin" false codeberg.origin;
  check bool "sourcehut origin" false srht.origin;
  (* forge field: explicit on github, auto-detected on others *)
  check (option (of_pp Config.pp_forge)) "github forge" (Some Config.Github) github.forge;
  check (option (of_pp Config.pp_forge)) "gitlab forge" None gitlab.forge;
  (* token: explicit on github *)
  check (option string) "github token" (Some "ghp_test123") github.token;
  check (option string) "gitlab token" None gitlab.token;
  (* access types *)
  check (of_pp Config.pp_access) "github access" Config.SSH github.access;
  check (of_pp Config.pp_access) "gitlab access" Config.HTTPS gitlab.access;
  (* forge resolution *)
  check
    (option (of_pp Config.pp_forge))
    "github resolved forge"
    (Some Config.Github)
    (Config.resolve_forge github);
  check
    (option (of_pp Config.pp_forge))
    "gitlab resolved forge"
    (Some Config.Gitlab)
    (Config.resolve_forge gitlab);
  check
    (option (of_pp Config.pp_forge))
    "codeberg resolved forge"
    (Some Config.Codeberg)
    (Config.resolve_forge codeberg);
  check
    (option (of_pp Config.pp_forge))
    "sourcehut resolved forge"
    (Some Config.Sourcehut)
    (Config.resolve_forge srht);
  check int "repo count" 3 (List.length cfg.repo);
  let nix = List.assoc "nix" cfg.repo in
  check (option string) "nix description" (Some "forked from nixos/nix") nix.description;
  (* per-repo branch override *)
  check (option string) "nix branch" (Some "master") nix.branch;
  let nixpkgs = List.assoc "nixpkgs" cfg.repo in
  check (option string) "nixpkgs branch" None nixpkgs.branch
;;

let () =
  run
    "config"
    [ ( "all"
      , [ test_case "default" `Quick test_config_default
        ; test_case "simple" `Quick test_config_simple
        ; test_case "complex" `Quick test_config_complex
        ] )
    ]
;;
