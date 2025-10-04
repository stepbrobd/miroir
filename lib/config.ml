open Ppx_deriving_toml_runtime

(* https://github.com/andreypopp/ppx_deriving/blob/main/toml/ppx_deriving_toml_runtime.ml dont have ppx for map? *)

type general =
  { home : string
  ; concurrency : int
  ; env : (string * string) list
  }
[@@deriving toml, show]

type platform =
  { origin : bool
  ; domain : string
  ; user : string
  }
[@@deriving toml, show]

type repo =
  { description : string
  ; visibility : string
  ; archived : bool
  }
[@@deriving toml, show]

type config =
  { general : general
  ; platform : (string * platform) list
  ; repo : (string * repo) list
  }
[@@deriving toml, show]

let parse_config toml_string = config_of_toml (Otoml.Parser.from_string toml_string)
