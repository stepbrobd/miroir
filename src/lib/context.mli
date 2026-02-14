(** execution context built from config *)

(** a named git remote *)
type remote =
  { name : string
  ; uri : string
  }

(** context for running git operations on a repo *)
type context =
  { env : (string * string) list
  ; branch : string
  ; fetch : remote list
  ; push : remote list
  }

(** generate a remote URI based on access method *)
val make_uri
  :  access:Config.access
  -> domain:string
  -> user:string
  -> repo:string
  -> string

(** expand ~/ prefix to $HOME *)
val expand_home : string -> string

(** build (path, context) pairs for all non-archived repos *)
val make_all : Config.config -> (string * context) list
