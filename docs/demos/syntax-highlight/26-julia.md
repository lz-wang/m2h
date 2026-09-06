---
title: Julia 语法高亮示例
description: 以 julia 围栏代码块渲染 julia.jl，验证 Julia 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# Julia 语法高亮示例

以下 `julia.jl` 示例使用 ` ```julia ` 围栏代码块渲染：

```julia
# Julia — 高性能科学计算语言，Lisp 风格语法 + 静态类型推断 + JIT 编译。

# ============================================================
# 注释、变量、常量、基本类型
# ============================================================
# 单行注释
#=
多行注释：#= ... =#，可嵌套
=#

x = 42                  # Int（机器字长，通常 Int64）
f = 3.14                # Float64
s = "Julia"             # String
flag = true             # Bool（小写 true/false）
nothing_val = nothing   # Nothing（单例）

const PI_APPROX = 3.14159      # const 声明常量（全局推荐）

# 类型标注通常用在函数签名与结构体字段（见后文）
function typed_add(a::Int, b::Int)::Int
    return a + b
end

# ============================================================
# 字符串插值
# ============================================================
greeting = "Hello, $s!"                 # $ 变量插值
expr = "2 + 3 = $(2 + 3)"               # $(...) 表达式插值
multi = """
triple-quoted string
spans lines, $s interpolation OK
"""

# ============================================================
# 元组、命名元组、多重返回值
# ============================================================
t = (1, "two", 3.0)                     # 元组（不可变）
a, b, c = t                             # 解构
nt = (name = "Alice", age = 30)         # 命名元组
nt.name                                 # 用 . 访问字段

function minmax(v)
    return minimum(v), maximum(v)       # 多重返回（元组）
end
lo, hi = minmax([3, 1, 4, 1, 5, 9])

# ============================================================
# 控制流：if / elseif / for / while
# ============================================================
n = 7
if n > 10
    println("big")
elseif n == 10
    println("equal")
else
    println("small")
end

for i in 1:5                            # 范围 1:5（闭区间）
    i % 2 == 0 && continue              # 短路：偶数跳过
    i == 5 && break
    println(i)
end

k = 0
while k < 3
    global k += 1                       # 顶层循环中修改全局需 global
end

# ============================================================
# 数组与矩阵：1-based 索引（注意与 Python/C 不同）
# ============================================================
arr = [10, 20, 30, 40, 50]              # Vector{Int}（一维）
mat = [1 2 3; 4 5 6]                    # 2×3 矩阵（空格分列，分号分行）
arr[1]                                  # 第一个元素（1-based！）
arr[end]                                # end = 末尾索引
arr[2:4]                                # 切片 [20, 30, 40]
mat[1, 2]                               # 矩阵取元素 [行, 列]
mat[:, 3]                               # 取整列
size(mat)                               # 维度 -> (2, 3)

zeros(2, 3)                             # 全零矩阵
rand(3, 3)                              # 随机矩阵
reshape(1:6, 2, 3)                      # 重塑形状

# ============================================================
# 类型系统：struct / mutable struct / 参数类型
# ============================================================
struct Point                           # 不可变结构体（字段不可改）
    x::Float64
    y::Float64
end

mutable struct Counter                 # 可变结构体（字段可改）
    value::Int
end
ctr = Counter(0)
ctr.value += 1                         # 合法：mutable

# 参数类型（泛型）
struct Pair{T, S}
    first::T
    second::S
end
p = Pair{Int, String}(1, "one")

# ============================================================
# 多重派发：根据【所有参数】类型选择方法（Julia 的核心）
# ============================================================
collide(a, b) = "unknown objects collide"

collide(a::Int, b::Int) = "two ints: $a + $b = $(a + b)"
collide(a::String, b::String) = "two strings: $a & $b"
collide(a::Point, b::Point) = "points: dx=$(a.x-b.x), dy=$(a.y-b.y)"

println(collide(1, 2))
println(collide("x", "y"))
println(collide(Point(1.0, 2.0), Point(3.0, 4.0)))

# ============================================================
# 广播：在函数/运算符后加 . 逐元素作用
# ============================================================
nums = [1.0, 2.0, 3.0, 4.0]
squared = nums .^ 2                    # 广播 .^
roots = sqrt.(nums)                    # 函数广播 sqrt.
sumsq = sum(nums .^ 2)                 # 广播 + 归约

# ============================================================
# Lambda 与高阶函数
# ============================================================
square = x -> x * x                    # 单参数箭头函数
add = (a, b) -> a + b                  # 多参数
map(x -> x + 1, 1:3)                  # -> [2, 3, 4]
filter(x -> x > 2, [1, 2, 3, 4])      # -> [3, 4]
reduce(+, 1:5)                         # -> 15

# ============================================================
# 宏：以 @ 调用，接收代码本身并变换
# ============================================================
# @time 测量运行时间与内存分配
# @time sum(1:1_000_000)
# secs = @elapsed sleep(0.1)           # 只返回耗时（秒）

macro sayhi(name)
    return :(println("Hi, ", $name))   # :( ) 构造表达式，$ 插值参数
end
# @sayhi "Julia"

# ============================================================
# 模块：module / using / import
# ============================================================
# module MyTools
#     export greet
#     greet(name) = println("Hello, $name")
#     _secret() = "not exported"
# end
#
# using MyTools            # 导入所有 export 的名字到当前作用域
# import MyTools: _secret  # 只导入指定名字
# greet("Alice")

# ============================================================
# 异常：try / catch / finally
# ============================================================
try
    error("boom!")                     # 主动抛出 ErrorException
catch e
    println("caught: ", e.msg)
finally
    println("cleanup")
end
```
