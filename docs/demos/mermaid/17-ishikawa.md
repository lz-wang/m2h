---
title: 石川图 (Ishikawa Diagram)
description: Mermaid 石川图（鱼骨图）语法参考
tags:
  - mermaid
  - ishikawa
create_date: 2026-08-29
update_date: 2026-08-29
---

石川图（鱼骨图）用于表示特定事件或问题的原因。图表类似鱼骨架，主要问题在鱼头，原因从脊椎分出。



> [!WARNING]
> 这是 Mermaid 的新图表类型，语法可能在后续版本中演进。
>

## 语法

```mermaid
ishikawa-beta
    Blurry Photo
    Process
        Out of focus
        Shutter speed too slow
        Protective film not removed
        Beautification filter applied
    User
        Shaky hands
    Equipment
        LENS
            Inappropriate lens
            Damaged lens
            Dirty lens
        SENSOR
            Damaged sensor
            Dirty sensor
    Environment
        Subject moved too quickly
        Too dark
```

````markdown
```mermaid
ishikawa-beta
    Blurry Photo
    Process
        Out of focus
        Shutter speed too slow
        Protective film not removed
        Beautification filter applied
    User
        Shaky hands
    Equipment
        LENS
            Inappropriate lens
            Damaged lens
            Dirty lens
        SENSOR
            Damaged sensor
            Dirty sensor
    Environment
        Subject moved too quickly
        Too dark
```
````

- 第一行是事件（问题）
- 后续行是事件的原因
- 缩进表示"鱼骨"层级结构

## 参考

- [Ishikawa - Mermaid](https://mermaid.js.org/syntax/ishikawa.html)
