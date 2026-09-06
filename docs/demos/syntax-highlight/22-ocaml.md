---
title: OCaml 语法高亮示例
description: 以 ocaml 围栏代码块渲染 ocaml.ml，验证 OCaml 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# OCaml 语法高亮示例

以下 `ocaml.ml` 示例使用 ` ```ocaml ` 围栏代码块渲染：

```ocaml
(* OCaml: ML 系函数式语言,严格求值,强类型,函数式与命令式混合 *)

(* ============ let 绑定与函数 = *)
let x = 42
let y = "hello"

let add a b = a + b
let double x = x * 2

(* let rec:递归函数 *)
let rec factorial n =
  if n <= 1 then 1
  else n * factorial (n - 1)

let rec fib n =
  match n with
  | 0 -> 0
  | 1 -> 1
  | _ -> fib (n - 1) + fib (n - 2)

(* ============ 元组 = *)
let point = (3, 4)
let (a, b) = point
let swap (x, y) = (y, x)

(* ============ 代数数据类型(变体)= *)
type color =
  | Red
  | Green
  | Blue

type shape =
  | Circle of float
  | Rectangle of float * float
  | Triangle of float * float * float

let area = function
  | Circle r -> 3.14159265 *. r *. r
  | Rectangle (w, h) -> w *. h
  | Triangle (a, b, c) ->
    let s = (a +. b +. c) /. 2.0 in
    sqrt (s *. (s -. a) *. (s -. b) *. (s -. c))

(* ============ 记录(record)= *)
type person = {
  name : string;
  age : int;
  email : string option;
}

let make_person n a = { name = n; age = a; email = None }
let set_email p e = { p with email = Some e }

(* ============ Option 类型 = *)
let safe_div a b =
  if b = 0 then None
  else Some (a / b)

(* ============ 列表与 List 模块 = *)
let nums = [1; 2; 3; 4; 5]
let doubled = List.map (fun x -> x * 2) nums
let evens = List.filter (fun x -> x mod 2 = 0) nums
let total = List.fold_left (+) 0 nums

let rec length = function
  | [] -> 0
  | _ :: rest -> 1 + length rest

(* ============ 模块与签名 = *)
module type STACK = sig
  type 'a t
  val empty : 'a t
  val push : 'a -> 'a t -> 'a t
  val top : 'a t -> 'a option
end

module ListStack : STACK = struct
  type 'a t = 'a list
  let empty = []
  let push x s = x :: s
  let top = function
    | [] -> None
    | x :: _ -> Some x
end

(* ============ 可变引用 ref / ! / := = *)
let make_counter () =
  let c = ref 0 in
  let inc () = c := !c + 1 in
  inc (); inc (); inc ();
  !c

(* ============ 命令式控制流 for / while = *)
let sum_to n =
  let acc = ref 0 in
  for i = 1 to n do
    acc := !acc + i
  done;
  !acc

let countdown n =
  let i = ref n in
  while !i > 0 do
    Printf.printf "count = %d\n" !i;
    decr i
  done

(* ============ 高阶函数 = *)
let compose f g x = f (g x)
let inc = (+) 1
let twice f x = f (f x)

(* ============ printf 简要 = *)
let greet name = Printf.printf "你好, %s!\n" name

(* ============ 主入口 = *)
let () =
  Printf.printf "==== OCaml 语法演示 ====\n";
  Printf.printf "factorial 5 = %d\n" (factorial 5);
  Printf.printf "fib 10 = %d\n" (fib 10);
  Printf.printf "area = %f\n" (area (Circle 5.0));
  Printf.printf "counter = %d\n" (make_counter ());
  Printf.printf "sum_to 100 = %d\n" (sum_to 100);
  List.iter (Printf.printf "%d ") doubled;
  print_newline ();
  greet "Alice"
```
