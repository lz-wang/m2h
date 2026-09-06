---
title: Objective-C 语法高亮示例
description: objc.m 使用 objective-c 围栏标签，即 Chroma 的 Objective-C 词法器别名。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# Objective-C 语法高亮示例

objc.m 使用 objective-c 围栏标签，即 Chroma 的 Objective-C 词法器别名。

```objective-c
/* Objective-C 语言：C 的面向对象超集，通过消息传递与运行时实现动态特性。 */
#import <Foundation/Foundation.h>

// ===== 协议（@protocol）：类似 Java 接口，声明需要实现的方法 =====
@protocol Drawable <NSObject>
- (void)draw;            // @required（默认）必须实现
@optional
- (NSString *)title;     // @optional 可选实现
@end

// ===== 类声明：@interface，继承 NSObject 并采纳协议 =====
@interface Shape : NSObject <Drawable>
// @property 自动生成 getter/setter；ARC 下 strong 为强引用
@property (nonatomic, strong) NSString *name;
// assign：基本类型（NSInteger/CGFloat 等）用赋值语义，不涉及引用计数
@property (nonatomic, assign) NSInteger sides;
- (instancetype)initWithName:(NSString *)name sides:(NSInteger)sides;
- (CGFloat)area; // 基类默认实现，子类可覆盖
// 类方法（+）：通过类名调用，类似静态方法
+ (Shape *)factoryWithName:(NSString *)name;
@end

// ===== 类实现：@implementation =====
@implementation Shape
// 现代 Objective-C 默认合成属性（_name / _sides），无需手写 @synthesize
// @synthesize name = _name; // 显式合成（即默认行为）
- (instancetype)initWithName:(NSString *)name sides:(NSInteger)sides {
    self = [super init]; // 先调用父类初始化
    if (self) {
        // NSString 惯例用 copy 防止可变字符串被外部修改（即使属性声明为 strong）
        _name = [name copy];
        _sides = sides;
    }
    return self;
}
- (void)draw {
    NSLog(@"Drawing shape: %@", self.name); // 消息传递 [receiver selector]
}
- (CGFloat)area {
    return 0.0;
}
- (NSString *)title {
    return self.name; // 实现可选协议方法
}
+ (Shape *)factoryWithName:(NSString *)name {
    return [[Shape alloc] initWithName:name sides:1];
}
@end

// ===== 子类：继承并覆盖方法 =====
@interface Rectangle : Shape
@property (nonatomic, assign) CGFloat width;
@property (nonatomic, assign) CGFloat height;
- (instancetype)initWithName:(NSString *)name width:(CGFloat)w height:(CGFloat)h;
@end

@implementation Rectangle
- (instancetype)initWithName:(NSString *)name width:(CGFloat)w height:(CGFloat)h {
    self = [super initWithName:name sides:4]; // 调用父类指定初始化器
    if (self) {
        _width = w;
        _height = h;
    }
    return self;
}
// 覆盖父类方法（多态）
- (CGFloat)area {
    return self.width * self.height;
}
- (void)draw {
    NSLog(@"Rectangle %@: %.2f x %.2f (area=%.2f)", self.name, self.width, self.height, [self area]);
}
@end

// ===== 分类（category）：为已有类追加方法，无需源码、不可加实例变量 =====
@interface Shape (Helpers)
- (BOOL)isPolygon;
@end

@implementation Shape (Helpers)
- (BOOL)isPolygon {
    return self.sides >= 3;
}
@end

// ===== 块（block）：用 ^ 声明的闭包，可捕获变量 =====
typedef CGFloat (^MapperBlock)(CGFloat);

// ===== main 函数：程序入口 =====
int main(int argc, const char *argv[]) {
    @autoreleasepool { // ARC 下的自动释放池
        Shape *s = [[Shape alloc] initWithName:@"Generic" sides:5];
        [s draw]; // 实例方法调用 [obj msg]
        NSLog(@"isPolygon=%d sides=%ld", [s isPolygon], (long)s.sides);

        Rectangle *r = [[Rectangle alloc] initWithName:@"Box" width:3.0 height:4.0];
        [r draw];
        NSLog(@"area=%.2f title=%@", [r area], [r title]);

        // weak 引用：不增加引用计数；对象释放后自动置 nil（ARC）
        __weak Shape *weakRef = r;
        NSLog(@"weak name: %@", weakRef.name); // r 仍存活，故可访问

        // NSString 拼接：stringWithFormat:
        NSString *greeting = [NSString stringWithFormat:@"Hello, %@!", r.name];
        NSLog(@"%@", greeting);

        // NSArray：不可变数组（字面量 @[ ... ]）
        NSArray *shapes = @[s, r];
        NSLog(@"count=%lu", (unsigned long)shapes.count);

        // NSDictionary：键值映射（字面量 @{ key: value }）
        NSDictionary *info = @{
            @"name": r.name,
            @"area": @([r area]), // @(expr) NSNumber 字面量
        };
        NSLog(@"info=%@", info);

        // 快速枚举遍历
        for (Shape *item in shapes) {
            [item draw];
        }

        // 块的声明与调用：将值映射为平方
        MapperBlock square = ^(CGFloat x) {
            return x * x;
        };
        NSLog(@"square(5)=%.2f", square(5.0));

        // 协议类型变量 id<Drawable>：可指向任何实现该协议的对象
        id<Drawable> drawable = r;
        [drawable draw];

        // 类方法（+）调用
        Shape *made = [Shape factoryWithName:@"Circle"];
        NSLog(@"factory made: %@", made.name);
    }
    return 0;
}
```
