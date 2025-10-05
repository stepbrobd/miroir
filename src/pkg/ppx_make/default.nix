{ buildDunePackage
, fetchFromGitHub
, bisect_ppx
, ounit2
, ppx_show
, ppxlib
, stdcompat
}:

buildDunePackage (finalAttrs: {
  pname = "ppx_make";
  version = "0.3.4";

  src = fetchFromGitHub {
    owner = "bn-d";
    repo = "ppx_make";
    tag = "v${finalAttrs.version}";
    hash = "sha256-jR+2l5JcB3wT0YsnQCTwptarp4cZwi8GFweQEwSn4oo=";
  };

  env.DUNE_CACHE = "disabled";

  buildInputs = [
    ppxlib
  ];

  doCheck = true;
  checkInputs = [
    bisect_ppx
    ounit2
    ppx_show
    stdcompat
  ];
})
