---
title: Ruby 语法高亮示例
description: 以 ruby 围栏代码块渲染 ruby.rb，验证 Ruby 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# Ruby 语法高亮示例

以下 `ruby.rb` 示例使用 ` ```ruby ` 围栏代码块渲染：

```ruby
# Ruby — 面向开发者快乐设计的动态、面向对象语言，一切皆对象。

# ============================================================
# 注释：单行用 #，多行用 =begin/=end
# ============================================================
=begin
这是多行注释。
Ruby 中多行注释必须 =begin/=end 顶格书写。
=end

# ============================================================
# 变量：局部 / 实例 @ / 类 @@ / 全局 $
# ============================================================
local_var = "I am local"        # 局部变量
@instance_var = "instance"      # 实例变量（@）
@@class_var = 0                 # 类变量（@@）
$global_var = "global"          # 全局变量（$）

# 符号（symbol）—— 轻量不可变标识符，常用作哈希键
status = :active

# 字符串：单引号（原样）与双引号（支持插值/转义）
single = 'no interpolation \n literally'
double = "#{local_var}, status=#{status}"        # 双引号插值

# ============================================================
# 运算符
# ============================================================
sum = 5 + 3
power = 2 ** 8              # 幂
divmod = 17.divmod(5)       # -> [3, 2]（商与余数）
range = (1..5).to_a         # 范围 -> [1, 2, 3, 4, 5]
repeat = "ab" * 3           # 字符串重复 -> "ababab"
combined = sum <=> 10        # 太空船运算符 -> -1

# ============================================================
# 控制流：if / unless / case / while / until / 迭代器
# ============================================================
if sum > 7
  puts "big"
elsif sum == 7
  puts "equal"
else
  puts "small"
end

# unless（除非）—— if 的否定形式
unless status == :inactive
  puts "still active"
end

# case/when（when 自动用 === 比较）
case status
when :active   then puts "running"
when :inactive then puts "stopped"
else                puts "unknown"
end

# while 与 until
count = 0
while count < 3
  count += 1
end
until count == 0
  count -= 1
end

# 迭代器（Ruby 的精髓）
(1..3).each { |n| puts n }                          # 块 { |x| ... }
%w[apple banana].each_with_index do |fruit, i|      # 块 do ... end
  puts "#{i}: #{fruit}"
end

# ============================================================
# 方法（def）：关键字参数 / 默认值 / 多返回值
# ============================================================
def greet(name, prefix: "Hi")    # 关键字参数 + 默认值
  "#{prefix}, #{name}!"          # 最后一个表达式即返回值
end
puts greet("Ruby", prefix: "Hello")

def minmax(arr)                  # 返回多值（实际是数组）
  [arr.min, arr.max]
end
lo, hi = minmax([3, 1, 4, 1, 5]) # 多重赋值解构

# ============================================================
# 块 / proc / lambda
# ============================================================
# 块（Block）—— 每个方法都可隐式接收一个块
result = [1, 2, 3].map do |x|
  x * 10
end

# proc —— 可存为变量、不严格检查参数个数
doubler = Proc.new { |x| x * 2 }
puts doubler.call(5)             # 也可用 doubler.(5) / doubler[5]

# lambda —— 严格检查参数个数、return 只退出自身
adder = ->(x, y) { x + y }       # -> 箭头 lambda 语法
puts adder.call(2, 3)

# ============================================================
# 类与模块、混入（include）
# ============================================================
module Walkable                  # 模块：方法集合，不可实例化
  def walk
    "#{self.class} is walking"
  end
end

class Animal
  attr_accessor :name            # 自动生成 getter/setter
  attr_reader :sound             # 只读属性

  def initialize(name)           # 构造方法
    @name = name
    @sound = "..."
  end

  def speak
    "#{@name} makes a sound"
  end
end

class Dog < Animal               # 继承（<）
  include Walkable               # 混入模块

  def speak                      # 方法重写
    "#{@name}: Woof!"
  end
end

rex = Dog.new("Rex")
puts rex.speak                   # -> "Rex: Woof!"
puts rex.walk                    # -> "Dog is walking"
puts rex.respond_to?(:name)      # 反射

# ============================================================
# 异常：begin / rescue / ensure / raise
# ============================================================
begin
  raise "boom!" if count.even?   # raise 抛出 RuntimeError
rescue ZeroDivisionError => e
  puts "specific: #{e.message}"
rescue => e                      # 捕获任意 StandardError
  puts "caught: #{e.message}"
ensure
  puts "always runs"             # ensure 等价于 finally
end

# ============================================================
# 哈希与数组
# ============================================================
colors = ["red", "green", "blue"]               # 数组
person = { name: "Alice", age: 30 }             # 哈希（symbol 键简写）
person[:city] = "NYC"                           # 新增键
person.each { |k, v| puts "#{k}=#{v}" }
puts colors.first(2).join(", ")                 # -> "red, green"
puts colors.map(&:upcase)                       # symbol to proc
```
