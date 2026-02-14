(** terminal output for concurrent repo operations.

    manages a fixed-size region for structured display of repo and
    remote operations. each repo slot has a header line, and each
    remote within that slot has an info line and an output line.

    layout for [repos] repo slots and [remotes] remotes per slot:
      total lines per slot = 1 + 2 * remotes
      total lines = repos * (1 + 2 * remotes)

    remote output lines are independent: writing to one remote's
    output never touches another's. when a remote finishes, its
    output retains the last printed line.

    when stdout is a tty, uses ansi escapes for in-place updates.
    when not a tty, falls back to sequential printing. *)

(** display state shared across fibers *)
type t

(** create a display for structured operations.
    [repos] = number of concurrent repo slots,
    [remotes] = number of remotes per repo. *)
val make : repos:int -> remotes:int -> t

(** set the repo header line for repo slot [r] *)
val repo : t -> int -> string -> unit

(** set the remote info line for repo slot [r], remote [j] *)
val remote : t -> int -> int -> string -> unit

(** set the remote output line for repo slot [r], remote [j] *)
val output : t -> int -> int -> string -> unit

(** clear all lines for repo slot [r] (for slot reuse) *)
val clear : t -> int -> unit

(** move cursor below the display region *)
val finish : t -> unit
