open Cmdliner
open Miroir

type params = { config : string [@aka [ "c" ]] [@default ""] [@env "MIROIR_CONFIG"] }
[@@deriving cmdliner]

let main () =
  let f p = Config.show_config p.config in
  let info = Cmd.info "miroir" ~version:(Version.get ()) ~doc:"repo manager wannabe?" in
  let term = Term.(const f $ params_cmdliner_term ()) in
  let cmd = Cmd.v info term in
  Cmd.eval cmd
;;

let () = if !Sys.interactive then () else exit (main ())
