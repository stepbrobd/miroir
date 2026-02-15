open Common

let remotes _ = 1

let run ~mgr ~path ~(ctx : Miroir.Context.context) ~disp ~slot ~sem:_ ~force ~args =
  let name = repo_name path in
  Miroir.Display.repo disp slot (Printf.sprintf "%s :: pull" name);
  let* () = ensure_repo path in
  (* pull uses origin remote, index 0 *)
  let j = 0 in
  let out = Miroir.Display.output disp slot j in
  let info s = Miroir.Display.remote disp slot j s in
  (* check for dirty working tree *)
  if (not force) && is_dirty ~mgr ~cwd:path ~env:ctx.env
  then (
    let msg = "dirty working tree, use --force to override" in
    info (Printf.sprintf "error: %s" msg);
    Error msg)
  else
    (* force: reset working tree before pulling *)
    let* () =
      if force
      then (
        info "resetting...";
        run ~mgr ~cwd:path ~env:ctx.env ~silent:true [ "reset"; "--hard"; "HEAD" ])
      else Ok ()
    in
    info "pulling...";
    let* () =
      run
        ~mgr
        ~cwd:path
        ~env:ctx.env
        ~on_output:out
        ([ "pull"; "origin"; ctx.branch ] @ args)
    in
    info "updating submodules...";
    let r =
      run
        ~mgr
        ~cwd:path
        ~env:ctx.env
        ~on_output:out
        [ "submodule"; "update"; "--recursive"; "--remote" ]
    in
    (match r with
     | Ok () -> info "done"
     | Error e -> info (Printf.sprintf "error: %s" e));
    r
;;
