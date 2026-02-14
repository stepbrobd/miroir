let ( let* ) = Result.bind

let available () =
  match Unix.system "command -v git > /dev/null 2>&1" with
  | Unix.WEXITED 0 -> Ok ()
  | _ -> Error "git is not available in PATH"
;;

let run ~mgr ~cwd ~env ?(silent = false) args =
  let env = Array.of_list (List.map (fun (k, v) -> k ^ "=" ^ v) env) in
  let cmd = "git" :: args in
  try
    if silent
    then (
      let null = Eio.Path.(cwd / "/dev/null") in
      Eio.Path.with_open_out ~create:(`If_missing 0o644) null (fun sink ->
        Eio.Process.run mgr ~cwd ~env ~stdout:sink ~stderr:sink cmd);
      Ok ())
    else (
      Eio.Process.run mgr ~cwd ~env cmd;
      Ok ())
  with
  | Eio.Exn.Io _ as ex -> Error (Printexc.to_string ex)
;;

let init ~mgr ~path ~(ctx : Context.context) ?(args = []) () =
  Printf.printf "Miroir :: Repo :: Init :: %s:\n%!" (snd path);
  let dir = Eio.Path.(path / ".git") in
  let* () =
    match Eio.Path.kind ~follow:false dir with
    | `Not_found ->
      (try
         Eio.Path.mkdir ~perm:0o755 path;
         run
           ~mgr
           ~cwd:path
           ~env:ctx.env
           ~silent:false
           ([ "init"; "--initial-branch=" ^ ctx.branch ] @ args)
       with
       | Eio.Exn.Io _ as ex -> Error (Printexc.to_string ex))
    | _ -> run ~mgr ~cwd:path ~env:ctx.env ~silent:true [ "remote" ]
  in
  let* () =
    List.fold_left
      (fun acc (r : Context.remote) ->
         let* () = acc in
         run ~mgr ~cwd:path ~env:ctx.env ~silent:true [ "remote"; "add"; "origin"; r.uri ])
      (Ok ())
      ctx.fetch
  in
  let* () =
    List.fold_left
      (fun acc (r : Context.remote) ->
         let* () = acc in
         run ~mgr ~cwd:path ~env:ctx.env ~silent:true [ "remote"; "add"; r.name; r.uri ])
      (Ok ())
      ctx.push
  in
  let* () = run ~mgr ~cwd:path ~env:ctx.env ([ "fetch"; "--all" ] @ args) in
  let* () =
    run ~mgr ~cwd:path ~env:ctx.env [ "submodule"; "update"; "--recursive"; "--init" ]
  in
  let* () =
    run ~mgr ~cwd:path ~env:ctx.env [ "reset"; "--hard"; "origin/" ^ ctx.branch ]
  in
  let* () = run ~mgr ~cwd:path ~env:ctx.env [ "checkout"; ctx.branch ] in
  run
    ~mgr
    ~cwd:path
    ~env:ctx.env
    [ "branch"; "--set-upstream-to=origin/" ^ ctx.branch; ctx.branch ]
;;

let pull ~mgr ~path ~(ctx : Context.context) ?(args = []) () =
  Printf.printf "Miroir :: Repo :: Pull :: %s:\n%!" (snd path);
  let dir = Eio.Path.(path / ".git") in
  match Eio.Path.kind ~follow:false dir with
  | `Not_found -> Error (Printf.sprintf "fatal: %s is not a git repository" (snd path))
  | _ ->
    let* () = run ~mgr ~cwd:path ~env:ctx.env ([ "pull"; "origin"; ctx.branch ] @ args) in
    run ~mgr ~cwd:path ~env:ctx.env [ "submodule"; "update"; "--recursive"; "--remote" ]
;;

let push ~mgr ~path ~(ctx : Context.context) ?(args = []) () =
  Printf.printf "Miroir :: Repo :: Push :: %s:\n%!" (snd path);
  let dir = Eio.Path.(path / ".git") in
  match Eio.Path.kind ~follow:false dir with
  | `Not_found -> Error (Printf.sprintf "fatal: %s is not a git repository" (snd path))
  | _ ->
    List.iter
      (fun (r : Context.remote) -> Printf.printf "  Pushing to %s...\n%!" r.name)
      ctx.push;
    (* push to all remotes in parallel *)
    let results = ref [] in
    let mu = Eio.Mutex.create () in
    Eio.Fiber.all
      (List.map
         (fun (r : Context.remote) () ->
            let res =
              run ~mgr ~cwd:path ~env:ctx.env ([ "push"; r.name; ctx.branch ] @ args)
            in
            Eio.Mutex.lock mu;
            results := (r.name, res) :: !results;
            Eio.Mutex.unlock mu)
         ctx.push);
    let results = !results in
    List.iter
      (fun (name, res) ->
         match res with
         | Ok () -> Printf.printf "%s: success\n%!" name
         | Error e -> Printf.eprintf "%s: %s\n%!" name e)
      results;
    (* report first error if any *)
    (match List.find_opt (fun (_, r) -> Result.is_error r) results with
     | Some (name, Error e) -> Error (Printf.sprintf "push to %s failed: %s" name e)
     | _ -> Ok ())
;;

let exec ~mgr ~path ~(ctx : Context.context) ~cmd =
  Printf.printf "Miroir :: Repo :: Exec :: %s:\n%!" (snd path);
  Printf.printf "$ %s\n%!" (String.concat " " cmd);
  match cmd with
  | [] -> Ok ()
  | _prog :: _ ->
    let env = Array.of_list (List.map (fun (k, v) -> k ^ "=" ^ v) ctx.env) in
    (try
       Eio.Process.run mgr ~cwd:path ~env cmd;
       Ok ()
     with
     | Eio.Exn.Io _ as ex -> Error (Printexc.to_string ex))
;;
