---
title: Elixir 语法高亮示例
description: 以 elixir 围栏代码块渲染 elixir.ex，验证 Elixir 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# Elixir 语法高亮示例

以下 `elixir.ex` 示例使用 ` ```elixir ` 围栏代码块渲染：

```elixir
# Elixir 语言：基于 Erlang VM 的函数式语言，不可变数据与 Actor 并发模型。
# 文件扩展名：.ex 经编译（elixirc）；.exs 为脚本（elixir file.exs 直接运行）。

# ===== 模块与函数：defmodule / def（公开）/ defp（私有） =====
defmodule Examples.Math do
  def square(n), do: n * n

  # 多子句函数 + 守卫（when）：按顺序匹配并附加条件
  def classify(n) when n < 0, do: :negative
  def classify(0), do: :zero
  def classify(n) when n > 0, do: :positive

  defp double(n), do: n * 2          # 私有，仅模块内可见
  def use_double(n), do: double(n)   # 公开函数调用私有函数
end

# ===== 模式匹配：= 是匹配运算符，^ 钉住已有变量值 =====
defmodule Examples.Match do
  def first({a, _b, _c}), do: a                  # 解构元组
  def only_ok({_a, :ok, _c}), do: :matched_ok    # 仅匹配第二元素为 :ok
  def head([h | _t]), do: h                       # 列表头尾解构
  def name_of(%{name: name}), do: name            # 映射只匹配存在的键
end

# ===== 协议（defprotocol / defimpl）：基于类型的运行期多态派发 =====
defprotocol Examples.Json do
  @doc "将值序列化为 JSON 字符串（示意）"
  def to_json(value)
end

defimpl Examples.Json, for: Map do
  def to_json(map), do: "{...map with #{map_size(map)} keys...}"
end

defimpl Examples.Json, for: List do
  def to_json(list), do: "[#{Enum.join(list, ",")}]"
end

# ===== 宏（quote / unquote）：编译期代码生成（简要） =====
defmodule Examples.Macros do
  # 自定义 my_unless 宏：在编译期改写为 if 取反
  defmacro my_unless(condition, do: block) do
    quote do
      if !unquote(condition), do: unquote(block)
    end
  end

  # use 时自动 import 本模块，便于调用宏
  defmacro __using__(_opts) do
    quote do
      import Examples.Macros
    end
  end
end

# ===== 入口模块 =====
defmodule Examples.Main do
  use Examples.Macros

  def run do
    # 不可变数据：变量本身不可变，重新绑定即指向新值
    list = [1, 2, 3]                       # 列表（单链表）
    tuple = {:ok, 42}                      # 元组
    kw = [name: "Alice", age: 30]          # 关键字列表（原子键的二元组列表）
    map = %{"lang" => "Elixir", 1 => :one} # 映射，键可为任意值
    atom_map = %{status: :active}          # 原子键映射，可用 . 访问

    IO.puts("list=#{inspect(list)}")
    IO.puts("tuple=#{inspect(tuple)}")
    IO.puts("kw=#{inspect(kw)}")
    IO.puts("map=#{inspect(map)}")
    IO.puts("atom_map.status=#{atom_map.status}")

    # 函数调用
    IO.puts("square(5)=#{Examples.Math.square(5)}")
    IO.puts("classify(-3)=#{inspect(Examples.Math.classify(-3))}")

    # 模式匹配函数
    IO.puts("first=#{Examples.Match.first({1, 2, 3})}")
    IO.puts("head=#{Examples.Match.head([10, 20, 30])}")
    IO.puts("name=#{Examples.Match.name_of(%{name: "Bob", x: 1})}")

    # pin 运算符 ^：用已有变量的值进行匹配
    expected = :ok
    case {:ok, "data"} do
      {^expected, data} -> IO.puts("matched: #{data}")
      _ -> IO.puts("no match")
    end

    # 管道运算符 |>：把左侧结果作为右侧函数的第一个参数
    result =
      [1, 2, 3, 4, 5]
      |> Enum.map(fn x -> x * 2 end)
      |> Enum.filter(fn x -> x > 4 end)
      |> Enum.reduce(0, fn x, acc -> acc + x end)
    IO.puts("pipeline=#{inspect(result)}")

    # 匿名函数（fn -> end）与 & 捕获语法
    add = fn a, b -> a + b end
    IO.puts("add(2,3)=#{add.(2, 3)}")          # 匿名函数用 . 调用
    inc = &(&1 + 1)                            # 等价于 fn x -> x + 1 end
    IO.puts("inc(9)=#{inc.(9)}")
    squares = Enum.map([1, 2, 3], &(&1 * &1))   # 捕获用作枚举参数
    IO.puts("squares=#{inspect(squares)}")

    # 协议多态派发
    IO.puts("map json=#{Examples.Json.to_json(%{a: 1})}")
    IO.puts("list json=#{Examples.Json.to_json([1, 2, 3])}")

    # 宏生成的 my_unless
    my_unless 1 > 5 do
      IO.puts("my_unless: 1 is not greater than 5")
    end

    # 字符串（双引号 UTF-8 二进制）与字符列表（单引号，整数列表）
    bin = "你好 Elixir"
    chars = 'abc'
    IO.puts("string=#{bin} byte_size=#{byte_size(bin)}")
    IO.puts("charlist=#{inspect(chars)} to_string=#{to_string(chars)}")

    # Agent / GenServer：Elixir 的轻量并发状态容器与通用服务器（OTP）。
    # 下面以注释示意，实际中通常封装为独立模块。
    #
    # Agent（保存/读取简单状态）：
    #   {:ok, pid} = Agent.start_link(fn -> 0 end)
    #   Agent.update(pid, fn state -> state + 1 end)
    #   Agent.get(pid, fn state -> state end)   #=> 1
    #
    # GenServer（承载状态机与业务逻辑）：
    #   use GenServer
    #   def init(state), do: {:ok, state}
    #   def handle_call(:get, _from, state), do: {:reply, state, state}
    #   def handle_cast({:set, v}, _state), do: {:noreply, v}

    IO.puts("done")
  end
end

Examples.Main.run()
```
