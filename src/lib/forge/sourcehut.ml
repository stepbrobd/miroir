let ( let* ) = Result.bind

module Config = Miroir.Config
module Fetch = Miroir.Fetch

let base = "https://git.sr.ht/query"

let vis_str = function
  | Config.Public -> "PUBLIC"
  | Config.Private -> "PRIVATE"
;;

let gql client ~token query vars =
  let body =
    `Assoc [ "query", `String query; "variables", `Assoc vars ] |> Yojson.Safe.to_string
  in
  Fetch.json client ~meth:POST ~url:base ~token ~body ()
;;

let check code body =
  if code >= 200 && code < 300
  then (
    try
      let j = Yojson.Safe.from_string body in
      match Yojson.Safe.Util.member "errors" j with
      | `Null | `List [] -> Ok j
      | `List errs ->
        let msgs =
          List.filter_map
            (fun e ->
               match Yojson.Safe.Util.member "message" e with
               | `String s -> Some s
               | _ -> None)
            errs
        in
        Error (String.concat "; " msgs)
      | _ -> Ok j
    with
    | ex -> Error (Printexc.to_string ex))
  else Error (Printf.sprintf "sourcehut api error (%d): %s" code body)
;;

let create client ~token ~user:_ (m : Forge_intf.meta) =
  let q =
    {|mutation ($name: String!, $visibility: Visibility!, $description: String) {
      createRepository(name: $name, visibility: $visibility, description: $description) {
        id
      }
    }|}
  in
  let vars =
    [ "name", `String m.name
    ; "visibility", `String (vis_str m.vis)
    ; "description", `String (Option.value ~default:"" m.desc)
    ]
  in
  let code, data = gql client ~token q vars in
  match check code data with
  | Ok _ -> Ok ()
  | Error e
    when let s = String.lowercase_ascii e in
         String.starts_with ~prefix:"name" s || String.starts_with ~prefix:"repository" s
    -> Error "already exists"
  | Error e -> Error e
;;

(* sourcehut needs repo id for updates *)
let repo_id client ~token ~name =
  let q =
    {|query ($name: String!) {
      me {
        repository(name: $name) {
          id
        }
      }
    }|}
  in
  let vars = [ "name", `String name ] in
  let code, data = gql client ~token q vars in
  let* j = check code data in
  let open Yojson.Safe.Util in
  match j |> member "data" |> member "me" |> member "repository" |> member "id" with
  | `Int id -> Ok id
  | _ -> Error "sourcehut: could not find repository id"
;;

let update client ~token ~user:_ (m : Forge_intf.meta) =
  let* id = repo_id client ~token ~name:m.name in
  let q =
    {|mutation ($id: Int!, $input: RepoInput!) {
      updateRepository(id: $id, input: $input) {
        id
      }
    }|}
  in
  let input =
    `Assoc
      [ "name", `String m.name
      ; "description", `String (Option.value ~default:"" m.desc)
      ; "visibility", `String (vis_str m.vis)
      ]
  in
  let vars = [ "id", `Int id; "input", input ] in
  let code, data = gql client ~token q vars in
  let* _ = check code data in
  Ok ()
;;

let archive client ~token ~user:_ ~name:_ _flag =
  (* sourcehut does not support archiving via api *)
  ignore (client, token);
  Error "sourcehut does not support archive via api"
;;

let delete client ~token ~user:_ ~name =
  let* id = repo_id client ~token ~name in
  let q =
    {|mutation ($id: Int!) {
      deleteRepository(id: $id) {
        id
      }
    }|}
  in
  let vars = [ "id", `Int id ] in
  let code, data = gql client ~token q vars in
  let* _ = check code data in
  Ok ()
;;

let list client ~token ~user:_ =
  let q =
    {|query {
      me {
        repositories {
          results {
            name
          }
        }
      }
    }|}
  in
  let code, data = gql client ~token q [] in
  let* j = check code data in
  let open Yojson.Safe.Util in
  try
    let results =
      j
      |> member "data"
      |> member "me"
      |> member "repositories"
      |> member "results"
      |> to_list
    in
    let names =
      List.filter_map
        (fun r ->
           match member "name" r with
           | `String s -> Some s
           | _ -> None)
        results
    in
    Ok names
  with
  | ex -> Error (Printexc.to_string ex)
;;

let sync client ~token ~user (m : Forge_intf.meta) =
  match create client ~token ~user m with
  | Ok () -> Ok ()
  | Error "already exists" -> update client ~token ~user m
  | Error e -> Error e
;;
