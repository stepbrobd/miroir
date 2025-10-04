(lang dune 3.20)
(generate_opam_files true)

(name miroir)
(version 2025.104.0)
(source (github stepbrobd/miroir))
(license MIT)
(authors "Yifei Sun <ysun@hey.com>")
(maintainers "Yifei Sun <ysun@hey.com>")

(package
 (name miroir)
 (synopsis "repo manager wannabe?")
 (description "declaratively synchorize git repositories (including metadata) to multiple forges")
 (depends
   ocaml
   cmdliner
   dune-build-info
   otoml
   ppx_deriving
   ppx_deriving_cmdliner
   ppx_deriving_toml
   (alcotest :with-test)))
