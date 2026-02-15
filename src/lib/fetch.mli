(** HTTP client for forge API requests *)

(** opaque HTTP client handle *)
type client

(** create a TLS-enabled HTTP client using system CA certificates *)
val make_client : _ Eio.Net.t -> client

(** HTTP methods *)
type meth =
  | GET
  | POST
  | PUT
  | PATCH
  | DELETE

(** make an HTTP request, returns (status_code, body) *)
val request
  :  client
  -> meth:meth
  -> url:string
  -> ?headers:(string * string) list
  -> ?body:string
  -> unit
  -> int * string

(** make an authenticated JSON request with Bearer token *)
val json
  :  client
  -> meth:meth
  -> url:string
  -> token:string
  -> ?body:string
  -> unit
  -> int * string
