---
title: Python 语法高亮示例
description: 以 python 围栏代码块渲染 python.py，验证 Python 的语法高亮效果。
tags:
  - 语法高亮
  - 编程语言
create_date: 2026-09-06
update_date: 2026-09-06
---

# Python 语法高亮示例

以下 `python.py` 示例使用 ` ```python ` 围栏代码块渲染：

```python
# Python — 简洁、动态类型的解释型语言，强调可读性与生态。

# ============================================================
# 变量与类型注解、数字、字符串、字节串
# ============================================================
import asyncio
import json
import os
from dataclasses import dataclass, field
from functools import lru_cache
from typing import Optional

name: str = "Python"            # 字符串
version: float = 3.12           # 浮点数
count: int = 42                 # 整数
flag: bool = True               # 布尔
nothing: None = None            # None
raw: bytes = b"\x00\xff"        # 字节串

# ============================================================
# 运算符
# ============================================================
floor = 7 // 2      # 整除 -> 3
power = 2 ** 10     # 幂运算 -> 1024
rem = 10 % 3        # 取模 -> 1
text = f"{name} {version}, count={count}"   # f-string 插值
words = text.split()                        # 字符串方法

# ============================================================
# 控制流：if / for / while / match-case
# ============================================================
if count > 50:
    print("large")
elif count > 10:
    print("medium")
else:
    print("small")

for i in range(3):
    if i == 1:
        continue
    print(i)

while count > 39:
    count -= 1
    if count == 39:
        break

# match-case（Python 3.10+ 结构化模式匹配）
command = "quit"
match command:
    case "quit":
        print("bye")
    case "help":
        print("help info")
    case _:
        print("unknown")

# ============================================================
# 函数：默认参数 / *args / **kwargs / 关键字参数 / lambda
# ============================================================
def greet(name: str, prefix: str = "Hello") -> str:
    return f"{prefix}, {name}!"                  # 关键字参数 + 默认值

def sum_all(*args: int, debug: bool = False) -> int:
    if debug:                                    # *args 收集位置参数
        print("args =", args)
    return sum(args)                             # **kwargs 省略，见下

def with_kwargs(**kwargs: str) -> None:          # **kwargs 收集关键字参数
    for k, v in kwargs.items():
        print(k, v)

square = lambda x: x * x                         # lambda 表达式
print(greet("World"), greet("Py", prefix="Hi"))
print(sum_all(1, 2, 3, debug=True))
with_kwargs(lang="Python", type="dynamic")
print(square(5))

# ============================================================
# 推导式：列表 / 字典 / 集合
# ============================================================
squares = [x * x for x in range(5)]               # 列表推导式
index_map = {k: v for k, v in enumerate(squares)} # 字典推导式
unique = {c for c in "banana"}                    # 集合推导式

# ============================================================
# 类：继承 / 属性 / 数据类
# ============================================================
class Animal:
    def __init__(self, name: str) -> None:
        self._name = name                        # 受保护属性（约定）

    @property
    def name(self) -> str:                       # 只读属性
        return self._name

    def speak(self) -> str:
        return "..."

class Dog(Animal):                               # 继承
    def speak(self) -> str:                      # 方法重写
        return f"{self.name}: Woof!"

@dataclass                                       # 数据类：自动生成 __init__/__repr__
class Point:
    x: float
    y: float = 0.0
    tags: list[str] = field(default_factory=list)

dog = Dog("Rex")
print(dog.speak(), Point(1.0, 2.0, tags=["a"]))

# ============================================================
# 异常：try / except / finally / raise
# ============================================================
def safe_div(a: int, b: int) -> Optional[float]:
    try:
        return a / b
    except ZeroDivisionError:
        raise ValueError("b cannot be zero")     # 重新抛出更具体的异常
    finally:
        print("division attempt finished")

print(safe_div(10, 2))

# ============================================================
# 装饰器
# ============================================================
def trace(func):
    def wrapper(*args, **kwargs):
        print(f"calling {func.__name__}")
        return func(*args, **kwargs)
    return wrapper

@trace
def echo(msg: str) -> str:
    return msg

@lru_cache(maxsize=128)                          # 标准库装饰器：缓存
def fib(n: int) -> int:
    return n if n < 2 else fib(n - 1) + fib(n - 2)

print(echo("hi"), fib(10))

# ============================================================
# 生成器（yield）
# ============================================================
def counter(limit: int):
    i = 0
    while i < limit:
        yield i                                  # 惰性产出
        i += 1

print(list(counter(3)))

# ============================================================
# 模块导入与标准库（os / json 简要）
# ============================================================
print(os.path.basename("/tmp/file.txt"))         # os 模块
payload = json.dumps({"a": 1, "b": [2, 3]})      # json 序列化
parsed = json.loads(payload)                      # json 反序列化

# ============================================================
# 并发：asyncio 简要示例
# ============================================================
async def fetch(task_id: int) -> str:
    await asyncio.sleep(0.01)                    # 模拟 IO 等待
    return f"task-{task_id} done"

async def main() -> None:
    results = await asyncio.gather(fetch(1), fetch(2))  # 并发执行
    print(results)

asyncio.run(main())
```
