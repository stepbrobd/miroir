{ lib
, buildDunePackage
, alcotest
, ca-certs
, cmdliner
, cohttp
, cohttp-eio
, containers
, dune-build-info
, eio
, eio_main
, gitMinimal
, otoml
, ppx_deriving
, ppx_deriving_toml
, ppx_subliner
, ppx_yojson_conv
, ppx_yojson_conv_lib
, ppxlib
, tls
, tls-eio
, yojson
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
    ca-certs
    cmdliner
    cohttp
    (cohttp-eio.overrideAttrs { __darwinAllowLocalNetworking = true; })
    dune-build-info
    eio
    eio_main
    otoml
    ppx_deriving
    ppx_deriving_toml
    ppx_subliner
    ppx_yojson_conv
    ppx_yojson_conv_lib
    ppxlib
    tls
    tls-eio
    yojson
  ];

  doCheck = true;

  nativeCheckInputs = [
    gitMinimal
  ];

  checkInputs = [
    alcotest
    containers
  ];
})
