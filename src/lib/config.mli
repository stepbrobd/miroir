type general =
  { home : string
  ; branch : string
  ; concurrency : int
  ; env : (string * string) list
  }

val show_general : general -> Ppx_deriving_runtime.string
val general_to_toml : general -> Otoml.t
val general_of_toml : Otoml.t -> general

type access =
  | HTTPS
  | SSH

val pp_access : Format.formatter -> access -> unit
val show_access : access -> Ppx_deriving_runtime.string
val access_to_toml : access -> Otoml.t
val access_of_toml : Otoml.t -> access

type forge =
  | Github
  | Gitlab
  | Codeberg
  | Sourcehut

val pp_forge : Format.formatter -> forge -> unit
val show_forge : forge -> Ppx_deriving_runtime.string
val forge_to_toml : forge -> Otoml.t
val forge_of_toml : Otoml.t -> forge
val forge_of_domain : string -> forge option

type platform =
  { origin : bool
  ; domain : string
  ; user : string
  ; access : access
  ; token : string option
  ; forge : forge option
  }

val show_platform : platform -> Ppx_deriving_runtime.string
val platform_of_toml : Otoml.t -> platform
val platform_to_toml : platform -> Otoml.t

type visibility =
  | Public
  | Private

val show_visibility : visibility -> Ppx_deriving_runtime.string
val visibility_to_toml : visibility -> Otoml.t
val visibility_of_toml : Otoml.t -> visibility

type repo =
  { description : string option
  ; visibility : visibility
  ; archived : bool
  ; branch : string option
  }

val show_repo : repo -> Ppx_deriving_runtime.string
val repo_of_toml : Otoml.t -> repo
val repo_to_toml : repo -> Otoml.t

type config =
  { general : general
  ; platform : (string * platform) list
  ; repo : (string * repo) list
  }

val show_config : config -> Ppx_deriving_runtime.string
val config_to_toml : config -> Otoml.t
val config_of_toml : Otoml.t -> config
val resolve_forge : platform -> forge option
val resolve_token : string -> platform -> string option
val config_of_string : string -> config
