(* metadata for a repo to sync to a forge *)
type meta =
  { name : string
  ; desc : string option
  ; vis : Miroir.Config.visibility
  ; archived : bool
  }

(* module type that all forge implementations must satisfy *)
module type S = sig
  (* create a repo on the forge *)
  val create
    :  Miroir.Fetch.client
    -> token:string
    -> user:string
    -> meta
    -> (unit, string) result

  (* update repo metadata (description, visibility) *)
  val update
    :  Miroir.Fetch.client
    -> token:string
    -> user:string
    -> meta
    -> (unit, string) result

  (* set the archived status of a repo *)
  val archive
    :  Miroir.Fetch.client
    -> token:string
    -> user:string
    -> name:string
    -> bool
    -> (unit, string) result

  (* delete a repo from the forge *)
  val delete
    :  Miroir.Fetch.client
    -> token:string
    -> user:string
    -> name:string
    -> (unit, string) result

  (* list all repos for the authenticated user *)
  val list
    :  Miroir.Fetch.client
    -> token:string
    -> user:string
    -> (string list, string) result

  (* create-or-update: tries create, falls back to update if already exists *)
  val sync
    :  Miroir.Fetch.client
    -> token:string
    -> user:string
    -> meta
    -> (unit, string) result
end
