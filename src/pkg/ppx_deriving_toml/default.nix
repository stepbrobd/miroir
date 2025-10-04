{ buildDunePackage
, fetchFromGitHub
, containers
, otoml
, ppxlib
}:

buildDunePackage (finalAttrs: {
  pname = "ppx_deriving_toml";
  version = "0.4";

  src = fetchFromGitHub {
    owner = "andreypopp";
    repo = "ppx_deriving";
    tag = finalAttrs.version;
    hash = "sha256-RhrdKabDg4koXV0bRw+NWXOTzeU8bKRj1+5b3po1J8c=";
  };

  env.DUNE_CACHE = "disabled";

  patchPhase = ''
    runHook prePatch
    echo '(data_only_dirs json)' > dune
    runHook postPatch
  '';

  buildInputs = [
    containers
    otoml
    ppxlib
  ];

  doCheck = true;
})
