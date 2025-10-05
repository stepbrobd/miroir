open Miroir

type params =
  { config : string [@default ""] [@env "MIROIR_CONFIG"] [@names [ "c"; "config" ]] }
[@@deriving subliner]

let show_config config_file =
  In_channel.with_open_text config_file In_channel.input_all
  |> Config.config_of_string
  |> Config.show_config
  |> print_endline
;;

let main { config } = show_config config

(** repo manager wannabe? *)
[%%subliner.term eval.params <- main]
  [@@name "miroir"] [@@version Version.get ()]
