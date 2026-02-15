open Common

let remotes _ = 1

let run ~mgr ~path ~(ctx : Miroir.Context.context) ~disp ~slot ~sem:_ ~force:_ ~args =
  let name = repo_name path in
  Miroir.Display.repo disp slot (Printf.sprintf "%s :: init" name);
  (* use origin remote slot (index 0) for init progress *)
  let j = 0 in
  let out = Miroir.Display.output disp slot j in
  let info s = Miroir.Display.remote disp slot j s in
  let dir = Eio.Path.(path / ".git") in
  info "initializing...";
  let* () =
    match Eio.Path.kind ~follow:false dir with
    | `Not_found ->
      (try
         Eio.Path.mkdir ~perm:0o755 path;
         run
           ~mgr
           ~cwd:path
           ~env:ctx.env
           ~on_output:out
           ([ "init"; "--initial-branch=" ^ ctx.branch ] @ args)
       with
       | Eio.Exn.Io _ as ex -> Error (Printexc.to_string ex))
    | _ -> run ~mgr ~cwd:path ~env:ctx.env ~silent:true [ "remote" ]
  in
  info "adding remotes...";
  let* () =
    List.fold_left
      (fun acc (r : Miroir.Context.remote) ->
         let* () = acc in
         run ~mgr ~cwd:path ~env:ctx.env ~silent:true [ "remote"; "add"; "origin"; r.uri ])
      (Ok ())
      ctx.fetch
  in
  let* () =
    List.fold_left
      (fun acc (r : Miroir.Context.remote) ->
         let* () = acc in
         run ~mgr ~cwd:path ~env:ctx.env ~silent:true [ "remote"; "add"; r.name; r.uri ])
      (Ok ())
      ctx.push
  in
  info "fetching...";
  let* () =
    run ~mgr ~cwd:path ~env:ctx.env ~on_output:out ([ "fetch"; "--all" ] @ args)
  in
  info "updating submodules...";
  let* () =
    run
      ~mgr
      ~cwd:path
      ~env:ctx.env
      ~on_output:out
      [ "submodule"; "update"; "--recursive"; "--init" ]
  in
  info "resetting...";
  let* () =
    run
      ~mgr
      ~cwd:path
      ~env:ctx.env
      ~on_output:out
      [ "reset"; "--hard"; "origin/" ^ ctx.branch ]
  in
  info "checking out...";
  let* () = run ~mgr ~cwd:path ~env:ctx.env ~on_output:out [ "checkout"; ctx.branch ] in
  info "setting upstream...";
  let r =
    run
      ~mgr
      ~cwd:path
      ~env:ctx.env
      ~on_output:out
      [ "branch"; "--set-upstream-to=origin/" ^ ctx.branch; ctx.branch ]
  in
  (match r with
   | Ok () -> info "done"
   | Error e -> info (Printf.sprintf "error: %s" e));
  r
;;
