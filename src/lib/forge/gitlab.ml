let ( let* ) = Result.bind

module Config = Miroir.Config
module Fetch = Miroir.Fetch

let base = "https://gitlab.com/api/v4"

let vis_str = function
  | Config.Public -> "public"
  | Config.Private -> "private"
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
        | `List msgs ->
          List.filter_map
            (function
              | `String s -> Some s
              | _ -> None)
            msgs
          |> String.concat ", "
        | _ -> body
      with
      | _ -> body
    in
    Error (Printf.sprintf "gitlab api error (%d): %s" code msg))
;;

let exists code = code = 400 (* bad request: has already been taken *)

let create client ~token ~user:_ (m : Forge_intf.meta) =
  let body =
    `Assoc
      [ "name", `String m.name
      ; "description", `String (Option.value ~default:"" m.desc)
      ; "visibility", `String (vis_str m.vis)
      ; "initialize_with_readme", `Bool false
      ]
    |> Yojson.Safe.to_string
  in
  let url = base ^ "/projects" in
  let code, data = Fetch.json client ~meth:POST ~url ~token ~body () in
  if exists code then Error "already exists" else check code data
;;

(* gitlab needs project id for updates; look it up by path *)
let project_id client ~token ~user ~name =
  let path = Uri.pct_encode (user ^ "/" ^ name) in
  let url = Printf.sprintf "%s/projects/%s" base path in
  let code, data = Fetch.json client ~meth:GET ~url ~token () in
  if code >= 200 && code < 300
  then (
    try
      let j = Yojson.Safe.from_string data in
      match Yojson.Safe.Util.member "id" j with
      | `Int id -> Ok id
      | _ -> Error "gitlab: could not parse project id"
    with
    | ex -> Error (Printexc.to_string ex))
  else Error (Printf.sprintf "gitlab: could not find project (%d)" code)
;;

let update client ~token ~user (m : Forge_intf.meta) =
  let* id = project_id client ~token ~user ~name:m.name in
  let body =
    `Assoc
      [ "name", `String m.name
      ; "description", `String (Option.value ~default:"" m.desc)
      ; "visibility", `String (vis_str m.vis)
      ; "archived", `Bool m.archived
      ]
    |> Yojson.Safe.to_string
  in
  let url = Printf.sprintf "%s/projects/%d" base id in
  let code, data = Fetch.json client ~meth:PUT ~url ~token ~body () in
  check code data
;;

let archive client ~token ~user ~name flag =
  let* id = project_id client ~token ~user ~name in
  let action = if flag then "archive" else "unarchive" in
  let url = Printf.sprintf "%s/projects/%d/%s" base id action in
  let code, data = Fetch.json client ~meth:POST ~url ~token () in
  check code data
;;

let delete client ~token ~user ~name =
  let* id = project_id client ~token ~user ~name in
  let url = Printf.sprintf "%s/projects/%d" base id in
  let code, data = Fetch.json client ~meth:DELETE ~url ~token () in
  check code data
;;

let list client ~token ~user:_ =
  let url = base ^ "/projects?owned=true&per_page=100" in
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
  else Error (Printf.sprintf "gitlab api error (%d): %s" code data)
;;

let sync client ~token ~user (m : Forge_intf.meta) =
  match create client ~token ~user m with
  | Ok () -> Ok ()
  | Error "already exists" ->
    let* () = update client ~token ~user m in
    if m.archived then archive client ~token ~user ~name:m.name true else Ok ()
  | Error e -> Error e
;;
