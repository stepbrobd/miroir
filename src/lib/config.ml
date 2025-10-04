open Otoml
open Ppx_deriving_toml_runtime

type general =
  { home : string
        [@toml.default "~/"]
        (* the root directory of where users want to put all their repos at *)
  ; concurrency : int
        [@toml.default 1] (* number of parallelism if the task can be run concurrently *)
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

type platform =
  { origin : bool (* whether this git forge is considered as the fetch target *)
  ; domain : string (* domain name for the git forge *)
  ; user : string (* used to determine full repo url *)
  ; access : access [@toml.default SSH] (* how to pull/push *)
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
  ; archived : bool [@toml.default false]
    (* if true, repo will not be pulled/pushed, but metadata will still be managed *)
    (* TODO: allow override *)
  }
[@@deriving show, toml]

type config =
  { general : general
  ; platform : (string * platform) list [@toml.default []] [@toml.assoc_table]
  ; repo : (string * repo) list [@toml.default []] [@toml.assoc_table]
  }
[@@deriving show, toml, toml_assoc_table]

let config_of_string str = config_of_toml (Parser.from_string str)
