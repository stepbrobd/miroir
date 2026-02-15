open Miroir

type args =
  { config : string [@default ""] [@names [ "c"; "config" ]] [@env "MIROIR_CONFIG"]
  ; name : string [@default ""] [@names [ "n"; "name" ]]
  ; all : bool [@names [ "a"; "all" ]]
  ; force : bool [@names [ "f"; "force" ]]
  ; args : string list [@pos_all]
  }
[@@deriving subliner]

type cmds =
  | Init of args (** Initialize repo(s) (destructive, uncommitted changes will be lost) *)
  | Fetch of args (** Fetch from all remotes *)
  | Pull of args (** Pull from origin *)
  | Push of args (** Push to all remotes *)
  | Exec of args (** Execute command in repo(s) *)
  | Sync of args (** Sync metadata to all forges *)
[@@deriving subliner]

let get_targets { config; name; all; _ } =
  match Git.Common.available () with
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

(* slot pool: manages display slot allocation for repo concurrency.
   slots are indices [0..n-1] into the display's repo slot region
   acquire blocks via condition variable when all slots are in use,
   release frees one and wakes waiters. uses Eio.Condition.await
   (not await_no_mutex) to atomically register + unlock, avoiding
   missed wakeups between unlock and wait
   https://ocaml-multicore.github.io/eio/eio/Eio/Condition/index.html *)
type pool =
  { mu : Eio.Mutex.t
  ; cond : Eio.Condition.t
  ; free : int Queue.t
  }

let pool_make n =
  let q = Queue.create () in
  for i = 0 to n - 1 do
    Queue.push i q
  done;
  { mu = Eio.Mutex.create (); cond = Eio.Condition.create (); free = q }
;;

let pool_acquire p =
  Eio.Mutex.use_rw ~protect:false p.mu (fun () ->
    while Queue.is_empty p.free do
      Eio.Condition.await p.cond p.mu
    done;
    Queue.pop p.free)
;;

let pool_release p slot =
  Eio.Mutex.use_rw ~protect:false p.mu (fun () -> Queue.push slot p.free);
  Eio.Condition.broadcast p.cond
;;

(* run an op on each target, allocating display lines based on op's needs *)
let run_on ~fs ~mgr ~targets ~ctxs ~cfg (module M : Git.Op) ~force ~args =
  let nplatforms = List.length cfg.Config.platform in
  let nr = M.remotes nplatforms in
  let errors = ref [] in
  let err_mu = Eio.Mutex.create () in
  let add_err repo msg =
    Eio.Mutex.use_rw ~protect:true err_mu (fun () -> errors := (repo, msg) :: !errors)
  in
  if nr = 0
  then (
    (* no display needed, run sequentially *)
    let disp = Display.make ~repos:1 ~remotes:0 in
    let sem = Eio.Semaphore.make 1 in
    List.iter
      (fun target ->
         let ctx = List.assoc target ctxs in
         let path = Eio.Path.(fs / target) in
         match M.run ~mgr ~path ~ctx ~disp ~slot:0 ~sem ~force ~args with
         | Ok () -> ()
         | Error e ->
           let name = Filename.basename target in
           add_err name e;
           Printf.eprintf "error: %s :: %s\n%!" name e)
      targets)
  else (
    let nrepos = List.length targets in
    let rc = min cfg.general.concurrency.repo nrepos in
    let rc_remote = cfg.general.concurrency.remote in
    let mc = if rc_remote = 0 then nr else min rc_remote nr in
    let disp = Display.make ~repos:rc ~remotes:nr in
    let pool = pool_make rc in
    let remote_sem = Eio.Semaphore.make mc in
    Eio.Fiber.all
      (List.map
         (fun target () ->
            let slot = pool_acquire pool in
            Fun.protect
              ~finally:(fun () -> pool_release pool slot)
              (fun () ->
                 Display.clear disp slot;
                 let ctx = List.assoc target ctxs in
                 let path = Eio.Path.(fs / target) in
                 match M.run ~mgr ~path ~ctx ~disp ~slot ~sem:remote_sem ~force ~args with
                 | Ok () -> ()
                 | Error e ->
                   let name = Filename.basename target in
                   add_err name e;
                   Display.repo disp slot (Printf.sprintf "error: %s" e)))
         targets);
    Display.finish disp);
  (* print error summary *)
  let errs = List.rev !errors in
  if errs <> []
  then (
    Printf.eprintf "\n";
    List.iter (fun (repo, msg) -> Printf.eprintf "error: %s :: %s\n" repo msg) errs)
;;

(* common setup: eio runtime + targets + dispatch *)
let with_op args op =
  Eio_main.run (fun env ->
    let fs = Eio.Stdenv.fs env in
    let mgr = Eio.Stdenv.process_mgr env in
    let targets, ctxs, cfg = get_targets args in
    run_on ~fs ~mgr ~targets ~ctxs ~cfg op ~force:args.force ~args:args.args)
;;

let sync_repo ~client ~cfg ~disp ~slot ~sem name =
  let repo =
    match List.assoc_opt name cfg.Config.repo with
    | Some r -> r
    | None ->
      Display.repo disp slot (Printf.sprintf "%s :: sync :: no repo config" name);
      { Config.description = None; visibility = Private; archived = false; branch = None }
  in
  Display.repo disp slot (Printf.sprintf "%s :: sync" name);
  let platforms = cfg.platform in
  let errors = ref [] in
  let mu = Eio.Mutex.create () in
  Eio.Fiber.all
    (List.mapi
       (fun j (pname, (p : Config.platform)) () ->
          Display.remote disp slot j (Printf.sprintf "%s :: waiting..." pname);
          Eio.Semaphore.acquire sem;
          Fun.protect
            ~finally:(fun () -> Eio.Semaphore.release sem)
            (fun () ->
               match Config.resolve_forge p, Config.resolve_token pname p with
               | None, _ ->
                 Display.remote disp slot j (Printf.sprintf "%s :: skipped" pname);
                 Display.output disp slot j "unknown forge"
               | _, None ->
                 Display.remote disp slot j (Printf.sprintf "%s :: skipped" pname);
                 Display.output disp slot j "no token"
               | Some forge, Some token ->
                 Display.remote disp slot j (Printf.sprintf "%s :: syncing..." pname);
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
                    Display.remote disp slot j (Printf.sprintf "%s :: done" pname);
                    Display.output
                      disp
                      slot
                      j
                      (Printf.sprintf "synced on %s" (Config.show_forge forge))
                  | Error e ->
                    Display.remote disp slot j (Printf.sprintf "%s :: error" pname);
                    Display.output disp slot j e;
                    Eio.Mutex.use_rw ~protect:true mu (fun () ->
                      errors := Printf.sprintf "%s/%s" pname e :: !errors))))
       platforms);
  match !errors with
  | [] -> Ok ()
  | errs -> Error (String.concat "; " (List.rev errs))
;;

(** Repo manager wannabe? *)
[%%subliner.cmds
  eval.cmds
  <- (function
       | Init args -> with_op args (module Git.Init)
       | Fetch args -> with_op args (module Git.Fetch)
       | Pull args -> with_op args (module Git.Pull)
       | Push args -> with_op args (module Git.Push)
       | Exec args -> with_op args (module Git.Exec)
       | Sync args ->
         Eio_main.run (fun env ->
           let net = Eio.Stdenv.net env in
           let targets, _ctxs, cfg = get_targets args in
           let nrepos = List.length targets in
           let nremotes = List.length cfg.platform in
           let rc = min cfg.general.concurrency.repo nrepos in
           let rc_remote = cfg.general.concurrency.remote in
           let mc = if rc_remote = 0 then nremotes else min rc_remote nremotes in
           let client = Fetch.make_client net in
           let disp = Display.make ~repos:rc ~remotes:nremotes in
           let pool = pool_make rc in
           let remote_sem = Eio.Semaphore.make mc in
           let errors = ref [] in
           let err_mu = Eio.Mutex.create () in
           Eio.Fiber.all
             (List.map
                (fun target () ->
                   let slot = pool_acquire pool in
                   Fun.protect
                     ~finally:(fun () -> pool_release pool slot)
                     (fun () ->
                        Display.clear disp slot;
                        let name = Filename.basename target in
                        match sync_repo ~client ~cfg ~disp ~slot ~sem:remote_sem name with
                        | Ok () -> ()
                        | Error e ->
                          Eio.Mutex.use_rw ~protect:true err_mu (fun () ->
                            errors := (name, e) :: !errors)))
                targets);
           Display.finish disp;
           let errs = List.rev !errors in
           if errs <> []
           then (
             Printf.eprintf "\n";
             List.iter
               (fun (repo, msg) -> Printf.eprintf "error: %s :: %s\n" repo msg)
               errs)))]
  [@@name "miroir"] [@@version Version.get ()]
