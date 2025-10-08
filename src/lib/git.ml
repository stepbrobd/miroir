let available () =
  match Unix.system "command -v git > /dev/null 2>&1" with
  | Unix.WEXITED 0 -> Ok ()
  | _ -> Error "git is not available in PATH"
;;

let run ?(silent = false) ~cwd ~env args =
  let env_array = Array.of_list (List.map (fun (k, v) -> k ^ "=" ^ v) env) in
  let cmd = "git" :: args in
  let stdout_fd, stderr_fd =
    if silent
    then (
      let devnull = Unix.openfile "/dev/null" [ Unix.O_WRONLY ] 0o666 in
      devnull, devnull)
    else Unix.stdout, Unix.stderr
  in
  let pid =
    Unix.create_process_env
      "git"
      (Array.of_list cmd)
      env_array
      Unix.stdin
      stdout_fd
      stderr_fd
  in
  if silent then Unix.close stdout_fd;
  let old_cwd = Sys.getcwd () in
  Sys.chdir cwd;
  let result =
    match Unix.waitpid [] pid with
    | _, Unix.WEXITED 0 -> Ok ()
    | _, Unix.WEXITED n -> Error (Printf.sprintf "git exited with code %d" n)
    | _, Unix.WSIGNALED n -> Error (Printf.sprintf "git killed by signal %d" n)
    | _, Unix.WSTOPPED n -> Error (Printf.sprintf "git stopped by signal %d" n)
  in
  Sys.chdir old_cwd;
  result
;;

let init ~path ~(ctx : Context.context) ?(args = []) () =
  Printf.printf "Miroir :: Repo :: Init :: %s:\n%!" path;
  let git_dir = Filename.concat path ".git" in
  (if not (Sys.file_exists git_dir)
   then (
     Unix.mkdir path 0o755;
     run ~cwd:path ~env:ctx.env ([ "init"; "--initial-branch=" ^ ctx.branch ] @ args))
   else (
     match run ~silent:true ~cwd:path ~env:ctx.env [ "remote" ] with
     | Error e -> Error e
     | Ok () -> Ok ()))
  |> ignore;
  List.iter
    (fun (_name, uri) ->
       run ~silent:true ~cwd:path ~env:ctx.env [ "remote"; "add"; "origin"; uri ]
       |> ignore)
    ctx.fetch_remotes;
  List.iter
    (fun (name, uri) ->
       run ~silent:true ~cwd:path ~env:ctx.env [ "remote"; "add"; name; uri ] |> ignore)
    ctx.push_remotes;
  run ~cwd:path ~env:ctx.env ([ "fetch"; "--all" ] @ args) |> ignore;
  run ~cwd:path ~env:ctx.env [ "submodule"; "update"; "--recursive"; "--init" ] |> ignore;
  run ~cwd:path ~env:ctx.env [ "reset"; "--hard"; "origin/" ^ ctx.branch ] |> ignore;
  run ~cwd:path ~env:ctx.env [ "checkout"; ctx.branch ] |> ignore;
  run
    ~cwd:path
    ~env:ctx.env
    [ "branch"; "--set-upstream-to=origin/" ^ ctx.branch; ctx.branch ]
  |> ignore
;;

let pull ~path ~(ctx : Context.context) ?(args = []) () =
  Printf.printf "Miroir :: Repo :: Pull :: %s:\n%!" path;
  let git_dir = Filename.concat path ".git" in
  if not (Sys.file_exists git_dir)
  then Error (Printf.sprintf "fatal: %s is not a git repository" path)
  else (
    run ~cwd:path ~env:ctx.env ([ "pull"; "origin"; ctx.branch ] @ args) |> ignore;
    run ~cwd:path ~env:ctx.env [ "submodule"; "update"; "--recursive"; "--remote" ]
    |> ignore;
    Ok ())
;;

let push ~path ~(ctx : Context.context) ?(args = []) () =
  Printf.printf "Miroir :: Repo :: Push :: %s:\n%!" path;
  let git_dir = Filename.concat path ".git" in
  if not (Sys.file_exists git_dir)
  then Error (Printf.sprintf "fatal: %s is not a git repository" path)
  else (
    let all_remotes = ctx.push_remotes @ [ "origin", "" ] in
    List.iter (fun (name, _) -> Printf.printf "  Pushing to %s...\n%!" name) all_remotes;
    let results =
      List.map
        (fun (name, _) ->
           let result =
             run ~cwd:path ~env:ctx.env ([ "push"; name; ctx.branch ] @ args)
           in
           name, result)
        all_remotes
    in
    List.iter
      (fun (name, result) ->
         match result with
         | Ok () -> Printf.printf "%s: success\n%!" name
         | Error e -> Printf.eprintf "%s: %s\n%!" name e)
      results;
    Ok ())
;;

let exec ~path ~(ctx : Context.context) ~cmd =
  Printf.printf "Miroir :: Repo :: Exec :: %s:\n%!" path;
  Printf.printf "$ %s\n%!" (String.concat " " cmd);
  match cmd with
  | [] -> ()
  | prog :: _args ->
    let env_array = Array.of_list (List.map (fun (k, v) -> k ^ "=" ^ v) ctx.env) in
    let pid =
      Unix.create_process_env
        prog
        (Array.of_list cmd)
        env_array
        Unix.stdin
        Unix.stdout
        Unix.stderr
    in
    Unix.waitpid [] pid |> ignore
;;
