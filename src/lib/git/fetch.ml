(* fetch from all remotes concurrently.
   remotes = n so each remote gets its own display line. *)

open Common

let remotes n = n

let run ~mgr ~path ~(ctx : Miroir.Context.context) ~disp ~slot ~sem ~force:_ ~args =
  let name = repo_name path in
  Miroir.Display.repo disp slot (Printf.sprintf "%s :: fetch" name);
  let* () = ensure_repo path in
  let results = ref [] in
  let mu = Eio.Mutex.create () in
  Eio.Fiber.all
    (List.map
       (fun (r : Miroir.Context.remote) () ->
          let j = remote_index ctx r.name in
          Miroir.Display.remote disp slot j (Printf.sprintf "%s :: waiting..." r.name);
          Eio.Semaphore.acquire sem;
          Fun.protect
            ~finally:(fun () -> Eio.Semaphore.release sem)
            (fun () ->
               Miroir.Display.remote
                 disp
                 slot
                 j
                 (Printf.sprintf "%s :: fetching..." r.name);
               let res =
                 run
                   ~mgr
                   ~cwd:path
                   ~env:ctx.env
                   ~on_output:(Miroir.Display.output disp slot j)
                   ([ "fetch"; r.name ] @ args)
               in
               (match res with
                | Ok () ->
                  Miroir.Display.remote disp slot j (Printf.sprintf "%s :: done" r.name)
                | Error e ->
                  Miroir.Display.remote disp slot j (Printf.sprintf "%s :: error" r.name);
                  Miroir.Display.output disp slot j e);
               Eio.Mutex.use_rw ~protect:true mu (fun () ->
                 results := (r.name, res) :: !results)))
       ctx.push);
  let results = !results in
  match List.find_opt (fun (_, r) -> Result.is_error r) results with
  | Some (rname, Error e) -> Error (Printf.sprintf "fetch from %s failed: %s" rname e)
  | _ -> Ok ()
;;
