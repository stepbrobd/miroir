type context =
  { env : (string * string) list
  ; branch : string
  ; fetch_remotes : (string * string) list
  ; push_remotes : (string * string) list
  }

let make_remote_uri ~domain ~user ~repo_name =
  match user with
  | "" -> Printf.sprintf "git@%s:%s" domain repo_name
  | _ -> Printf.sprintf "git@%s:%s/%s" domain user repo_name
;;

let make_context ~general ~platforms ~repo_name ~branch =
  let base_env =
    Unix.environment ()
    |> Array.to_list
    |> List.filter_map (fun s ->
      match String.split_on_char '=' s with
      | k :: rest when rest <> [] -> Some (k, String.concat "=" rest)
      | _ -> None)
  in
  let env = base_env @ general in
  let fetch_remotes =
    List.filter_map
      (fun (name, (p : Config.platform)) ->
         if p.origin
         then Some (name, make_remote_uri ~domain:p.domain ~user:p.user ~repo_name)
         else None)
      platforms
  in
  let push_remotes =
    List.map
      (fun (name, (p : Config.platform)) ->
         name, make_remote_uri ~domain:p.domain ~user:p.user ~repo_name)
      platforms
  in
  { env; branch; fetch_remotes; push_remotes }
;;

let expand_home path =
  if String.starts_with ~prefix:"~/" path
  then (
    let home = Sys.getenv "HOME" in
    home ^ String.sub path 1 (String.length path - 1))
  else path
;;

let make_contexts (cfg : Config.config) =
  let home = expand_home cfg.general.home in
  List.filter_map
    (fun (repo_name, (repo : Config.repo)) ->
       if repo.archived
       then None
       else (
         let path = Filename.concat home repo_name in
         let branch = "master" in
         let ctx =
           make_context
             ~general:cfg.general.env
             ~platforms:cfg.platform
             ~repo_name
             ~branch
         in
         Some (path, ctx)))
    cfg.repo
;;
