open Eio.Std

type client = Cohttp_eio.Client.t

let make_client net =
  let auth =
    match Ca_certs.authenticator () with
    | Ok a -> a
    | Error (`Msg m) -> failwith ("ca-certs: " ^ m)
  in
  let tls =
    match Tls.Config.client ~authenticator:auth () with
    | Ok c -> c
    | Error (`Msg m) -> failwith ("tls config: " ^ m)
  in
  let https =
    Some
      (fun _uri conn ->
        (Tls_eio.client_of_flow tls conn
          :> [ Eio.Flow.two_way_ty | Eio.Resource.close_ty ] r))
  in
  Cohttp_eio.Client.make ~https net
;;

let body_to_string body = Eio.Buf_read.(of_flow ~max_size:(1024 * 1024) body |> take_all)

type meth =
  | GET
  | POST
  | PUT
  | PATCH
  | DELETE

let cohttp_meth = function
  | GET -> `GET
  | POST -> `POST
  | PUT -> `PUT
  | PATCH -> `PATCH
  | DELETE -> `DELETE
;;

let request client ~meth ~url ?(headers = []) ?body () =
  let uri = Uri.of_string url in
  let hdr =
    Http.Header.of_list
      (("User-Agent", "miroir") :: ("Accept", "application/json") :: headers)
  in
  let body =
    match body with
    | Some b -> Some (Cohttp_eio.Body.of_string b)
    | None -> None
  in
  Switch.run (fun sw ->
    let resp, rbody =
      Cohttp_eio.Client.call client ~sw ~headers:hdr ?body (cohttp_meth meth) uri
    in
    let status = Http.Response.status resp in
    let code = Http.Status.to_int status in
    let data = body_to_string rbody in
    code, data)
;;

let json client ~meth ~url ~token ?body () =
  let headers =
    [ "Authorization", "Bearer " ^ token; "Content-Type", "application/json" ]
  in
  request client ~meth ~url ~headers ?body ()
;;
