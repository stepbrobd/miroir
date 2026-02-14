let ( let* ) = Result.bind

let available () =
  match Unix.system "command -v git > /dev/null 2>&1" with
  | Unix.WEXITED 0 -> Ok ()
  | _ -> Error "git is not available in PATH"
;;

(* read lines from a flow, call [f] on each non-empty line *)
let drain flow f =
  let buf = Eio.Buf_read.of_flow ~max_size:(1024 * 1024) flow in
  try
    while true do
      let line = Eio.Buf_read.line buf in
      if String.length line > 0 then f line
    done
  with
  | End_of_file -> ()
;;

(* run a git command, route output through [on_output] callback *)
let run ~mgr ~cwd ~env ?(silent = false) ?(on_output = fun _ -> ()) args =
  let env = Array.of_list (List.map (fun (k, v) -> k ^ "=" ^ v) env) in
  let cmd = "git" :: args in
  try
    if silent
    then (
      let null = Eio.Path.(cwd / "/dev/null") in
      Eio.Path.with_open_out ~create:(`If_missing 0o644) null (fun sink ->
        Eio.Process.run mgr ~cwd ~env ~stdout:sink ~stderr:sink cmd);
      Ok ())
    else
      Eio.Switch.run (fun sw ->
        let r, w = Eio.Process.pipe ~sw mgr in
        let child = Eio.Process.spawn ~sw mgr ~cwd ~env ~stdout:w ~stderr:w cmd in
        Eio.Flow.close w;
        drain r on_output;
        match Eio.Process.await child with
        | `Exited 0 -> Ok ()
        | `Exited n ->
          Error (Printf.sprintf "%s exited with code %d" (String.concat " " cmd) n)
        | `Signaled n ->
          Error (Printf.sprintf "%s killed by signal %d" (String.concat " " cmd) n))
  with
  | Eio.Exn.Io _ as ex -> Error (Printexc.to_string ex)
;;

(* find the index of a remote name in the push list.
   raises [Not_found] if the name is not present. *)
let remote_index (ctx : Miroir.Context.context) name =
  let rec go i = function
    | [] -> raise Not_found
    | (r : Miroir.Context.remote) :: _ when String.equal r.name name -> i
    | _ :: rest -> go (i + 1) rest
  in
  go 0 ctx.push
;;

(* extract repo basename from an eio path *)
let repo_name path = Filename.basename (snd path)

(* check that .git exists at path, return error if not *)
let ensure_repo path =
  let dir = Eio.Path.(path / ".git") in
  match Eio.Path.kind ~follow:false dir with
  | `Not_found -> Error (Printf.sprintf "fatal: %s is not a git repository" (snd path))
  | _ -> Ok ()
;;
