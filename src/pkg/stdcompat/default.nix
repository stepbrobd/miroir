{ buildDunePackage
, fetchFromGitHub
}:

buildDunePackage (finalAttrs: {
  pname = "stdcompat";
  version = "21.1";

  src = fetchFromGitHub {
    owner = "ocamllibs";
    repo = "stdcompat";
    tag = finalAttrs.version;
    hash = "sha256-ptqky7DMc8ggaFr1U8bikQ2eNp5uGcvXNqInHigzY5U=";
  };

  env.DUNE_CACHE = "disabled";

  dontConfigure = true;
})
