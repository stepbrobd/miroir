type general =
  { home : string
  ; concurrency : int
  ; env : (string * string) list
  }

val general_of_toml : Otoml.t -> general
val general_to_toml : general -> Otoml.t
val show_general : general -> Ppx_deriving_runtime.string

type access =
  | HTTPS
  | SSH

val access_to_toml : access -> Otoml.t
val access_of_toml : Otoml.t -> access
val show_access : access -> Ppx_deriving_runtime.string

type platform =
  { origin : bool
  ; domain : string
  ; user : string
  ; access : access
  }

val platform_of_toml : Otoml.t -> platform
val platform_to_toml : platform -> Otoml.t
val show_platform : platform -> Ppx_deriving_runtime.string

type visibility =
  | Public
  | Private

val visibility_to_toml : visibility -> Otoml.t
val visibility_of_toml : Otoml.t -> visibility
val show_visibility : visibility -> Ppx_deriving_runtime.string

type repo =
  { description : string option
  ; visibility : visibility
  ; archived : bool
  }

val repo_of_toml : Otoml.t -> repo
val repo_to_toml : repo -> Otoml.t
val show_repo : repo -> Ppx_deriving_runtime.string

type config =
  { general : general
  ; platform : (string * platform) list
  ; repo : (string * repo) list
  }

val config_to_toml : config -> Otoml.t
val config_of_toml : Otoml.t -> config
val config_of_string : string -> config
val show_config : config -> Ppx_deriving_runtime.string
