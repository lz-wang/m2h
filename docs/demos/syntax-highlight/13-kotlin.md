---
title: Kotlin 语法高亮示例
description: 以 kotlin 围栏代码块渲染 kotlin.kt，验证 Kotlin 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# Kotlin 语法高亮示例

以下 `kotlin.kt` 示例使用 ` ```kotlin ` 围栏代码块渲染：

```kotlin
// Kotlin：现代、空安全、与 Java 完全互操作的简洁静态类型语言

// ========== 顶层函数（默认参数 + 命名参数） ==========
fun greet(name: String, greeting: String = "Hello"): String =
	"$greeting, $name!"

// 可变参数 + 表达式函数体
fun sum(vararg nums: Int): Int = nums.sum()

// ========== 数据类（自动生成 equals/hashCode/toString/copy） ==========
data class Point(val x: Int, val y: Int)

// ========== 开放类（默认 final，需 open 才能被继承） ==========
open class Animal(val name: String) {
	open fun sound(): String = "..."   // 可被重写的方法
}

// 接口：可带默认实现
interface Speaker {
	fun speak(): String
	fun defaultSpeak() = speak()       // 接口默认方法
}

// 继承父类并实现接口
class Dog(name: String) : Animal(name), Speaker {
	override fun sound(): String = "Woof"
	override fun speak(): String = "$name: ${sound()}"
}

// ========== 扩展函数：为已有类型添加方法 ==========
fun String.shout(): String = this.uppercase() + "!!!"

// ========== 枚举 ==========
enum class Direction { NORTH, SOUTH, EAST, WEST }

// ========== 单例对象（object） ==========
object Config {
	const val version = "1.0"
	fun info() = "v$version"
}

fun main() {
	// ---- val 不可变 / var 可变 / 类型推断 ----
	val pi = 3.14
	var count = 0
	count += 1

	// ---- 基本类型 ----
	val i: Int = 10
	val d: Double = 2.5
	val flag: Boolean = true

	// ---- 空安全 ----
	val nullable: String? = null
	val len: Int? = nullable?.length        // 安全调用
	val safe: Int = nullable?.length ?: 0   // 空合（Elvis）
	// val crash = nullable!!                // 非空断言（为空则抛 NPE）

	// ---- 字符串模板 $ ----
	println("pi=$pi, len=$safe, msg=${greet("Kotlin")}")

	// ---- 运算符 ----
	val combined = i + d
	val inRange = i in 1..10

	// ---- 控制流：if 表达式（有返回值） ----
	val grade = if (i >= 5) "big" else "small"

	// ---- when（比 switch 更强，也是表达式） ----
	val parity = when (i % 2) {
		0 -> "even"
		1 -> "odd"
		else -> "?"
	}

	// ---- 范围 for ----
	for (x in 1..5) print("$x ")
	println()
	for (x in 10 downTo 1 step 3) print("$x ")
	println()

	// ---- while ----
	var n = 3
	while (n > 0) n--

	// ---- 集合与函数式操作 ----
	val nums = listOf(1, 2, 3, 4, 5)
	val doubled = nums.map { it * 2 }
	val evens = nums.filter { it % 2 == 0 }
	val total = nums.reduce { acc, v -> acc + v }

	val map = mapOf("a" to 1, "b" to 2)
	map.forEach { (k, v) -> println("$k=$v") }

	// ---- 高阶函数：函数作参数 ----
	val sq: (Int) -> Int = { x -> x * x }
	println("applyTwice=${applyTwice(sq, 3)}")   // 81

	// ---- 命名参数 ----
	println(greet(name = "Bob", greeting = "Hi"))

	// ---- 类、继承、接口 ----
	val dog = Dog("Rex")
	println("${dog.name} says ${dog.sound()} / ${dog.speak()}")

	// ---- 扩展函数 ----
	println("hello".shout())

	// ---- 数据类 copy ----
	val p = Point(1, 2)
	val p2 = p.copy(x = 5)
	println("$p -> $p2")

	println("grade=$grade parity=$parity total=$total combined=$combined ${Config.info()}")
}

// 高阶函数：接收函数参数
fun applyTwice(f: (Int) -> Int, x: Int): Int = f(f(x))

// ========== 协程简要说明（注释，不引入运行依赖） ==========
// import kotlinx.coroutines.*
// fun main() = runBlocking {
//     launch { doWork() }              // 启动不返回结果的协程（发射即忘）
//     val a = async { computeA() }      // 并发计算并返回结果
//     val b = async { computeB() }
//     println(a.await() + b.await())    // 等待并汇总结果
// }
// 协程是轻量级线程：launch 用于"发射即忘"，async 用于并发取值。
```
