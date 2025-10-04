open Ppx_deriving_toml_runtime

type general =
  { home : string
        [@toml.default "~/"]
        (* the root directory of where users want to put all their repos at *)
  ; concurrency : int
        [@toml.default 1] (* number of parallelism if the task can be run concurrently *)
  ; env : (string * string) list option [@toml.option]
    (* environment variables to be made available *)
  }
[@@deriving toml, show]

type access =
  | HTTPS
  | SSH
[@@deriving toml, show]

type platform =
  { origin : bool (* whether this git forge is considered as the fetch target *)
  ; domain : string (* domain name for the git forge *)
  ; user : string (* used to determine full repo url *)
  ; access : access [@toml.default SSH] (* how to pull/push *)
  }
[@@deriving toml, show]

type visibility =
  | Public
  | Private
[@@deriving toml, show]

type repo =
  { description : string option [@toml.option] (* repo description *)
  ; visibility : visibility [@toml.default Private] (* public or private *)
  ; archived : bool [@toml.default false]
    (* if true, repo will not be pulled/pushed, but metadata will still be managed *)
  }
[@@deriving toml, show]

type config =
  { general : general
  ; platform : (string * platform) list [@toml.default []]
  ; repo : (string * repo) list [@toml.default []]
  }
[@@deriving toml, show]

let parse_config str = config_of_toml (Otoml.Parser.from_string str)

let show_config config_file =
  let config =
    In_channel.with_open_text config_file In_channel.input_all |> parse_config
  in
  Printf.printf "Home: %s\n" config.general.home;
  Printf.printf "Concurrency: %d\n" config.general.concurrency
;;
