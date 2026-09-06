---
title: F# 语法高亮示例
description: 以 fsharp 围栏代码块渲染 fsharp.fs，验证 F# 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# F# 语法高亮示例

以下 `fsharp.fs` 示例使用 ` ```fsharp ` 围栏代码块渲染：

```fsharp
// F#: .NET 平台上的 ML 系函数式语言,类型推断强大,函数式与面向对象兼备

namespace SyntaxDemo

open System

// ============ 注释 ============
// 单行注释用 //
(* 多行注释用 (* ... *),可嵌套 *)

module Demo =

    // ============ let 与 let rec ============
    let x = 42
    let add a b = a + b

    let rec factorial n =
        if n <= 1 then 1
        else n * factorial (n - 1)

    // ============ 类型推断 ============
    let inferInt = 10
    let inferFloat = 3.14
    let inferList = [1; 2; 3]
    let inferFn x = x + 1

    // ============ 元组 ============
    let point = (3, 4)
    let (a, b) = point
    let swap (x, y) = (y, x)

    // ============ 可区分联合 ============
    type Color =
        | Red
        | Green
        | Blue

    type Shape =
        | Circle of radius: float
        | Rectangle of width: float * height: float
        | Triangle of float * float * float

    // ============ 模式匹配 ============
    let describeColor c =
        match c with
        | Red -> "红"
        | Green -> "绿"
        | Blue -> "蓝"

    let area shape =
        match shape with
        | Circle r -> Math.PI * r * r
        | Rectangle (w, h) -> w * h
        | Triangle (a, b, c) ->
            let s = (a + b + c) / 2.0
            sqrt (s * (s - a) * (s - b) * (s - c))

    // 带守卫(when)的匹配
    let classify n =
        match n with
        | _ when n < 0 -> "负数"
        | 0 -> "零"
        | _ when n < 100 -> "小数"
        | _ -> "大数"

    // ============ 记录 ============
    type Person = {
        Name : string
        Age : int
        Email : string option
    }

    let alice = { Name = "Alice"; Age = 30; Email = None }
    let withAge p age = { p with Age = age }

    // ============ 列表 / 数组 ============
    let nums = [1; 2; 3; 4; 5]
    let arr = [| 10; 20; 30 |]
    let doubled = List.map (fun x -> x * 2) nums
    let arrSum = Array.sum arr

    // ============ 管道 |> 与组合 >> ============
    let processNums xs =
        xs
        |> List.filter (fun x -> x > 0)
        |> List.map (fun x -> x * x)
        |> List.sum

    let squareThenInc = (fun x -> x * x) >> (fun x -> x + 1)

    // ============ Option 与 Result ============
    let safeDiv a b =
        if b = 0.0 then None
        else Some (a / b)

    let parseScore (s: string) : Result<int, string> =
        match Int32.TryParse s with
        | true, n when n >= 0 && n <= 100 -> Ok n
        | _ -> Error (sprintf "无效分数: %s" s)

    // ============ async 异步工作流(简要)============
    let squareAsync n = async {
        return n * n
    }

    let sumAsync ns = async {
        let! results =
            ns
            |> List.map squareAsync
            |> Async.Parallel
        return Array.sum results
    }

    // ============ 高阶函数 ============
    let compose f g x = f (g x)
    let flip f x y = f y x

// ============ 入口 ============
module Main =

    [<EntryPoint>]
    let main argv =
        printfn "==== F# 语法演示 ===="
        printfn "factorial 5 = %d" (Demo.factorial 5)
        printfn "area circle = %f" (Demo.area (Demo.Circle 5.0))
        printfn "classify -3 = %s" (Demo.classify -3)
        printfn "processNums = %d" (Demo.processNums [-3; -1; 2; 4; 5])
        printfn "alice = %A" Demo.alice

        match Demo.parseScore "88" with
        | Ok n -> printfn "score = %d" n
        | Error e -> printfn "%s" e

        0
```
