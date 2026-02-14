(** configuration types and TOML parsing *)

(** global settings *)
type general =
  { home : string
  ; branch : string
  ; concurrency : int
  ; env : (string * string) list
  }

val show_general : general -> string
val general_to_toml : general -> Otoml.t
val general_of_toml : Otoml.t -> general

(** git transport protocol *)
type access =
  | HTTPS
  | SSH

val pp_access : Format.formatter -> access -> unit
val show_access : access -> string
val access_to_toml : access -> Otoml.t
val access_of_toml : Otoml.t -> access

(** supported forge types *)
type forge =
  | Github
  | Gitlab
  | Codeberg
  | Sourcehut

val pp_forge : Format.formatter -> forge -> unit
val show_forge : forge -> string
val forge_to_toml : forge -> Otoml.t
val forge_of_toml : Otoml.t -> forge

(** auto-detect forge type from domain name *)
val forge_of_domain : string -> forge option

(** a git hosting platform *)
type platform =
  { origin : bool
  ; domain : string
  ; user : string
  ; access : access
  ; token : string option
  ; forge : forge option
  }

val show_platform : platform -> string
val platform_of_toml : Otoml.t -> platform
val platform_to_toml : platform -> Otoml.t

(** repo visibility on forge *)
type visibility =
  | Public
  | Private

val show_visibility : visibility -> string
val visibility_to_toml : visibility -> Otoml.t
val visibility_of_toml : Otoml.t -> visibility

(** a managed repository *)
type repo =
  { description : string option
  ; visibility : visibility
  ; archived : bool
  ; branch : string option
  }

val show_repo : repo -> string
val repo_of_toml : Otoml.t -> repo
val repo_to_toml : repo -> Otoml.t

(** top-level config *)
type config =
  { general : general
  ; platform : (string * platform) list
  ; repo : (string * repo) list
  }

val show_config : config -> string
val config_to_toml : config -> Otoml.t
val config_of_toml : Otoml.t -> config

(** resolve forge type: explicit field > auto-detect from domain *)
val resolve_forge : platform -> forge option

(** resolve token: MIROIR_<NAME>_TOKEN env var > config token field *)
val resolve_token : string -> platform -> string option

(** parse a TOML string into config *)
val config_of_string : string -> config
