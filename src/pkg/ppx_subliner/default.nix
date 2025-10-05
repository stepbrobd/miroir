{ buildDunePackage
, fetchFromGitHub
, alcotest
, bisect_ppx
, cmdliner
, ppx_deriving_cmdliner
, ppx_make
, ppx_show
, ppxlib
}:

buildDunePackage (finalAttrs: {
  pname = "ppx_subliner";
  version = "0.2.1-unstable-2025-09-07";

  src = fetchFromGitHub {
    owner = "bn-d";
    repo = "ppx_subliner";
    rev = "6080aec67fa973acad4322818fc2362a64c03847";
    hash = "sha256-pmN1sduMBJa+mv2dqeVUet2HbTE5/9XvHh7SIxNwVXc=";
  };

  env.DUNE_CACHE = "disabled";

  buildInputs = [
    cmdliner
    ppx_make
    ppxlib
  ];

  doCheck = false;
  checkInputs = [
    alcotest
    bisect_ppx
    ppx_deriving_cmdliner
    ppx_show
  ];
})
