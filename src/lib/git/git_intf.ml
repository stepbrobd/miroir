(* shared module type for git operations.
   all operations conform to the same signature so dispatch
   in main.ml is uniform. [remotes] declares how many remote
   display lines the operation needs per repo slot.
   exec returns 0 (no display, runs serially with direct stdout). *)

module type Op = sig
  (* number of remote display lines needed per repo slot.
     0 = no display (exec), 1 = origin only (pull/init),
     n = all remotes (fetch/push). the caller passes in the total
     platform count; operations that need fewer just return
     what they need. *)
  val remotes : int -> int

  val run
    :  mgr:_ Eio.Process.mgr
    -> path:Eio.Fs.dir_ty Eio.Path.t
    -> ctx:Miroir.Context.context
    -> disp:Miroir.Display.t
    -> slot:int
    -> sem:Eio.Semaphore.t
    -> force:bool
    -> args:string list
    -> (unit, string) result
end
