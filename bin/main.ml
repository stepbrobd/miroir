open Cmdliner
open Miroir

type params = { config : string [@aka [ "c" ]] [@default ""] [@env "MIROIR_CONFIG"] }
[@@deriving cmdliner]

let show_config config_file =
  In_channel.with_open_text config_file In_channel.input_all
  |> Config.config_of_string
  |> Config.show_config
  |> print_endline
;;

let main () =
  let f p = show_config p.config in
  let info = Cmd.info "miroir" ~version:(Version.get ()) ~doc:"repo manager wannabe?" in
  let term = Term.(const f $ params_cmdliner_term ()) in
  let cmd = Cmd.v info term in
  Cmd.eval cmd
;;

let () = if !Sys.interactive then () else exit (main ())
