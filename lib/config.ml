open Otoml
open Ppx_deriving_toml_runtime

type general =
  { home : string
        [@toml.default "~/"]
        (* the root directory of where users want to put all their repos at *)
  ; concurrency : int
        [@toml.default 1] (* number of parallelism if the task can be run concurrently *)
  ; env : (string * string) list [@toml.default []]
    (* environment variables to be made available *)
  }
[@@deriving toml, show]

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
[@@deriving toml, show]

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
[@@deriving toml, show]

type config =
  { general : general
  ; platform : (string * platform) list [@toml.default []]
  ; repo : (string * repo) list [@toml.default []]
  }
[@@deriving toml, show]

let config_of_toml toml =
  let table_to_assoc_list of_toml = function
    | TomlTable items | TomlInlineTable items ->
      List.map (fun (k, v) -> k, of_toml v) items
    | _ -> of_toml_error "expected a table"
  in
  let get_table_opt key items =
    try Some (List.assoc key items) with
    | Not_found -> None
  in
  (* this ugly hunk of junk is needed because *i think* deriving toml cant natively parse ('a * 'b) list into tables *)
  match toml with
  | TomlTable items ->
    let general =
      match get_table_opt "general" items with
      | Some (TomlTable gen_items) ->
        let env =
          match get_table_opt "env" gen_items with
          | Some (TomlTable env_items | TomlInlineTable env_items) ->
            List.map (fun (k, v) -> k, string_of_toml v) env_items
          | Some _ -> of_toml_error "expected env to be a table"
          | None -> []
        in
        let gen_without_env = List.remove_assoc "env" gen_items in
        let g = general_of_toml (TomlTable gen_without_env) in
        { g with env }
      | Some _ -> of_toml_error "expected general to be a table"
      | None -> of_toml_error "missing required field: general"
    in
    let platform =
      match get_table_opt "platform" items with
      | Some t -> table_to_assoc_list platform_of_toml t
      | None -> []
    in
    let repo =
      match get_table_opt "repo" items with
      | Some t -> table_to_assoc_list repo_of_toml t
      | None -> []
    in
    { general; platform; repo }
  | _ -> of_toml_error "expected root to be a table"
;;

let config_of_string str = config_of_toml (Parser.from_string str)
