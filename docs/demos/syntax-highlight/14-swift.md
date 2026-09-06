---
title: Swift 语法高亮示例
description: 以 swift 围栏代码块渲染 swift.swift，验证 Swift 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# Swift 语法高亮示例

以下 `swift.swift` 示例使用 ` ```swift ` 围栏代码块渲染：

```swift
// Swift：Apple 平台的现代、类型安全语言，融合函数式与面向对象特性

import Foundation

// ========== 顶层函数：参数标签、默认值、可变参数 ==========
func greet(_ name: String, with greeting: String = "Hello") -> String {
	"\(greeting), \(name)!"
}

func sum(_ nums: Int...) -> Int {
	nums.reduce(0, +)
}

// ========== 协议 ==========
protocol Describable {
	var description: String { get }
}

// ========== 结构体（值类型） ==========
struct Point: Describable {
	var x: Double
	var y: Double
	var description: String { "(\(x), \(y))" }
	var magnitude: Double { (x * x + y * y).squareRoot() }   // 计算属性
}

// ========== 类（引用类型） ==========
class Vehicle {
	let wheels: Int
	var speed: Double = 0
	init(wheels: Int) { self.wheels = wheels }
	func accelerate(by amount: Double) { speed += amount }
}

final class Car: Vehicle {
	var brand: String
	init(brand: String) {
		self.brand = brand
		super.init(wheels: 4)
	}
}

// ========== 枚举（含关联值） ==========
enum NetworkResult {
	case success(Data)
	case failure(String)
}

// ========== 错误类型 ==========
enum AppError: Error {
	case invalidInput(String)
	case outOfRange
}

func validate(_ x: Int) throws -> Int {
	guard x >= 0 else { throw AppError.invalidInput("negative") }
	if x > 100 { throw AppError.outOfRange }
	return x
}

// ========== 泛型函数 ==========
func first<T>(_ items: [T]) -> T? { items.first }

let point = Point(x: 3, y: 4)
let car = Car(brand: "Tesla")
car.accelerate(by: 60)

// ---- var/let、类型推断、基本类型 ----
let pi = 3.14159
var count = 0
count += 1
let name: String = "Swift"

// ---- 可选 Optional ----
let optional: Int? = 42
let unwrapped: Int = optional ?? 0          // 空合（?? ）
let forced: Int = optional!                  // 强解包（!，已知非空时）
let chain: Int? = optional.map { $0 * 2 }    // 可选 map（链式操作）

// ---- 字符串插值 \( ) ----
print("pi=\(pi), count=\(count), msg=\(greet("World"))")

// ---- 控制流：if let（安全解包） ----
if let value = optional {
	print("got \(value)")
}

// guard let（提前退出，解包后在作用域内可用）
func process(_ data: String?) {
	guard let data else { return }
	print("processing \(data)")
}
process("payload")

// ---- switch 含范围匹配 ----
let score = 85
let grade: String
switch score {
case 90...100: grade = "A"
case 70..<90:  grade = "B"
case 60..<70:  grade = "C"
default:       grade = "F"
}

// ---- for-in ----
for i in 1...5 { print(i) }
for (index, value) in [10, 20, 30].enumerated() {
	print("\(index): \(value)")
}

// ---- while ----
var n = 3
while n > 0 { n -= 1 }

// ---- 集合：Array / Dictionary / Set ----
var fruits: [String] = ["apple", "banana"]
fruits.append("cherry")
let fruitCounts: [String: Int] = ["apple": 2, "banana": 3]
let tags: Set<String> = ["swift", "ios"]

// ---- 闭包与尾随闭包 ----
let nums = [1, 2, 3, 4, 5]
let doubled = nums.map { $0 * 2 }
let evens = nums.filter { $0 % 2 == 0 }
let total = nums.reduce(0) { $0 + $1 }

// ---- 错误处理：do / try / catch ----
do {
	let v = try validate(120)
	print("valid: \(v)")
} catch AppError.outOfRange {
	print("out of range")
} catch let AppError.invalidInput(msg) {
	print("invalid: \(msg)")
} catch {
	print("other error: \(error)")
}

// try? 将错误转为可选
let safe: Int? = try? validate(50)

// ---- 枚举关联值的 switch ----
let res: NetworkResult = .success(Data([0x48, 0x49]))
switch res {
case .success(let data): print("got \(data.count) bytes")
case .failure(let msg):  print("failed: \(msg)")
}

// ---- 协议使用（面向协议编程） ----
let items: [Describable] = [Point(x: 1, y: 2), Point(x: 5, y: 6)]
for it in items { print(it.description) }

print("grade=\(grade) total=\(total) first=\(first(nums) ?? -1) safe=\(safe ?? -1)")
print("point=\(point.description) mag=\(point.magnitude) car=\(car.brand)@\(car.speed)")
print("fruits=\(fruits) counts=\(fruitCounts) tags=\(tags)")

// MARK: - 说明
// protocol 可通过 extension 提供默认实现，是 Swift 复用与多态的核心机制。
```
