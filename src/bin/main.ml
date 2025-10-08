open Miroir

type args =
  { config : string [@default ""] [@names [ "c"; "config" ]] [@env "MIROIR_CONFIG"]
  ; name : string [@default ""] [@names [ "n"; "name" ]]
  ; all : bool [@default false] [@names [ "a"; "all" ]]
  ; args : string list [@pos_all]
  }
[@@deriving subliner]

type cmds =
  | Init of args (** Initialize repo(s) (destructive, uncommitted changes will be lost) *)
  | Pull of args (** Pull from origin *)
  | Push of args (** Push to all remotes *)
  | Exec of args (** Execute command in repo(s) *)
[@@deriving subliner]

let get_targets { config; name; all; _ } =
  match Git.available () with
  | Error e ->
    Printf.eprintf "Error: %s\n" e;
    exit 1
  | Ok () ->
    let cfg =
      In_channel.with_open_text config In_channel.input_all |> Config.config_of_string
    in
    let contexts = Context.make_contexts cfg in
    let home = Context.expand_home cfg.general.home in
    if name <> ""
    then [ Filename.concat home name ], contexts
    else if all
    then List.map fst contexts, contexts
    else (
      let cwd = Sys.getcwd () in
      match
        List.find_opt (fun (path, _) -> String.starts_with ~prefix:path cwd) contexts
      with
      | Some (path, _) -> [ path ], contexts
      | None ->
        Printf.eprintf "fatal: not a managed repository (cwd: %s)\n" cwd;
        exit 1)
;;

let run_on_targets ~targets ~contexts f =
  List.iter
    (fun path ->
       let ctx = List.assoc path contexts in
       f ~path ~ctx)
    targets
;;

(** Repo manager wannabe? *)
[%%subliner.cmds
  eval.cmds
  <- (function
       | Init args ->
         let targets, contexts = get_targets args in
         run_on_targets ~targets ~contexts (fun ~path ~ctx ->
           Git.init ~path ~ctx ~args:args.args ())
       | Pull args ->
         let targets, contexts = get_targets args in
         run_on_targets ~targets ~contexts (fun ~path ~ctx ->
           Git.pull ~path ~ctx ~args:args.args () |> ignore)
       | Push args ->
         let targets, contexts = get_targets args in
         run_on_targets ~targets ~contexts (fun ~path ~ctx ->
           Git.push ~path ~ctx ~args:args.args () |> ignore)
       | Exec args ->
         let targets, contexts = get_targets args in
         run_on_targets ~targets ~contexts (fun ~path ~ctx ->
           Git.exec ~path ~ctx ~cmd:args.args))]
  [@@name "miroir"] [@@version Version.get ()]
