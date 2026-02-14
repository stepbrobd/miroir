(** git operations via Eio subprocesses *)

(** check if git is available in PATH *)
val available : unit -> (unit, string) result

(** initialize a repo: clone, add remotes, fetch, reset *)
val init
  :  mgr:_ Eio.Process.mgr
  -> path:_ Eio.Path.t
  -> ctx:Context.context
  -> ?args:string list
  -> unit
  -> (unit, string) result

(** pull from origin and update submodules *)
val pull
  :  mgr:_ Eio.Process.mgr
  -> path:_ Eio.Path.t
  -> ctx:Context.context
  -> ?args:string list
  -> unit
  -> (unit, string) result

(** push to all configured remotes in parallel *)
val push
  :  mgr:_ Eio.Process.mgr
  -> path:_ Eio.Path.t
  -> ctx:Context.context
  -> ?args:string list
  -> unit
  -> (unit, string) result

(** execute an arbitrary command in repo directory *)
val exec
  :  mgr:_ Eio.Process.mgr
  -> path:_ Eio.Path.t
  -> ctx:Context.context
  -> cmd:string list
  -> (unit, string) result
