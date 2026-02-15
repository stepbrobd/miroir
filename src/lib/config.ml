open Otoml
open Ppx_deriving_toml_runtime

type concurrency =
  { repo : int [@toml.default 0] (* max repos processed in parallel, 0 = all *)
  ; remote : int [@toml.default 0] (* max remotes per repo in parallel, 0 = all *)
  }
[@@deriving show, toml]

let default_concurrency = { repo = 0; remote = 0 }

type general =
  { home : string
        [@toml.default "~/"]
        (* the root directory of where users want to put all their repos at *)
  ; branch : string [@toml.default "master"] (* default branch name *)
  ; concurrency : concurrency [@toml.default default_concurrency]
  ; env : (string * string) list [@toml.default []] [@toml.assoc_table]
    (* environment variables to be made available *)
  }
[@@deriving show, toml, toml_assoc_table]

(* weird shit, cant use deriving toml *)
type access =
  | HTTPS
  | SSH
[@@deriving show]

let access_to_toml = function
  | HTTPS -> TomlString "https"
  | SSH -> TomlString "ssh"
;;

let access_of_toml = function
  | TomlString s ->
    (match String.lowercase_ascii s with
     | "https" -> HTTPS
     | "ssh" -> SSH
     | _ -> of_toml_error "expected either `https` or `ssh`")
  | _ -> of_toml_error "expected string value for access"
;;

type forge =
  | Github
  | Gitlab
  | Codeberg
  | Sourcehut
[@@deriving show]

let forge_to_toml = function
  | Github -> TomlString "github"
  | Gitlab -> TomlString "gitlab"
  | Codeberg -> TomlString "codeberg"
  | Sourcehut -> TomlString "sourcehut"
;;

let forge_of_toml = function
  | TomlString s ->
    (match String.lowercase_ascii s with
     | "github" -> Github
     | "gitlab" -> Gitlab
     | "codeberg" -> Codeberg
     | "sourcehut" -> Sourcehut
     | _ -> of_toml_error "expected one of: github, gitlab, codeberg, sourcehut")
  | _ -> of_toml_error "expected string value for forge"
;;

(* auto-detect forge from domain *)
let forge_of_domain domain =
  let d = String.lowercase_ascii domain in
  if String.equal d "github.com" || String.starts_with ~prefix:"github." d
  then Some Github
  else if String.equal d "gitlab.com" || String.starts_with ~prefix:"gitlab." d
  then Some Gitlab
  else if String.equal d "codeberg.org"
  then Some Codeberg
  else if String.ends_with ~suffix:".sr.ht" d || String.equal d "sr.ht"
  then Some Sourcehut
  else None
;;

type platform =
  { origin : bool (* whether this git forge is considered as the fetch target *)
  ; domain : string (* domain name for the git forge *)
  ; user : string (* used to determine full repo uri *)
  ; access : access [@toml.default SSH] (* how to pull/push *)
  ; token : string option
        [@toml.option] (* api token, overridden by MIROIR_<NAME>_TOKEN *)
  ; forge : forge option [@toml.option]
    (* forge type, auto-detected from domain if omitted *)
  }
[@@deriving show, toml]

(* another weird shit *)
type visibility =
  | Public
  | Private
[@@deriving show]

let visibility_to_toml = function
  | Public -> TomlString "public"
  | Private -> TomlString "private"
;;

let visibility_of_toml = function
  | TomlString s ->
    (match String.lowercase_ascii s with
     | "public" -> Public
     | "private" -> Private
     | _ -> of_toml_error "expected either `public` or `private`")
  | _ -> of_toml_error "expected string value for visibility"
;;

type repo =
  { description : string option [@toml.option] (* repo description *)
  ; visibility : visibility [@toml.default Private] (* public or private *)
  ; archived : bool
        [@toml.default false]
        (* if true, repo will not be pulled/pushed, but metadata will still be managed *)
        (* TODO: allow override *)
  ; branch : string option [@toml.option] (* per-repo branch override *)
  }
[@@deriving show, toml]

type config =
  { general : general
  ; platform : (string * platform) list [@toml.default []] [@toml.assoc_table]
  ; repo : (string * repo) list [@toml.default []] [@toml.assoc_table]
  }
[@@deriving show, toml, toml_assoc_table]

(* resolve the effective forge for a platform: explicit > auto-detect *)
let resolve_forge (p : platform) =
  match p.forge with
  | Some f -> Some f
  | None -> forge_of_domain p.domain
;;

(* resolve token: env var MIROIR_<NAME>_TOKEN > config token field *)
let resolve_token name (p : platform) =
  let var = "MIROIR_" ^ String.uppercase_ascii name ^ "_TOKEN" in
  match Sys.getenv_opt var with
  | Some t -> Some t
  | None -> p.token
;;

let config_of_string str = config_of_toml (Parser.from_string str)
