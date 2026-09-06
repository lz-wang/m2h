---
title: Scala 语法高亮示例
description: 以 scala 围栏代码块渲染 scala.scala，验证 Scala 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# Scala 语法高亮示例

以下 `scala.scala` 示例使用 ` ```scala ` 围栏代码块渲染：

```scala
// Scala 3：融合面向对象与函数式、强类型、运行于 JVM 的语言

import scala.collection.immutable.{List, Map}

// ========== 顶层函数（默认参数） ==========
def greet(name: String, greeting: String = "Hello"): String =
	s"$greeting, $name!"

// 柯里化（多参数列表）
def add(a: Int)(b: Int): Int = a + b

// ========== 样例类（不可变，自带解构/相等性/copy） ==========
case class Point(x: Int, y: Int)

// ========== 普通类（Scala 3 缩进语法） ==========
class Counter(var value: Int = 0):
	def inc(): Counter =
		value += 1
		this                    // 链式调用返回自身

// ========== 单例对象 object ==========
object Config:
	val version = "1.0"

// ========== 枚举（Scala 3） ==========
enum Color(val hex: String):
	case Red   extends Color("#f00")
	case Green extends Color("#0f0")
	case Blue  extends Color("#00f")

// ========== 泛型方法 ==========
def first[A](xs: List[A]): Option[A] = xs.headOption

@main def run(): Unit =
	// ---- val/var 与类型推断 ----
	val pi = 3.14
	var count = 0
	count += 1

	// ---- 控制流：if 表达式（有返回值） ----
	val grade = if count > 0 then "ok" else "empty"

	// ---- for 推导式（yield 生成集合） ----
	val squares = for i <- 1 to 5 yield i * i

	// ---- while ----
	var n = 3
	while n > 0 do n -= 1

	// ---- 模式匹配（含解构） ----
	val p = Point(1, 2)
	val info = p match
		case Point(0, 0) => "origin"
		case Point(x, 0) => s"on x-axis at $x"
		case Point(x, y) => s"point ($x,$y)"

	// ---- 不可变集合 List / Map 与常见操作 ----
	val nums = List(1, 2, 3, 4, 5)
	val doubled = nums.map(_ * 2)
	val evens = nums.filter(_ % 2 == 0)
	val total = nums.fold(0)(_ + _)

	val m = Map("a" -> 1, "b" -> 2)
	m.foreach { case (k, v) => println(s"$k=$v") }

	// ---- Option：显式表达"可能无值"，取代 null ----
	val found: Option[Int] = nums.find(_ == 3)
	val result = found.getOrElse(-1)

	// ---- 高阶函数：函数作参数 ----
	val applyTwice = (f: Int => Int, x: Int) => f(f(x))
	println(s"applyTwice=${applyTwice(x => x + 1, 5)}")

	// ---- 柯里化调用 ----
	val addFive = add(5)
	println(s"5+3=${addFive(3)}")

	// ---- 类与单例 ----
	val c = Counter().inc().inc()
	println(s"counter=${c.value}, version=${Config.version}")

	// ---- 枚举 ----
	println(s"color=${Color.Red.hex}")

	println(s"grade=$grade total=$total result=$result info=$info")
	println(s"squares=$squares first=${first(nums)}")

// ========== Future 简要说明（注释，避免引入 ExecutionContext 噪声） ==========
// import scala.concurrent.{Future, Await}
// import scala.concurrent.ExecutionContext.Implicits.global
// import scala.concurrent.duration._
// val f: Future[Int] = Future { 42 }
// f.map(_ * 2).foreach(println)        // 异步映射
// val v = Await.result(f, 5.seconds)   // 阻塞取值
```
