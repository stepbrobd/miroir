(* terminal output for concurrent repo operations.

   layout for [repos] repo slots and [remotes] remotes per slot:

     slot 0:
       line 0: repo header       (bold cyan)
       line 1: remote 0 info     (yellow, indented 2)
       line 2: remote 0 output   (dim, indented 4)
       line 3: remote 1 info
       line 4: remote 1 output
       ...
     slot 1:
       line stride: repo header
       ...

   stride = 1 + 2 * remotes (lines per repo slot)
   total  = repos * stride

   remote output lines are independent: writing to one remote's
   output never affects another's. finished remotes retain their
   last output line.

   when stdout is not a tty, prints sequentially without ansi. *)

(* ansi escape helpers *)
let bold_cyan s = Printf.sprintf "\027[1;36m%s\027[0m" s
let yellow s = Printf.sprintf "\027[33m%s\027[0m" s
let dim s = Printf.sprintf "\027[2m%s\027[0m" s

type t =
  { stride : int (* lines per repo slot: 1 + 2 * remotes *)
  ; total : int (* repos * stride *)
  ; tty : bool
  ; mu : Eio.Mutex.t
  ; lines : string array (* flat array of all line contents *)
  ; cursor : int ref (* current cursor line, 0-indexed from region top *)
  }

let make ~repos ~remotes =
  let stride = 1 + (2 * remotes) in
  let total = repos * stride in
  let tty = Unix.isatty Unix.stdout in
  let t =
    { stride
    ; total
    ; tty
    ; mu = Eio.Mutex.create ()
    ; lines = Array.make (max 1 total) ""
    ; cursor = ref (max 0 (total - 1))
    }
  in
  if tty && total > 1
  then
    for _ = 1 to total - 1 do
      Printf.printf "\n%!"
    done;
  t
;;

(* move cursor to target line within the display region *)
let goto t line =
  let cur = !(t.cursor) in
  let delta = cur - line in
  if delta > 0
  then Printf.printf "\027[%dA" delta
  else if delta < 0
  then Printf.printf "\027[%dB" (abs delta);
  Printf.printf "\r%!";
  t.cursor := line
;;

let write_line_unlocked t line s =
  t.lines.(line) <- s;
  goto t line;
  Printf.printf "\027[K%s%!" s
;;

(* write a line in tty mode: move to target line and overwrite *)
let tty_write t line s =
  Eio.Mutex.use_rw ~protect:true t.mu (fun () -> write_line_unlocked t line s)
;;

(* write a line in non-tty mode: print sequentially with prefix *)
let seq_write t line prefix s =
  Eio.Mutex.use_rw ~protect:true t.mu (fun () ->
    t.lines.(line) <- s;
    Printf.printf "%s%s\n%!" prefix s)
;;

(* compute line index for repo header *)
let repo_line t r = r * t.stride

(* compute line index for remote info *)
let remote_line t r j = (r * t.stride) + 1 + (2 * j)

(* compute line index for remote output *)
let output_line t r j = (r * t.stride) + 2 + (2 * j)

let repo t r s =
  let line = repo_line t r in
  if t.tty then tty_write t line (bold_cyan s) else seq_write t line "" s
;;

let remote t r j s =
  let line = remote_line t r j in
  if t.tty then tty_write t line ("  " ^ yellow s) else seq_write t line "  " s
;;

let output t r j s =
  let line = output_line t r j in
  if t.tty then tty_write t line ("    " ^ dim s) else seq_write t line "    " s
;;

let clear t r =
  let base = r * t.stride in
  if t.tty
  then
    Eio.Mutex.use_rw ~protect:true t.mu (fun () ->
      for i = 0 to t.stride - 1 do
        write_line_unlocked t (base + i) ""
      done)
  else
    Eio.Mutex.use_rw ~protect:true t.mu (fun () ->
      for i = 0 to t.stride - 1 do
        t.lines.(base + i) <- ""
      done)
;;

let finish t =
  if t.tty && t.total > 0
  then
    Eio.Mutex.use_rw ~protect:true t.mu (fun () ->
      goto t (t.total - 1);
      Printf.printf "\n%!")
;;
