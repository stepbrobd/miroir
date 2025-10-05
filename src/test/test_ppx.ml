open Alcotest
open Otoml
open Ppx_deriving_toml_runtime

type empty =
  { k : int
  ; v : int
  }
[@@deriving toml, toml_assoc_table]

let test_ppx_empty () =
  let toml = "k = 6\nv = 9" in
  let parsed = empty_of_toml (Parser.from_string toml) in
  check int "empty k" 6 parsed.k;
  check int "empty v" 9 parsed.v
;;

type simple = { map : (string * int) list [@toml.assoc_table] }
[@@deriving toml, toml_assoc_table]

let test_ppx_nothing () =
  let toml = "" in
  let parsed = simple_of_toml (Parser.from_string toml) in
  check int "simple length" 0 (List.length parsed.map)
;;

let test_ppx_simple () =
  let toml = "[map]\nk = 6\nv = 9" in
  let parsed = simple_of_toml (Parser.from_string toml) in
  check int "simple length" 2 (List.length parsed.map);
  check int "simple k" 6 (List.assoc "k" parsed.map);
  check int "simple v" 9 (List.assoc "v" parsed.map)
;;

type complex =
  { a : int
  ; b : (string * empty) list [@toml.assoc_table]
  ; c : (string * simple) list [@toml.assoc_table]
  ; d : int
  }
[@@deriving toml, toml_assoc_table]

let test_ppx_complex () =
  let toml =
    {|
a = 1
d = 2

[b.first]
k = 6
v = 9

[b.second]
k = 4
v = 2

[c.foo.map]
x = 1
y = 2

[c.bar.map]
z = 3
|}
  in
  let parsed = complex_of_toml (Parser.from_string toml) in
  check int "complex a" 1 parsed.a;
  check int "complex d" 2 parsed.d;
  check int "complex b length" 2 (List.length parsed.b);
  let first = List.assoc "first" parsed.b in
  check int "complex b.first.k" 6 first.k;
  check int "complex b.first.v" 9 first.v;
  check int "complex c length" 2 (List.length parsed.c);
  let foo = List.assoc "foo" parsed.c in
  check int "complex c.foo.map length" 2 (List.length foo.map);
  check int "complex c.foo.map.x" 1 (List.assoc "x" foo.map)
;;

let () =
  run
    "ppx"
    [ ( "all"
      , [ test_case "empty" `Quick test_ppx_empty
        ; test_case "nothing" `Quick test_ppx_nothing
        ; test_case "simple" `Quick test_ppx_simple
        ; test_case "complex" `Quick test_ppx_complex
        ] )
    ]
;;
