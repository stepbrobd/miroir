(* exec runs sequentially, ignoring display and concurrency.
   output goes directly to stdout. remotes = 0 tells main.ml
   to skip display allocation entirely. *)

let remotes _ = 0

let run ~mgr ~path ~(ctx : Miroir.Context.context) ~disp:_ ~slot:_ ~sem:_ ~args:cmd =
  let name = Common.repo_name path in
  Printf.printf "%s :: exec :: %s\n%!" name (String.concat " " cmd);
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
