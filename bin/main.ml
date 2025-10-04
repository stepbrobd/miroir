open Miroir.Config

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
  let test_config () =
    let config_str = In_channel.with_open_text "config.toml" In_channel.input_all in
    let config = parse_config config_str in
    Printf.printf "Home: %s\n" config.general.home;
    Printf.printf "Concurrency: %d\n" config.general.concurrency;
    List.iter
      (fun (name, p) ->
         Printf.printf "Platform %s: %s@%s (origin=%b)\n" name p.user p.domain p.origin)
      config.platform;
    List.iter
      (fun (name, r) ->
         Printf.printf
           "Repo %s: %s (visibility=%s, archived=%b)\n"
           name
           r.description
           r.visibility
           r.archived)
      config.repo
  in
  let f p = show_params p |> print_endline in
  let info = Cmdliner.Cmd.info Sys.argv.(0) in
  let term = Cmdliner.Term.(const f $ params_cmdliner_term ()) in
  let cmd = Cmdliner.Cmd.v info term in
  exit (Cmdliner.Cmd.eval cmd)
;;

let () = main ()
