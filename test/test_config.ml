open Alcotest
open Miroir

let from filename =
  let path = Filename.concat "data" filename in
  In_channel.with_open_text path In_channel.input_all
;;

let test_config_default () =
  let toml = "[general]" in
  let config = Config.config_of_string toml in
  check string "default home" "~/" config.general.home;
  check int "platform count" 0 (List.length config.platform);
  check int "repo count" 0 (List.length config.repo)
;;

let test_config_simple () =
  let cfg = Config.config_of_string (from "simple.toml") in
  check string "home" "~/Workspace" cfg.general.home;
  check int "concurrency" 1 cfg.general.concurrency;
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
  check int "concurrency" 2 cfg.general.concurrency;
  check int "env length" 1 (List.length cfg.general.env);
  let git_ssh = List.assoc "GIT_SSH_COMMAND" cfg.general.env in
  check
    string
    "git ssh command"
    "ssh -o TcpKeepAlive=no -o ServerAliveInterval=10"
    git_ssh;
  check int "platform count" 2 (List.length cfg.platform);
  let github = List.assoc "github" cfg.platform in
  let gitlab = List.assoc "gitlab" cfg.platform in
  check bool "github origin" true github.origin;
  check bool "gitlab origin" false gitlab.origin;
  check int "repo count" 3 (List.length cfg.repo);
  let nix = List.assoc "nix" cfg.repo in
  check (option string) "nix description" (Some "forked from nixos/nix") nix.description
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
