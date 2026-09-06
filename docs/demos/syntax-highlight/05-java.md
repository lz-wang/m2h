---
title: Java 语法高亮示例
description: 以 java 围栏代码块渲染 java.java，验证 Java 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# Java 语法高亮示例

以下 `java.java` 示例使用 ` ```java ` 围栏代码块渲染：

```java
// Java：面向对象、静态类型、"一次编写到处运行"的通用语言
package com.example.demo;

import java.io.IOException;
import java.io.StringReader;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

// ========== 自定义注解（标记用法） ==========
@FunctionalInterface   // 标记为函数式接口（仅一个抽象方法）
interface MathOp {
	int apply(int a, int b);
}

// ========== 自定义异常 ==========
class InvalidScoreException extends Exception {
	public InvalidScoreException(String msg) {
		super(msg);
	}
}

// ========== 抽象类：体现继承与多态 ==========
abstract class Person {
	private final String name;   // 封装：字段私有，仅通过 getter 暴露
	protected int age;

	public Person(String name, int age) {   // 构造器
		this.name = name;
		this.age = age;
	}

	public String getName() {              // getter
		return name;
	}

	public abstract String role();         // 抽象方法，子类必须实现

	@Override
	public String toString() {             // 重写 Object.toString
		return name + "(" + age + ")";
	}
}

// ========== 接口：可含默认方法 ==========
interface Greeter {
	String greeting();                     // 抽象方法
	default String loudGreeting() {        // 默认方法
		return greeting().toUpperCase();
	}
}

// ========== 继承 Person 并实现 Greeter ==========
class Student extends Person implements Greeter {
	public Student(String name, int age) {
		super(name, age);
	}

	@Override
	public String role() {                 // 多态：子类自己的角色
		return "student";
	}

	@Override
	public String greeting() {
		return "hi, I'm " + getName();
	}

	// 方法重载：同名、不同参数列表
	public int score() {
		return 0;
	}

	public int score(int extra) {
		return score() + extra;
	}
}

// ========== 枚举（可带构造器与字段） ==========
enum Color {
	RED("#f00"), GREEN("#0f0"), BLUE("#00f");

	private final String hex;

	Color(String hex) {
		this.hex = hex;
	}

	public String hex() {
		return hex;
	}
}

// ========== 主类（非 public，避免与文件名 java.java 冲突） ==========
class Demo {
	public static void main(String[] args) {
		// ---- 变量、基本类型与包装类 ----
		int a = 10;
		double pi = 3.14;
		boolean ok = true;
		Integer boxed = a;                 // 自动装箱
		String s = "Java";

		// ---- 运算符与三元 ----
		int sum = a + 5;
		boolean positive = ok && (a > 0);
		String tier0 = ok ? "yes" : "no";

		// ---- 控制流：if / else ----
		if (a > 5) {
			System.out.println("big");
		} else {
			System.out.println("small");
		}

		// ---- switch 语句（箭头形式，Java 14+） ----
		switch (a) {
			case 1 -> System.out.println("one");
			default -> System.out.println("other");
		}

		// ---- switch 表达式 ----
		String tier = switch (a) {
			case 0 -> "zero";
			case 1, 2 -> "low";
			default -> "high";
		};
		System.out.println("tier=" + tier);

		// ---- for / while ----
		for (int i = 0; i < 3; i++) {
			System.out.println("i=" + i);
		}
		int n = 3;
		while (n-- > 0) {
			System.out.println("n=" + n);
		}

		// ---- 泛型集合 List / Map ----
		List<String> names = new ArrayList<>(List.of("Alice", "Bob"));
		names.add("Carol");
		Map<String, Integer> ages = new HashMap<>();
		ages.put("Alice", 20);
		ages.put("Bob", 22);

		// ---- Lambda 与 Stream API ----
		List<Integer> lengths = names.stream()
				.filter(x -> x.length() > 3)
				.map(String::length)
				.sorted()
				.collect(Collectors.toList());
		System.out.println("lengths=" + lengths);

		// ---- 静态方法调用（可变参数） ----
		System.out.println("sum=" + add(1, 2, 3));

		// ---- 异常：try / catch / finally ----
		try {
			checkScore(120);
		} catch (InvalidScoreException e) {
			System.out.println("invalid: " + e.getMessage());
		} finally {
			System.out.println("checked");
		}

		// ---- try-with-resources：自动关闭实现 AutoCloseable 的资源 ----
		try (var reader = new StringReader("hello")) {
			int ch = reader.read();
			System.out.println("first char=" + (char) ch);
		} catch (IOException e) {
			e.printStackTrace();
		}

		// ---- 枚举、多态、接口默认方法 ----
		System.out.println("color=" + Color.RED.hex());
		Student stu = new Student("Alice", 20);
		System.out.println(stu.greeting() + " / " + stu.loudGreeting());

		// ---- 函数式接口与 Lambda ----
		MathOp adder = (x, y) -> x + y;
		System.out.println("2+3=" + adder.apply(2, 3));
	}

	// 静态方法：可变参数
	static int add(int... nums) {
		int t = 0;
		for (int x : nums) {
			t += x;
		}
		return t;
	}

	// 抛出自定义异常
	static void checkScore(int score) throws InvalidScoreException {
		if (score < 0 || score > 100) {
			throw new InvalidScoreException("score out of range: " + score);
		}
	}
}
```
