---
title: Dart 语法高亮示例
description: 以 dart 围栏代码块渲染 dart.dart，验证 Dart 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# Dart 语法高亮示例

以下 `dart.dart` 示例使用 ` ```dart ` 围栏代码块渲染：

```dart
// Dart — 类型安全、空安全的现代客户端语言（Flutter 的官方语言）

// 库与导入（简要）
import 'dart:async';
import 'dart:collection';

// ============================================================
// 一、入口与变量声明
// ============================================================

void main() {
  // var：类型由编译器推断
  var city = 'Shanghai';      // String
  // final：运行时常量，只能赋值一次
  final now = DateTime.now();
  // const：编译期常量
  const pi = 3.14159;

  print('Hello, $city! pi=$pi, now=$now');

  // 基本类型
  int age = 30;
  double price = 9.95;
  String lang = 'Dart';
  bool ok = true;

  // ============================================================
  // 二、空安全（Dart 2.12+）
  // ============================================================

  String? maybe;              // 可空类型，可为 null
  String sure = 'non-null';   // 非空，必须初始化

  // ?. 安全调用，maybe 为 null 时返回 null
  print(maybe?.length);       // null

  // ! 非空断言：告诉编译器“我确定它不为 null”
  maybe = 'hi';
  print(maybe!.length);       // 2

  // ?? 空值合并：左侧为 null 时取右侧
  String display = maybe ?? 'default';

  // ============================================================
  // 三、字符串插值与运算符
  // ============================================================

  var name = 'Alice';
  print('Name: $name, length: ${name.length}'); // 插值

  print(5 ~/ 2);     // 2，~/ 整除
  print(5 % 2);      // 1，取余
  print(2 == 2);     // true
  print(1 < 2 && 3 > 2); // 逻辑与

  // ============================================================
  // 四、控制流
  // ============================================================

  if (age >= 18) {
    print('adult');
  } else {
    print('minor');
  }

  // switch 支持 String 与字面量
  switch (lang) {
    case 'Dart':
      print('dart lang');
      break;
    case 'Rust':
      print('rust lang');
      break;
    default:
      print('other');
  }

  for (var i = 0; i < 3; i++) {
    if (i == 1) continue; // 跳过本次
    print('for $i');
  }

  var count = 3;
  while (count > 0) {
    count--;
  }

  // ============================================================
  // 五、集合：List / Set / Map
  // ============================================================

  var list = [1, 2, 3];                  // List<int>
  list.add(4);
  var doubled = list.map((n) => n * 2).toList();

  var set = <String>{'a', 'b', 'a'};     // Set<String>，去重
  var map = <String, int>{              // Map<String,int>
    'one': 1,
    'two': 2,
  };
  map['three'] = 3;

  for (var entry in map.entries) {
    print('${entry.key} -> ${entry.value}');
  }
  print('doubled=$doubled, set=$set');

  // ============================================================
  // 六、函数：可选参数 / 命名参数 / 默认值
  // ============================================================

  // 位置可选参数 [ ]，带默认值
  String greet(String who, [String prefix = 'Hello']) => '$prefix, $who!';

  // 命名参数 { }，调用时用 name: value
  void configure({required String host, int port = 8080, bool verbose = false}) {
    print('host=$host port=$port verbose=$verbose');
  }

  print(greet('World'));
  configure(host: 'localhost', verbose: true);

  // ============================================================
  // 七、类：构造函数与继承
  // ============================================================

  var dog = Dog('Rex', 3);
  dog.speak();
  print(dog.describe());

  // ============================================================
  // 八、泛型
  // ============================================================

  var stack = Stack<int>();
  stack.push(1);
  stack.push(2);
  print('stack pop = ${stack.pop()}');

  // ============================================================
  // 九、异步：Future / async / await
  // ============================================================

  fetchAndPrint(); // 异步触发，不阻塞 main
}

// ============================================================
// 类定义（顶层）
// ============================================================

class Animal {
  String name;
  int age;

  // 简写构造函数：自动给字段赋值
  Animal(this.name, this.age);

  String describe() => '${name} (${age}y)';
}

class Dog extends Animal {
  // super 调用父类构造
  Dog(String name, int age) : super(name, age);

  void speak() => print('Woof! I am $name.');
}

// ============================================================
// 泛型类
// ============================================================

class Stack<T> {
  final _items = Queue<T>();

  void push(T item) => _items.addLast(item);

  T pop() {
    if (_items.isEmpty) {
      throw StateError('stack is empty');
    }
    return _items.removeLast();
  }
}

// ============================================================
// 异步函数：async 返回 Future，await 等待结果
// ============================================================

Future<String> fetchData() async {
  // 模拟网络延迟
  await Future.delayed(Duration(milliseconds: 100));
  return 'data from server';
}

Future<void> fetchAndPrint() async {
  try {
    final result = await fetchData();
    print('async result: $result');
  } catch (e) {
    print('error: $e');
  }
}
```
