---
title: Groovy 语法高亮示例
description: 以 groovy 围栏代码块渲染 groovy.groovy，验证 Groovy 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# Groovy 语法高亮示例

以下 `groovy.groovy` 示例使用 ` ```groovy ` 围栏代码块渲染：

```groovy
/* Groovy 语言：运行于 JVM 的动态语言，语法简洁、与 Java 完全互操作。 */

// @Grab：通过 Grape 在脚本运行时自动拉取依赖（此处仅声明演示，未实际使用）
// @Grab('org.apache.commons:commons-lang3:3.14.0')

// ===== 变量：def 动态类型 vs 强类型 =====
def dynamic = 'I am a String'   // 动态类型，运行期决定
String typed = "I am typed"      // 强类型，编译期检查
int count = 10
def inferred = 3.14              // 类型由字面量推导为 BigDecimal

// ===== 字符串：单引号(纯)/双引号(GString 插值)/三引号(多行) =====
def name = 'Groovy'
def greeting = "Hello, ${name}!"          // GString 插值 ${}
def multi = """\
多行字符串第一行
第二行：${name} 版本
"""
println greeting
print multi

// ===== 列表与映射（字面量语法） =====
def list = [1, 2, 3, 4, 5]               // ArrayList
def map = [name: 'Alice', age: 30]       // LinkedHashMap
println "list=$list map=$map"

// ===== 范围（Range）：1..5 含右端，1..<5 不含右端 =====
def inclusive = 1..5
def exclusive = 1..<5
println "inclusive=$inclusive size=${inclusive.size()}"
println "exclusive=${exclusive.toList()}"

// ===== 闭包：可赋值、可作参数传递 =====
def square = { x -> x * x }
def usingIt = { it * 2 }                // 单参闭包默认用 it
println "square(4)=${square(4)} it(4)=${usingIt(4)}"

// ===== 集合方法：each / collect / find / findAll =====
def nums = [1, 2, 3, 4, 5, 6]
print "each: "
nums.each { n -> print "$n " }; println()
def doubled = nums.collect { it * 2 }            // map 映射
def firstEven = nums.find { it % 2 == 0 }         // 第一个匹配
def evens = nums.findAll { it % 2 == 0 }          // 全部匹配
println "doubled=$doubled firstEven=$firstEven evens=$evens"

// ===== Groovy 真值（truthy）：非空非零即为真 =====
assert !''          // 空字符串为假
assert 'text'       // 非空字符串为真
assert !0            // 0 为假
assert 1             // 非零为真
assert ![]           // 空集合为假
assert [1]           // 非空集合为真
assert !null         // null 为假

// ===== 控制流 =====
def score = 85
if (score >= 90) {
    println "A"
} else if (score >= 80) {
    println "B"
} else {
    println "C"
}

// switch 支持多种类型分支（范围、正则、类型等）
def level = { s ->
    switch (s) {
        case 90..100: return 'A'
        case 80..<90: return 'B'
        case 70..<80: return 'C'
        case ~/\d+/:  return 'numeric'   // 正则匹配
        default:      return 'unknown'
    }
}
println "level(85)=${level(85)}"

// for 遍历范围
for (i in 1..3) print "$i "; println()

// while 循环
def n = 0
while (n < 3) { n++ }
println "n=$n"

// ===== 类与属性（属性默认生成 getter/setter） =====
class Person {
    String name
    int age

    Person(String name, int age) {
        this.name = name
        this.age = age
    }

    String describe() {
        "Person(name=${name}, age=${age})"
    }
}

def p = new Person('Bob', 25)
println p.describe()
p.age = 26                       // 调用 setter
println "age now=${p.age}"       // 调用 getter

// ===== 字符串处理 =====
def text = "Hello World Groovy"
println text.toUpperCase()               // 调用 Java String 方法
println text.tokenize(' ')               // 按空格分词
println text.replaceAll('o', '0')        // 正则替换
println "length=${text.length()}"

// ===== Java 互操作：直接使用 Java 标准库 =====
def sb = new StringBuilder()
sb.append('Java').append(' ').append('interop')
println sb.toString()
def now = new java.util.Date()
println "now=$now"

// ===== 动态类型：参数无类型，按运行时类型分派 =====
def dynamicAdd = { a, b -> a + b }
println dynamicAdd(1, 2)          // 整数相加
println dynamicAdd('a', 'b')      // 字符串拼接

// ===== 闭包默认参数与方法默认值 =====
def greet = { String who = 'World', String prefix = 'Hi' ->
    "$prefix, $who!"
}
println greet()
println greet('Alice')
println greet('Alice', 'Hey')

println 'done'
```
