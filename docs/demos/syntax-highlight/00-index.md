---
title: 语法高亮演示索引
description: 43 个编程语言与配置文件示例，验证 GFM 围栏代码块的 Chroma 语法高亮效果
tags:
  - 语法高亮
create_date: 2026-09-06
update_date: 2026-09-06
---

# 语法高亮演示索引

m2h 的围栏代码块由 goldmark-highlighting 调用 Chroma 渲染，GitHub 风格配色以 CSS class 输出，亮暗主题共用一套 token 类。本目录每个示例对应 `~/playground/programing-languages` 下一个真实样例文件，围栏标签即 Chroma 语言别名；亮暗两套主题下逐个打开可核对高亮效果。

启动预览：

```bash
m2h web docs/demos
```

## 编程语言（32）

| 文件 | 源示例 | fence 标签 |
| --- | --- | --- |
| [Python](01-python.md) | `python.py` | `python` |
| [Go](02-go.md) | `go.go` | `go` |
| [JavaScript](03-javascript.md) | `javascript.js` | `javascript` |
| [TypeScript](04-typescript.md) | `typescript.ts` | `typescript` |
| [Java](05-java.md) | `java.java` | `java` |
| [C](06-c.md) | `c.c` | `c` |
| [C++](07-cpp.md) | `cpp.cpp` | `cpp` |
| [Rust](08-rust.md) | `rust.rs` | `rust` |
| [Ruby](09-ruby.md) | `ruby.rb` | `ruby` |
| [PHP](10-php.md) | `php.php` | `php` |
| [C#](11-csharp.md) | `csharp.cs` | `csharp` |
| [Bash](12-bash.md) | `bash.sh` | `bash` |
| [Kotlin](13-kotlin.md) | `kotlin.kt` | `kotlin` |
| [Swift](14-swift.md) | `swift.swift` | `swift` |
| [Lua](15-lua.md) | `lua.lua` | `lua` |
| [Scala](16-scala.md) | `scala.scala` | `scala` |
| [Dart](17-dart.md) | `dart.dart` | `dart` |
| [SQL](18-sql.md) | `sql.sql` | `sql` |
| [HTML](19-html.md) | `html.html` | `html` |
| [CSS](20-css.md) | `css.css` | `css` |
| [Haskell](21-haskell.md) | `haskell.hs` | `haskell` |
| [OCaml](22-ocaml.md) | `ocaml.ml` | `ocaml` |
| [F#](23-fsharp.md) | `fsharp.fs` | `fsharp` |
| [Clojure](24-clojure.md) | `clojure.clj` | `clojure` |
| [R](25-r.md) | `r.R` | `r` |
| [Julia](26-julia.md) | `julia.jl` | `julia` |
| [Perl](27-perl.md) | `perl.pl` | `perl` |
| [PowerShell](28-powershell.md) | `powershell.ps1` | `powershell` |
| [Zig](29-zig.md) | `zig.zig` | `zig` |
| [Objective-C](30-objective-c.md) | `objc.m` | `objective-c` |
| [Groovy](31-groovy.md) | `groovy.groovy` | `groovy` |
| [Elixir](32-elixir.md) | `elixir.ex` | `elixir` |

## 标记语言（1）

| 文件 | 源示例 | fence 标签 |
| --- | --- | --- |
| [Markdown](33-markdown.md) | `markdown.md` | `markdown` |

`markdown.md` 自身含代码围栏，示例中外层围栏使用 5 个反引号包裹。

## 配置文件（10）

| 文件 | 源示例 | fence 标签 |
| --- | --- | --- |
| [JSON](34-json.md) | `data.json` | `json` |
| [YAML](35-yaml.md) | `config.yaml` | `yaml` |
| [TOML](36-toml.md) | `config.toml` | `toml` |
| [XML](37-xml.md) | `data.xml` | `xml` |
| [INI](38-ini.md) | `app.ini` | `ini` |
| [环境变量 (.env)](39-env.md) | `.env.example` | `bash` |
| [Makefile](40-makefile.md) | `Makefile` | `makefile` |
| [Dockerfile](41-dockerfile.md) | `Dockerfile` | `dockerfile` |
| [Git 配置 (.gitconfig)](42-gitconfig.md) | `.gitconfig` | `ini` |
| [Nginx](43-nginx.md) | `nginx.conf` | `nginx` |

特殊映射：`objc.m` 使用 `objective-c` 标签；`.env.example` 使用 `bash` 标签；`.gitconfig` 使用 `ini` 标签（Chroma 无独立 gitconfig 词法器）。

