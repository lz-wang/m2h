---
title: Zig 语法高亮示例
description: 以 zig 围栏代码块渲染 zig.zig，验证 Zig 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# Zig 语法高亮示例

以下 `zig.zig` 示例使用 ` ```zig ` 围栏代码块渲染：

```zig
// Zig 语言：无隐藏控制流、无隐藏内存分配的系统编程语言，以编译期求值与显式错误处理著称。
const std = @import("std");

// ===== 常量与变量：const 不可变，var 可变；类型可显式标注或由字面量推导 =====
const PI: f64 = 3.14159;
const VERSION = "0.13.0"; // 推导为 *const [6:0]u8
const I32_MAX: i32 = std.math.maxInt(i32);
const U32_VAL: u32 = 42;
const BOOL_VAL: bool = true;

// 编译期求值：comptime 强制在编译期计算
const COMPTIME_SQUARE: i32 = comptime blk: {
    const v: i32 = 4;
    break :blk v * v;
};

// ===== 错误集（error set）：可命名的错误值集合 =====
const ParseError = error{
    Empty,
    Invalid,
    Overflow,
};

// ===== 函数：fn 名(参数) 返回类型 =====
fn add(a: i32, b: i32) i32 {
    return a + b;
}

// 可选类型 ?T：可能为 null
fn lookup(key: []const u8) ?i32 {
    if (std.mem.eql(u8, key, "answer")) return 42;
    return null;
}

// 错误联合返回类型 !T（推断错误集）：用 try 传播、catch 捕获
fn parse_int(s: []const u8) !i32 {
    if (s.len == 0) return error.Empty;
    return std.fmt.parseInt(i32, s, 10); // parseInt 返回 !i32，自动传播
}

// ===== 结构体 =====
const Point = struct {
    x: f64,
    y: f64,

    // 方法：首参为 self 的 const 指针
    fn distance(self: *const Point) f64 {
        return @sqrt(self.x * self.x + self.y * self.y);
    }
};

// ===== 枚举 =====
const Color = enum {
    red,
    green,
    blue,

    fn hex(self: Color) u32 {
        return switch (self) { // switch 是表达式
            .red => 0xFF0000,
            .green => 0x00FF00,
            .blue => 0x0000FF,
        };
    }
};

// ===== 标记联合（tagged union）：可作为代数数据类型 =====
const Shape = union(enum) {
    circle: f64, // 半径
    square: f64, // 边长
    rect: struct { w: f64, h: f64 },

    fn area(self: Shape) f64 {
        return switch (self) {
            // 模式捕获载荷值
            .circle => |r| PI * r * r,
            .square => |s| s * s,
            .rect => |r| r.w * r.h,
        };
    }
};

// ===== 泛型：编译期类型参数 comptime T: type =====
fn max_value(comptime T: type, a: T, b: T) T {
    return if (a > b) a else b;
}

// 泛型容器：函数返回 type（编译期生成新类型）
fn Stack(comptime T: type) type {
    return struct {
        items: []T,
        len: usize,

        const Self = @This();

        fn init(buf: []T) Self {
            return .{ .items = buf, .len = 0 };
        }
        fn push(self: *Self, v: T) void {
            self.items[self.len] = v;
            self.len += 1;
        }
        fn pop(self: *Self) ?T {
            if (self.len == 0) return null;
            self.len -= 1;
            return self.items[self.len];
        }
    };
}

// ===== 分配器（Allocator）：堆分配需显式传入分配器，语言无全局 new =====
fn build_list(allocator: std.mem.Allocator) !std.ArrayList(i32) {
    var list = std.ArrayList(i32).init(allocator);
    // errdefer：仅在函数返回错误时执行（用于回滚已申请资源）
    errdefer list.deinit();
    var i: i32 = 0;
    while (i < 5) : (i += 1) {
        try list.append(i * 10); // try 失败会触发上面的 errdefer
    }
    return list; // 成功则所有权转移给调用者
}

pub fn main() !void {
    std.debug.print("zig demo v{s}\n", .{VERSION});
    std.debug.print("types: i32max={d} u32={d} bool={}\n", .{ I32_MAX, U32_VAL, BOOL_VAL });

    // 变量绑定与函数调用
    const sum = add(3, 4);
    std.debug.print("sum={d}\n", .{sum});

    // if 是表达式，可直接产出值
    const n: i32 = 7;
    const parity = if (n % 2 == 0) "even" else "odd";
    std.debug.print("{d} is {s}\n", .{ n, parity });

    // while（含 continue 表达式 i += 1）
    var i: usize = 0;
    while (i < 3) : (i += 1) std.debug.print("i={d} ", .{i});
    std.debug.print("\n", .{});

    // for 遍历数组
    const arr = [_]i32{ 1, 2, 3, 4 };
    for (arr) |v| std.debug.print("{d} ", .{v});
    std.debug.print("\n", .{});

    // 切片：数组的视图 full[1..4]
    const full = [_]i32{ 10, 20, 30, 40, 50 };
    const slice: []const i32 = full[1..4];
    for (slice) |v| std.debug.print("{d} ", .{v});
    std.debug.print("\n", .{});

    // switch 多分支
    const c = Color.green;
    std.debug.print("green hex={x}\n", .{c.hex()});

    // 可选类型解包：if (opt) |val|
    if (lookup("answer")) |ans| {
        std.debug.print("answer={d}\n", .{ans});
    } else {
        std.debug.print("not found\n", .{});
    }

    // 错误处理：catch 捕获并提前返回
    const parsed = parse_int("123") catch |err| {
        std.debug.print("parse failed: {}\n", .{err});
        return;
    };
    std.debug.print("parsed={d}\n", .{parsed});

    // 结构体与方法
    const p = Point{ .x = 3, .y = 4 };
    std.debug.print("distance={d}\n", .{p.distance()});

    // 联合：不同载荷形态
    const s1 = Shape{ .circle = 2.0 };
    const s2 = Shape{ .rect = .{ .w = 3, .h = 4 } };
    std.debug.print("areas: {d} {d}\n", .{ s1.area(), s2.area() });

    // 指针：*T 指针，ptr.* 解引用
    var x: i32 = 10;
    const ptr: *i32 = &x;
    ptr.* += 5;
    std.debug.print("x={d}\n", .{x});

    // 泛型与 comptime：不同类型实参在编译期生成不同函数
    std.debug.print("max(i32)={d}\n", .{max_value(i32, 10, 20)});
    std.debug.print("max(f64)={d}\n", .{max_value(f64, 1.5, 2.5)});
    std.debug.print("comptime square={d}\n", .{COMPTIME_SQUARE});

    // 泛型 Stack（使用栈上缓冲区，零堆分配）
    var buf: [8]i32 = undefined;
    var stack = Stack(i32).init(&buf);
    stack.push(11);
    stack.push(22);
    std.debug.print("pop={d}\n", .{stack.pop().?});

    // 分配器与 defer（函数退出必定执行，无论成功失败）
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit(); // defer：成功/失败都执行
    const allocator = gpa.allocator();

    var list = try build_list(allocator);
    defer list.deinit(); // 成功路径上释放
    for (list.items) |v| std.debug.print("{d} ", .{v});
    std.debug.print("\n", .{});

    _ = ParseError; // 引用错误集以供查阅
}
```
