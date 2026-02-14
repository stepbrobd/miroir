let ( let* ) = Result.bind

module Config = Miroir.Config
module Fetch = Miroir.Fetch

let base = "https://api.github.com"

let private_of_vis = function
  | Config.Public -> false
  | Config.Private -> true
;;

let check code body =
  if code >= 200 && code < 300
  then Ok ()
  else (
    let msg =
      try
        let j = Yojson.Safe.from_string body in
        match Yojson.Safe.Util.member "message" j with
        | `String s -> s
        | _ -> body
      with
      | _ -> body
    in
    Error (Printf.sprintf "github api error (%d): %s" code msg))
;;

let exists code =
  code = 422 (* unprocessable: already exists *) || code = 409 (* conflict *)
;;

let create client ~token ~user:_ (m : Forge_intf.meta) =
  let body =
    `Assoc
      [ "name", `String m.name
      ; "description", `String (Option.value ~default:"" m.desc)
      ; "private", `Bool (private_of_vis m.vis)
      ; "auto_init", `Bool false
      ]
    |> Yojson.Safe.to_string
  in
  let url = base ^ "/user/repos" in
  let code, data = Fetch.json client ~meth:POST ~url ~token ~body () in
  if exists code then Error "already exists" else check code data
;;

let update client ~token ~user (m : Forge_intf.meta) =
  let body =
    `Assoc
      [ "name", `String m.name
      ; "description", `String (Option.value ~default:"" m.desc)
      ; "private", `Bool (private_of_vis m.vis)
      ; "archived", `Bool m.archived
      ]
    |> Yojson.Safe.to_string
  in
  let url = Printf.sprintf "%s/repos/%s/%s" base user m.name in
  let code, data = Fetch.json client ~meth:PATCH ~url ~token ~body () in
  check code data
;;

let archive client ~token ~user ~name flag =
  let body = `Assoc [ "archived", `Bool flag ] |> Yojson.Safe.to_string in
  let url = Printf.sprintf "%s/repos/%s/%s" base user name in
  let code, data = Fetch.json client ~meth:PATCH ~url ~token ~body () in
  check code data
;;

let delete client ~token ~user ~name =
  let url = Printf.sprintf "%s/repos/%s/%s" base user name in
  let code, data = Fetch.json client ~meth:DELETE ~url ~token () in
  check code data
;;

let list client ~token ~user:_ =
  let url = base ^ "/user/repos?per_page=100&type=owner" in
  let code, data = Fetch.json client ~meth:GET ~url ~token () in
  if code >= 200 && code < 300
  then (
    try
      let j = Yojson.Safe.from_string data in
      let names =
        Yojson.Safe.Util.to_list j
        |> List.filter_map (fun r ->
          match Yojson.Safe.Util.member "name" r with
          | `String s -> Some s
          | _ -> None)
      in
      Ok names
    with
    | ex -> Error (Printexc.to_string ex))
  else Error (Printf.sprintf "github api error (%d): %s" code data)
;;

let sync client ~token ~user (m : Forge_intf.meta) =
  match create client ~token ~user m with
  | Ok () -> Ok ()
  | Error "already exists" ->
    let* () = update client ~token ~user m in
    if m.archived then archive client ~token ~user ~name:m.name true else Ok ()
  | Error e -> Error e
;;
