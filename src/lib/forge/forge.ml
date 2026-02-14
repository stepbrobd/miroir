include Forge_intf

let dispatch (forge : Miroir.Config.forge) : (module S) =
  match forge with
  | Github -> (module Github)
  | Gitlab -> (module Gitlab)
  | Codeberg -> (module Codeberg)
  | Sourcehut -> (module Sourcehut)
;;
