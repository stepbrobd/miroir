(* TODO *)
type params =
  { username : string (** Your Github username *)
  ; api_key : string (** Your Github API key *)
  ; command : string [@pos 0] [@docv "CMD"] (** The Github API command to run *)
  ; dry_run : bool (** Don't really run this command *)
  ; time_to_wait : float [@default 0.] (** Just an example of another type *)
  }
[@@deriving cmdliner, show]

let main () =
  let f p = show_params p |> print_endline in
  let info = Cmdliner.Cmd.info Sys.argv.(0) in
  let term = Cmdliner.Term.(const f $ params_cmdliner_term ()) in
  let cmd = Cmdliner.Cmd.v info term in
  exit (Cmdliner.Cmd.eval cmd)
;;

let () = main ()
