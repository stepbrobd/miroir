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
  | Sync of args (** Sync metadata to all forges *)
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
    let ctxs = Context.make_all cfg in
    let home = Context.expand_home cfg.general.home in
    if name <> ""
    then [ Filename.concat home name ], ctxs, cfg
    else if all
    then List.map fst ctxs, ctxs, cfg
    else (
      let cwd = Sys.getcwd () in
      match List.find_opt (fun (path, _) -> String.starts_with ~prefix:path cwd) ctxs with
      | Some (path, _) -> [ path ], ctxs, cfg
      | None ->
        Printf.eprintf "fatal: not a managed repository (cwd: %s)\n" cwd;
        exit 1)
;;

(* run f on each target, bounded by concurrency semaphore *)
let run_on ~fs ~mgr ~targets ~ctxs ~sem f =
  Eio.Fiber.all
    (List.map
       (fun target () ->
          Eio.Semaphore.acquire sem;
          Fun.protect
            ~finally:(fun () -> Eio.Semaphore.release sem)
            (fun () ->
               let ctx = List.assoc target ctxs in
               let path = Eio.Path.(fs / target) in
               match f ~mgr ~path ~ctx with
               | Ok () -> ()
               | Error e -> Printf.eprintf "Error: %s\n%!" e))
       targets)
;;

let sync_repo ~client ~cfg name =
  let repo =
    match List.assoc_opt name cfg.Config.repo with
    | Some r -> r
    | None ->
      Printf.eprintf "warning: no repo config for %s\n%!" name;
      { Config.description = None; visibility = Private; archived = false; branch = None }
  in
  List.iter
    (fun (pname, (p : Config.platform)) ->
       match Config.resolve_forge p, Config.resolve_token pname p with
       | None, _ ->
         Printf.eprintf "warning: unknown forge for platform %s, skipping\n%!" pname
       | _, None -> Printf.eprintf "warning: no token for platform %s, skipping\n%!" pname
       | Some forge, Some token ->
         let module F = (val Forge.dispatch forge : Forge.S) in
         let meta =
           { Forge.name
           ; desc = repo.description
           ; vis = repo.visibility
           ; archived = repo.archived
           }
         in
         (match F.sync client ~token ~user:p.user meta with
          | Ok () ->
            Printf.printf "%s/%s: synced on %s\n%!" pname name (Config.show_forge forge)
          | Error e -> Printf.eprintf "%s/%s: %s\n%!" pname name e))
    cfg.platform
;;

(** Repo manager wannabe? *)
[%%subliner.cmds
  eval.cmds
  <- (function
       | Init args ->
         Eio_main.run (fun env ->
           let fs = Eio.Stdenv.fs env in
           let mgr = Eio.Stdenv.process_mgr env in
           let targets, ctxs, cfg = get_targets args in
           let sem = Eio.Semaphore.make cfg.general.concurrency in
           run_on ~fs ~mgr ~targets ~ctxs ~sem (fun ~mgr ~path ~ctx ->
             Git.init ~mgr ~path ~ctx ~args:args.args ()))
       | Pull args ->
         Eio_main.run (fun env ->
           let fs = Eio.Stdenv.fs env in
           let mgr = Eio.Stdenv.process_mgr env in
           let targets, ctxs, cfg = get_targets args in
           let sem = Eio.Semaphore.make cfg.general.concurrency in
           run_on ~fs ~mgr ~targets ~ctxs ~sem (fun ~mgr ~path ~ctx ->
             Git.pull ~mgr ~path ~ctx ~args:args.args ()))
       | Push args ->
         Eio_main.run (fun env ->
           let fs = Eio.Stdenv.fs env in
           let mgr = Eio.Stdenv.process_mgr env in
           let targets, ctxs, cfg = get_targets args in
           let sem = Eio.Semaphore.make cfg.general.concurrency in
           run_on ~fs ~mgr ~targets ~ctxs ~sem (fun ~mgr ~path ~ctx ->
             Git.push ~mgr ~path ~ctx ~args:args.args ()))
       | Exec args ->
         Eio_main.run (fun env ->
           let fs = Eio.Stdenv.fs env in
           let mgr = Eio.Stdenv.process_mgr env in
           let targets, ctxs, cfg = get_targets args in
           let sem = Eio.Semaphore.make cfg.general.concurrency in
           run_on ~fs ~mgr ~targets ~ctxs ~sem (fun ~mgr ~path ~ctx ->
             Git.exec ~mgr ~path ~ctx ~cmd:args.args))
       | Sync args ->
         Eio_main.run (fun env ->
           let net = Eio.Stdenv.net env in
           let targets, _ctxs, cfg = get_targets args in
           let client = Fetch.make_client net in
           let sem = Eio.Semaphore.make cfg.general.concurrency in
           Eio.Fiber.all
             (List.map
                (fun target () ->
                   Eio.Semaphore.acquire sem;
                   Fun.protect
                     ~finally:(fun () -> Eio.Semaphore.release sem)
                     (fun () ->
                        let name = Filename.basename target in
                        sync_repo ~client ~cfg name))
                targets)))]
  [@@name "miroir"] [@@version Version.get ()]
