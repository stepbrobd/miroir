{ lib
, buildDunePackage
, alcotest
, cmdliner
, containers
, dune-build-info
, otoml
, ppx_deriving
, ppx_deriving_cmdliner
, ppx_deriving_toml
, ppxlib
}:

buildDunePackage (finalAttrs: {
  pname = "miroir";
  meta.mainProgram = finalAttrs.pname;
  version = with lib; pipe ../../../dune-project [
    readFile
    (match ".*\\(version ([^\n]+)\\).*")
    head
  ];

  src = with lib.fileset; toSource {
    root = ../../../.;
    fileset = unions [
      ../../../src
      ../../../dune-project
      ../../../miroir.opam
    ];
  };

  env.DUNE_CACHE = "disabled";

  buildInputs = [
    cmdliner
    dune-build-info
    otoml
    ppx_deriving
    ppx_deriving_cmdliner
    ppx_deriving_toml
    ppxlib
  ];

  doCheck = true;
  checkInputs = [
    alcotest
    containers
  ];
})
