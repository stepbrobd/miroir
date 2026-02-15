type remote =
  { name : string
  ; uri : string
  }

type context =
  { env : (string * string) list
  ; branch : string
  ; fetch : remote list
  ; push : remote list
  }

let make_uri ~access ~domain ~user ~repo =
  match (access : Config.access), user with
  | SSH, "" -> Printf.sprintf "git@%s:%s" domain repo
  | SSH, _ -> Printf.sprintf "git@%s:%s/%s" domain user repo
  | HTTPS, "" -> Printf.sprintf "https://%s/%s.git" domain repo
  | HTTPS, _ -> Printf.sprintf "https://%s/%s/%s.git" domain user repo
;;

let make ~env ~platforms ~repo ~branch =
  let base =
    Unix.environment ()
    |> Array.to_list
    |> List.filter_map (fun s ->
      match String.split_on_char '=' s with
      | k :: rest when rest <> [] -> Some (k, String.concat "=" rest)
      | _ -> None)
  in
  let env = base @ env in
  let fetch =
    List.filter_map
      (fun (name, (p : Config.platform)) ->
         if p.origin
         then
           Some
             { name; uri = make_uri ~access:p.access ~domain:p.domain ~user:p.user ~repo }
         else None)
      platforms
  in
  (match fetch with
   | [] -> Printf.eprintf "warning: no platform has origin = true for %s\n" repo
   | [ _ ] -> ()
   | _ ->
     Printf.eprintf "fatal: multiple platforms have origin = true\n";
     exit 1);
  let push =
    List.map
      (fun (name, (p : Config.platform)) ->
         { name; uri = make_uri ~access:p.access ~domain:p.domain ~user:p.user ~repo })
      platforms
  in
  { env; branch; fetch; push }
;;

let expand_home path =
  let get_home () =
    match Sys.getenv_opt "HOME" with
    | Some h -> h
    | None ->
      Printf.eprintf "fatal: $HOME is not set\n";
      exit 1
  in
  if String.equal path "~"
  then get_home ()
  else if String.starts_with ~prefix:"~/" path
  then get_home () ^ String.sub path 1 (String.length path - 1)
  else path
;;

let make_all (cfg : Config.config) =
  let home = expand_home cfg.general.home in
  List.filter_map
    (fun (name, (repo : Config.repo)) ->
       if repo.archived
       then None
       else (
         let path = Filename.concat home name in
         let branch =
           match repo.branch with
           | Some b -> b
           | None -> cfg.general.branch
         in
         let ctx = make ~env:cfg.general.env ~platforms:cfg.platform ~repo:name ~branch in
         Some (path, ctx)))
    cfg.repo
;;
