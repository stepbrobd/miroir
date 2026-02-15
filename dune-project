(lang dune 3.20)
(generate_opam_files false)
(executables_implicit_empty_intf true)

(name miroir)
(version 2026.215.1)
(source (github stepbrobd/miroir))
(license MIT)
(authors "Yifei Sun <ysun@hey.com>")
(maintainers "Yifei Sun <ysun@hey.com>")

(package
 (name miroir)
 (synopsis "repo manager wannabe?")
 (description "declaratively synchronize git repositories (including metadata) to multiple forges")
 (depends
   ocaml
   ca-certs
   cmdliner
   cohttp-eio
   dune-build-info
   eio_main
   otoml
   ppx_deriving
   ppx_deriving_toml
   ppx_subliner
   ppxlib
   tls-eio
   yojson
   (alcotest :with-test)))
