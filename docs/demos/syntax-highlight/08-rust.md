---
title: Rust 语法高亮示例
description: 以 rust 围栏代码块渲染 rust.rs，验证 Rust 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# Rust 语法高亮示例

以下 `rust.rs` 示例使用 ` ```rust ` 围栏代码块渲染：

```rust
// Rust 语言：内存安全、零成本抽象的系统编程语言，无 GC 借助所有权保证安全。
use std::collections::HashMap;
use std::thread;

// ===== 函数：最后一行无分号即返回值 =====
fn add(a: i32, b: i32) -> i32 {
    a + b
}

// ===== 结构体与方法 =====
struct Rectangle {
    width: f64,
    height: f64,
}

impl Rectangle {
    fn area(&self) -> f64 {
        self.width * self.height
    }
    fn scale(&mut self, f: f64) {
        self.width *= f;
        self.height *= f;
    }
}

// ===== 枚举：可携带数据的代数类型 =====
enum Message {
    Quit,
    Move { x: i32, y: i32 },
    Write(String),
}

// ===== trait：行为抽象（类似接口） =====
trait Greet {
    fn say(&self) -> String;
}

struct User {
    name: String,
}

impl Greet for User {
    fn say(&self) -> String {
        format!("hi {}", self.name)
    }
}

// ===== 泛型 + trait bound =====
fn largest<T: PartialOrd>(list: &[T]) -> &T {
    let mut big = &list[0];
    for item in &list[1..] {
        if item > big {
            big = item;
        }
    }
    big
}

// ===== 错误处理：Result 与 ? 运算符 =====
fn parse_num(s: &str) -> Result<i32, std::num::ParseIntError> {
    let n: i32 = s.parse()?;
    Ok(n * 2)
}

fn main() {
    // 变量默认不可变，mut 显式声明可变
    let mut x = 5;
    x += 1;
    const PI: f64 = 3.14159;
    println!("x={} PI={}", x, PI);

    // 所有权与借用：clone 避免移动
    let s1 = String::from("hello");
    let s2 = s1.clone();
    let r = &s2; // 不可变引用
    println!("{} {} {}", s1, s2, r);

    // String 与 &str
    let text: &str = "world";
    let owned: String = text.to_string();
    println!("{}", owned);

    // 控制流：if 表达式
    let n = 7;
    let parity = if n % 2 == 0 { "even" } else { "odd" };
    println!("{}", parity);

    // match 模式匹配
    let msg = Message::Write(String::from("hi"));
    match msg {
        Message::Quit => println!("quit"),
        Message::Move { x, y } => println!("move {x},{y}"),
        Message::Write(t) => println!("write {}", t),
    }

    // loop / while / for
    let mut i = 0;
    loop {
        i += 1;
        if i >= 3 {
            break;
        }
    }
    while i > 0 {
        i -= 1;
    }
    for v in 1..=5 {
        print!("{} ", v);
    }
    println!();

    // Vec 与 HashMap
    let mut vec = Vec::new();
    vec.push(1);
    vec.push(2);
    let mut map = HashMap::new();
    map.insert("a", 1);
    map.insert("b", 2);
    println!("vec {:?} map {:?}", vec, map);

    // 闭包
    let double = |n| n * 2;
    println!("double(21)={}", double(21));

    // Option 与 if let
    let opt: Option<i32> = Some(5);
    if let Some(v) = opt {
        println!("v={}", v);
    }

    // 错误处理
    match parse_num("21") {
        Ok(v) => println!("parsed {}", v),
        Err(e) => println!("err {}", e),
    }

    // trait 调用
    let u = User {
        name: String::from("Alice"),
    };
    println!("{}", u.say());

    // 泛型
    let nums = vec![3, 1, 4, 1, 5, 9];
    println!("largest = {}", largest(&nums));

    // 线程（消息传递风格：join 取回结果）
    let handle = thread::spawn(|| 42);
    println!("thread: {}", handle.join().unwrap());
}
```
